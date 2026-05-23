package coinbase

import "regexp"

const (
	SandboxRESTURL = "https://api-public.sandbox.exchange.coinbase.com"

	pathPing         = "/api/v3/brokerage/time"
	pathProductBook  = "/api/v3/brokerage/market/product_book"
	pathTickerFmt    = "/api/v3/brokerage/products/%s/ticker"
	pathCandlesFmt   = "/api/v3/brokerage/products/%s/candles"
	pathOrders       = "/api/v3/brokerage/orders"
	pathBatchCancel  = "/api/v3/brokerage/orders/batch_cancel"
	pathOrderFmt     = "/api/v3/brokerage/orders/historical/%s"
	pathOrdersBatch  = "/api/v3/brokerage/orders/historical/batch"
	pathAccounts     = "/api/v3/brokerage/accounts"
	pathPortfolios   = "/api/v3/brokerage/portfolios"
	pathPositionsFmt = "/api/v3/brokerage/portfolios/%s/positions"

	defaultMaxRetries = 3
	coinbaseBurst     = 30
)

var symbolRegex = regexp.MustCompile(`^[A-Z0-9]{2,10}-[A-Z0-9]{2,10}$`)

func validSymbol(s string) bool { return symbolRegex.MatchString(s) }

type Config struct {
	RESTBaseURL string
	APIKey      string
	APISecret   string
	Passphrase  string
}

type bookResponse struct {
	ProductID string          `json:"product_id"`
	Bids      []bookLevelResp `json:"bids"`
	Asks      []bookLevelResp `json:"asks"`
}

type bookLevelResp struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

type tickerResponse struct {
	Price string `json:"price"`
}

type candleResponse struct {
	Candles []candleEntry `json:"candles"`
}

type candleEntry struct {
	Open   string `json:"open"`
	High   string `json:"high"`
	Low    string `json:"low"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
}

type orderRequest struct {
	ClientOrderID string      `json:"client_order_id"`
	ProductID     string      `json:"product_id"`
	Side          string      `json:"side"`
	OrderConfig   orderConfig `json:"order_configuration"`
}

type orderConfig struct {
	MarketIOC *marketOrderConfig `json:"market_market_ioc,omitempty"`
	LimitGTC  *limitOrderConfig  `json:"limit_limit_gtc,omitempty"`
}

type marketOrderConfig struct {
	BaseSize string `json:"base_size,omitempty"`
}

type limitOrderConfig struct {
	BaseSize   string `json:"base_size"`
	LimitPrice string `json:"limit_price"`
}

type orderResponse struct {
	Success       bool   `json:"success"`
	OrderID       string `json:"order_id"`
	ClientOrderID string `json:"client_order_id"`
	Status        string `json:"status"`
	FilledSize    string `json:"filled_size"`
}

type cancelResponse struct {
	Results []cancelResult `json:"results"`
}

type cancelResult struct {
	Success       bool   `json:"success"`
	OrderID       string `json:"order_id"`
	FailureReason string `json:"failure_reason,omitempty"`
}

type ordersResponse struct {
	Orders []orderEntry `json:"orders"`
}

type orderEntry struct {
	OrderID       string `json:"order_id"`
	ClientOrderID string `json:"client_order_id"`
	ProductID     string `json:"product_id"`
	Side          string `json:"side"`
	OrderType     string `json:"order_type"`
	Status        string `json:"status"`
	FilledSize    string `json:"filled_size"`
	FilledValue   string `json:"filled_value"`
}

type accountsResponse struct {
	Accounts []accountEntry `json:"accounts"`
}

type accountEntry struct {
	Currency  string `json:"currency"`
	Available string `json:"available_balance"`
	Hold      string `json:"hold"`
}

type portfoliosResponse struct {
	Portfolios []portfolioEntry `json:"portfolios"`
}

type portfolioEntry struct {
	UUID string `json:"uuid"`
}

type positionResponse struct {
	Positions []positionEntry `json:"positions"`
}

type positionEntry struct {
	ProductID     string `json:"product_id"`
	Size          string `json:"size"`
	EntryPrice    string `json:"entry_vwap"`
	UnrealizedPnL string `json:"value_at_cost"`
}

type errorResponse struct {
	Message string `json:"message"`
}
