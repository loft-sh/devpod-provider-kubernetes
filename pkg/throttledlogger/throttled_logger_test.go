package throttledlogger

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/loft-sh/log"
)

type MockLogger struct {
	log.Logger
	Messages []string
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.Messages = append(m.Messages, fmt.Sprintf(format, args...))
}

func TestThrottledLogger(t *testing.T) {
	now := time.Now()
	interval := time.Millisecond * 50
	timer := &Timer{
		nextMessage:  now.Add(interval),
		tickInterval: interval,
	}

	mockLogger := &MockLogger{}

	tLogger := &ThrottledLogger{
		logger: mockLogger, // Note: Assuming that the type of log.Logger is compatible.
		timer:  timer,
	}

	// Test: When timer indicates that interval hasn't passed
	tLogger.Infof("This is a test %s", "message")
	assert.Len(t, mockLogger.Messages, 0) // No log should be recorded

	// Test: When timer indicates that interval has passed
	time.Sleep(interval + time.Millisecond*1)
	tLogger.Infof("This is another test %s", "message")
	assert.Len(t, mockLogger.Messages, 1)
	assert.Equal(t, "This is another test message", mockLogger.Messages[0])
}
