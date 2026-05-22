package mcp

import (
	"context"
	"encoding/json"
)

const (
	NameGetTicker    = "get_ticker"
	NameGetOrderBook = "get_order_book"
	NameGetCandles   = "get_candles"
	NamePlaceOrder   = "place_order"
	NameCancelOrder  = "cancel_order"
	NameGetPositions = "get_positions"
	NameGetBalances  = "get_balances"
	NameStartBot     = "start_bot"
	NameStopBot      = "stop_bot"
	NameListBots     = "list_bots"
	NameAddMarket    = "add_market"
	NameGetBotStatus = "get_bot_status"
)

type Command interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	Execute(ctx context.Context, rawArgs json.RawMessage) (string, error)
}
