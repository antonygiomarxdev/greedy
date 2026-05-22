package shared

type BotConfig interface {
	ID() string
	Name() string
	StrategyType() string
	Symbol() string
	DataDir() string
	Validate() error
}
