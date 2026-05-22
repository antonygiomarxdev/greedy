package mcp

func (s *Server) ListPrompts() []PromptDef {
	return []PromptDef{
		{
			Name:        "analyze_portfolio",
			Description: "Generate a portfolio risk and exposure analysis",
			Arguments:   []PromptArg{},
		},
		{
			Name:        "review_trades",
			Description: "Review today's trading activity with P&L breakdown",
			Arguments: []PromptArg{
				{Name: "symbol", Description: "Filter by symbol (optional)", Required: false},
				{Name: "period", Description: "Time period: today, week, month", Required: false},
			},
		},
		{
			Name:        "suggest_strategy",
			Description: "Suggest DCA or GRID strategy parameters based on current market conditions",
			Arguments: []PromptArg{
				{Name: "symbol", Description: "Trading symbol", Required: true},
				{Name: "strategy_type", Description: "Strategy type: dca, grid", Required: true},
			},
		},
	}
}
