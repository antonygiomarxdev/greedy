package debouncer

import (
	"sync"
	"time"
)

var _ Debouncer = (*BotDebouncer)(nil)

type BotDebouncer struct {
	cooldown    time.Duration
	burstLimit  int
	burstWindow time.Duration
	lastExec    time.Time
	executions  []time.Time
	mu          sync.Mutex
}

func New(cooldown time.Duration, burstLimit int, burstWindow time.Duration) *BotDebouncer {
	return &BotDebouncer{
		cooldown:    cooldown,
		burstLimit:  burstLimit,
		burstWindow: burstWindow,
	}
}

func (d *BotDebouncer) CanExecute() bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if time.Since(d.lastExec) < d.cooldown {
		return false
	}

	cutoff := time.Now().Add(-d.burstWindow)
	recent := 0
	for _, ts := range d.executions {
		if ts.After(cutoff) {
			recent++
		}
	}
	return recent < d.burstLimit
}

func (d *BotDebouncer) RecordExecution() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	d.lastExec = now
	d.executions = append(d.executions, now)

	cutoff := now.Add(-d.burstWindow)
	cleaned := d.executions[:0]
	for _, ts := range d.executions {
		if ts.After(cutoff) {
			cleaned = append(cleaned, ts)
		}
	}
	d.executions = cleaned
}

func (d *BotDebouncer) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.lastExec = time.Time{}
	d.executions = nil
}
