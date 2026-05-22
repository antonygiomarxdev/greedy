package mcp

type GetTickerParams struct {
	Symbol string `json:"symbol"`
}

type GetOrderBookParams struct {
	Symbol string `json:"symbol"`
	Depth  int    `json:"depth"`
}

type GetCandlesParams struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
	Limit    int    `json:"limit"`
}

type PlaceOrderParams struct {
	Symbol   string  `json:"symbol"`
	Side     string  `json:"side"`
	Type     string  `json:"type"`
	Quantity float64 `json:"quantity"`
	Price    float64 `json:"price"`
}

type CancelOrderParams struct {
	OrderID string `json:"order_id"`
}

type StartBotParams struct {
	StrategyFile string `json:"strategy_file"`
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
