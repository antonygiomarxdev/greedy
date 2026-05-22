package mcp

import (
	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/bot/strategy"
	"github.com/antonygiomarxdev/greedy/internal/config"
)

func newDCAStrategy(cfg config.DCAConfig) bot.Strategy {
	return strategy.NewDCA(cfg)
}
