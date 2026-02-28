package dd

import (
	"context"
	"fmt"
	"io"
	"os"
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

// HookErrorHandler handles errors that occur during hook execution.
// This allows custom error handling strategies such as logging, metrics,
// or ignoring errors for non-critical hooks.
//
// Parameters:
//   - event: The hook event type that triggered the error
//   - hookCtx: The context provided to the hook
//   - err: The error returned by the hook
//
// The handler should not panic. If it does, the panic will be recovered
// and logged to stderr.
type HookErrorHandler func(event HookEvent, hookCtx *HookContext, err error)

// DefaultHookErrorHandler logs hook errors to stderr.
// This is the default error handler used when no custom handler is set.
func DefaultHookErrorHandler(event HookEvent, hookCtx *HookContext, err error) {
	fmt.Fprintf(os.Stderr, "dd: hook error for event %s: %v\n", event, err)
}

// HookErrorRecorder records hook errors for later inspection.
// This is useful for testing or monitoring hook health.
//
// Usage:
//
//	recorder := NewHookErrorRecorder()
//	registry := NewHookRegistryWithErrorHandler(recorder.Handler())
//	// ... after hooks run ...
//	errors := recorder.Errors()
//	for _, err := range errors {
//	    log.Printf("Hook error: %v", err)
//	}
type HookErrorRecorder struct {
	mu     sync.Mutex
	errors []HookErrorInfo
}

// HookErrorInfo contains information about a hook error.
type HookErrorInfo struct {
	Event     HookEvent
	Timestamp time.Time
	Error     error
	Message   string // The log message being processed (if applicable)
}

// NewHookErrorRecorder creates a new HookErrorRecorder.
func NewHookErrorRecorder() *HookErrorRecorder {
	return &HookErrorRecorder{
		errors: make([]HookErrorInfo, 0),
	}
}

// Handler returns a HookErrorHandler that records errors to this recorder.
func (r *HookErrorRecorder) Handler() HookErrorHandler {
	return func(event HookEvent, hookCtx *HookContext, err error) {
		r.mu.Lock()
		defer r.mu.Unlock()

		info := HookErrorInfo{
			Event:     event,
			Timestamp: time.Now(),
			Error:     err,
		}
		if hookCtx != nil {
			info.Message = hookCtx.Message
		}

		r.errors = append(r.errors, info)
	}
}

// Errors returns all recorded errors.
func (r *HookErrorRecorder) Errors() []HookErrorInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]HookErrorInfo, len(r.errors))
	copy(result, r.errors)
	return result
}

// Count returns the number of recorded errors.
func (r *HookErrorRecorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.errors)
}

// Clear removes all recorded errors.
func (r *HookErrorRecorder) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errors = r.errors[:0]
}

// HasErrors returns true if any errors have been recorded.
func (r *HookErrorRecorder) HasErrors() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.errors) > 0
}

// HookRegistry manages a collection of hooks organized by event type.
// It is thread-safe and supports dynamic hook registration.
//
// Error Handling Behavior:
//   - By default, Trigger returns the first error from a hook and stops execution
//   - If an error handler is set via SetErrorHandler, all hooks are executed
//     regardless of errors, and errors are passed to the handler
//   - For BeforeLog events, an error still prevents the log from being written
//     even with an error handler set
type HookRegistry struct {
	mu           sync.RWMutex
	hooks        map[HookEvent][]Hook
	errorHandler HookErrorHandler
}

// NewHookRegistry creates a new empty hook registry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		hooks: make(map[HookEvent][]Hook),
	}
}

// NewHookRegistryWithErrorHandler creates a registry with a custom error handler.
// When an error handler is set, all hooks are executed even if some fail,
// and errors are passed to the handler instead of being returned immediately.
func NewHookRegistryWithErrorHandler(handler HookErrorHandler) *HookRegistry {
	return &HookRegistry{
		hooks:        make(map[HookEvent][]Hook),
		errorHandler: handler,
	}
}

// SetErrorHandler sets the error handler for this registry.
// Pass nil to remove the error handler and restore default behavior.
func (r *HookRegistry) SetErrorHandler(handler HookErrorHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errorHandler = handler
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
//
// Error Handling Behavior:
//   - If no error handler is set (default): hooks are executed in order;
//     if any hook returns an error, execution stops and that error is returned.
//   - If an error handler is set: all hooks are executed regardless of errors;
//     each error is passed to the error handler, and the first error is returned.
//
// For BeforeLog events, an error prevents the log from being written
// regardless of whether an error handler is set.
func (r *HookRegistry) Trigger(ctx context.Context, event HookEvent, hookCtx *HookContext) error {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	hooks := r.hooks[event]
	handler := r.errorHandler
	r.mu.RUnlock()

	if len(hooks) == 0 {
		return nil
	}

	var firstErr error

	for _, hook := range hooks {
		if err := hook(ctx, hookCtx); err != nil {
			if handler != nil {
				// Call the error handler
				handler(event, hookCtx, err)
				// Record first error to return later
				if firstErr == nil {
					firstErr = err
				}
			} else {
				// Default behavior: stop on first error
				return err
			}
		}
	}

	return firstErr
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
