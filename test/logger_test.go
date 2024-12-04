package test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"memex/internal/memex/logger"
)

// MockLogger implements logger.Logger for testing
type MockLogger struct {
	messages []string
	mu       sync.Mutex
}

func (l *MockLogger) Log(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, fmt.Sprintf(format, args...))
}

func (l *MockLogger) GetMessages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.messages...)
}

func TestLogger(t *testing.T) {
	t.Run("NoopLogger", func(t *testing.T) {
		// NoopLogger should not panic
		noop := &logger.NoopLogger{}
		noop.Log("test message")
		noop.Log("formatted %s %d", "message", 42)
	})

	t.Run("Default Logger", func(t *testing.T) {
		// Save original default logger
		original := logger.DefaultLogger
		defer func() {
			logger.SetLogger(original)
		}()

		mock := &MockLogger{}
		logger.SetLogger(mock)

		// Test simple message
		logger.Log("test message")
		messages := mock.GetMessages()
		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}
		if messages[0] != "test message" {
			t.Errorf("wrong message: got %q, want %q", messages[0], "test message")
		}

		// Test formatted message
		logger.Log("number: %d, string: %s", 42, "test")
		messages = mock.GetMessages()
		if len(messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(messages))
		}
		expected := "number: 42, string: test"
		if messages[1] != expected {
			t.Errorf("wrong formatted message: got %q, want %q", messages[1], expected)
		}
	})

	t.Run("Concurrent Logging", func(t *testing.T) {
		mock := &MockLogger{}
		logger.SetLogger(mock)
		defer logger.SetLogger(&logger.NoopLogger{})

		// Log messages concurrently
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				logger.Log("message %d", n)
			}(i)
		}
		wg.Wait()

		// Verify all messages were logged
		messages := mock.GetMessages()
		if len(messages) != 100 {
			t.Errorf("expected 100 messages, got %d", len(messages))
		}

		// Verify all numbers were logged
		logged := make(map[int]bool)
		for _, msg := range messages {
			var n int
			if _, err := fmt.Sscanf(msg, "message %d", &n); err != nil {
				t.Errorf("invalid message format: %q", msg)
				continue
			}
			logged[n] = true
		}
		for i := 0; i < 100; i++ {
			if !logged[i] {
				t.Errorf("missing message for number %d", i)
			}
		}
	})

	t.Run("Format Strings", func(t *testing.T) {
		mock := &MockLogger{}
		logger.SetLogger(mock)
		defer logger.SetLogger(&logger.NoopLogger{})

		testCases := []struct {
			format   string
			args     []interface{}
			expected string
		}{
			{
				format:   "string: %s",
				args:     []interface{}{"test"},
				expected: "string: test",
			},
			{
				format:   "number: %d",
				args:     []interface{}{42},
				expected: "number: 42",
			},
			{
				format:   "float: %.2f",
				args:     []interface{}{3.14159},
				expected: "float: 3.14",
			},
			{
				format:   "multiple: %s, %d, %.1f",
				args:     []interface{}{"test", 42, 3.14},
				expected: "multiple: test, 42, 3.1",
			},
			{
				format:   "escaped %%",
				args:     nil,
				expected: "escaped %",
			},
		}

		for _, tc := range testCases {
			logger.Log(tc.format, tc.args...)
		}

		messages := mock.GetMessages()
		if len(messages) != len(testCases) {
			t.Fatalf("expected %d messages, got %d", len(testCases), len(messages))
		}

		for i, tc := range testCases {
			if messages[i] != tc.expected {
				t.Errorf("case %d: got %q, want %q", i, messages[i], tc.expected)
			}
		}
	})

	t.Run("Empty Format String", func(t *testing.T) {
		mock := &MockLogger{}
		logger.SetLogger(mock)
		defer logger.SetLogger(&logger.NoopLogger{})

		// Empty format string should work
		logger.Log("")
		messages := mock.GetMessages()
		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}
		if messages[0] != "" {
			t.Errorf("expected empty message, got %q", messages[0])
		}
	})

	t.Run("Long Messages", func(t *testing.T) {
		mock := &MockLogger{}
		logger.SetLogger(mock)
		defer logger.SetLogger(&logger.NoopLogger{})

		// Create a long message
		longStr := strings.Repeat("x", 1000)
		logger.Log("long message: %s", longStr)

		messages := mock.GetMessages()
		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}

		expected := "long message: " + longStr
		if messages[0] != expected {
			t.Errorf("long message mismatch:\ngot: %q\nwant: %q", messages[0], expected)
		}
	})
}
