package mcp

import (
	"github.com/antonygiomarxdev/greedy/internal/bot"
	dexchange "github.com/antonygiomarxdev/greedy/internal/domain/exchange"
	"github.com/antonygiomarxdev/greedy/internal/domain/tool"
)

type commandFactory func(ex dexchange.Exchange, sup *bot.Supervisor) tool.Command

var commandFactories []commandFactory

func RegisterCommandFactory(fn commandFactory) {
	commandFactories = append(commandFactories, fn)
}
