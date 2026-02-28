package dd

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// ContextExtractor is a function that extracts logging fields from a context.
// This allows integration with various tracing frameworks (OpenTelemetry, Jaeger, etc.)
// by providing custom field extraction logic.
//
// Example:
//
//	// OpenTelemetry trace extractor
//	otelExtractor := func(ctx context.Context) []Field {
//	    span := trace.SpanFromContext(ctx)
//	    if !span.SpanContext().IsValid() {
//	        return nil
//	    }
//	    return []Field{
//	        String("trace_id", span.SpanContext().TraceID().String()),
//	        String("span_id", span.SpanContext().SpanID().String()),
//	    }
//	}
type ContextExtractor func(ctx context.Context) []Field

// ContextExtractorRegistry manages a collection of context extractors.
// It is thread-safe and supports dynamic addition of extractors.
// Uses atomic.Pointer for lock-free reads.
type ContextExtractorRegistry struct {
	extractorsPtr atomic.Pointer[[]ContextExtractor]
	mu            sync.Mutex // protects modification operations
}

// NewContextExtractorRegistry creates a new empty extractor registry.
func NewContextExtractorRegistry() *ContextExtractorRegistry {
	r := &ContextExtractorRegistry{}
	emptySlice := make([]ContextExtractor, 0)
	r.extractorsPtr.Store(&emptySlice)
	return r
}

// Add adds a context extractor to the registry.
// If the extractor is nil, it is ignored.
// This method is thread-safe.
func (r *ContextExtractorRegistry) Add(extractor ContextExtractor) {
	if extractor == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// Load current extractors
	currentPtr := r.extractorsPtr.Load()
	current := *currentPtr

	// Create new slice with the extractor added
	newExtractors := make([]ContextExtractor, len(current)+1)
	copy(newExtractors, current)
	newExtractors[len(current)] = extractor

	// Atomically swap the pointer
	r.extractorsPtr.Store(&newExtractors)
}

// Extract executes all registered extractors and returns the combined fields.
// Extractors are called in the order they were added.
// This method is thread-safe and uses lock-free reads.
func (r *ContextExtractorRegistry) Extract(ctx context.Context) []Field {
	if r == nil {
		return nil
	}

	// Lock-free read
	extractorsPtr := r.extractorsPtr.Load()
	if extractorsPtr == nil {
		return nil
	}
	extractors := *extractorsPtr

	if len(extractors) == 0 {
		return nil
	}

	// Pre-allocate result slice with estimated capacity
	var fields []Field
	for _, extractor := range extractors {
		extracted := extractor(ctx)
		if len(extracted) > 0 {
			fields = append(fields, extracted...)
		}
	}

	return fields
}

// Clone creates a copy of the registry with the same extractors.
// The extractors themselves are shared (functions are not copied).
func (r *ContextExtractorRegistry) Clone() *ContextExtractorRegistry {
	if r == nil {
		return nil
	}

	extractorsPtr := r.extractorsPtr.Load()
	if extractorsPtr == nil {
		return NewContextExtractorRegistry()
	}
	extractors := *extractorsPtr

	clone := &ContextExtractorRegistry{}
	clonedSlice := make([]ContextExtractor, len(extractors))
	copy(clonedSlice, extractors)
	clone.extractorsPtr.Store(&clonedSlice)

	return clone
}

// Count returns the number of registered extractors.
func (r *ContextExtractorRegistry) Count() int {
	if r == nil {
		return 0
	}
	extractorsPtr := r.extractorsPtr.Load()
	if extractorsPtr == nil {
		return 0
	}
	return len(*extractorsPtr)
}

// Clear removes all registered extractors.
func (r *ContextExtractorRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	emptySlice := make([]ContextExtractor, 0)
	r.extractorsPtr.Store(&emptySlice)
}

// Singleton default registry
var (
	defaultRegistry     *ContextExtractorRegistry
	defaultRegistryOnce sync.Once
)

// DefaultContextExtractorRegistry returns a singleton registry with the default extractors.
// The default extractors extract trace_id, span_id, and request_id from context values.
// This function is thread-safe and uses sync.Once for initialization.
func DefaultContextExtractorRegistry() *ContextExtractorRegistry {
	defaultRegistryOnce.Do(func() {
		registry := NewContextExtractorRegistry()
		registry.Add(defaultTraceIDExtractor)
		registry.Add(defaultSpanIDExtractor)
		registry.Add(defaultRequestIDExtractor)
		defaultRegistry = registry
	})
	return defaultRegistry
}

// defaultTraceIDExtractor extracts trace_id from context.
// Supports both type-safe ContextKey and string keys for backward compatibility.
func defaultTraceIDExtractor(ctx context.Context) []Field {
	if ctx == nil {
		return nil
	}

	// Try type-safe key first
	if traceID := ctx.Value(ContextKeyTraceID); traceID != nil {
		return []Field{String("trace_id", stringValue(traceID))}
	}

	// Fall back to string key for backward compatibility
	if traceID := ctx.Value("trace_id"); traceID != nil {
		return []Field{String("trace_id", stringValue(traceID))}
	}

	return nil
}

// defaultSpanIDExtractor extracts span_id from context.
// Supports both type-safe ContextKey and string keys for backward compatibility.
func defaultSpanIDExtractor(ctx context.Context) []Field {
	if ctx == nil {
		return nil
	}

	// Try type-safe key first
	if spanID := ctx.Value(ContextKeySpanID); spanID != nil {
		return []Field{String("span_id", stringValue(spanID))}
	}

	// Fall back to string key for backward compatibility
	if spanID := ctx.Value("span_id"); spanID != nil {
		return []Field{String("span_id", stringValue(spanID))}
	}

	return nil
}

// defaultRequestIDExtractor extracts request_id from context.
// Supports both type-safe ContextKey and string keys for backward compatibility.
func defaultRequestIDExtractor(ctx context.Context) []Field {
	if ctx == nil {
		return nil
	}

	// Try type-safe key first
	if requestID := ctx.Value(ContextKeyRequestID); requestID != nil {
		return []Field{String("request_id", stringValue(requestID))}
	}

	// Fall back to string key for backward compatibility
	if requestID := ctx.Value("request_id"); requestID != nil {
		return []Field{String("request_id", stringValue(requestID))}
	}

	return nil
}

// stringValue converts any value to its string representation.
func stringValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
