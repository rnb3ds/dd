package dd

import (
	"context"
	"io"
	"sync"
	"time"
)

// HookEvent represents the type of event that triggers a hook.
type HookEvent int

const (
	// HookBeforeLog is triggered before a log message is written.
	// Hooks can modify fields or abort logging by returning an error.
	HookBeforeLog HookEvent = iota

	// HookAfterLog is triggered after a log message is successfully written.
	HookAfterLog

	// HookOnFilter is triggered when sensitive data is filtered.
	HookOnFilter

	// HookOnRotate is triggered when a log file is rotated.
	HookOnRotate

	// HookOnClose is triggered when the logger is closed.
	HookOnClose

	// HookOnError is triggered when a write error occurs.
	HookOnError
)

// String returns the string representation of the hook event.
func (e HookEvent) String() string {
	switch e {
	case HookBeforeLog:
		return "BeforeLog"
	case HookAfterLog:
		return "AfterLog"
	case HookOnFilter:
		return "OnFilter"
	case HookOnRotate:
		return "OnRotate"
	case HookOnClose:
		return "OnClose"
	case HookOnError:
		return "OnError"
	default:
		return "Unknown"
	}
}

// HookContext provides contextual information for hook execution.
type HookContext struct {
	// Event is the type of hook event being triggered.
	Event HookEvent

	// Level is the log level for log-related events.
	Level LogLevel

	// Message is the log message (may be empty for non-log events).
	Message string

	// Fields are the structured fields attached to the log entry (after filtering).
	Fields []Field

	// OriginalFields are the fields before sensitive data filtering.
	// This allows hooks to access the original values if needed.
	OriginalFields []Field

	// Error contains any error that occurred (for OnError events).
	Error error

	// Timestamp is when the event occurred.
	Timestamp time.Time

	// Writer is the target writer (for write-related events).
	Writer io.Writer

	// Additional metadata can be stored here.
	Metadata map[string]any
}

// Hook is a function that is called during logging lifecycle events.
// If a BeforeLog hook returns an error, the log entry is not written.
// For other events, the error is logged but does not prevent the operation.
type Hook func(ctx context.Context, hookCtx *HookContext) error

// HookRegistry manages a collection of hooks organized by event type.
// It is thread-safe and supports dynamic hook registration.
type HookRegistry struct {
	mu    sync.RWMutex
	hooks map[HookEvent][]Hook
}

// NewHookRegistry creates a new empty hook registry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		hooks: make(map[HookEvent][]Hook),
	}
}

// Add registers a hook for a specific event type.
// If the hook is nil, it is ignored.
// Multiple hooks can be registered for the same event.
// Hooks are executed in the order they were added.
func (r *HookRegistry) Add(event HookEvent, hook Hook) {
	if hook == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks[event] = append(r.hooks[event], hook)
}

// Remove removes all hooks for a specific event type.
func (r *HookRegistry) Remove(event HookEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.hooks, event)
}

// Trigger executes all hooks registered for the given event.
// Hooks are executed in order; if any hook returns an error,
// execution stops and that error is returned.
// For BeforeLog events, an error prevents the log from being written.
func (r *HookRegistry) Trigger(ctx context.Context, event HookEvent, hookCtx *HookContext) error {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	hooks := r.hooks[event]
	r.mu.RUnlock()

	if len(hooks) == 0 {
		return nil
	}

	for _, hook := range hooks {
		if err := hook(ctx, hookCtx); err != nil {
			return err
		}
	}

	return nil
}

// Clone creates a copy of the registry with the same hooks.
// The hooks themselves are shared (functions are not copied).
func (r *HookRegistry) Clone() *HookRegistry {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	clone := &HookRegistry{
		hooks: make(map[HookEvent][]Hook, len(r.hooks)),
	}

	for event, hooks := range r.hooks {
		clone.hooks[event] = append([]Hook(nil), hooks...)
	}

	return clone
}

// Count returns the total number of registered hooks.
func (r *HookRegistry) Count() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, hooks := range r.hooks {
		count += len(hooks)
	}
	return count
}

// CountFor returns the number of hooks registered for a specific event.
func (r *HookRegistry) CountFor(event HookEvent) int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.hooks[event])
}

// Clear removes all registered hooks.
func (r *HookRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = make(map[HookEvent][]Hook)
}

// ClearFor removes all hooks for a specific event type.
func (r *HookRegistry) ClearFor(event HookEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.hooks, event)
}

// HookBuilder provides a fluent interface for building hook registries.
type HookBuilder struct {
	registry *HookRegistry
}

// NewHookBuilder creates a new HookBuilder with an empty registry.
func NewHookBuilder() *HookBuilder {
	return &HookBuilder{
		registry: NewHookRegistry(),
	}
}

// BeforeLog adds a hook for the BeforeLog event.
func (b *HookBuilder) BeforeLog(hook Hook) *HookBuilder {
	b.registry.Add(HookBeforeLog, hook)
	return b
}

// AfterLog adds a hook for the AfterLog event.
func (b *HookBuilder) AfterLog(hook Hook) *HookBuilder {
	b.registry.Add(HookAfterLog, hook)
	return b
}

// OnFilter adds a hook for the OnFilter event.
func (b *HookBuilder) OnFilter(hook Hook) *HookBuilder {
	b.registry.Add(HookOnFilter, hook)
	return b
}

// OnRotate adds a hook for the OnRotate event.
func (b *HookBuilder) OnRotate(hook Hook) *HookBuilder {
	b.registry.Add(HookOnRotate, hook)
	return b
}

// OnClose adds a hook for the OnClose event.
func (b *HookBuilder) OnClose(hook Hook) *HookBuilder {
	b.registry.Add(HookOnClose, hook)
	return b
}

// OnError adds a hook for the OnError event.
func (b *HookBuilder) OnError(hook Hook) *HookBuilder {
	b.registry.Add(HookOnError, hook)
	return b
}

// Build returns the configured HookRegistry.
func (b *HookBuilder) Build() *HookRegistry {
	return b.registry
}
