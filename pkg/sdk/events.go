package sdk

import (
	"fmt"
	"sync"
	"time"

	"memex/pkg/sdk/types"
)

// EventType represents different types of module events
type EventType string

const (
	// Module lifecycle events
	EventModuleLoaded   EventType = "module.loaded"
	EventModuleUnloaded EventType = "module.unloaded"

	// Command events
	EventCommandStarted   EventType = "command.started"
	EventCommandCompleted EventType = "command.completed"
	EventCommandError     EventType = "command.error"
)

// Event represents a module system event
type Event struct {
	Type      EventType              // Type of event
	Module    types.Module           // Module that triggered the event
	Command   string                 // Command being executed (for command events)
	Args      []string               // Command arguments (for command events)
	Error     error                  // Error if any (for error events)
	Timestamp time.Time              // When the event occurred
	Data      map[string]interface{} // Additional event data
}

// EventHandler is called when an event occurs
type EventHandler func(Event)

// EventEmitter handles event subscription and emission
type EventEmitter struct {
	handlers map[EventType][]EventHandler
	mu       sync.RWMutex
}

// NewEventEmitter creates a new event emitter
func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		handlers: make(map[EventType][]EventHandler),
	}
}

// Subscribe adds a handler for an event type
func (e *EventEmitter) Subscribe(eventType EventType, handler EventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.handlers[eventType] = append(e.handlers[eventType], handler)
}

// Unsubscribe removes a handler for an event type
func (e *EventEmitter) Unsubscribe(eventType EventType, handler EventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()

	handlers := e.handlers[eventType]
	newHandlers := make([]EventHandler, 0, len(handlers))

	// Find all handlers that don't match the one we're removing
	for _, h := range handlers {
		if fmt.Sprintf("%p", h) != fmt.Sprintf("%p", handler) {
			newHandlers = append(newHandlers, h)
		}
	}

	e.handlers[eventType] = newHandlers
}

// Emit sends an event to all subscribed handlers
func (e *EventEmitter) Emit(event Event) {
	e.mu.RLock()
	// Make a copy of handlers to avoid race conditions
	handlers := make([]EventHandler, len(e.handlers[event.Type]))
	copy(handlers, e.handlers[event.Type])
	e.mu.RUnlock()

	event.Timestamp = time.Now()

	for _, handler := range handlers {
		handler(event)
	}
}

// Helper methods for common events

func (e *EventEmitter) EmitModuleLoaded(mod types.Module) {
	e.Emit(Event{
		Type:   EventModuleLoaded,
		Module: mod,
	})
}

func (e *EventEmitter) EmitModuleUnloaded(mod types.Module) {
	e.Emit(Event{
		Type:   EventModuleUnloaded,
		Module: mod,
	})
}

func (e *EventEmitter) EmitCommandStarted(mod types.Module, cmd string, args []string) {
	e.Emit(Event{
		Type:    EventCommandStarted,
		Module:  mod,
		Command: cmd,
		Args:    args,
	})
}

func (e *EventEmitter) EmitCommandCompleted(mod types.Module, cmd string, args []string) {
	e.Emit(Event{
		Type:    EventCommandCompleted,
		Module:  mod,
		Command: cmd,
		Args:    args,
	})
}

func (e *EventEmitter) EmitCommandError(mod types.Module, cmd string, args []string, err error) {
	e.Emit(Event{
		Type:    EventCommandError,
		Module:  mod,
		Command: cmd,
		Args:    args,
		Error:   err,
	})
}
