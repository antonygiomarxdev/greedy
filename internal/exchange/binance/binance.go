package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/exchange/baseconnector"
	"github.com/antonygiomarxdev/greedy/internal/ratelimiter"
	"github.com/antonygiomarxdev/greedy/internal/shared"
	"github.com/gorilla/websocket"
)

var _ shared.Exchange = (*Connector)(nil)

type Connector struct {
	baseconnector.BaseConnector
	cfg    Config
	key    string
	secret string

	ordersMu     sync.RWMutex
	orderSymbols map[string]string
}

func New(cfg Config) *Connector {
	if cfg.RESTBaseURL == "" {
		cfg.RESTBaseURL = RESTURL
	}
	return &Connector{
		BaseConnector: baseconnector.BaseConnector{
			Client: &http.Client{Timeout: 30 * time.Second},
			RL:     ratelimiter.NewTokenBucket(binanceBurst, time.Second),
		},
		cfg:          cfg,
		key:          cfg.APIKey,
		secret:       cfg.APISecret,
		orderSymbols: make(map[string]string),
	}
}

func (c *Connector) Name() string { return string(shared.ProviderBinance) }

func (c *Connector) Ping(ctx context.Context) error {
	_, err := c.publicRequest(ctx, http.MethodGet, pathPing, nil)
	return err
}

func (c *Connector) GetOrderBook(ctx context.Context, symbol string, depth int) (*shared.OrderBook, error) {
	params := map[string]string{"symbol": symbol}
	if depth > 0 {
		params["limit"] = strconv.Itoa(depth)
	}
	data, err := c.publicRequest(ctx, http.MethodGet, pathDepth, params)
	if err != nil {
		return nil, err
	}
	var dr depthResponse
	if err := json.Unmarshal(data, &dr); err != nil {
		return nil, fmt.Errorf("parse order book: %w", err)
	}
	return convertBook(&dr, symbol), nil
}

func (c *Connector) GetTicker(ctx context.Context, symbol string) (*shared.Ticker, error) {
	if !validSymbol(symbol) {
		return nil, fmt.Errorf("%w: %q", shared.ErrSymbolNotFound, symbol)
	}
	params := map[string]string{"symbol": symbol}
	data, err := c.publicRequest(ctx, http.MethodGet, pathTicker, params)
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
	params := map[string]string{
		"symbol":   symbol,
		"interval": convertInterval(interval),
	}
	if limit > 0 {
		params["limit"] = strconv.Itoa(limit)
	}
	data, err := c.publicRequest(ctx, http.MethodGet, pathKlines, params)
	if err != nil {
		return nil, err
	}
	var kr klineResponse
	if err := json.Unmarshal(data, &kr); err != nil {
		return nil, fmt.Errorf("parse klines: %w", err)
	}
	candles := make([]shared.Candle, len(kr))
	for i, k := range kr {
		if len(k) < 6 {
			continue
		}
		ts := int64(k[0].(float64)) / 1000
		open, _ := strconv.ParseFloat(k[1].(string), 64)
		high, _ := strconv.ParseFloat(k[2].(string), 64)
		low, _ := strconv.ParseFloat(k[3].(string), 64)
		closev, _ := strconv.ParseFloat(k[4].(string), 64)
		volume, _ := strconv.ParseFloat(k[5].(string), 64)
		candles[i] = shared.Candle{
			Symbol:   symbol,
			Interval: string(interval),
			OpenTime: time.Unix(ts, 0),
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
	if !validSymbol(symbol) {
		return nil, fmt.Errorf("%w: %q", shared.ErrSymbolNotFound, symbol)
	}

	lower := strings.ToLower(symbol)
	u := WSURL + lower + "@depth@100ms"

	ws, _, err := websocket.DefaultDialer.DialContext(ctx, u, nil)
	if err != nil {
		return nil, fmt.Errorf("binance ws dial: %w", err)
	}

	ch := make(chan *shared.OrderBookUpdate, 32)

	go func() {
		defer func() { _ = ws.Close() }()
		defer close(ch)

		cleanup := make(chan struct{})
		defer close(cleanup)

		go func() {
			select {
			case <-ctx.Done():
				_ = ws.Close()
			case <-cleanup:
			}
		}()

		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				return
			}
			var du depthUpdate
			if err := json.Unmarshal(msg, &du); err != nil {
				continue
			}
			update := &shared.OrderBookUpdate{Symbol: symbol}
			for _, b := range du.Bids {
				price, _ := strconv.ParseFloat(b[0], 64)
				qty, _ := strconv.ParseFloat(b[1], 64)
				update.Bids = append(update.Bids, shared.BookLevel{Price: price, Quantity: qty})
			}
			for _, a := range du.Asks {
				price, _ := strconv.ParseFloat(a[0], 64)
				qty, _ := strconv.ParseFloat(a[1], 64)
				update.Asks = append(update.Asks, shared.BookLevel{Price: price, Quantity: qty})
			}
			select {
			case ch <- update:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

func (c *Connector) PlaceOrder(ctx context.Context, req shared.OrderRequest) (*shared.Order, error) {
	if !validSymbol(req.Symbol) {
		return nil, fmt.Errorf("%w: %q", shared.ErrSymbolNotFound, req.Symbol)
	}
	orderReq := orderRequest{
		Symbol:           req.Symbol,
		Side:             strings.ToUpper(string(req.Side)),
		Type:             strings.ToUpper(string(req.Type)),
		Quantity:         fmt.Sprintf("%.8f", req.Quantity),
		NewClientOrderID: req.ClientOrderID,
	}
	if req.Type == shared.TypeLimit {
		orderReq.Price = fmt.Sprintf("%.8f", req.Price)
		orderReq.TimeInForce = "GTC"
	}
	data, err := c.signedRequest(ctx, http.MethodPost, pathOrder, orderReq.toMap())
	if err != nil {
		return nil, err
	}
	var resp orderResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse order response: %w", err)
	}
	order := convertOrder(&resp, req)
	c.trackOrder(order.ID, order.Symbol)
	return order, nil
}

func (c *Connector) trackOrder(orderID, symbol string) {
	c.ordersMu.Lock()
	c.orderSymbols[orderID] = symbol
	c.ordersMu.Unlock()
}

func (c *Connector) lookupOrder(orderID string) (string, bool) {
	c.ordersMu.RLock()
	sym, ok := c.orderSymbols[orderID]
	c.ordersMu.RUnlock()
	return sym, ok
}

func (c *Connector) CancelOrder(ctx context.Context, orderID string) error {
	sym, ok := c.lookupOrder(orderID)
	if !ok {
		return fmt.Errorf("binance: unknown order %s — cannot cancel without symbol", orderID)
	}
	params := map[string]string{
		"symbol":  sym,
		"orderId": orderID,
	}
	data, err := c.signedRequest(ctx, http.MethodDelete, pathOrder, params)
	if err != nil {
		return err
	}
	var cr cancelResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return fmt.Errorf("parse cancel response: %w", err)
	}
	if cr.Status == "CANCELED" {
		return nil
	}
	return shared.ErrOrderNotFound
}

func (c *Connector) GetOrder(ctx context.Context, orderID string) (*shared.Order, error) {
	sym, ok := c.lookupOrder(orderID)
	if !ok {
		return nil, fmt.Errorf("binance: unknown order %s — cannot get without symbol", orderID)
	}
	params := map[string]string{
		"symbol":  sym,
		"orderId": orderID,
	}
	data, err := c.signedRequest(ctx, http.MethodGet, pathOrder, params)
	if err != nil {
		return nil, err
	}
	var resp orderResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse order: %w", err)
	}
	return convertOrderResponse(&resp), nil
}

func (c *Connector) ListOpenOrders(ctx context.Context, symbol string) ([]shared.Order, error) {
	params := map[string]string{}
	if symbol != "" {
		params["symbol"] = symbol
	}
	data, err := c.signedRequest(ctx, http.MethodGet, pathOpenOrders, params)
	if err != nil {
		return nil, err
	}
	var orders []orderResponse
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("parse open orders: %w", err)
	}
	result := make([]shared.Order, len(orders))
	for i, o := range orders {
		order := convertOrderResponse(&o)
		c.trackOrder(order.ID, order.Symbol)
		result[i] = *order
	}
	return result, nil
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
	data, err := c.signedRequest(ctx, http.MethodGet, pathAccount, nil)
	if err != nil {
		return nil, err
	}
	var ar accountResponse
	if err := json.Unmarshal(data, &ar); err != nil {
		return nil, fmt.Errorf("parse account: %w", err)
	}
	bals := make([]shared.Balance, 0, len(ar.Balances))
	for _, b := range ar.Balances {
		free, _ := strconv.ParseFloat(b.Free, 64)
		locked, _ := strconv.ParseFloat(b.Locked, 64)
		if free == 0 && locked == 0 {
			continue
		}
		bals = append(bals, shared.Balance{
			Asset:  b.Asset,
			Free:   free,
			Locked: locked,
			Total:  free + locked,
		})
	}
	return bals, nil
}

func (c *Connector) GetPosition(ctx context.Context, symbol string) (*shared.Position, error) {
	return &shared.Position{Symbol: symbol}, nil
}

func (c *Connector) ListPositions(ctx context.Context) ([]shared.Position, error) {
	return nil, nil
}

func (c *Connector) publicRequest(ctx context.Context, method, path string, params map[string]string) ([]byte, error) {
	urlStr := c.cfg.RESTBaseURL + path
	if len(params) > 0 {
		urlStr += "?" + encodeParams(params)
	}
	resp, err := c.Do(ctx, &baseconnector.Request{
		Method:  method,
		URL:     urlStr,
		Headers: nil,
	})
	if err != nil {
		return nil, err
	}
	if err := checkError(resp.Body, resp.StatusCode); err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (c *Connector) signedRequest(ctx context.Context, method, path string, params map[string]string) ([]byte, error) {
	if params == nil {
		params = make(map[string]string)
	}
	params["timestamp"] = strconv.FormatInt(time.Now().UnixMilli(), 10)

	query := encodeParams(params)
	sig := c.sign(query)
	query += "&signature=" + sig

	fullURL := c.cfg.RESTBaseURL + path + "?" + query
	var body io.Reader

	if method == http.MethodPost || method == http.MethodPut {
		body = strings.NewReader(query)
		fullURL = c.cfg.RESTBaseURL + path
	}

	headers := map[string]string{"X-MBX-APIKEY": c.key}
	if method == http.MethodPost || method == http.MethodPut {
		headers["Content-Type"] = "application/x-www-form-urlencoded"
	}

	resp, err := c.Do(ctx, &baseconnector.Request{
		Method:  method,
		URL:     fullURL,
		Headers: headers,
		Body:    body,
	})
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, shared.ErrAuthFailed
	}

	if err := checkError(resp.Body, resp.StatusCode); err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func (c *Connector) sign(query string) string {
	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write([]byte(query))
	return hex.EncodeToString(mac.Sum(nil))
}

func encodeParams(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(url.QueryEscape(k))
		buf.WriteByte('=')
		buf.WriteString(url.QueryEscape(params[k]))
	}
	return buf.String()
}

func checkError(data []byte, statusCode int) error {
	if statusCode < 400 {
		return nil
	}
	var er errorResponse
	if json.Unmarshal(data, &er) == nil && er.Msg != "" {
		return fmt.Errorf("binance: %s (code %d)", er.Msg, er.Code)
	}
	return fmt.Errorf("binance: http %d", statusCode)
}

func convertInterval(interval shared.CandleInterval) string {
	switch interval {
	case shared.Interval1m:
		return "1m"
	case shared.Interval5m:
		return "5m"
	case shared.Interval15m:
		return "15m"
	case shared.Interval1h:
		return "1h"
	case shared.Interval4h:
		return "4h"
	case shared.Interval1d:
		return "1d"
	default:
		return "1h"
	}
}

func convertBook(dr *depthResponse, symbol string) *shared.OrderBook {
	ob := &shared.OrderBook{
		Symbol: symbol,
		Time:   time.Now(),
	}
	for _, b := range dr.Bids {
		price, _ := strconv.ParseFloat(b[0], 64)
		size, _ := strconv.ParseFloat(b[1], 64)
		ob.Bids = append(ob.Bids, shared.BookLevel{Price: price, Quantity: size})
	}
	for _, a := range dr.Asks {
		price, _ := strconv.ParseFloat(a[0], 64)
		size, _ := strconv.ParseFloat(a[1], 64)
		ob.Asks = append(ob.Asks, shared.BookLevel{Price: price, Quantity: size})
	}
	return ob
}

func convertOrder(resp *orderResponse, req shared.OrderRequest) *shared.Order {
	filledQty, _ := strconv.ParseFloat(resp.FilledQty, 64)
	cumQuote, _ := strconv.ParseFloat(resp.CumQuote, 64)
	avgPrice := 0.0
	if filledQty > 0 {
		avgPrice = cumQuote / filledQty
	}
	return &shared.Order{
		ID:             strconv.FormatInt(resp.OrderID, 10),
		ClientOrderID:  resp.ClientOrderID,
		Symbol:         req.Symbol,
		Side:           req.Side,
		Type:           req.Type,
		Price:          avgPrice,
		Quantity:       req.Quantity,
		FilledQuantity: filledQty,
		Status:         convertStatus(resp.Status),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func convertOrderResponse(resp *orderResponse) *shared.Order {
	qty, _ := strconv.ParseFloat(resp.OrigQty, 64)
	filledQty, _ := strconv.ParseFloat(resp.FilledQty, 64)
	cumQuote, _ := strconv.ParseFloat(resp.CumQuote, 64)
	avgPrice := 0.0
	if filledQty > 0 {
		avgPrice = cumQuote / filledQty
	}
	return &shared.Order{
		ID:             strconv.FormatInt(resp.OrderID, 10),
		ClientOrderID:  resp.ClientOrderID,
		Symbol:         resp.Symbol,
		Side:           shared.OrderSide(strings.ToLower(resp.Side)),
		Type:           shared.OrderType(strings.ToLower(resp.Type)),
		Price:          avgPrice,
		Quantity:       qty,
		FilledQuantity: filledQty,
		Status:         convertStatus(resp.Status),
		CreatedAt:      time.UnixMilli(resp.TransactTime),
		UpdatedAt:      time.UnixMilli(resp.TransactTime),
	}
}

func convertStatus(s string) shared.OrderStatus {
	switch strings.ToUpper(s) {
	case "NEW", "PARTIALLY_FILLED":
		return shared.StatusOpen
	case "FILLED":
		return shared.StatusFilled
	case "CANCELED", "PENDING_CANCEL":
		return shared.StatusCancelled
	case "REJECTED", "EXPIRED":
		return shared.StatusRejected
	default:
		return shared.StatusOpen
	}
}
