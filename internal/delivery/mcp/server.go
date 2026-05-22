package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/bot/strategy"
	dexchange "github.com/antonygiomarxdev/greedy/internal/domain/exchange"
	"github.com/antonygiomarxdev/greedy/internal/domain/tool"
)

type Server struct {
	exchange    dexchange.Exchange
	supervisor  *bot.Supervisor
	db          *sql.DB
	logger      *slog.Logger
	commands    map[string]tool.Command
	rpcHandlers map[string]rpcHandlerFunc
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func NewServer(ex dexchange.Exchange, sup *bot.Supervisor, database *sql.DB, stratReg *strategy.Registry) *Server {
	s := &Server{
		exchange:    ex,
		supervisor:  sup,
		db:          database,
		logger:      slog.Default().With("component", "mcp"),
		commands:    make(map[string]tool.Command),
		rpcHandlers: make(map[string]rpcHandlerFunc),
	}

	s.registerCommands(stratReg)
	s.registerRPCHandlers()

	return s
}

func (s *Server) registerCommands(stratReg *strategy.Registry) {
	s.RegisterCommand(&getTickerCommand{ex: s.exchange})
	s.RegisterCommand(&getOrderBookCommand{ex: s.exchange})
	s.RegisterCommand(&getCandlesCommand{ex: s.exchange})
	s.RegisterCommand(&placeOrderCommand{ex: s.exchange})
	s.RegisterCommand(&cancelOrderCommand{ex: s.exchange})
	s.RegisterCommand(&getPositionsCommand{ex: s.exchange})
	s.RegisterCommand(&getBalancesCommand{ex: s.exchange})
	s.RegisterCommand(&startBotCommand{sup: s.supervisor, strategyRegistry: stratReg})
	s.RegisterCommand(&stopBotCommand{sup: s.supervisor})
	s.RegisterCommand(&listBotsCommand{sup: s.supervisor})
	s.RegisterCommand(&addMarketCommand{ex: s.exchange})
	s.RegisterCommand(&getBotStatusCommand{sup: s.supervisor})
}

func (s *Server) RegisterCommand(cmd tool.Command) {
	s.commands[cmd.Name()] = cmd
}

func (s *Server) ListTools() []ToolDef {
	tools := make([]ToolDef, 0, len(s.commands))
	for _, cmd := range s.commands {
		tools = append(tools, ToolDef{
			Name:        cmd.Name(),
			Description: cmd.Description(),
			InputSchema: cmd.InputSchema(),
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
}

func (s *Server) CallTool(ctx context.Context, name string, rawArgs json.RawMessage) (string, error) {
	cmd, ok := s.commands[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return cmd.Execute(ctx, rawArgs)
}

func jsonString(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
