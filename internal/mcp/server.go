package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/config"
	"github.com/antonygiomarxdev/greedy/internal/exchange"
)

type Server struct {
	exchange   exchange.Exchange
	supervisor *bot.Supervisor
	db         *sql.DB
	logger     *slog.Logger
}

func NewServer(ex exchange.Exchange, sup *bot.Supervisor, database *sql.DB) *Server {
	return &Server{
		exchange:   ex,
		supervisor: sup,
		db:         database,
		logger:     slog.Default().With("component", "mcp"),
	}
}

// Tool definitions and handlers

type ToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

func (s *Server) ListTools() []ToolDef {
	return []ToolDef{
		{
			Name:        "get_ticker",
			Description: "Get current price for a trading symbol",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol": map[string]interface{}{
						"type":        "string",
						"description": "Trading pair symbol, e.g. BTC-USD",
					},
				},
				"required": []string{"symbol"},
			},
		},
		{
			Name:        "get_order_book",
			Description: "Get current order book for a symbol",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol": map[string]interface{}{"type": "string"},
					"depth":  map[string]interface{}{"type": "integer", "default": 10},
				},
				"required": []string{"symbol"},
			},
		},
		{
			Name:        "get_candles",
			Description: "Get OHLCV candles for a symbol",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol":   map[string]interface{}{"type": "string"},
					"interval": map[string]interface{}{"type": "string", "default": "1h"},
					"limit":    map[string]interface{}{"type": "integer", "default": 24},
				},
				"required": []string{"symbol"},
			},
		},
		{
			Name:        "place_order",
			Description: "Place a market or limit order on the exchange",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol":   map[string]interface{}{"type": "string"},
					"side":     map[string]interface{}{"type": "string", "enum": []string{"buy", "sell"}},
					"type":     map[string]interface{}{"type": "string", "enum": []string{"market", "limit"}, "default": "market"},
					"quantity": map[string]interface{}{"type": "number"},
					"price":    map[string]interface{}{"type": "number", "description": "Required for limit orders"},
				},
				"required": []string{"symbol", "side", "quantity"},
			},
		},
		{
			Name:        "cancel_order",
			Description: "Cancel an open order by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"order_id": map[string]interface{}{"type": "string"},
				},
				"required": []string{"order_id"},
			},
		},
		{
			Name:        "get_positions",
			Description: "Get all current positions with P&L",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "get_balances",
			Description: "Get account balances",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "start_bot",
			Description: "Start a trading bot from a YAML strategy file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"strategy_file": map[string]interface{}{"type": "string", "description": "Path to YAML strategy file"},
				},
				"required": []string{"strategy_file"},
			},
		},
		{
			Name:        "stop_bot",
			Description: "Stop a running trading bot",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"bot_id": map[string]interface{}{"type": "string"},
				},
				"required": []string{"bot_id"},
			},
		},
		{
			Name:        "list_bots",
			Description: "List all active trading bots with status and P&L",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

func (s *Server) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	switch name {
	case "get_ticker":
		return s.handleGetTicker(ctx, args)
	case "get_order_book":
		return s.handleGetOrderBook(ctx, args)
	case "get_candles":
		return s.handleGetCandles(ctx, args)
	case "place_order":
		return s.handlePlaceOrder(ctx, args)
	case "cancel_order":
		return s.handleCancelOrder(ctx, args)
	case "get_positions":
		return s.handleGetPositions(ctx, args)
	case "get_balances":
		return s.handleGetBalances(ctx, args)
	case "start_bot":
		return s.handleStartBot(ctx, args)
	case "stop_bot":
		return s.handleStopBot(ctx, args)
	case "list_bots":
		return s.handleListBots(ctx, args)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) handleGetTicker(ctx context.Context, args map[string]interface{}) (string, error) {
	symbol := getString(args, "symbol")
	ticker, err := s.exchange.GetTicker(ctx, symbol)
	if err != nil {
		return "", err
	}
	return toJSON(ticker)
}

func (s *Server) handleGetOrderBook(ctx context.Context, args map[string]interface{}) (string, error) {
	symbol := getString(args, "symbol")
	depth := getInt(args, "depth", 10)
	book, err := s.exchange.GetOrderBook(ctx, symbol, depth)
	if err != nil {
		return "", err
	}
	return toJSON(book)
}

func (s *Server) handleGetCandles(ctx context.Context, args map[string]interface{}) (string, error) {
	symbol := getString(args, "symbol")
	interval := getStringDefault(args, "interval", "1h")
	limit := getInt(args, "limit", 24)
	candles, err := s.exchange.GetCandles(ctx, symbol, exchange.CandleInterval(interval), limit)
	if err != nil {
		return "", err
	}
	return toJSON(candles)
}

func (s *Server) handlePlaceOrder(ctx context.Context, args map[string]interface{}) (string, error) {
	symbol := getString(args, "symbol")
	side := exchange.OrderSide(getString(args, "side"))
	orderType := exchange.OrderType(getStringDefault(args, "type", "market"))
	quantity := getFloat(args, "quantity")
	price := getFloatDefault(args, "price", 0)

	order, err := s.exchange.PlaceOrder(ctx, exchange.OrderRequest{
		Symbol:   symbol,
		Side:     side,
		Type:     orderType,
		Quantity: quantity,
		Price:    price,
	})
	if err != nil {
		return "", err
	}
	return toJSON(order)
}

func (s *Server) handleCancelOrder(ctx context.Context, args map[string]interface{}) (string, error) {
	orderID := getString(args, "order_id")
	if err := s.exchange.CancelOrder(ctx, orderID); err != nil {
		return "", err
	}
	return `{"cancelled": true, "order_id": "` + orderID + `"}`, nil
}

func (s *Server) handleGetPositions(ctx context.Context, args map[string]interface{}) (string, error) {
	positions, err := s.exchange.ListPositions(ctx)
	if err != nil {
		return "", err
	}
	return toJSON(positions)
}

func (s *Server) handleGetBalances(ctx context.Context, args map[string]interface{}) (string, error) {
	balances, err := s.exchange.ListBalances(ctx)
	if err != nil {
		return "", err
	}
	return toJSON(balances)
}

func (s *Server) handleStartBot(ctx context.Context, args map[string]interface{}) (string, error) {
	stratFile := getString(args, "strategy_file")

	cfg, err := config.LoadStrategyFile(stratFile)
	if err != nil {
		return "", fmt.Errorf("load strategy: %w", err)
	}

	// Build strategy
	var strat bot.Strategy
	switch cfg.Strategy.Type {
	case "dca":
		dcaCfg := config.DefaultDCAConfig()
		dcaCfg.Symbol = cfg.Strategy.Symbol
		if v, ok := config.ParseFloatParam(cfg.Strategy.Params, "base_order_size"); ok {
			dcaCfg.BaseOrderSize = v
		}
		if v, ok := config.ParseDurationParam(cfg.Strategy.Params, "frequency"); ok {
			dcaCfg.Frequency = v
		}
		strat = newDCAStrategy(dcaCfg)
	default:
		return "", fmt.Errorf("unknown strategy: %s", cfg.Strategy.Type)
	}

	botID := cfg.ID
	if botID == "" {
		botID = fmt.Sprintf("bot-%d", time.Now().Unix())
	}

	if err := s.supervisor.StartBot(ctx, botID, *cfg, strat); err != nil {
		return "", fmt.Errorf("start bot: %w", err)
	}

	return fmt.Sprintf(`{"started": true, "bot_id": "%s", "strategy": "%s"}`, botID, cfg.Strategy.Type), nil
}

func (s *Server) handleStopBot(ctx context.Context, args map[string]interface{}) (string, error) {
	botID := getString(args, "bot_id")
	if err := s.supervisor.StopBot(botID); err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"stopped": true, "bot_id": "%s"}`, botID), nil
}

func (s *Server) handleListBots(ctx context.Context, args map[string]interface{}) (string, error) {
	bots := s.supervisor.ListBots()
	return toJSON(bots)
}

func toJSON(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func getString(args map[string]interface{}, key string) string {
	v, _ := args[key].(string)
	return v
}

func getStringDefault(args map[string]interface{}, key, def string) string {
	v, ok := args[key].(string)
	if !ok || v == "" {
		return def
	}
	return v
}

func getInt(args map[string]interface{}, key string, def int) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return def
	}
}

func getFloat(args map[string]interface{}, key string) float64 {
	switch v := args[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}

func getFloatDefault(args map[string]interface{}, key string, def float64) float64 {
	v, ok := args[key]
	if !ok {
		return def
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	default:
		return def
	}
}
