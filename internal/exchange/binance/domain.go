package binance

import "regexp"

const (
	RESTURL        = "https://api.binance.com"
	TestnetRESTURL = "https://testnet.binance.vision"

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
