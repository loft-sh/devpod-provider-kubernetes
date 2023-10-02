package throttled_logger

import (
	"time"

	"github.com/loft-sh/log"
)

type LoggingFunc func(string, ...interface{})

// ThrottledLogger is a logger that throttles the output,
// i.e. it only logs a message if a certain amount of time has passed since the last log message
type ThrottledLogger struct {
	logger log.Logger
	timer  *Timer
}

func NewThrottledLogger(logger log.Logger, throttlingInterval time.Duration) *ThrottledLogger {
	return &ThrottledLogger{
		logger: logger,
		timer:  NewTimer(throttlingInterval),
	}
}

func (t *ThrottledLogger) Infof(format string, args ...interface{}) {
	t.log(t.logger.Infof, format, args...)
}

func (t *ThrottledLogger) Debugf(format string, args ...interface{}) {
	t.log(t.logger.Debugf, format, args...)
}

func (t *ThrottledLogger) log(loggingFunc LoggingFunc, format string, args ...interface{}) {
	now := time.Now()
	if t.timer.IntervalPassed(now) {
		loggingFunc(format, args...)
		t.timer.Tick(now)
	}
}

type Timer struct {
	nextMessage  time.Time
	tickInterval time.Duration
}

func NewTimer(tickInterval time.Duration) *Timer {
	return &Timer{
		nextMessage:  time.Now().Add(tickInterval),
		tickInterval: tickInterval,
	}
}

func (t *Timer) Tick(now time.Time) {
	t.nextMessage = now.Add(t.tickInterval)
}

func (t *Timer) IntervalPassed(now time.Time) bool {
	return now.After(t.nextMessage)
}
