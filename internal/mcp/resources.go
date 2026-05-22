package mcp

type ResourceProvider interface {
	Resources() []ResourceDef
}

type PromptProvider interface {
	Prompts() []PromptDef
}

type staticResourceProvider struct {
	resources []ResourceDef
}

func (p *staticResourceProvider) Resources() []ResourceDef { return p.resources }

type staticPromptProvider struct {
	prompts []PromptDef
}

func (p *staticPromptProvider) Prompts() []PromptDef { return p.prompts }

var resourceProviders []ResourceProvider
var promptProviders []PromptProvider

func RegisterResourceProvider(p ResourceProvider) {
	resourceProviders = append(resourceProviders, p)
}

func RegisterPromptProvider(p PromptProvider) {
	promptProviders = append(promptProviders, p)
}

func init() {
	RegisterResourceProvider(&staticResourceProvider{resources: []ResourceDef{
		{URI: "portfolio://summary", Name: "Portfolio Summary", Description: "Full portfolio snapshot with total P&L, exposure, and positions", MimeType: "application/json"},
		{URI: "market://prices/{symbol}", Name: "Live Price", Description: "Real-time price for any trading symbol", MimeType: "application/json"},
		{URI: "bot://{id}/status", Name: "Bot Status", Description: "Detailed status, open orders, and P&L for a specific bot", MimeType: "application/json"},
	}})

	RegisterPromptProvider(&staticPromptProvider{prompts: []PromptDef{
		{Name: "analyze_portfolio", Description: "Generate a portfolio risk and exposure analysis", Arguments: []PromptArg{}},
		{Name: "review_trades", Description: "Review today's trading activity with P&L breakdown", Arguments: []PromptArg{{Name: "symbol", Description: "Filter by symbol (optional)", Required: false}, {Name: "period", Description: "Time period: today, week, month", Required: false}}},
		{Name: "suggest_strategy", Description: "Suggest DCA or GRID strategy parameters based on current market conditions", Arguments: []PromptArg{{Name: "symbol", Description: "Trading symbol", Required: true}, {Name: "strategy_type", Description: "Strategy type: dca, grid", Required: true}}},
	}})
}
