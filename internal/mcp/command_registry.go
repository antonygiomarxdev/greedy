package mcp

import (
	dexchange "github.com/antonygiomarxdev/greedy/internal/domain/exchange"
	"github.com/antonygiomarxdev/greedy/internal/domain/tool"
	"github.com/antonygiomarxdev/greedy/internal/trading"
)

type commandFactory func(ex dexchange.Exchange, sup *trading.Supervisor) tool.Command

var commandFactories []commandFactory

func RegisterCommandFactory(fn commandFactory) {
	commandFactories = append(commandFactories, fn)
}
