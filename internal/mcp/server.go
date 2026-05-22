package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"

	"github.com/antonygiomarxdev/greedy/internal/shared"
	"github.com/antonygiomarxdev/greedy/internal/trading"
)

type Server struct {
	exchange    shared.Exchange
	supervisor  *trading.Supervisor
	db          *sql.DB
	logger      *slog.Logger
	commands    map[string]Command
	rpcHandlers map[string]rpcHandlerFunc
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func NewServer(ex shared.Exchange, sup *trading.Supervisor, database *sql.DB) *Server {
	s := &Server{
		exchange:    ex,
		supervisor:  sup,
		db:          database,
		logger:      slog.Default().With("component", "mcp"),
		commands:    make(map[string]Command),
		rpcHandlers: make(map[string]rpcHandlerFunc),
	}

	for _, factory := range commandFactories {
		cmd := factory(ex, sup)
		s.commands[cmd.Name()] = cmd
	}
	s.registerRPCHandlers()

	return s
}

func (s *Server) RegisterCommand(cmd Command) {
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

func (s *Server) ListResources() []ResourceDef {
	var out []ResourceDef
	for _, p := range resourceProviders {
		out = append(out, p.Resources()...)
	}
	return out
}

func (s *Server) ListPrompts() []PromptDef {
	var out []PromptDef
	for _, p := range promptProviders {
		out = append(out, p.Prompts()...)
	}
	return out
}

func (s *Server) ReadResource(uri string) (string, error) {
	switch uri {
	case "portfolio://summary":
		return s.buildPortfolioSummary()
	default:
		return "", fmt.Errorf("resource not found: %s", uri)
	}
}

func (s *Server) GetPrompt(name string, args map[string]string) ([]promptMessage, error) {
	return []promptMessage{
		{Role: "user", Content: fmt.Sprintf("Please execute the %s prompt with arguments %v", name, args)},
	}, nil
}

func (s *Server) buildPortfolioSummary() (string, error) {
	ctx := context.Background()
	positions, err := s.exchange.ListPositions(ctx)
	if err != nil {
		return "", fmt.Errorf("list positions: %w", err)
	}
	balances, err := s.exchange.ListBalances(ctx)
	if err != nil {
		return "", fmt.Errorf("list balances: %w", err)
	}

	summary := map[string]any{
		"balances":  balances,
		"positions": positions,
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return "", fmt.Errorf("marshal summary: %w", err)
	}
	return string(data), nil
}
