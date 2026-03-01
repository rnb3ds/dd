//go:build examples

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cybergodev/dd"
)

// Context & Hooks - Tracing Integration and Lifecycle Events
//
// Topics covered:
// 1. Type-safe context keys (trace_id, span_id, request_id)
// 2. Context-aware logging methods
// 3. Custom context extractors
// 4. Hook system for lifecycle events
// 5. OpenTelemetry-style integration
func main() {
	fmt.Println("=== DD Context & Hooks ===\n")

	section1ContextKeys()
	section2ContextLogging()
	section3CustomExtractors()
	section4Hooks()

	fmt.Println("\n✅ Context & Hooks examples completed!")
}

// Section 1: Type-safe context keys
func section1ContextKeys() {
	fmt.Println("1. Context Keys")
	fmt.Println("----------------")

	ctx := context.Background()

	// Add tracing metadata to context
	ctx = dd.WithTraceID(ctx, "trace-abc123")
	ctx = dd.WithSpanID(ctx, "span-def456")
	ctx = dd.WithRequestID(ctx, "req-789xyz")

	// Retrieve values
	traceID := dd.GetTraceID(ctx)
	spanID := dd.GetSpanID(ctx)
	requestID := dd.GetRequestID(ctx)

	fmt.Printf("  Trace ID: %s\n", traceID)
	fmt.Printf("  Span ID: %s\n", spanID)
	fmt.Printf("  Request ID: %s\n", requestID)

	fmt.Println()
}

// Section 2: Context-aware logging
func section2ContextLogging() {
	fmt.Println("2. Context-Aware Logging")
	fmt.Println("-------------------------")

	// Configure logger with context extraction
	cfg := dd.DefaultConfig()
	cfg.Format = dd.FormatJSON

	logger, _ := dd.New(cfg)
	defer logger.Close()

	// Create context with trace info
	ctx := dd.WithTraceID(context.Background(), "trace-123")
	ctx = dd.WithSpanID(ctx, "span-456")

	// Context-aware logging automatically extracts trace info
	logger.InfoCtx(ctx, "Processing request")
	logger.InfoWithCtx(ctx, "User action",
		dd.String("action", "login"),
		dd.String("user", "alice"),
	)

	// Printf-style with context
	logger.InfofCtx(ctx, "Order %d processed", 12345)

	fmt.Println("✓ Trace IDs automatically included in output\n")
}

// Section 3: Custom context extractors
func section3CustomExtractors() {
	fmt.Println("3. Custom Context Extractors")
	fmt.Println("------------------------------")

	// Custom extractor for tenant ID
	tenantExtractor := func(ctx context.Context) []dd.Field {
		if tenantID := ctx.Value("tenant_id"); tenantID != nil {
			return []dd.Field{dd.String("tenant_id", tenantID.(string))}
		}
		return nil
	}

	// Custom extractor for user ID
	userExtractor := func(ctx context.Context) []dd.Field {
		if userID := ctx.Value("user_id"); userID != nil {
			return []dd.Field{dd.Int("user_id", userID.(int))}
		}
		return nil
	}

	cfg := dd.DefaultConfig()
	cfg.Format = dd.FormatJSON
	cfg.ContextExtractors = []dd.ContextExtractor{tenantExtractor, userExtractor}

	logger, _ := dd.New(cfg)
	defer logger.Close()

	// Context with custom values
	ctx := context.WithValue(context.Background(), "tenant_id", "tenant-abc")
	ctx = context.WithValue(ctx, "user_id", 12345)

	logger.InfoCtx(ctx, "Custom extractors applied")

	// Add extractors at runtime
	logger.AddContextExtractor(func(ctx context.Context) []dd.Field {
		return []dd.Field{dd.String("service", "my-service")}
	})

	logger.InfoCtx(ctx, "Runtime extractor added")

	fmt.Println()
}

// Section 4: Hook system
func section4Hooks() {
	fmt.Println("4. Hook System")
	fmt.Println("---------------")

	// Create hook registry with builder
	hooks := dd.NewHookBuilder().
		BeforeLog(func(ctx context.Context, hctx *dd.HookContext) error {
			fmt.Printf("  [BeforeLog] Level: %s, Msg: %s\n",
				hctx.Level.String(), hctx.Message)
			return nil
		}).
		AfterLog(func(ctx context.Context, hctx *dd.HookContext) error {
			fmt.Printf("  [AfterLog] Completed: %s\n", hctx.Message)
			return nil
		}).
		OnError(func(ctx context.Context, hctx *dd.HookContext) error {
			fmt.Printf("  [OnError] %v\n", hctx.Error)
			return nil
		}).
		Build()

	cfg := dd.DefaultConfig()
	cfg.Format = dd.FormatJSON
	cfg.Hooks = hooks

	logger, _ := dd.New(cfg)
	defer logger.Close()

	// Log messages trigger hooks
	logger.Info("This triggers BeforeLog and AfterLog hooks")

	// Add hooks at runtime
	logger.AddHook(dd.HookOnFilter, func(ctx context.Context, hctx *dd.HookContext) error {
		fmt.Printf("  [OnFilter] Data was filtered\n")
		return nil
	})

	fmt.Println()
}

// Example: OpenTelemetry-style integration
func exampleOpenTelemetry() {
	// This shows how to integrate with OpenTelemetry
	// (requires opentelemetry-go package)

	/*
		import (
			"go.opentelemetry.io/otel/trace"
		)

		otelExtractor := func(ctx context.Context) []dd.Field {
			span := trace.SpanFromContext(ctx)
			if !span.SpanContext().IsValid() {
				return nil
			}
			return []dd.Field{
				dd.String("trace_id", span.SpanContext().TraceID().String()),
				dd.String("span_id", span.SpanContext().SpanID().String()),
				dd.Bool("sampled", span.SpanContext().IsSampleled()),
			}
		}

		cfg := dd.DefaultConfig()
		cfg.ContextExtractors = []dd.ContextExtractor{otelExtractor}
		logger, _ := dd.New(cfg)
	*/

	fmt.Println("OpenTelemetry integration example (see source code)")
}

// Example: Request-scoped logging
func exampleRequestScopedLogging() {
	cfg := dd.DefaultConfig()
	cfg.Format = dd.FormatJSON
	cfg.File = &dd.FileConfig{Path: "logs/requests.log"}

	logger, _ := dd.New(cfg)
	defer logger.Close()

	// Simulated HTTP handler
	handler := func(ctx context.Context, path string) {
		ctx = dd.WithRequestID(ctx, fmt.Sprintf("req-%d", time.Now().UnixNano()))
		ctx = dd.WithTraceID(ctx, "trace-from-header")

		logger.InfoWithCtx(ctx, "Request started",
			dd.String("path", path),
		)

		// Business logic...

		logger.InfoWithCtx(ctx, "Request completed",
			dd.Int("status", 200),
			dd.Duration("duration", 50*time.Millisecond),
		)
	}

	handler(context.Background(), "/api/users")
}
