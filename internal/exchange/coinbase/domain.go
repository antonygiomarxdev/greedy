package coinbase

import "time"

const (
	SandboxRESTURL    = "https://api-public.sandbox.exchange.coinbase.com"
	defaultMaxRetries = 3
)

type Config struct {
	RESTBaseURL string
	APIKey      string
	APISecret   string
	Passphrase  string
}

type productResponse struct {
	ProductID string `json:"product_id"`
	Price     string `json:"price"`
}

type tickerResponse struct {
	Price  string `json:"price"`
	Time   string `json:"time"`
	Bid    string `json:"bid"`
	Ask    string `json:"ask"`
	Volume string `json:"volume"`
}

type bookResponse struct {
	ProductID string          `json:"product_id"`
	Bids      []bookLevelResp `json:"bids"`
	Asks      []bookLevelResp `json:"asks"`
	Time      string          `json:"time"`
}

type bookLevelResp struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

type candleResponse struct {
	Candles []candleEntry `json:"candles"`
}

type candleEntry struct {
	Start  string `json:"start"`
	Low    string `json:"low"`
	High   string `json:"high"`
	Open   string `json:"open"`
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
	QuoteSize string `json:"quote_size,omitempty"`
	BaseSize  string `json:"base_size,omitempty"`
}

type limitOrderConfig struct {
	BaseSize   string `json:"base_size"`
	LimitPrice string `json:"limit_price"`
	PostOnly   bool   `json:"post_only"`
}

type orderResponse struct {
	Success       bool        `json:"success"`
	OrderID       string      `json:"order_id"`
	ClientOrderID string      `json:"client_order_id"`
	OrderConfig   orderConfig `json:"order_configuration"`
	Status        string      `json:"status"`
	FilledSize    string      `json:"filled_size"`
	TotalFees     string      `json:"total_fees"`
	CreatedTime   string      `json:"created_time"`
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
	Orders  []orderEntry `json:"orders"`
	HasNext bool         `json:"has_next"`
	Cursor  string       `json:"cursor"`
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
	AvgPrice      string `json:"average_filled_price"`
	TotalFees     string `json:"total_fees"`
	CreatedTime   string `json:"created_time"`
	CancelMsg     string `json:"cancel_message,omitempty"`
}

type accountsResponse struct {
	Accounts []accountEntry `json:"accounts"`
}

type accountEntry struct {
	Currency      string `json:"currency"`
	Available     string `json:"available_balance"`
	Hold          string `json:"hold"`
	PortfolioUUID string `json:"portfolio_uuid"`
}

type portfoliosResponse struct {
	Portfolios []portfolioEntry `json:"portfolios"`
}

type portfolioEntry struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type positionResponse struct {
	Positions []positionEntry `json:"positions"`
}

type positionEntry struct {
	ProductID     string `json:"product_id"`
	Size          string `json:"size"`
	EntryPrice    string `json:"entry_vwap"`
	CurrentPrice  string `json:"current_price"`
	UnrealizedPnL string `json:"value_at_cost"`
}

type serverTimeResponse struct {
	ISO   string `json:"iso"`
	Epoch int64  `json:"epoch"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"error_details"`
}

type rateLimiterConfig struct {
	Burst          int
	RefillDuration time.Duration
}
