package test

import (
	"testing"

	"memex/internal/memex/logger"
)

// TestLogger implements a logger for testing
type TestLogger struct {
	t *testing.T
}

// NewTestLogger creates a new test logger and sets it as the default logger
func NewTestLogger(t *testing.T) *TestLogger {
	l := &TestLogger{t: t}
	logger.SetLogger(l)
	return l
}

// Log logs a message with formatting
func (l *TestLogger) Log(format string, args ...interface{}) {
	l.t.Helper()
	l.t.Logf("TEST LOG: "+format, args...)
}
