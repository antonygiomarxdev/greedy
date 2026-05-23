package mcp

import (
	"github.com/antonygiomarxdev/greedy/internal/exchange"
	"github.com/antonygiomarxdev/greedy/internal/trading"
)

type commandFactory func(reg *exchange.Registry, sup *trading.Supervisor) Command

var commandFactories []commandFactory

func RegisterCommandFactory(fn commandFactory) {
	commandFactories = append(commandFactories, fn)
}
