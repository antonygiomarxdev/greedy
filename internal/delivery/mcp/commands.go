package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/bot/strategy"
	dexchange "github.com/antonygiomarxdev/greedy/internal/domain/exchange"
	"github.com/antonygiomarxdev/greedy/internal/domain/tool"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/exchange/paper"
)

func init() {
	RegisterCommandFactory(func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command {
		return &getTickerCommand{ex: ex}
	})
	RegisterCommandFactory(func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command {
		return &getOrderBookCommand{ex: ex}
	})
	RegisterCommandFactory(func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command {
		return &getCandlesCommand{ex: ex}
	})
	RegisterCommandFactory(func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command {
		return &placeOrderCommand{ex: ex}
	})
	RegisterCommandFactory(func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command {
		return &cancelOrderCommand{ex: ex}
	})
	RegisterCommandFactory(func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command {
		return &getPositionsCommand{ex: ex}
	})
	RegisterCommandFactory(func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command {
		return &getBalancesCommand{ex: ex}
	})
	RegisterCommandFactory(func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command {
		return &startBotCommand{sup: sup}
	})
	RegisterCommandFactory(func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command {
		return &stopBotCommand{sup: sup}
	})
	RegisterCommandFactory(func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command {
		return &listBotsCommand{sup: sup}
	})
	RegisterCommandFactory(func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command {
		return &addMarketCommand{ex: ex}
	})
	RegisterCommandFactory(func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command {
		return &getBotStatusCommand{sup: sup}
	})
}

type getTickerCommand struct{ ex dexchange.Exchange }

func (c *getTickerCommand) Name() string        { return tool.NameGetTicker }
func (c *getTickerCommand) Description() string { return "Get current price for a trading symbol" }
func (c *getTickerCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}}, "required": []string{"symbol"}}
}

func (c *getTickerCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p GetTickerParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	ticker, err := c.ex.GetTicker(ctx, p.Symbol)
	if err != nil {
		return "", err
	}
	return jsonString(ticker)
}

type getOrderBookCommand struct{ ex dexchange.Exchange }

func (c *getOrderBookCommand) Name() string        { return tool.NameGetOrderBook }
func (c *getOrderBookCommand) Description() string { return "Get current order book for a symbol" }
func (c *getOrderBookCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "depth": map[string]any{"type": "integer", "default": 10}}, "required": []string{"symbol"}}
}

func (c *getOrderBookCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p GetOrderBookParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if p.Depth == 0 {
		p.Depth = 10
	}
	book, err := c.ex.GetOrderBook(ctx, p.Symbol, p.Depth)
	if err != nil {
		return "", err
	}
	return jsonString(book)
}

type getCandlesCommand struct{ ex dexchange.Exchange }

func (c *getCandlesCommand) Name() string        { return tool.NameGetCandles }
func (c *getCandlesCommand) Description() string { return "Get OHLCV candles for a symbol" }
func (c *getCandlesCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "interval": map[string]any{"type": "string", "default": "1h"}, "limit": map[string]any{"type": "integer", "default": 24}}, "required": []string{"symbol"}}
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
	candles, err := c.ex.GetCandles(ctx, p.Symbol, dexchange.CandleInterval(p.Interval), p.Limit)
	if err != nil {
		return "", err
	}
	return jsonString(candles)
}

type placeOrderCommand struct{ ex dexchange.Exchange }

func (c *placeOrderCommand) Name() string { return tool.NamePlaceOrder }
func (c *placeOrderCommand) Description() string {
	return "Place a market or limit order on the exchange"
}
func (c *placeOrderCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "side": map[string]any{"type": "string", "enum": []string{"buy", "sell"}}, "type": map[string]any{"type": "string", "enum": []string{"market", "limit"}, "default": "market"}, "quantity": map[string]any{"type": "number"}, "price": map[string]any{"type": "number"}}, "required": []string{"symbol", "side", "quantity"}}
}

func (c *placeOrderCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p PlaceOrderParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	order, err := c.ex.PlaceOrder(ctx, dexchange.OrderRequest{
		Symbol:   p.Symbol,
		Side:     dexchange.OrderSide(p.Side),
		Type:     dexchange.OrderType(p.Type),
		Quantity: p.Quantity,
		Price:    p.Price,
	})
	if err != nil {
		return "", err
	}
	return jsonString(order)
}

type cancelOrderCommand struct{ ex dexchange.Exchange }

func (c *cancelOrderCommand) Name() string        { return tool.NameCancelOrder }
func (c *cancelOrderCommand) Description() string { return "Cancel an open order by ID" }
func (c *cancelOrderCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"order_id": map[string]any{"type": "string"}}, "required": []string{"order_id"}}
}

func (c *cancelOrderCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p CancelOrderParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if err := c.ex.CancelOrder(ctx, p.OrderID); err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"cancelled": true, "order_id": "%s"}`, p.OrderID), nil
}

type getPositionsCommand struct{ ex dexchange.Exchange }

func (c *getPositionsCommand) Name() string        { return tool.NameGetPositions }
func (c *getPositionsCommand) Description() string { return "Get all current positions with P&L" }
func (c *getPositionsCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (c *getPositionsCommand) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	positions, err := c.ex.ListPositions(ctx)
	if err != nil {
		return "", err
	}
	return jsonString(positions)
}

type getBalancesCommand struct{ ex dexchange.Exchange }

func (c *getBalancesCommand) Name() string        { return tool.NameGetBalances }
func (c *getBalancesCommand) Description() string { return "Get account balances" }
func (c *getBalancesCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (c *getBalancesCommand) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	balances, err := c.ex.ListBalances(ctx)
	if err != nil {
		return "", err
	}
	return jsonString(balances)
}

type startBotCommand struct {
	sup *bot.Supervisor
}

func (c *startBotCommand) Name() string { return tool.NameStartBot }
func (c *startBotCommand) Description() string {
	return "Start a trading bot from a YAML strategy file"
}
func (c *startBotCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"strategy_file": map[string]any{"type": "string"}}, "required": []string{"strategy_file"}}
}

func (c *startBotCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p StartBotParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}

	cfg, err := config.LoadStrategyFile(p.StrategyFile, nil)
	if err != nil {
		return "", fmt.Errorf("load strategy: %w", err)
	}

	strat, err := strategy.Build(cfg.Strategy.Type, cfg.Strategy.Symbol, cfg.Strategy.Params)
	if err != nil {
		return "", fmt.Errorf("build strategy: %w", err)
	}

	botID := cfg.ID
	if botID == "" {
		botID = fmt.Sprintf("bot-%d", time.Now().Unix())
	}

	if err := c.sup.StartBot(ctx, botID, *cfg, strat); err != nil {
		return "", fmt.Errorf("start bot: %w", err)
	}

	return fmt.Sprintf(`{"started": true, "bot_id": "%s", "strategy": "%s", "symbol": "%s"}`, botID, cfg.Strategy.Type, cfg.Strategy.Symbol), nil
}

type stopBotCommand struct{ sup *bot.Supervisor }

func (c *stopBotCommand) Name() string        { return tool.NameStopBot }
func (c *stopBotCommand) Description() string { return "Stop a running trading bot" }
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

type listBotsCommand struct{ sup *bot.Supervisor }

func (c *listBotsCommand) Name() string { return tool.NameListBots }
func (c *listBotsCommand) Description() string {
	return "List all active trading bots with status and P&L"
}
func (c *listBotsCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (c *listBotsCommand) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return jsonString(c.sup.ListBots())
}

type addMarketCommand struct{ ex dexchange.Exchange }

func (c *addMarketCommand) Name() string { return tool.NameAddMarket }
func (c *addMarketCommand) Description() string {
	return "Add a new market/symbol with a simulated price feed"
}
func (c *addMarketCommand) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "initial_price": map[string]any{"type": "number", "default": 50000}, "drift": map[string]any{"type": "number", "default": 0.1}, "volatility": map[string]any{"type": "number", "default": 0.3}, "liquidity_levels": map[string]any{"type": "integer", "default": 10}, "liquidity_depth": map[string]any{"type": "number", "default": 100}}, "required": []string{"symbol"}}
}

func (c *addMarketCommand) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var p AddMarketParams
	if err := json.Unmarshal(rawArgs, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}

	pex, ok := c.ex.(dexchange.MarketLifecycleManager)
	if !ok {
		return "", fmt.Errorf("exchange does not support AddMarket")
	}

	if p.InitialPrice == 0 {
		p.InitialPrice = dexchange.DefaultBasePrice
	}
	if p.Drift == 0 {
		p.Drift = dexchange.DefaultRandomWalkDrift
	}
	if p.Volatility == 0 {
		p.Volatility = dexchange.DefaultRandomWalkVolatility
	}
	if p.LiquidityLevels == 0 {
		p.LiquidityLevels = dexchange.DefaultLiquidityLevels
	}
	if p.LiquidityDepth == 0 {
		p.LiquidityDepth = dexchange.DefaultLiquidityDepth
	}

	pex.AddMarket(p.Symbol, paper.NewRandomWalkFeed(p.Symbol, p.InitialPrice, p.Drift, p.Volatility, dexchange.DefaultTickInterval))
	pex.SeedLiquidity(p.Symbol, p.LiquidityLevels, p.LiquidityDepth)
	pex.StartFeeds(ctx)

	return fmt.Sprintf(`{"added": true, "symbol": "%s", "price": %.2f}`, p.Symbol, p.InitialPrice), nil
}

type getBotStatusCommand struct{ sup *bot.Supervisor }

func (c *getBotStatusCommand) Name() string { return tool.NameGetBotStatus }
func (c *getBotStatusCommand) Description() string {
	return "Get detailed status, P&L, and configuration of a running bot"
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
