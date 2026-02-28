// Package dd provides a high-performance, thread-safe logging library.
package dd

import (
	"context"
	"io"
	"time"
)

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
