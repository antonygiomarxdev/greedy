package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/credentials"
	"github.com/antonygiomarxdev/greedy/internal/exchange"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/paper"
	"github.com/antonygiomarxdev/greedy/internal/shared"
	"github.com/antonygiomarxdev/greedy/internal/trading"
	"github.com/antonygiomarxdev/greedy/internal/trading/strategy"
)

func init() {
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &getTickerCommand{reg: reg}
	})
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &getOrderBookCommand{reg: reg}
	})
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &getCandlesCommand{reg: reg}
	})
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &placeOrderCommand{reg: reg}
	})
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &cancelOrderCommand{reg: reg}
	})
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &getPositionsCommand{reg: reg}
	})
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &getBalancesCommand{reg: reg}
	})
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &startBotCommand{sup: sup}
	})
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &stopBotCommand{sup: sup}
	})
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &listBotsCommand{sup: sup}
	})
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &addMarketCommand{reg: reg, sup: sup}
	})
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &getBotStatusCommand{sup: sup}
	})
	RegisterCommandFactory(func(reg *exchange.Registry, sup *trading.Supervisor) Command {
		return &getOrderHistoryCommand{sup: sup}
	})
}

func resolveExchange(reg *exchange.Registry, provider string) shared.Exchange {
	return reg.GetOrDefault(shared.ExchangeProvider(provider))
}

type getTickerCommand struct{ reg *exchange.Registry }

func (c *getTickerCommand) Name() string { return NameGetTicker }
func (c *getTickerCommand) Description() string {
	return "Get current price for a trading symbol. Returns symbol, price, and timestamp. Symbol format: BTC-USD, ETH-USD. Optional exchange parameter to target a specific exchange."
}
func (c *getTickerCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "exchange": map[string]any{"type": "string", "description": "Target exchange (paper, coinbase, binance). Defaults to paper."}}, "required": []string{"symbol"}}
}

func (c *getTickerCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p GetTickerParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	ex := resolveExchange(c.reg, p.Exchange)
	ticker, err := ex.GetTicker(ctx, p.Symbol)
	if err != nil {
		return "", err
	}
	return jsonString(ticker)
}

type getOrderBookCommand struct{ reg *exchange.Registry }

func (c *getOrderBookCommand) Name() string { return NameGetOrderBook }
func (c *getOrderBookCommand) Description() string {
	return "Get current order book bids and asks for a symbol. Specify depth to limit levels returned. Optional exchange parameter to target a specific exchange."
}
func (c *getOrderBookCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "depth": map[string]any{"type": "integer", "default": 10}, "exchange": map[string]any{"type": "string", "description": "Target exchange (paper, coinbase, binance). Defaults to paper."}}, "required": []string{"symbol"}}
}

func (c *getOrderBookCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p GetOrderBookParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if p.Depth == 0 {
		p.Depth = 10
	}
	ex := resolveExchange(c.reg, p.Exchange)
	book, err := ex.GetOrderBook(ctx, p.Symbol, p.Depth)
	if err != nil {
		return "", err
	}
	return jsonString(book)
}

type getCandlesCommand struct{ reg *exchange.Registry }

func (c *getCandlesCommand) Name() string { return NameGetCandles }
func (c *getCandlesCommand) Description() string {
	return "Get OHLCV candles for a symbol. Interval options: 1m, 5m, 15m, 1h, 4h, 1d. Optional exchange parameter to target a specific exchange."
}
func (c *getCandlesCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "interval": map[string]any{"type": "string", "default": "1h"}, "limit": map[string]any{"type": "integer", "default": 24}, "exchange": map[string]any{"type": "string", "description": "Target exchange (paper, coinbase, binance). Defaults to paper."}}, "required": []string{"symbol"}}
}

func (c *getCandlesCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p GetCandlesParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if p.Interval == "" {
		p.Interval = "1h"
	}
	if p.Limit == 0 {
		p.Limit = 24
	}
	ex := resolveExchange(c.reg, p.Exchange)
	candles, err := ex.GetCandles(ctx, p.Symbol, shared.CandleInterval(p.Interval), p.Limit)
	if err != nil {
		return "", err
	}
	return jsonString(candles)
}

type placeOrderCommand struct{ reg *exchange.Registry }

func (c *placeOrderCommand) Name() string { return NamePlaceOrder }
func (c *placeOrderCommand) Description() string {
	return "Place a market or limit order. Side: buy/sell. Type: market/limit. For market orders, omit price. Optional exchange parameter to target a specific exchange. Returns order ID, status, and fill details."
}
func (c *placeOrderCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "side": map[string]any{"type": "string", "enum": []string{"buy", "sell"}}, "type": map[string]any{"type": "string", "enum": []string{"market", "limit"}, "default": "market"}, "quantity": map[string]any{"type": "number"}, "price": map[string]any{"type": "number"}, "exchange": map[string]any{"type": "string", "description": "Target exchange (paper, coinbase, binance). Defaults to paper."}}, "required": []string{"symbol", "side", "quantity"}}
}

func (c *placeOrderCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p PlaceOrderParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	ex := resolveExchange(c.reg, p.Exchange)
	order, err := ex.PlaceOrder(ctx, shared.OrderRequest{
		Symbol:   p.Symbol,
		Side:     shared.OrderSide(p.Side),
		Type:     shared.OrderType(p.Type),
		Quantity: p.Quantity,
		Price:    p.Price,
	})
	if err != nil {
		return "", err
	}
	return jsonString(order)
}

type cancelOrderCommand struct{ reg *exchange.Registry }

func (c *cancelOrderCommand) Name() string { return NameCancelOrder }
func (c *cancelOrderCommand) Description() string {
	return "Cancel an open order by ID. Use list_bots or get_bot_status to find order IDs. Optional exchange parameter to target a specific exchange. Returns success or error if order not found."
}
func (c *cancelOrderCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"order_id": map[string]any{"type": "string"}, "exchange": map[string]any{"type": "string", "description": "Target exchange (paper, coinbase, binance). Defaults to paper."}}, "required": []string{"order_id"}}
}

func (c *cancelOrderCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p CancelOrderParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	ex := resolveExchange(c.reg, p.Exchange)
	if err := ex.CancelOrder(ctx, p.OrderID); err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"cancelled": true, "order_id": "%s"}`, p.OrderID), nil
}

type getPositionsCommand struct{ reg *exchange.Registry }

func (c *getPositionsCommand) Name() string { return NameGetPositions }
func (c *getPositionsCommand) Description() string {
	return "Get all current positions with quantity, average entry price, unrealized and realized P&L. Optional exchange parameter to target a specific exchange."
}
func (c *getPositionsCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"exchange": map[string]any{"type": "string", "description": "Target exchange (paper, coinbase, binance). Defaults to paper."}}}
}

func (c *getPositionsCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p GetPositionsParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	ex := resolveExchange(c.reg, p.Exchange)
	positions, err := ex.ListPositions(ctx)
	if err != nil {
		return "", err
	}
	return jsonString(positions)
}

type getBalancesCommand struct{ reg *exchange.Registry }

func (c *getBalancesCommand) Name() string { return NameGetBalances }
func (c *getBalancesCommand) Description() string {
	return "Get account balances for all assets. Returns free and total balance per asset. USD is the quote currency. Optional exchange parameter to target a specific exchange."
}
func (c *getBalancesCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"exchange": map[string]any{"type": "string", "description": "Target exchange (paper, coinbase, binance). Defaults to paper."}}}
}

func (c *getBalancesCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p GetBalancesParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	ex := resolveExchange(c.reg, p.Exchange)
	balances, err := ex.ListBalances(ctx)
	if err != nil {
		return "", err
	}
	return jsonString(balances)
}

type startBotCommand struct {
	sup *trading.Supervisor
}

func (c *startBotCommand) Name() string { return NameStartBot }
func (c *startBotCommand) Description() string {
	return "Start a trading bot. Provide strategy_file to load from YAML, or provide type + symbol + params inline. Supported types: dca, grid, signal. The bot runs in the daemon and places orders automatically. Use list_bots to monitor."
}
func (c *startBotCommand) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"strategy_file": map[string]any{"type": "string", "description": "Path to YAML strategy file (omit for inline params)"},
			"id":            map[string]any{"type": "string", "description": "Optional bot ID (auto-generated if omitted)"},
			"name":          map[string]any{"type": "string", "description": "Optional bot name"},
			"exchange":      map[string]any{"type": "string", "description": "Exchange to trade on (default: paper)"},
			"type":          map[string]any{"type": "string", "description": "Strategy type: dca, grid, signal (required if no strategy_file)"},
			"symbol":        map[string]any{"type": "string", "description": "Trading symbol, e.g. BTCUSDT or BTC-USD (required if no strategy_file)"},
			"params":        map[string]any{"type": "object", "description": "Strategy-specific parameters"},
		},
	}
}

func (c *startBotCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p StartBotParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}

	var cfg config.BotConfig
	var strat trading.Strategy

	if p.StrategyFile != "" {
		loaded, err := config.LoadStrategyFile(p.StrategyFile, nil)
		if err != nil {
			return "", fmt.Errorf("load strategy: %w", err)
		}
		cfg = *loaded
	} else {
		if p.Type == "" || p.Symbol == "" {
			return "", fmt.Errorf("either strategy_file or type + symbol are required")
		}
		if p.Params == nil {
			p.Params = make(map[string]interface{})
		}
		built, err := strategy.Build(p.Type, p.Symbol, p.Params)
		if err != nil {
			return "", fmt.Errorf("build strategy: %w", err)
		}
		strat = built
		exch := shared.ExchangeProvider(p.Exchange)
		if p.Exchange == "" {
			exch = shared.ProviderPaper
		}
		cfg = config.BotConfig{
			ID:       p.ID,
			Name:     p.Name,
			Exchange: exch,
			Strategy: config.StrategyConfig{
				Type:   p.Type,
				Symbol: p.Symbol,
				Params: p.Params,
			},
		}
	}

	botID := cfg.ID
	if botID == "" {
		botID = fmt.Sprintf("bot-%d", time.Now().Unix())
	}
	cfg.ID = botID

	if err := c.sup.StartBot(ctx, botID, cfg, strat); err != nil {
		return "", fmt.Errorf("start bot: %w", err)
	}

	return fmt.Sprintf(`{"started": true, "bot_id": "%s", "strategy": "%s", "symbol": "%s"}`, botID, cfg.Strategy.Type, cfg.Strategy.Symbol), nil
}

type stopBotCommand struct{ sup *trading.Supervisor }

func (c *stopBotCommand) Name() string { return NameStopBot }
func (c *stopBotCommand) Description() string {
	return "Stop a running trading bot by ID. Cancels all open orders. Bot state is persisted and can be inspected with status command."
}
func (c *stopBotCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"bot_id": map[string]any{"type": "string"}}, "required": []string{"bot_id"}}
}

func (c *stopBotCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p StopBotParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if err := c.sup.StopBot(p.BotID); err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"stopped": true, "bot_id": "%s"}`, p.BotID), nil
}

type listBotsCommand struct{ sup *trading.Supervisor }

func (c *listBotsCommand) Name() string { return NameListBots }
func (c *listBotsCommand) Description() string {
	return "List all active trading bots with strategy, symbol, status, and error info. Use get_bot_status for detailed P&L and open orders."
}
func (c *listBotsCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (c *listBotsCommand) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return jsonString(c.sup.ListBots())
}

type addMarketCommand struct {
	reg *exchange.Registry
	sup *trading.Supervisor
}

func (c *addMarketCommand) Name() string { return NameAddMarket }
func (c *addMarketCommand) Description() string {
	return "Add a new trading symbol with a simulated random-walk price feed. The symbol is auto-registered in the price streamer so bots can trade it immediately."
}
func (c *addMarketCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "initial_price": map[string]any{"type": "number", "default": 50000}, "drift": map[string]any{"type": "number", "default": 0.1}, "volatility": map[string]any{"type": "number", "default": 0.3}, "liquidity_levels": map[string]any{"type": "integer", "default": 10}, "liquidity_depth": map[string]any{"type": "number", "default": 100}}, "required": []string{"symbol"}}
}

func (c *addMarketCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p AddMarketParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}

	pex, ok := c.reg.Default().(shared.MarketLifecycleManager)
	if !ok {
		return "", fmt.Errorf("exchange does not support AddMarket")
	}

	if p.InitialPrice == 0 {
		p.InitialPrice = shared.DefaultBasePrice
	}
	if p.Drift == 0 {
		p.Drift = shared.DefaultRandomWalkDrift
	}
	if p.Volatility == 0 {
		p.Volatility = shared.DefaultRandomWalkVolatility
	}
	if p.LiquidityLevels == 0 {
		p.LiquidityLevels = shared.DefaultLiquidityLevels
	}
	if p.LiquidityDepth == 0 {
		p.LiquidityDepth = shared.DefaultLiquidityDepth
	}

	pex.AddMarket(p.Symbol, paper.NewRandomWalkFeed(p.Symbol, p.InitialPrice, p.Drift, p.Volatility, shared.DefaultTickInterval))
	pex.SeedLiquidity(p.Symbol, p.LiquidityLevels, p.LiquidityDepth)
	pex.StartFeeds(ctx)

	if streamer := c.sup.Streamer(); streamer != nil {
		if err := streamer.Register(ctx, p.Symbol, shared.DefaultTickInterval); err != nil {
			return "", fmt.Errorf("register symbol in streamer: %w", err)
		}
	}

	return fmt.Sprintf(`{"added": true, "symbol": "%s", "price": %.2f}`, p.Symbol, p.InitialPrice), nil
}

type getBotStatusCommand struct{ sup *trading.Supervisor }

func (c *getBotStatusCommand) Name() string { return NameGetBotStatus }
func (c *getBotStatusCommand) Description() string {
	return "Get detailed status, strategy config, P&L, and open orders for a specific bot. Use list_bots to discover bot IDs first."
}
func (c *getBotStatusCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"bot_id": map[string]any{"type": "string"}}, "required": []string{"bot_id"}}
}

func (c *getBotStatusCommand) Execute(_ context.Context, rawArgs json.RawMessage) (string, error) {
	var p GetBotStatusParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	bots := c.sup.ListBots()
	status, ok := bots[p.BotID]
	if !ok {
		return "", fmt.Errorf("bot %s not found", p.BotID)
	}
	return jsonString(status)
}

type setCredentialCommand struct {
	store *credentials.SQLiteStore
	key   *[32]byte
}

func (c *setCredentialCommand) Name() string { return "set_credential" }
func (c *setCredentialCommand) Description() string {
	return "Store encrypted API credentials for an exchange. Exchange: coinbase, binance. Label is a free identifier. Requires GREEDY_MASTER_PASSWORD at server start."
}
func (c *setCredentialCommand) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"exchange":   map[string]any{"type": "string"},
			"label":      map[string]any{"type": "string"},
			"api_key":    map[string]any{"type": "string"},
			"api_secret": map[string]any{"type": "string"},
			"passphrase": map[string]any{"type": "string"},
		},
		"required": []string{"exchange", "label", "api_key", "api_secret"},
	}
}

func (c *setCredentialCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p SetCredentialParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	cred := credentials.Credential{
		Exchange:   shared.ExchangeProvider(p.Exchange),
		Label:      p.Label,
		APIKey:     p.APIKey,
		APISecret:  p.APISecret,
		Passphrase: p.Passphrase,
	}
	if err := c.store.Set(ctx, cred, c.key); err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"stored": true, "exchange": "%s", "label": "%s"}`, p.Exchange, p.Label), nil
}

type listCredentialsCommand struct {
	store *credentials.SQLiteStore
}

func (c *listCredentialsCommand) Name() string { return "list_credentials" }
func (c *listCredentialsCommand) Description() string {
	return "List all stored credential identifiers (exchange + label). API keys are never exposed."
}
func (c *listCredentialsCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (c *listCredentialsCommand) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	metas, err := c.store.List(ctx)
	if err != nil {
		return "", err
	}
	if metas == nil {
		metas = []credentials.Meta{}
	}
	return jsonString(metas)
}

type deleteCredentialCommand struct {
	store *credentials.SQLiteStore
}

func (c *deleteCredentialCommand) Name() string { return "delete_credential" }
func (c *deleteCredentialCommand) Description() string {
	return "Delete a stored credential by exchange and label."
}
func (c *deleteCredentialCommand) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"exchange": map[string]any{"type": "string"},
			"label":    map[string]any{"type": "string"},
		},
		"required": []string{"exchange", "label"},
	}
}

func (c *deleteCredentialCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p DeleteCredentialParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if err := c.store.Delete(ctx, shared.ExchangeProvider(p.Exchange), p.Label); err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"deleted": true, "exchange": "%s", "label": "%s"}`, p.Exchange, p.Label), nil
}

type getOrderHistoryCommand struct {
	sup *trading.Supervisor
}

func (c *getOrderHistoryCommand) Name() string { return NameGetOrderHistory }
func (c *getOrderHistoryCommand) Description() string {
	return "Get historical orders with optional filters by bot_id or symbol. Returns order ID, side, type, price, quantity, filled quantity, and status."
}
func (c *getOrderHistoryCommand) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"bot_id": map[string]any{"type": "string", "description": "Filter by bot ID (optional)"},
			"symbol": map[string]any{"type": "string", "description": "Filter by symbol (optional)"},
			"limit":  map[string]any{"type": "integer", "default": 50, "description": "Max orders to return"},
		},
	}
}

func (c *getOrderHistoryCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p GetOrderHistoryParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if p.Limit <= 0 {
		p.Limit = 50
	}
	orders, err := c.sup.GetOrderHistory(p.BotID, p.Symbol, p.Limit)
	if err != nil {
		return "", fmt.Errorf("get order history: %w", err)
	}
	if orders == nil {
		orders = []shared.Order{}
	}
	return jsonString(orders)
}
