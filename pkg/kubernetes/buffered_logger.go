package kubernetes

import (
	"fmt"
	"io"
	"sync"

	"github.com/loft-sh/log"
	"github.com/loft-sh/log/survey"
	"github.com/sirupsen/logrus"
)

type bufferLogger struct {
	mu sync.Mutex

	logger log.Logger
	msgs   []logMsg
}

type logMsg struct {
	args   []interface{}
	format string
	level  logrus.Level
}

func NewBufferLogger(logger log.Logger) *bufferLogger {
	return &bufferLogger{
		logger: logger,
		msgs:   []logMsg{},
	}
}

var _ log.Logger = &bufferLogger{}

// Flush prints all of the buffered messages through the wrapped logger
func (b *bufferLogger) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, msg := range b.msgs {
		var content string
		if msg.format != "" {
			content = fmt.Sprintf(msg.format, msg.args...)
		} else {
			content = fmt.Sprint(msg.args...)
		}
		b.logger.WriteString(msg.level, content+"\n")
	}
	b.msgs = b.msgs[:0]
}

func (b *bufferLogger) Debug(args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.msgs = append(b.msgs, logMsg{level: logrus.DebugLevel, args: args})
}

func (b *bufferLogger) Debugf(format string, args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.msgs = append(b.msgs, logMsg{level: logrus.DebugLevel, args: args, format: format})
}

func (b *bufferLogger) Done(args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.msgs = append(b.msgs, logMsg{level: logrus.InfoLevel, args: args})
}

func (b *bufferLogger) Donef(format string, args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, logMsg{level: logrus.InfoLevel, args: args, format: format})
}

func (b *bufferLogger) Error(args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, logMsg{level: logrus.ErrorLevel, args: args})
}

func (b *bufferLogger) ErrorStreamOnly() log.Logger {
	return b
}

func (b *bufferLogger) Errorf(format string, args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, logMsg{level: logrus.ErrorLevel, args: args, format: format})
}

func (b *bufferLogger) Fatal(args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.logger.Fatal(args...)
}

func (b *bufferLogger) Fatalf(format string, args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.logger.Fatalf(format, args...)
}

func (b *bufferLogger) GetLevel() logrus.Level {
	return b.logger.GetLevel()
}

func (b *bufferLogger) Info(args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, logMsg{level: logrus.InfoLevel, args: args})
}

func (b *bufferLogger) Infof(format string, args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, logMsg{level: logrus.InfoLevel, args: args, format: format})
}

func (b *bufferLogger) Print(level logrus.Level, args ...interface{}) {
	b.logger.Print(level, args...)
}

func (b *bufferLogger) Printf(level logrus.Level, format string, args ...interface{}) {
	b.logger.Printf(level, format, args...)
}

func (b *bufferLogger) Question(params *survey.QuestionOptions) (string, error) {
	return b.logger.Question(params)
}

func (b *bufferLogger) SetLevel(level logrus.Level) {
	b.logger.SetLevel(level)
}

func (b *bufferLogger) Warn(args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, logMsg{level: logrus.WarnLevel, args: args})
}

func (b *bufferLogger) Warnf(format string, args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, logMsg{level: logrus.WarnLevel, args: args, format: format})
}

func (b *bufferLogger) WriteString(level logrus.Level, message string) {
	b.logger.WriteString(level, message)
}

func (b *bufferLogger) Writer(level logrus.Level, raw bool) io.WriteCloser {
	return b.logger.Writer(level, raw)
}
