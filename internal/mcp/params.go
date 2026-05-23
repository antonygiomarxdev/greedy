package mcp

type GetTickerParams struct {
	Symbol   string `json:"symbol"`
	Exchange string `json:"exchange,omitempty"`
}

type GetOrderBookParams struct {
	Symbol   string `json:"symbol"`
	Depth    int    `json:"depth"`
	Exchange string `json:"exchange,omitempty"`
}

type GetCandlesParams struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
	Limit    int    `json:"limit"`
	Exchange string `json:"exchange,omitempty"`
}

type PlaceOrderParams struct {
	Symbol   string  `json:"symbol"`
	Side     string  `json:"side"`
	Type     string  `json:"type"`
	Quantity float64 `json:"quantity"`
	Price    float64 `json:"price"`
	Exchange string  `json:"exchange,omitempty"`
}

type CancelOrderParams struct {
	OrderID  string `json:"order_id"`
	Exchange string `json:"exchange,omitempty"`
}

type GetPositionsParams struct {
	Exchange string `json:"exchange,omitempty"`
}

type GetBalancesParams struct {
	Exchange string `json:"exchange,omitempty"`
}

type GetOrderHistoryParams struct {
	BotID  string `json:"bot_id,omitempty"`
	Symbol string `json:"symbol,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type StartBotParams struct {
	StrategyFile string                 `json:"strategy_file,omitempty"`
	ID           string                 `json:"id,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Exchange     string                 `json:"exchange,omitempty"`
	Type         string                 `json:"type,omitempty"`
	Symbol       string                 `json:"symbol,omitempty"`
	Params       map[string]interface{} `json:"params,omitempty"`
}

type StopBotParams struct {
	BotID string `json:"bot_id"`
}

type AddMarketParams struct {
	Symbol          string  `json:"symbol"`
	InitialPrice    float64 `json:"initial_price"`
	Drift           float64 `json:"drift"`
	Volatility      float64 `json:"volatility"`
	LiquidityLevels int     `json:"liquidity_levels"`
	LiquidityDepth  float64 `json:"liquidity_depth"`
}

type GetBotStatusParams struct {
	BotID string `json:"bot_id"`
}

type SetCredentialParams struct {
	Exchange   string `json:"exchange"`
	Label      string `json:"label"`
	APIKey     string `json:"api_key"`
	APISecret  string `json:"api_secret"`
	Passphrase string `json:"passphrase,omitempty"`
}

type DeleteCredentialParams struct {
	Exchange string `json:"exchange"`
	Label    string `json:"label"`
}
