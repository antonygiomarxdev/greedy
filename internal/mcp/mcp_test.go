package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/exchange/paper"
)

func setupServer(t *testing.T) (*Server, context.Context) {
	t.Helper()
	ex := paper.New(0.001)
	ex.AddMarket("BTC-USD", paper.NewStaticFeed("BTC-USD", 50000))
	ex.SeedLiquidity("BTC-USD", 10, 100)

	sup := bot.NewSupervisor(ex, nil, bot.RestartNever)
	srv := NewServer(ex, sup, nil)
	ctx := context.Background()

	return srv, ctx
}

func TestListTools(t *testing.T) {
	srv, _ := setupServer(t)
	tools := srv.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected tools to be registered")
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}

	required := []string{"get_ticker", "get_order_book", "get_candles", "place_order",
		"cancel_order", "get_positions", "get_balances", "start_bot", "stop_bot", "list_bots"}
	for _, name := range required {
		if !names[name] {
			t.Fatalf("missing tool: %s", name)
		}
	}
}

func TestCallGetTicker(t *testing.T) {
	srv, ctx := setupServer(t)

	result, err := srv.CallTool(ctx, "get_ticker", map[string]interface{}{"symbol": "BTC-USD"})
	if err != nil {
		t.Fatal(err)
	}

	var ticker map[string]interface{}
	if err := json.Unmarshal([]byte(result), &ticker); err != nil {
		t.Fatal(err)
	}
	if ticker["symbol"] != "BTC-USD" {
		t.Fatal("expected BTC-USD")
	}
}

func TestCallGetPositions(t *testing.T) {
	srv, ctx := setupServer(t)

	result, err := srv.CallTool(ctx, "get_positions", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestCallGetBalances(t *testing.T) {
	srv, ctx := setupServer(t)

	result, err := srv.CallTool(ctx, "get_balances", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "USD") {
		t.Fatal("expected USD balance")
	}
}

func TestCallListBotsEmpty(t *testing.T) {
	srv, ctx := setupServer(t)

	result, err := srv.CallTool(ctx, "list_bots", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "{}") {
		t.Fatal("expected empty bots map")
	}
}

func TestCallUnknownTool(t *testing.T) {
	srv, ctx := setupServer(t)

	_, err := srv.CallTool(ctx, "nonexistent", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestCallGetTickerMissingSymbol(t *testing.T) {
	srv, ctx := setupServer(t)

	_, err := srv.CallTool(ctx, "get_ticker", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing symbol")
	}
}

func TestListResources(t *testing.T) {
	srv, _ := setupServer(t)

	resources := srv.ListResources()
	if len(resources) == 0 {
		t.Fatal("expected resources")
	}
}

func TestListPrompts(t *testing.T) {
	srv, _ := setupServer(t)

	prompts := srv.ListPrompts()
	if len(prompts) == 0 {
		t.Fatal("expected prompts")
	}
}

func TestInitializeHandshake(t *testing.T) {
	srv, ctx := setupServer(t)

	req := &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}
	resp, err := srv.handleInitialize(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	result, ok := resp.Result.(initResult)
	if !ok {
		t.Fatal("expected initResult")
	}
	if result.ServerInfo.Name != "greedy-trader" {
		t.Fatal("expected greedy-trader server name")
	}
}
