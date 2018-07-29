package consensus

import (
	"time"

	"github.com/kardiachain/go-kardia/lib/log"
)

var (
	tickTockBufferSize = 10
)

// TimeoutTicker is a timer that schedules timeouts
// conditional on the height/round/step in the timeoutInfo.
// The timeoutInfo.Duration may be non-positive.
type TimeoutTicker interface {
	Start() error
	Stop() error
	Chan() <-chan timeoutInfo       // on which to receive a timeout
	ScheduleTimeout(ti timeoutInfo) // reset the timer

	SetLogger(log.Logger)
}

// timeoutTicker wraps time.Timer,
// scheduling timeouts only for greater height/round/step
// than what it's already seen.
// Timeouts are scheduled along the tickChan,
// and fired on the tockChan.
type timeoutTicker struct {
	timer    *time.Timer
	tickChan chan timeoutInfo // for scheduling timeouts
	tockChan chan timeoutInfo // for notifying about them

	Logger log.Logger
}

// NewTimeoutTicker returns a new TimeoutTicker.
func NewTimeoutTicker() TimeoutTicker {
	tt := &timeoutTicker{
		timer:    time.NewTimer(0),
		tickChan: make(chan timeoutInfo, tickTockBufferSize),
		tockChan: make(chan timeoutInfo, tickTockBufferSize),
	}
	tt.stopTimer() // don't want to fire until the first scheduled timeout
	return tt
}

// Starts the timeout routine.
func (t *timeoutTicker) Start() error {
	panic("ticker.Start - Not yet implemented.")
	//go t.timeoutRoutine()
	//
	//return nil
}

// Stops the timeout routine.
func (t *timeoutTicker) Stop() error {
	panic("ticker.Stop - Not yet implemented.")
	//t.stopTimer()
	//return nil
}

// ScheduleTimeout schedules a new timeout by sending on the internal tickChan.
// The timeoutRoutine is always available to read from tickChan, so this won't block.
// The scheduling may fail if the timeoutRoutine has already scheduled a timeout for a later height/round/step.
func (t *timeoutTicker) ScheduleTimeout(ti timeoutInfo) {
	panic("ticker.ScheduleTimeout - Not yet implemented.")
	//t.tickChan <- ti
}

// Chan returns a channel on which timeouts are sent.
func (t *timeoutTicker) Chan() <-chan timeoutInfo {
	panic("ticker.Chan - Not yet implemented.")
	//return t.tockChan
}

// Sets a logger.
func (t *timeoutTicker) SetLogger(l log.Logger) {
	t.Logger = l
}

// stop the timer and drain if necessary
func (t *timeoutTicker) stopTimer() {
	// Stop() returns false if it was already fired or was stopped
	if !t.timer.Stop() {
		select {
		case <-t.timer.C:
		default:
			t.Logger.Debug("Timer already stopped")
		}
	}
}
