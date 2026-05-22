package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	dexchange "github.com/antonygiomarxdev/greedy/internal/domain/exchange"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/exchange/paper"
	"github.com/antonygiomarxdev/greedy/internal/usecases"
)

type Server struct {
	exchange   dexchange.Exchange
	supervisor *bot.Supervisor
	db         *sql.DB
	logger     *slog.Logger
}

func NewServer(ex dexchange.Exchange, sup *bot.Supervisor, database *sql.DB) *Server {
	return &Server{
		exchange:   ex,
		supervisor: sup,
		db:         database,
		logger:     slog.Default().With("component", "mcp"),
	}
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func (s *Server) ListTools() []ToolDef {
	return []ToolDef{
		{
			Name:        "get_ticker",
			Description: "Get current price for a trading symbol",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}}, "required": []string{"symbol"}},
		},
		{
			Name:        "get_order_book",
			Description: "Get current order book for a symbol",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "depth": map[string]any{"type": "integer", "default": 10}}, "required": []string{"symbol"}},
		},
		{
			Name:        "get_candles",
			Description: "Get OHLCV candles for a symbol",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "interval": map[string]any{"type": "string", "default": "1h"}, "limit": map[string]any{"type": "integer", "default": 24}}, "required": []string{"symbol"}},
		},
		{
			Name:        "place_order",
			Description: "Place a market or limit order on the exchange",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "side": map[string]any{"type": "string", "enum": []string{"buy", "sell"}}, "type": map[string]any{"type": "string", "enum": []string{"market", "limit"}, "default": "market"}, "quantity": map[string]any{"type": "number"}, "price": map[string]any{"type": "number"}}, "required": []string{"symbol", "side", "quantity"}},
		},
		{
			Name:        "cancel_order",
			Description: "Cancel an open order by ID",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"order_id": map[string]any{"type": "string"}}, "required": []string{"order_id"}},
		},
		{
			Name:        "get_positions",
			Description: "Get all current positions with P&L",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "get_balances",
			Description: "Get account balances",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "start_bot",
			Description: "Start a trading bot from a YAML strategy file",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"strategy_file": map[string]any{"type": "string"}}, "required": []string{"strategy_file"}},
		},
		{
			Name:        "stop_bot",
			Description: "Stop a running trading bot",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"bot_id": map[string]any{"type": "string"}}, "required": []string{"bot_id"}},
		},
		{
			Name:        "list_bots",
			Description: "List all active trading bots with status and P&L",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "add_market",
			Description: "Add a new market/symbol with a simulated price feed",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"symbol": map[string]any{"type": "string"}, "initial_price": map[string]any{"type": "number", "default": 50000}, "drift": map[string]any{"type": "number", "default": 0.1}, "volatility": map[string]any{"type": "number", "default": 0.3}, "liquidity_levels": map[string]any{"type": "integer", "default": 10}, "liquidity_depth": map[string]any{"type": "number", "default": 100}}, "required": []string{"symbol"}},
		},
		{
			Name:        "get_bot_status",
			Description: "Get detailed status, P&L, and configuration of a running bot",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"bot_id": map[string]any{"type": "string"}}, "required": []string{"bot_id"}},
		},
	}
}

func (s *Server) CallTool(ctx context.Context, name string, rawArgs json.RawMessage) (string, error) {
	switch name {
	case "get_ticker":
		var p GetTickerParams
		if err := json.Unmarshal(rawArgs, &p); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}
		return s.handleGetTicker(ctx, p)
	case "get_order_book":
		var p GetOrderBookParams
		if err := json.Unmarshal(rawArgs, &p); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}
		return s.handleGetOrderBook(ctx, p)
	case "get_candles":
		var p GetCandlesParams
		if err := json.Unmarshal(rawArgs, &p); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}
		return s.handleGetCandles(ctx, p)
	case "place_order":
		var p PlaceOrderParams
		if err := json.Unmarshal(rawArgs, &p); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}
		return s.handlePlaceOrder(ctx, p)
	case "cancel_order":
		var p CancelOrderParams
		if err := json.Unmarshal(rawArgs, &p); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}
		return s.handleCancelOrder(ctx, p)
	case "get_positions":
		return s.handleGetPositions(ctx)
	case "get_balances":
		return s.handleGetBalances(ctx)
	case "start_bot":
		var p StartBotParams
		if err := json.Unmarshal(rawArgs, &p); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}
		return s.handleStartBot(ctx, p)
	case "stop_bot":
		var p StopBotParams
		if err := json.Unmarshal(rawArgs, &p); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}
		return s.handleStopBot(ctx, p)
	case "list_bots":
		return s.handleListBots(ctx)
	case "add_market":
		var p AddMarketParams
		if err := json.Unmarshal(rawArgs, &p); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}
		return s.handleAddMarket(ctx, p)
	case "get_bot_status":
		var p GetBotStatusParams
		if err := json.Unmarshal(rawArgs, &p); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}
		return s.handleGetBotStatus(ctx, p)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) handleGetTicker(ctx context.Context, p GetTickerParams) (string, error) {
	ticker, err := s.exchange.GetTicker(ctx, p.Symbol)
	if err != nil {
		return "", err
	}
	return jsonString(ticker)
}

func (s *Server) handleGetOrderBook(ctx context.Context, p GetOrderBookParams) (string, error) {
	if p.Depth == 0 {
		p.Depth = 10
	}
	book, err := s.exchange.GetOrderBook(ctx, p.Symbol, p.Depth)
	if err != nil {
		return "", err
	}
	return jsonString(book)
}

func (s *Server) handleGetCandles(ctx context.Context, p GetCandlesParams) (string, error) {
	if p.Interval == "" {
		p.Interval = "1h"
	}
	if p.Limit == 0 {
		p.Limit = 24
	}
	candles, err := s.exchange.GetCandles(ctx, p.Symbol, dexchange.CandleInterval(p.Interval), p.Limit)
	if err != nil {
		return "", err
	}
	return jsonString(candles)
}

func (s *Server) handlePlaceOrder(ctx context.Context, p PlaceOrderParams) (string, error) {
	order, err := s.exchange.PlaceOrder(ctx, dexchange.OrderRequest{
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

func (s *Server) handleCancelOrder(ctx context.Context, p CancelOrderParams) (string, error) {
	if err := s.exchange.CancelOrder(ctx, p.OrderID); err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"cancelled": true, "order_id": "%s"}`, p.OrderID), nil
}

func (s *Server) handleGetPositions(_ context.Context) (string, error) {
	positions, err := s.exchange.ListPositions(context.Background())
	if err != nil {
		return "", err
	}
	return jsonString(positions)
}

func (s *Server) handleGetBalances(_ context.Context) (string, error) {
	balances, err := s.exchange.ListBalances(context.Background())
	if err != nil {
		return "", err
	}
	return jsonString(balances)
}

func (s *Server) handleStartBot(ctx context.Context, p StartBotParams) (string, error) {
	cfg, err := config.LoadStrategyFile(p.StrategyFile)
	if err != nil {
		return "", fmt.Errorf("load strategy: %w", err)
	}

	strat, err := usecases.BuildStrategy(cfg)
	if err != nil {
		return "", fmt.Errorf("build strategy: %w", err)
	}

	botID := cfg.ID
	if botID == "" {
		botID = fmt.Sprintf("bot-%d", time.Now().Unix())
	}

	if err := s.supervisor.StartBot(ctx, botID, *cfg, strat); err != nil {
		return "", fmt.Errorf("start bot: %w", err)
	}

	return fmt.Sprintf(`{"started": true, "bot_id": "%s", "strategy": "%s", "symbol": "%s"}`, botID, cfg.Strategy.Type, cfg.Strategy.Symbol), nil
}

func (s *Server) handleStopBot(ctx context.Context, p StopBotParams) (string, error) {
	if err := s.supervisor.StopBot(p.BotID); err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"stopped": true, "bot_id": "%s"}`, p.BotID), nil
}

func (s *Server) handleListBots(_ context.Context) (string, error) {
	bots := s.supervisor.ListBots()
	return jsonString(bots)
}

func (s *Server) handleAddMarket(ctx context.Context, p AddMarketParams) (string, error) {
	pex, ok := s.exchange.(interface {
		AddMarket(symbol string, feed interface{})
		SeedLiquidity(symbol string, levels int, depth float64)
		StartFeeds(ctx context.Context)
	})
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

func (s *Server) handleGetBotStatus(_ context.Context, p GetBotStatusParams) (string, error) {
	bots := s.supervisor.ListBots()
	status, ok := bots[p.BotID]
	if !ok {
		return "", fmt.Errorf("bot %s not found", p.BotID)
	}
	return jsonString(status)
}

func jsonString(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
