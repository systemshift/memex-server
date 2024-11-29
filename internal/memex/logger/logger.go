package logger

// Logger defines the interface for logging
type Logger interface {
	Log(format string, args ...interface{})
}

// NoopLogger implements a no-op logger
type NoopLogger struct{}

func (l *NoopLogger) Log(format string, args ...interface{}) {
	// Do nothing
}

// DefaultLogger is the default logger instance
var DefaultLogger Logger = &NoopLogger{}

// SetLogger sets the default logger
func SetLogger(l Logger) {
	DefaultLogger = l
}

// Log logs a message using the default logger
func Log(format string, args ...interface{}) {
	DefaultLogger.Log(format, args...)
}
