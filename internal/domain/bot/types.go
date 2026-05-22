package bot

type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusPaused   Status = "paused"
	StatusError    Status = "error"
	StatusStopping Status = "stopping"
)

type RestartPolicy int

const (
	RestartNever RestartPolicy = iota
	RestartAlways
	RestartOnError
)

type BotStatus struct {
	ID       string
	Name     string
	Strategy string
	Symbol   string
	Status   Status
	Error    error
}

type BotConfig interface {
	ID() string
	Name() string
	StrategyType() string
	Symbol() string
	DataDir() string
	Validate() error
}
