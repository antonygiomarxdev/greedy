package binance

import "regexp"

const (
	RESTURL        = "https://api.binance.com"
	TestnetRESTURL = "https://testnet.binance.vision"
	WSURL          = "wss://stream.binance.com:9443/ws/"

	pathPing       = "/api/v3/ping"
	pathDepth      = "/api/v3/depth"
	pathTicker     = "/api/v3/ticker/price"
	pathKlines     = "/api/v3/klines"
	pathOrder      = "/api/v3/order"
	pathOpenOrders = "/api/v3/openOrders"
	pathAccount    = "/api/v3/account"

	defaultMaxRetries = 3
	binanceBurst      = 20
)

var symbolRegex = regexp.MustCompile(`^[A-Z0-9]{5,12}$`)

func validSymbol(s string) bool { return symbolRegex.MatchString(s) }

type Config struct {
	RESTBaseURL string
	APIKey      string
	APISecret   string
}

type depthResponse struct {
	Bids [][2]string `json:"bids"`
	Asks [][2]string `json:"asks"`
}

type tickerResponse struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type klineResponse [][]any

type orderRequest struct {
	Symbol           string `json:"symbol"`
	Side             string `json:"side"`
	Type             string `json:"type"`
	Quantity         string `json:"quantity"`
	Price            string `json:"price,omitempty"`
	TimeInForce      string `json:"timeInForce,omitempty"`
	NewClientOrderID string `json:"newClientOrderId,omitempty"`
}

func (r *orderRequest) toMap() map[string]string {
	m := map[string]string{
		"symbol":           r.Symbol,
		"side":             r.Side,
		"type":             r.Type,
		"quantity":         r.Quantity,
		"newClientOrderId": r.NewClientOrderID,
	}
	if r.Price != "" {
		m["price"] = r.Price
	}
	if r.TimeInForce != "" {
		m["timeInForce"] = r.TimeInForce
	}
	return m
}

type orderResponse struct {
	Symbol        string `json:"symbol"`
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Status        string `json:"status"`
	FilledQty     string `json:"executedQty"`
	CumQuote      string `json:"cummulativeQuoteQty"`
	OrigQty       string `json:"origQty"`
	Price         string `json:"price"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	TransactTime  int64  `json:"transactTime"`
}

type cancelResponse struct {
	Symbol  string `json:"symbol"`
	OrderID int64  `json:"orderId"`
	Status  string `json:"status"`
}

type accountResponse struct {
	Balances []balanceEntry `json:"balances"`
}

type balanceEntry struct {
	Asset  string `json:"asset"`
	Free   string `json:"free"`
	Locked string `json:"locked"`
}

type errorResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type depthUpdate struct {
	EventType string     `json:"e"`
	EventTime int64      `json:"E"`
	Symbol    string     `json:"s"`
	Bids      [][2]string `json:"b"`
	Asks      [][2]string `json:"a"`
}
