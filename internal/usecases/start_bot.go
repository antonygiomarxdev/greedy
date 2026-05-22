package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/bot/strategy"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
)

type StartBotUseCase struct {
	supervisor *bot.Supervisor
	registry   *strategy.Registry
}

func NewStartBotUseCase(sup *bot.Supervisor, reg *strategy.Registry) *StartBotUseCase {
	return &StartBotUseCase{supervisor: sup, registry: reg}
}

func (uc *StartBotUseCase) Execute(ctx context.Context, cfg *config.BotConfig) (string, error) {
	strat, err := uc.registry.Build(cfg.Strategy.Type, cfg.Strategy.Symbol, cfg.Strategy.Params)
	if err != nil {
		return "", fmt.Errorf("build strategy: %w", err)
	}
	botID := cfg.ID
	if botID == "" {
		botID = fmt.Sprintf("bot-%d", time.Now().Unix())
	}

	if err := uc.supervisor.StartBot(ctx, botID, *cfg, strat); err != nil {
		return "", fmt.Errorf("start bot: %w", err)
	}

	return botID, nil
}
