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
	"sync"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/shared"
)

var _ shared.Exchange = (*Connector)(nil)

type Connector struct {
	client     *http.Client
	cfg        Config
	key        string
	secret     string
	pass       string
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
	burst      int
	refillDur  time.Duration
}

func New(cfg Config) *Connector {
	if cfg.RESTBaseURL == "" {
		cfg.RESTBaseURL = SandboxRESTURL
	}
	return &Connector{
		client:    &http.Client{Timeout: 30 * time.Second},
		cfg:       cfg,
		key:       cfg.APIKey,
		secret:    cfg.APISecret,
		pass:      cfg.Passphrase,
		tokens:    float64(30),
		burst:     30,
		refillDur: time.Second,
	}
}

func (c *Connector) Name() string { return string(shared.ProviderCoinbase) }

func (c *Connector) Ping(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodGet, "/api/v3/brokerage/time", nil)
	return err
}

func (c *Connector) GetOrderBook(ctx context.Context, symbol string, depth int) (*shared.OrderBook, error) {
	path := fmt.Sprintf("/api/v3/brokerage/market/product_book?product_id=%s", symbol)
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
	path := fmt.Sprintf("/api/v3/brokerage/products/%s/ticker", symbol)
	data, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var tr tickerResponse
	if err := json.Unmarshal(data, &tr); err != nil {
		return nil, fmt.Errorf("parse ticker: %w", err)
	}
	price, _ := strconv.ParseFloat(tr.Price, 64)
	return &shared.Ticker{
		Symbol: symbol,
		Price:  price,
		Time:   time.Now(),
	}, nil
}

func (c *Connector) GetCandles(ctx context.Context, symbol string, interval shared.CandleInterval, limit int) ([]shared.Candle, error) {
	gran := convertGranularity(interval)
	path := fmt.Sprintf("/api/v3/brokerage/products/%s/candles?granularity=%s", symbol, gran)
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
		open, _ := strconv.ParseFloat(ce.Open, 64)
		high, _ := strconv.ParseFloat(ce.High, 64)
		low, _ := strconv.ParseFloat(ce.Low, 64)
		closev, _ := strconv.ParseFloat(ce.Close, 64)
		volume, _ := strconv.ParseFloat(ce.Volume, 64)
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
	data, err := c.request(ctx, http.MethodPost, "/api/v3/brokerage/orders", body)
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
	data, err := c.request(ctx, http.MethodPost, "/api/v3/brokerage/orders/batch_cancel", body)
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
	path := fmt.Sprintf("/api/v3/brokerage/orders/historical/%s", orderID)
	data, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var oe orderEntry
	if err := json.Unmarshal(data, &oe); err != nil {
		return nil, fmt.Errorf("parse order: %w", err)
	}
	return convertOrderEntry(&oe), nil
}

func (c *Connector) ListOpenOrders(ctx context.Context, symbol string) ([]shared.Order, error) {
	path := "/api/v3/brokerage/orders/historical/batch?order_status=OPEN"
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
	orders := make([]shared.Order, len(or.Orders))
	for i, oe := range or.Orders {
		orders[i] = *convertOrderEntry(&oe)
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
	data, err := c.request(ctx, http.MethodGet, "/api/v3/brokerage/accounts", nil)
	if err != nil {
		return nil, err
	}
	var ar accountsResponse
	if err := json.Unmarshal(data, &ar); err != nil {
		return nil, fmt.Errorf("parse accounts: %w", err)
	}
	bals := make([]shared.Balance, len(ar.Accounts))
	for i, a := range ar.Accounts {
		free, _ := strconv.ParseFloat(a.Available, 64)
		locked, _ := strconv.ParseFloat(a.Hold, 64)
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
	data, err := c.request(ctx, http.MethodGet, "/api/v3/brokerage/portfolios", nil)
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
	puuid := pr.Portfolios[0].UUID
	posPath := fmt.Sprintf("/api/v3/brokerage/portfolios/%s/positions", puuid)
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
		size, _ := strconv.ParseFloat(pe.Size, 64)
		entry, _ := strconv.ParseFloat(pe.EntryPrice, 64)
		cur, _ := strconv.ParseFloat(pe.CurrentPrice, 64)
		unrealized, _ := strconv.ParseFloat(pe.UnrealizedPnL, 64)
		positions[i] = shared.Position{
			Symbol:        pe.ProductID,
			Quantity:      size,
			AvgEntryPrice: entry,
			UnrealizedPnL: unrealized,
		}
		_ = cur
	}
	return positions, nil
}

func (c *Connector) request(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if err := c.waitToken(ctx); err != nil {
		return nil, err
	}

	url := c.cfg.RESTBaseURL + path
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	sig := c.sign(method, path, timestamp, body)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CB-ACCESS-KEY", c.key)
	req.Header.Set("CB-ACCESS-SIGN", sig)
	req.Header.Set("CB-ACCESS-TIMESTAMP", timestamp)
	if c.pass != "" {
		req.Header.Set("CB-ACCESS-PASSPHRASE", c.pass)
	}

	for attempt := 0; attempt < defaultMaxRetries; attempt++ {
		resp, err := c.client.Do(req)
		if err != nil {
			if attempt == defaultMaxRetries-1 {
				return nil, fmt.Errorf("http request: %w", err)
			}
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			continue
		}
		defer resp.Body.Close()

		respData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt == defaultMaxRetries-1 {
				return nil, shared.ErrRateLimited
			}
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		if resp.StatusCode == http.StatusUnauthorized {
			return nil, shared.ErrAuthFailed
		}

		if resp.StatusCode >= 400 {
			var eResp errorResponse
			if json.Unmarshal(respData, &eResp) == nil {
				return nil, fmt.Errorf("coinbase: %s: %s", eResp.Error, eResp.Message)
			}
			return nil, fmt.Errorf("coinbase: http %d: %s", resp.StatusCode, string(respData))
		}

		return respData, nil
	}

	return nil, shared.ErrExchangeDown
}

func (c *Connector) sign(method, path, timestamp string, body []byte) string {
	msg := timestamp + method + path
	if body != nil {
		msg += string(body)
	}
	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *Connector) waitToken(ctx context.Context) error {
	c.mu.Lock()
	now := time.Now()
	elapsed := now.Sub(c.lastRefill).Seconds()
	c.tokens += elapsed * float64(c.burst) / c.refillDur.Seconds()
	if c.tokens > float64(c.burst) {
		c.tokens = float64(c.burst)
	}
	c.lastRefill = now

	if c.tokens >= 1 {
		c.tokens--
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	waitTime := time.Duration((1 - c.tokens) * float64(c.refillDur) / float64(c.burst) * 1e9)
	select {
	case <-time.After(waitTime):
	case <-ctx.Done():
		return ctx.Err()
	}

	c.mu.Lock()
	c.tokens = 0
	c.lastRefill = time.Now()
	c.mu.Unlock()
	return nil
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

func convertOrderEntry(oe *orderEntry) *shared.Order {
	qty, _ := strconv.ParseFloat(oe.FilledSize, 64)
	fv, _ := strconv.ParseFloat(oe.FilledValue, 64)
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
	}
}

func convertStatus(s string) shared.OrderStatus {
	switch strings.ToUpper(s) {
	case "OPEN":
		return shared.StatusOpen
	case "PENDING":
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
