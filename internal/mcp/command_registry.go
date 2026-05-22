package mcp

import (
	"github.com/antonygiomarxdev/greedy/internal/domain/tool"
	"github.com/antonygiomarxdev/greedy/internal/shared"
	"github.com/antonygiomarxdev/greedy/internal/trading"
)

type commandFactory func(ex shared.Exchange, sup *trading.Supervisor) tool.Command

var commandFactories []commandFactory

func RegisterCommandFactory(fn commandFactory) {
	commandFactories = append(commandFactories, fn)
}
