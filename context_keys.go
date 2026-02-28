package dd

import "context"

// ContextKey is a type-safe key for context values.
// Using a custom type prevents key collisions with other packages
// that may also store values in context.
//
// Example:
//
//	ctx := dd.WithTraceID(context.Background(), "abc123")
//	traceID := dd.GetTraceID(ctx)
type ContextKey string

const (
	// ContextKeyTraceID is the context key for trace ID.
	// This key is used by default context extractors to retrieve
	// the trace ID from context.
	ContextKeyTraceID ContextKey = "trace_id"

	// ContextKeySpanID is the context key for span ID.
	// This key is used by default context extractors to retrieve
	// the span ID from context.
	ContextKeySpanID ContextKey = "span_id"

	// ContextKeyRequestID is the context key for request ID.
	// This key is used by default context extractors to retrieve
	// the request ID from context.
	ContextKeyRequestID ContextKey = "request_id"
)

// WithTraceID adds a trace ID to the context.
// This is the type-safe way to store trace IDs that will be
// automatically extracted by the logger's context extractors.
//
// Example:
//
//	ctx := dd.WithTraceID(context.Background(), "trace-123")
//	logger.InfoCtx(ctx, "processing request") // Will include trace_id field
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ContextKeyTraceID, traceID)
}

// WithSpanID adds a span ID to the context.
// This is the type-safe way to store span IDs that will be
// automatically extracted by the logger's context extractors.
//
// Example:
//
//	ctx := dd.WithSpanID(context.Background(), "span-456")
//	logger.InfoCtx(ctx, "processing request") // Will include span_id field
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, ContextKeySpanID, spanID)
}

// WithRequestID adds a request ID to the context.
// This is the type-safe way to store request IDs that will be
// automatically extracted by the logger's context extractors.
//
// Example:
//
//	ctx := dd.WithRequestID(context.Background(), "req-789")
//	logger.InfoCtx(ctx, "processing request") // Will include request_id field
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ContextKeyRequestID, requestID)
}

// GetTraceID retrieves the trace ID from the context.
// Returns an empty string if no trace ID is found.
//
// Example:
//
//	traceID := dd.GetTraceID(ctx)
//	if traceID != "" {
//	    // Trace ID is present
//	}
func GetTraceID(ctx context.Context) string {
	if v := ctx.Value(ContextKeyTraceID); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetSpanID retrieves the span ID from the context.
// Returns an empty string if no span ID is found.
//
// Example:
//
//	spanID := dd.GetSpanID(ctx)
//	if spanID != "" {
//	    // Span ID is present
//	}
func GetSpanID(ctx context.Context) string {
	if v := ctx.Value(ContextKeySpanID); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetRequestID retrieves the request ID from the context.
// Returns an empty string if no request ID is found.
//
// Example:
//
//	requestID := dd.GetRequestID(ctx)
//	if requestID != "" {
//	    // Request ID is present
//	}
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(ContextKeyRequestID); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
