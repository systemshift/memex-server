package test

import (
	"fmt"
	"testing"
	"time"

	"memex/pkg/sdk"
)

func TestEventEmitter(t *testing.T) {
	// Test basic subscription and emission
	t.Run("basic", func(t *testing.T) {
		emitter := sdk.NewEventEmitter()
		var received bool
		handler := func(e sdk.Event) {
			if e.Type != sdk.EventModuleLoaded {
				t.Errorf("wrong event type: got %v, want %v", e.Type, sdk.EventModuleLoaded)
			}
			received = true
		}

		emitter.Subscribe(sdk.EventModuleLoaded, handler)
		emitter.Emit(sdk.Event{Type: sdk.EventModuleLoaded})

		if !received {
			t.Error("handler was not called")
		}
	})

	// Test multiple handlers
	t.Run("multiple handlers", func(t *testing.T) {
		emitter := sdk.NewEventEmitter()
		count := 0
		handler1 := func(sdk.Event) { count++ }
		handler2 := func(sdk.Event) { count++ }

		emitter.Subscribe(sdk.EventModuleLoaded, handler1)
		emitter.Subscribe(sdk.EventModuleLoaded, handler2)
		emitter.Emit(sdk.Event{Type: sdk.EventModuleLoaded})

		if count != 2 { // Two handlers registered
			t.Errorf("wrong handler count: got %v, want 2", count)
		}
	})

	// Test unsubscribe
	t.Run("unsubscribe", func(t *testing.T) {
		emitter := sdk.NewEventEmitter()
		var called bool
		handler := func(sdk.Event) { called = true }

		emitter.Subscribe(sdk.EventModuleUnloaded, handler)
		emitter.Unsubscribe(sdk.EventModuleUnloaded, handler)
		emitter.Emit(sdk.Event{Type: sdk.EventModuleUnloaded})

		if called {
			t.Error("handler was called after unsubscribe")
		}
	})

	// Test event data
	t.Run("event data", func(t *testing.T) {
		emitter := sdk.NewEventEmitter()
		testMod := &mockModule{id: "test", name: "Test"}
		testCmd := "test"
		testArgs := []string{"arg1", "arg2"}
		testErr := fmt.Errorf("test error")

		var receivedEvent sdk.Event
		handler := func(e sdk.Event) {
			receivedEvent = e
		}

		// Test command error event
		emitter.Subscribe(sdk.EventCommandError, handler)
		emitter.EmitCommandError(testMod, testCmd, testArgs, testErr)

		if receivedEvent.Type != sdk.EventCommandError {
			t.Error("wrong event type")
		}
		if receivedEvent.Module != testMod {
			t.Error("wrong module")
		}
		if receivedEvent.Command != testCmd {
			t.Error("wrong command")
		}
		if len(receivedEvent.Args) != len(testArgs) {
			t.Error("wrong args")
		}
		if receivedEvent.Error != testErr {
			t.Error("wrong error")
		}
		if receivedEvent.Timestamp.IsZero() {
			t.Error("timestamp not set")
		}
	})

	// Test event timing
	t.Run("timing", func(t *testing.T) {
		emitter := sdk.NewEventEmitter()
		var timestamp time.Time
		handler := func(e sdk.Event) {
			timestamp = e.Timestamp
		}

		emitter.Subscribe(sdk.EventCommandStarted, handler)
		before := time.Now()
		emitter.EmitCommandStarted(nil, "", nil)
		after := time.Now()

		if timestamp.Before(before) || timestamp.After(after) {
			t.Error("event timestamp outside expected range")
		}
	})
}

func TestManagerEvents(t *testing.T) {
	mgr := sdk.NewManager()
	mod := &mockModule{id: "test", name: "Test"}

	var events []sdk.EventType
	handler := func(e sdk.Event) {
		events = append(events, e.Type)
	}

	// Subscribe to all event types
	mgr.Events().Subscribe(sdk.EventModuleLoaded, handler)
	mgr.Events().Subscribe(sdk.EventCommandStarted, handler)
	mgr.Events().Subscribe(sdk.EventCommandCompleted, handler)
	mgr.Events().Subscribe(sdk.EventCommandError, handler)

	// Test module registration events
	if err := mgr.RegisterModule(mod); err != nil {
		t.Errorf("RegisterModule() error = %v", err)
	}

	// Test command events
	mgr.HandleCommand("test", "test", nil)

	// Verify event sequence
	expected := []sdk.EventType{
		sdk.EventModuleLoaded,
		sdk.EventCommandStarted,
		sdk.EventCommandCompleted,
	}

	if len(events) != len(expected) {
		t.Errorf("got %d events, want %d", len(events), len(expected))
	} else {
		for i, want := range expected {
			if events[i] != want {
				t.Errorf("event[%d] = %v, want %v", i, events[i], want)
			}
		}
	}
}

func TestLoaderEvents(t *testing.T) {
	mgr := sdk.NewManager()
	loader := sdk.NewModuleLoader(mgr)
	mod := &mockModule{id: "test", name: "Test"}

	var events []sdk.EventType
	handler := func(e sdk.Event) {
		events = append(events, e.Type)
	}

	// Subscribe to all event types
	loader.Events().Subscribe(sdk.EventModuleLoaded, handler)
	loader.Events().Subscribe(sdk.EventModuleUnloaded, handler)

	// Test load/unload events
	if err := loader.LoadModule("test", mod); err != nil {
		t.Errorf("LoadModule() error = %v", err)
	}

	if err := loader.UnloadModule("test"); err != nil {
		t.Errorf("UnloadModule() error = %v", err)
	}

	// Verify event sequence
	expected := []sdk.EventType{
		sdk.EventModuleLoaded,
		sdk.EventModuleUnloaded,
	}

	if len(events) != len(expected) {
		t.Errorf("got %d events, want %d", len(events), len(expected))
	} else {
		for i, want := range expected {
			if events[i] != want {
				t.Errorf("event[%d] = %v, want %v", i, events[i], want)
			}
		}
	}
}
