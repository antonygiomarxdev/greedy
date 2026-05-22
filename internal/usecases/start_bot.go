package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
)

type StartBotUseCase struct {
	supervisor *bot.Supervisor
}

func NewStartBotUseCase(sup *bot.Supervisor) *StartBotUseCase {
	return &StartBotUseCase{supervisor: sup}
}

func (uc *StartBotUseCase) Execute(ctx context.Context, cfg *config.BotConfig) (string, error) {
	strat := BuildStrategy(cfg)
	botID := cfg.ID
	if botID == "" {
		botID = fmt.Sprintf("bot-%d", time.Now().Unix())
	}

	if err := uc.supervisor.StartBot(ctx, botID, *cfg, strat); err != nil {
		return "", fmt.Errorf("start bot: %w", err)
	}

	return botID, nil
}
