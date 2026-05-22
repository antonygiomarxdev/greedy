package debouncer

type Debouncer interface {
	CanExecute() bool
	RecordExecution()
	Reset()
}
