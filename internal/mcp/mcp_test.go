package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	exch "github.com/antonygiomarxdev/greedy/internal/exchange"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/paper"
	"github.com/antonygiomarxdev/greedy/internal/trading"
)

func setupServer(t *testing.T) (*Server, context.Context) {
	t.Helper()
	ex := paper.New(0.001)
	ex.AddMarket("BTC-USD", paper.NewStaticFeed("BTC-USD", 50000))
	ex.SeedLiquidity("BTC-USD", 10, 100)

	reg := exch.NewRegistry(ex)
	sup := trading.NewSupervisor(reg, nil, trading.RestartNever)
	srv := NewServer(reg, sup, nil, nil)
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

	required := []string{"add_market", "cancel_order", "get_balances", "get_bot_status", "get_candles", "get_order_book", "get_positions", "get_ticker", "list_bots", "place_order", "start_bot", "stop_bot"}
	for _, name := range required {
		if !names[name] {
			t.Fatalf("missing tool: %s", name)
		}
	}
}

func callTool(t *testing.T, srv *Server, ctx context.Context, name string, params any) (string, error) {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return srv.CallTool(ctx, name, raw)
}

func TestCallGetTicker(t *testing.T) {
	srv, ctx := setupServer(t)

	result, err := callTool(t, srv, ctx, "get_ticker", GetTickerParams{Symbol: "BTC-USD"})
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

	result, err := callTool(t, srv, ctx, "get_positions", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestCallGetBalances(t *testing.T) {
	srv, ctx := setupServer(t)

	result, err := callTool(t, srv, ctx, "get_balances", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "USD") {
		t.Fatal("expected USD balance")
	}
}

func TestCallListBotsEmpty(t *testing.T) {
	srv, ctx := setupServer(t)

	result, err := callTool(t, srv, ctx, "list_bots", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "{}") {
		t.Fatal("expected empty bots map")
	}
}

func TestCallUnknownTool(t *testing.T) {
	srv, ctx := setupServer(t)

	_, err := callTool(t, srv, ctx, "nonexistent", struct{}{})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestCallGetTickerMissingSymbol(t *testing.T) {
	srv, ctx := setupServer(t)

	_, err := srv.CallTool(ctx, "get_ticker", json.RawMessage(`{"symbol":""}`))
	if err == nil {
		t.Fatal("expected error for empty symbol")
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
