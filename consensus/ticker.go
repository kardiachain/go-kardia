package consensus

import (
	"github.com/kardiachain/go-kardia/log"
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

// NewTimeoutTicker returns a new TimeoutTicker.
func NewTimeoutTicker() TimeoutTicker {
	panic("Not yet implemented.")
	return nil
	//tt := &timeoutTicker{
	//	timer:    time.NewTimer(0),
	//	tickChan: make(chan timeoutInfo, tickTockBufferSize),
	//	tockChan: make(chan timeoutInfo, tickTockBufferSize),
	//}
	//tt.BaseService = *cmn.NewBaseService(nil, "TimeoutTicker", tt)
	//tt.stopTimer() // don't want to fire until the first scheduled timeout
	//return tt
}
