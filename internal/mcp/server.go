package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/antonygiomarxdev/greedy/internal/credentials"
	"github.com/antonygiomarxdev/greedy/internal/exchange"
	"github.com/antonygiomarxdev/greedy/internal/shared"
	"github.com/antonygiomarxdev/greedy/internal/trading"
)

type Server struct {
	reg         *exchange.Registry
	supervisor  *trading.Supervisor
	db          *sql.DB
	masterKey   *[32]byte
	credStore   *credentials.SQLiteStore
	logger      *slog.Logger
	commands    map[string]Command
	rpcHandlers map[string]rpcHandlerFunc
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func NewServer(reg *exchange.Registry, sup *trading.Supervisor, database *sql.DB, masterKey *[32]byte) *Server {
	s := &Server{
		reg:         reg,
		supervisor:  sup,
		db:          database,
		masterKey:   masterKey,
		logger:      slog.Default().With("component", "mcp"),
		commands:    make(map[string]Command),
		rpcHandlers: make(map[string]rpcHandlerFunc),
	}

	for _, factory := range commandFactories {
		cmd := factory(reg, sup)
		s.commands[cmd.Name()] = cmd
	}
	if masterKey != nil && database != nil {
		s.credStore = credentials.NewSQLiteStore(database)
		s.commands["set_credential"] = &setCredentialCommand{store: s.credStore, key: masterKey}
		s.commands["list_credentials"] = &listCredentialsCommand{store: s.credStore}
		s.commands["delete_credential"] = &deleteCredentialCommand{store: s.credStore}
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
	if uri == "portfolio://summary" {
		return s.buildPortfolioSummary()
	}
	if strings.HasPrefix(uri, "bot://") && strings.HasSuffix(uri, "/history") {
		id := strings.TrimPrefix(uri, "bot://")
		id = strings.TrimSuffix(id, "/history")
		return s.buildBotHistory(id)
	}
	if strings.HasPrefix(uri, "bot://") && strings.HasSuffix(uri, "/status") {
		id := strings.TrimPrefix(uri, "bot://")
		id = strings.TrimSuffix(id, "/status")
		return s.buildBotStatusResource(id)
	}
	return "", fmt.Errorf("resource not found: %s", uri)
}

func (s *Server) GetPrompt(name string, args map[string]string) ([]promptMessage, error) {
	return []promptMessage{
		{Role: "user", Content: fmt.Sprintf("Please execute the %s prompt with arguments %v", name, args)},
	}, nil
}

func (s *Server) buildPortfolioSummary() (string, error) {
	ctx := context.Background()
	ex := s.reg.Default()
	positions, err := ex.ListPositions(ctx)
	if err != nil {
		return "", fmt.Errorf("list positions: %w", err)
	}
	balances, err := ex.ListBalances(ctx)
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

func (s *Server) buildBotHistory(botID string) (string, error) {
	orders, err := s.supervisor.GetOrderHistory(botID, "", 100)
	if err != nil {
		return "", fmt.Errorf("get order history: %w", err)
	}
	if orders == nil {
		orders = []shared.Order{}
	}
	data, err := json.Marshal(orders)
	if err != nil {
		return "", fmt.Errorf("marshal history: %w", err)
	}
	return string(data), nil
}

func (s *Server) buildBotStatusResource(botID string) (string, error) {
	bots := s.supervisor.ListBots()
	status, ok := bots[botID]
	if !ok {
		return "", fmt.Errorf("bot %s not found", botID)
	}
	data, err := json.Marshal(status)
	if err != nil {
		return "", fmt.Errorf("marshal bot status: %w", err)
	}
	return string(data), nil
}
