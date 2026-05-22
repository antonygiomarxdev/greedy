package bot

import "context"

type BotManager interface {
	StartBot(ctx context.Context, id string, cfg BotConfig, strat Strategy) error
	StopBot(id string) error
	PauseBot(id string) error
	ResumeBot(id string) error
	ListBots() map[string]BotStatus
	Shutdown()
}

type BotRunner interface {
	Status() Status
	Error() error
	Run(ctx context.Context)
}
