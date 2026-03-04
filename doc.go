// Package dd provides a high-performance, thread-safe logging library for Go.
//
// dd (short for "data-driven" or "distributed debugger") is designed for production
// workloads with a focus on performance, security, and structured logging. It provides
// multiple output formats, sensitive data filtering, and seamless context integration.
//
// # Features
//
//   - Thread-safe: All operations are safe for concurrent use
//   - Multiple log levels: Debug, Info, Warn, Error, Fatal
//   - Flexible output: Console, file, or any io.Writer
//   - Structured logging: Key-value field support with type-safe helpers
//   - JSON format: Built-in JSON output with configurable field names
//   - Sensitive data filtering: Automatic redaction of passwords, tokens, etc.
//   - Context integration: Extract trace IDs and request IDs from context
//   - Log rotation: Built-in file rotation with compression support
//   - Lifecycle hooks: Extensible hook system for custom behavior
//   - Log sampling: Reduce log volume in high-throughput scenarios
//   - Zero allocations: Optimized for minimal GC pressure
//
// # Quick Start
//
// Basic usage:
//
//	package main
//
//	import "github.com/cybergodev/dd"
//
//	func main() {
//	    // Create a logger with default settings
//	    logger, _ := dd.New()
//
//	    // Simple logging
//	    logger.Info("Application started")
//	    logger.Errorf("User %s logged in", "john")
//
//	    // Structured logging
//	    logger.InfoWith("Request processed",
//	        dd.String("method", "GET"),
//	        dd.Int("status", 200),
//	        dd.Duration("latency", 150*time.Millisecond),
//	    )
//
//	    // Clean up
//	    logger.Close()
//	}
//
// # Configuration
//
// Using Config struct (recommended):
//
//	cfg := dd.DefaultConfig()
//	cfg.Level = dd.LevelDebug
//	cfg.Format = dd.FormatJSON
//	cfg.DynamicCaller = true
//	logger, _ := dd.New(cfg)
//
// With file output:
//
//	cfg := dd.DefaultConfig()
//	cfg.File = &dd.FileConfig{
//	    Path:       "app.log",
//	    MaxSizeMB:  100,
//	    MaxBackups: 10,
//	    Compress:   true,
//	}
//	logger, _ := dd.New(cfg)
//
// Using presets:
//
//	// Development preset
//	logger := dd.Must(dd.DevelopmentConfig())
//
//	// JSON preset
//	logger := dd.Must(dd.JSONConfig())
//
// # Structured Logging
//
// Create type-safe fields:
//
//	logger.InfoWith("User action",
//	    dd.String("user_id", "123"),
//	    dd.String("action", "login"),
//	    dd.Time("timestamp", time.Now()),
//	    dd.Err(err),
//	)
//
// Chain fields for reuse:
//
//	userLogger := logger.WithFields(dd.String("user_id", "123"))
//	userLogger.Info("Login successful")
//	userLogger.Error("Permission denied")
//
// # Context Integration
//
// Use type-safe context keys for tracing:
//
//	ctx := dd.WithTraceID(context.Background(), "trace-123")
//	ctx = dd.WithSpanID(ctx, "span-456")
//	ctx = dd.WithRequestID(ctx, "req-789")
//
//	logger.InfoCtx(ctx, "Processing request")
//	// Output will include trace_id, span_id, and request_id fields
//
// # Sensitive Data Filtering
//
// Enable automatic filtering of sensitive data:
//
//	cfg := dd.DefaultConfig()
//	cfg.Security = dd.DefaultSecurityConfig()
//	logger, _ := dd.New(cfg)
//
//	logger.Info("User logged in", "password", "secret123")
//	// Output: User logged in password=***REDACTED***
//
// # File Output with Rotation
//
//	cfg := dd.DefaultConfig()
//	cfg.File = &dd.FileConfig{
//	    Path:       "logs/app.log",
//	    MaxSizeMB:  100,
//	    MaxBackups: 5,
//	    MaxAge:     7 * 24 * time.Hour,
//	    Compress:   true,
//	}
//	cfg.Format = dd.FormatJSON
//	logger, _ := dd.New(cfg)
//
// # Interface for Testing
//
// Use the LogProvider interface for dependency injection:
//
//	type Service struct {
//	    logger dd.LogProvider
//	}
//
//	func NewService(logger dd.LogProvider) *Service {
//	    return &Service{logger: logger}
//	}
//
// # Performance
//
// dd is optimized for high-throughput scenarios:
//
//   - Sync.Pool for message buffer reuse
//   - Atomic operations for thread-safe state
//   - Lock-free reads for writers and extractors
//   - Minimal allocations in hot paths
//
// # Log Levels
//
// From lowest to highest priority:
//
//   - LevelDebug: Detailed information for debugging
//   - LevelInfo: General operational information
//   - LevelWarn: Warning conditions that may indicate problems
//   - LevelError: Error conditions that should be investigated
//   - LevelFatal: Severe errors that require program termination
//
// # Thread Safety
//
// All Logger methods are safe for concurrent use. You can:
//
//   - Share a single logger across goroutines
//   - Add/remove writers at runtime
//   - Change log level dynamically
//   - Modify context extractors and hooks
//
// # Graceful Shutdown
//
// Always close the logger before exit:
//
//	logger.Close()
//
// For Fatal logs, use custom fatal handler:
//
//	cfg := dd.DefaultConfig()
//	cfg.FatalHandler = func() {
//	    // Custom cleanup
//	    logger.Close()
//	    os.Exit(1)
//	}
//	logger, _ := dd.New(cfg)
package dd

import (
	"context"
	"io"
	"time"
)

// ============================================================================
// Interfaces
// ============================================================================

// CoreLogger defines the core logging interface with basic logging methods.
// This is the primary interface for basic logging operations.
// Use this interface for dependency injection when you only need logging methods.
type CoreLogger interface {
	// Core logging methods - Debug level
	Debug(args ...any)
	Debugf(format string, args ...any)
	DebugWith(msg string, fields ...Field)

	// Core logging methods - Info level
	Info(args ...any)
	Infof(format string, args ...any)
	InfoWith(msg string, fields ...Field)

	// Core logging methods - Warn level
	Warn(args ...any)
	Warnf(format string, args ...any)
	WarnWith(msg string, fields ...Field)

	// Core logging methods - Error level
	Error(args ...any)
	Errorf(format string, args ...any)
	ErrorWith(msg string, fields ...Field)

	// Core logging methods - Fatal level
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	FatalWith(msg string, fields ...Field)

	// Field chaining
	WithFields(fields ...Field) *LoggerEntry
	WithField(key string, value any) *LoggerEntry
}

// ContextLogger extends CoreLogger with context-aware logging methods.
// Use this interface when you need to extract context values (trace IDs, etc.)
// into your log entries.
type ContextLogger interface {
	CoreLogger

	// Context-aware methods - Debug level
	DebugCtx(ctx context.Context, args ...any)
	DebugfCtx(ctx context.Context, format string, args ...any)
	DebugWithCtx(ctx context.Context, msg string, fields ...Field)

	// Context-aware methods - Info level
	InfoCtx(ctx context.Context, args ...any)
	InfofCtx(ctx context.Context, format string, args ...any)
	InfoWithCtx(ctx context.Context, msg string, fields ...Field)

	// Context-aware methods - Warn level
	WarnCtx(ctx context.Context, args ...any)
	WarnfCtx(ctx context.Context, format string, args ...any)
	WarnWithCtx(ctx context.Context, msg string, fields ...Field)

	// Context-aware methods - Error level
	ErrorCtx(ctx context.Context, args ...any)
	ErrorfCtx(ctx context.Context, format string, args ...any)
	ErrorWithCtx(ctx context.Context, msg string, fields ...Field)

	// Context-aware methods - Fatal level
	FatalCtx(ctx context.Context, args ...any)
	FatalfCtx(ctx context.Context, format string, args ...any)
	FatalWithCtx(ctx context.Context, msg string, fields ...Field)
}

// LevelLogger extends CoreLogger with level management methods.
type LevelLogger interface {
	CoreLogger

	// Level management
	GetLevel() LogLevel
	SetLevel(level LogLevel) error
	IsLevelEnabled(level LogLevel) bool
	IsDebugEnabled() bool
	IsInfoEnabled() bool
	IsWarnEnabled() bool
	IsErrorEnabled() bool
	IsFatalEnabled() bool
}

// ConfigurableLogger extends CoreLogger with configuration and lifecycle methods.
type ConfigurableLogger interface {
	CoreLogger

	// Level management
	GetLevel() LogLevel
	SetLevel(level LogLevel) error

	// Writer management
	AddWriter(writer io.Writer) error
	RemoveWriter(writer io.Writer) error
	WriterCount() int

	// Lifecycle
	Flush() error
	Close() error
	IsClosed() bool

	// Configuration
	SetSecurityConfig(config *SecurityConfig)
	GetSecurityConfig() *SecurityConfig
	SetWriteErrorHandler(handler WriteErrorHandler)

	// Context extractors
	AddContextExtractor(extractor ContextExtractor) error
	MustAddContextExtractor(extractor ContextExtractor)
	SetContextExtractors(extractors ...ContextExtractor) error
	MustSetContextExtractors(extractors ...ContextExtractor)
	GetContextExtractors() []ContextExtractor

	// Hooks
	AddHook(event HookEvent, hook Hook) error
	MustAddHook(event HookEvent, hook Hook)
	SetHooks(registry *HookRegistry) error
	MustSetHooks(registry *HookRegistry)
	GetHooks() *HookRegistry

	// Sampling
	SetSampling(config *SamplingConfig)
	GetSampling() *SamplingConfig
}

// LogProvider is the full interface combining all logging capabilities.
// This interface enables dependency injection, mocking, and testing.
// The concrete Logger type implements this interface.
//
// Example usage with dependency injection:
//
//	type Service struct {
//	    logger dd.LogProvider
//	}
//
//	func NewService(logger dd.LogProvider) *Service {
//	    return &Service{logger: logger}
//	}
//
//	// In production
//	logger, _ := dd.New()
//	service := NewService(logger)
//
//	// In tests
//	mockLogger := &MockLogger{}
//	service := NewService(mockLogger)
type LogProvider interface {
	// Level management
	GetLevel() LogLevel
	SetLevel(level LogLevel) error
	IsLevelEnabled(level LogLevel) bool
	IsDebugEnabled() bool
	IsInfoEnabled() bool
	IsWarnEnabled() bool
	IsErrorEnabled() bool
	IsFatalEnabled() bool

	// Core logging methods
	Log(level LogLevel, args ...any)
	Logf(level LogLevel, format string, args ...any)
	LogWith(level LogLevel, msg string, fields ...Field)

	// Context-aware core methods
	LogCtx(ctx context.Context, level LogLevel, args ...any)
	LogfCtx(ctx context.Context, level LogLevel, format string, args ...any)
	LogWithCtx(ctx context.Context, level LogLevel, msg string, fields ...Field)

	// Convenience methods - Debug level
	Debug(args ...any)
	Debugf(format string, args ...any)
	DebugWith(msg string, fields ...Field)

	// Convenience methods - Info level
	Info(args ...any)
	Infof(format string, args ...any)
	InfoWith(msg string, fields ...Field)

	// Convenience methods - Warn level
	Warn(args ...any)
	Warnf(format string, args ...any)
	WarnWith(msg string, fields ...Field)

	// Convenience methods - Error level
	Error(args ...any)
	Errorf(format string, args ...any)
	ErrorWith(msg string, fields ...Field)

	// Convenience methods - Fatal level
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	FatalWith(msg string, fields ...Field)

	// Context-aware convenience methods - Debug level
	DebugCtx(ctx context.Context, args ...any)
	DebugfCtx(ctx context.Context, format string, args ...any)
	DebugWithCtx(ctx context.Context, msg string, fields ...Field)

	// Context-aware convenience methods - Info level
	InfoCtx(ctx context.Context, args ...any)
	InfofCtx(ctx context.Context, format string, args ...any)
	InfoWithCtx(ctx context.Context, msg string, fields ...Field)

	// Context-aware convenience methods - Warn level
	WarnCtx(ctx context.Context, args ...any)
	WarnfCtx(ctx context.Context, format string, args ...any)
	WarnWithCtx(ctx context.Context, msg string, fields ...Field)

	// Context-aware convenience methods - Error level
	ErrorCtx(ctx context.Context, args ...any)
	ErrorfCtx(ctx context.Context, format string, args ...any)
	ErrorWithCtx(ctx context.Context, msg string, fields ...Field)

	// Context-aware convenience methods - Fatal level
	FatalCtx(ctx context.Context, args ...any)
	FatalfCtx(ctx context.Context, format string, args ...any)
	FatalWithCtx(ctx context.Context, msg string, fields ...Field)

	// Field chaining
	WithFields(fields ...Field) *LoggerEntry
	WithField(key string, value any) *LoggerEntry

	// Writer management
	AddWriter(writer io.Writer) error
	RemoveWriter(writer io.Writer) error
	WriterCount() int

	// Lifecycle
	Flush() error
	Close() error
	IsClosed() bool

	// Configuration
	SetSecurityConfig(config *SecurityConfig)
	GetSecurityConfig() *SecurityConfig
	SetWriteErrorHandler(handler WriteErrorHandler)

	// Context extractors
	AddContextExtractor(extractor ContextExtractor) error
	MustAddContextExtractor(extractor ContextExtractor)
	SetContextExtractors(extractors ...ContextExtractor) error
	MustSetContextExtractors(extractors ...ContextExtractor)
	GetContextExtractors() []ContextExtractor

	// Hooks
	AddHook(event HookEvent, hook Hook) error
	MustAddHook(event HookEvent, hook Hook)
	SetHooks(registry *HookRegistry) error
	MustSetHooks(registry *HookRegistry)
	GetHooks() *HookRegistry

	// Sampling
	SetSampling(config *SamplingConfig)
	GetSampling() *SamplingConfig

	// Debug utilities
	Print(args ...any)
	Println(args ...any)
	Printf(format string, args ...any)
	Text(data ...any)
	Textf(format string, args ...any)
	JSON(data ...any)

	// Filter goroutine monitoring
	ActiveFilterGoroutines() int32
	WaitForFilterGoroutines(timeout time.Duration) bool
}

// Note: Compile-time interface verification is done in logger.go
// The concrete Logger struct implements all these interfaces.
