package coinbase

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/exchange/baseconnector"
	"github.com/antonygiomarxdev/greedy/internal/ratelimiter"
	"github.com/antonygiomarxdev/greedy/internal/shared"
)

var _ shared.Exchange = (*Connector)(nil)

type Connector struct {
	baseconnector.BaseConnector
	cfg  Config
	key  string
	sec  string
	pass string
}

func New(cfg Config) *Connector {
	if cfg.RESTBaseURL == "" {
		cfg.RESTBaseURL = SandboxRESTURL
	}
	return &Connector{
		BaseConnector: baseconnector.BaseConnector{
			Client: &http.Client{Timeout: 30 * time.Second},
			RL:     ratelimiter.NewTokenBucket(coinbaseBurst, time.Second),
		},
		cfg:  cfg,
		key:  cfg.APIKey,
		sec:  cfg.APISecret,
		pass: cfg.Passphrase,
	}
}

func (c *Connector) Name() string { return string(shared.ProviderCoinbase) }

func (c *Connector) Ping(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodGet, pathPing, nil)
	return err
}

func (c *Connector) GetOrderBook(ctx context.Context, symbol string, depth int) (*shared.OrderBook, error) {
	path := fmt.Sprintf("%s?product_id=%s", pathProductBook, symbol)
	if depth > 0 {
		path += fmt.Sprintf("&limit=%d", depth)
	}
	data, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var br bookResponse
	if err := json.Unmarshal(data, &br); err != nil {
		return nil, fmt.Errorf("parse order book: %w", err)
	}
	return convertBook(&br), nil
}

func (c *Connector) GetTicker(ctx context.Context, symbol string) (*shared.Ticker, error) {
	if !validSymbol(symbol) {
		return nil, fmt.Errorf("%w: %q", shared.ErrSymbolNotFound, symbol)
	}
	path := fmt.Sprintf(pathTickerFmt, symbol)
	data, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var tr tickerResponse
	if err := json.Unmarshal(data, &tr); err != nil {
		return nil, fmt.Errorf("parse ticker: %w", err)
	}
	price, err := strconv.ParseFloat(tr.Price, 64)
	if err != nil {
		return nil, fmt.Errorf("parse ticker price %q: %w", tr.Price, err)
	}
	return &shared.Ticker{
		Symbol: symbol,
		Price:  price,
		Time:   time.Now(),
	}, nil
}

func (c *Connector) GetCandles(ctx context.Context, symbol string, interval shared.CandleInterval, limit int) ([]shared.Candle, error) {
	if !validSymbol(symbol) {
		return nil, fmt.Errorf("%w: %q", shared.ErrSymbolNotFound, symbol)
	}
	gran := convertGranularity(interval)
	path := fmt.Sprintf(pathCandlesFmt+"?granularity=%s", symbol, gran)
	if limit > 0 {
		path += fmt.Sprintf("&limit=%d", limit)
	}
	data, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var cr candleResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return nil, fmt.Errorf("parse candles: %w", err)
	}
	candles := make([]shared.Candle, len(cr.Candles))
	for i, ce := range cr.Candles {
		open, err := strconv.ParseFloat(ce.Open, 64)
		if err != nil {
			return nil, fmt.Errorf("parse candle open: %w", err)
		}
		high, err := strconv.ParseFloat(ce.High, 64)
		if err != nil {
			return nil, fmt.Errorf("parse candle high: %w", err)
		}
		low, err := strconv.ParseFloat(ce.Low, 64)
		if err != nil {
			return nil, fmt.Errorf("parse candle low: %w", err)
		}
		closev, err := strconv.ParseFloat(ce.Close, 64)
		if err != nil {
			return nil, fmt.Errorf("parse candle close: %w", err)
		}
		volume, err := strconv.ParseFloat(ce.Volume, 64)
		if err != nil {
			return nil, fmt.Errorf("parse candle volume: %w", err)
		}
		candles[i] = shared.Candle{
			Symbol:   symbol,
			Interval: string(interval),
			Open:     open,
			High:     high,
			Low:      low,
			Close:    closev,
			Volume:   volume,
		}
	}
	return candles, nil
}

func (c *Connector) SubscribeOrderBook(ctx context.Context, symbol string) (<-chan *shared.OrderBookUpdate, error) {
	return nil, fmt.Errorf("coinbase: websocket not yet implemented")
}

func (c *Connector) PlaceOrder(ctx context.Context, req shared.OrderRequest) (*shared.Order, error) {
	if !validSymbol(req.Symbol) {
		return nil, fmt.Errorf("%w: %q", shared.ErrSymbolNotFound, req.Symbol)
	}
	or := orderRequest{
		ClientOrderID: req.ClientOrderID,
		ProductID:     req.Symbol,
		Side:          string(req.Side),
	}
	if req.Type == shared.TypeMarket {
		or.OrderConfig = orderConfig{
			MarketIOC: &marketOrderConfig{
				BaseSize: fmt.Sprintf("%.8f", req.Quantity),
			},
		}
	} else {
		or.OrderConfig = orderConfig{
			LimitGTC: &limitOrderConfig{
				BaseSize:   fmt.Sprintf("%.8f", req.Quantity),
				LimitPrice: fmt.Sprintf("%.8f", req.Price),
			},
		}
	}
	body, err := json.Marshal(or)
	if err != nil {
		return nil, fmt.Errorf("marshal order: %w", err)
	}
	data, err := c.request(ctx, http.MethodPost, pathOrders, body)
	if err != nil {
		return nil, err
	}
	var resp orderResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse order response: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("coinbase: order failed: order_id=%s", resp.OrderID)
	}
	return convertOrder(&resp, req), nil
}

func (c *Connector) CancelOrder(ctx context.Context, orderID string) error {
	body, _ := json.Marshal(map[string][]string{"order_ids": {orderID}})
	data, err := c.request(ctx, http.MethodPost, pathBatchCancel, body)
	if err != nil {
		return err
	}
	var cr cancelResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return fmt.Errorf("parse cancel: %w", err)
	}
	for _, r := range cr.Results {
		if r.OrderID == orderID {
			if !r.Success {
				return fmt.Errorf("coinbase: cancel failed: %s", r.FailureReason)
			}
			return nil
		}
	}
	return shared.ErrOrderNotFound
}

func (c *Connector) GetOrder(ctx context.Context, orderID string) (*shared.Order, error) {
	path := fmt.Sprintf(pathOrderFmt, orderID)
	data, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var oe orderEntry
	if err := json.Unmarshal(data, &oe); err != nil {
		return nil, fmt.Errorf("parse order: %w", err)
	}
	return convertOrderEntry(&oe)
}

func (c *Connector) ListOpenOrders(ctx context.Context, symbol string) ([]shared.Order, error) {
	path := pathOrdersBatch + "?order_status=OPEN"
	if symbol != "" {
		path += "&product_id=" + symbol
	}
	data, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var or ordersResponse
	if err := json.Unmarshal(data, &or); err != nil {
		return nil, fmt.Errorf("parse orders: %w", err)
	}
	orders := make([]shared.Order, 0, len(or.Orders))
	for i := range or.Orders {
		o, err := convertOrderEntry(&or.Orders[i])
		if err != nil {
			return nil, err
		}
		orders = append(orders, *o)
	}
	return orders, nil
}

func (c *Connector) GetBalance(ctx context.Context, asset string) (*shared.Balance, error) {
	bals, err := c.ListBalances(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range bals {
		if strings.EqualFold(b.Asset, asset) {
			return &b, nil
		}
	}
	return &shared.Balance{Asset: asset}, nil
}

func (c *Connector) ListBalances(ctx context.Context) ([]shared.Balance, error) {
	data, err := c.request(ctx, http.MethodGet, pathAccounts, nil)
	if err != nil {
		return nil, err
	}
	var ar accountsResponse
	if err := json.Unmarshal(data, &ar); err != nil {
		return nil, fmt.Errorf("parse accounts: %w", err)
	}
	bals := make([]shared.Balance, len(ar.Accounts))
	for i, a := range ar.Accounts {
		free, err := strconv.ParseFloat(a.Available, 64)
		if err != nil {
			return nil, fmt.Errorf("parse balance %s: %w", a.Currency, err)
		}
		locked, err := strconv.ParseFloat(a.Hold, 64)
		if err != nil {
			return nil, fmt.Errorf("parse hold %s: %w", a.Currency, err)
		}
		bals[i] = shared.Balance{
			Asset:  a.Currency,
			Free:   free,
			Locked: locked,
			Total:  free + locked,
		}
	}
	return bals, nil
}

func (c *Connector) GetPosition(ctx context.Context, symbol string) (*shared.Position, error) {
	pos, err := c.ListPositions(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range pos {
		if p.Symbol == symbol {
			return &p, nil
		}
	}
	return &shared.Position{Symbol: symbol}, nil
}

func (c *Connector) ListPositions(ctx context.Context) ([]shared.Position, error) {
	data, err := c.request(ctx, http.MethodGet, pathPortfolios, nil)
	if err != nil {
		return nil, err
	}
	var pr portfoliosResponse
	if err := json.Unmarshal(data, &pr); err != nil {
		return nil, fmt.Errorf("parse portfolios: %w", err)
	}
	if len(pr.Portfolios) == 0 {
		return nil, nil
	}
	posPath := fmt.Sprintf(pathPositionsFmt, pr.Portfolios[0].UUID)
	data, err = c.request(ctx, http.MethodGet, posPath, nil)
	if err != nil {
		return nil, err
	}
	var presp positionResponse
	if err := json.Unmarshal(data, &presp); err != nil {
		return nil, fmt.Errorf("parse positions: %w", err)
	}
	positions := make([]shared.Position, len(presp.Positions))
	for i, pe := range presp.Positions {
		size, err := strconv.ParseFloat(pe.Size, 64)
		if err != nil {
			return nil, fmt.Errorf("parse position size: %w", err)
		}
		entry, err := strconv.ParseFloat(pe.EntryPrice, 64)
		if err != nil {
			return nil, fmt.Errorf("parse entry price: %w", err)
		}
		unrealized, err := strconv.ParseFloat(pe.UnrealizedPnL, 64)
		if err != nil {
			return nil, fmt.Errorf("parse unrealized PnL: %w", err)
		}
		positions[i] = shared.Position{
			Symbol:        pe.ProductID,
			Quantity:      size,
			AvgEntryPrice: entry,
			UnrealizedPnL: unrealized,
		}
	}
	return positions, nil
}

func (c *Connector) request(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	fullURL := c.cfg.RESTBaseURL + path
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	headers := map[string]string{
		"Content-Type":        "application/json",
		"CB-ACCESS-KEY":       c.key,
		"CB-ACCESS-SIGN":      c.sign(method, path, timestamp, body),
		"CB-ACCESS-TIMESTAMP": timestamp,
	}
	if c.pass != "" {
		headers["CB-ACCESS-PASSPHRASE"] = c.pass
	}

	resp, err := c.Do(ctx, &baseconnector.Request{
		Method:  method,
		URL:     fullURL,
		Headers: headers,
		Body:    bodyReader,
	})
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, shared.ErrAuthFailed
	}

	if resp.StatusCode >= 400 {
		var eResp errorResponse
		if json.Unmarshal(resp.Body, &eResp) == nil && eResp.Message != "" {
			return nil, fmt.Errorf("coinbase: %s", eResp.Message)
		}
		return nil, fmt.Errorf("coinbase: http %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func (c *Connector) sign(method, path, timestamp string, body []byte) string {
	msg := timestamp + method + path
	if body != nil {
		msg += string(body)
	}
	mac := hmac.New(sha256.New, []byte(c.sec))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func convertGranularity(interval shared.CandleInterval) string {
	switch interval {
	case shared.Interval1m:
		return "ONE_MINUTE"
	case shared.Interval5m:
		return "FIVE_MINUTE"
	case shared.Interval15m:
		return "FIFTEEN_MINUTE"
	case shared.Interval1h:
		return "ONE_HOUR"
	case shared.Interval4h:
		return "FOUR_HOUR"
	case shared.Interval1d:
		return "ONE_DAY"
	default:
		return "ONE_HOUR"
	}
}

func convertBook(br *bookResponse) *shared.OrderBook {
	ob := &shared.OrderBook{
		Symbol: br.ProductID,
		Time:   time.Now(),
	}
	for _, b := range br.Bids {
		price, _ := strconv.ParseFloat(b.Price, 64)
		size, _ := strconv.ParseFloat(b.Size, 64)
		ob.Bids = append(ob.Bids, shared.BookLevel{Price: price, Quantity: size})
	}
	for _, a := range br.Asks {
		price, _ := strconv.ParseFloat(a.Price, 64)
		size, _ := strconv.ParseFloat(a.Size, 64)
		ob.Asks = append(ob.Asks, shared.BookLevel{Price: price, Quantity: size})
	}
	return ob
}

func convertOrder(resp *orderResponse, req shared.OrderRequest) *shared.Order {
	filledSize, _ := strconv.ParseFloat(resp.FilledSize, 64)
	return &shared.Order{
		ID:             resp.OrderID,
		ClientOrderID:  resp.ClientOrderID,
		Symbol:         req.Symbol,
		Side:           req.Side,
		Type:           req.Type,
		Quantity:       req.Quantity,
		FilledQuantity: filledSize,
		Status:         convertStatus(resp.Status),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func convertOrderEntry(oe *orderEntry) (*shared.Order, error) {
	qty, err := strconv.ParseFloat(oe.FilledSize, 64)
	if err != nil {
		return nil, fmt.Errorf("parse filled size: %w", err)
	}
	fv, err := strconv.ParseFloat(oe.FilledValue, 64)
	if err != nil {
		return nil, fmt.Errorf("parse filled value: %w", err)
	}
	avgPrice := 0.0
	if qty > 0 {
		avgPrice = fv / qty
	}
	return &shared.Order{
		ID:             oe.OrderID,
		ClientOrderID:  oe.ClientOrderID,
		Symbol:         oe.ProductID,
		Side:           shared.OrderSide(oe.Side),
		Type:           shared.OrderType(oe.OrderType),
		Price:          avgPrice,
		Quantity:       qty,
		FilledQuantity: qty,
		Status:         convertStatus(oe.Status),
	}, nil
}

func convertStatus(s string) shared.OrderStatus {
	switch strings.ToUpper(s) {
	case "OPEN", "PENDING":
		return shared.StatusOpen
	case "FILLED":
		return shared.StatusFilled
	case "CANCELLED":
		return shared.StatusCancelled
	case "EXPIRED", "REJECTED":
		return shared.StatusRejected
	default:
		return shared.StatusOpen
	}
}
