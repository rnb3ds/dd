// Package dd provides a high-performance, thread-safe logging library.
package dd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybergodev/dd/internal"
)

var (
	messagePool = sync.Pool{
		New: func() any {
			buf := make([]byte, 0, DefaultBufferSize)
			return &buf
		},
	}
)

// Compile-time interface verification
var _ LogProvider = (*Logger)(nil)

// Re-export log level types and constants
type LogLevel = internal.LogLevel

const (
	LevelDebug = internal.LevelDebug
	LevelInfo  = internal.LevelInfo
	LevelWarn  = internal.LevelWarn
	LevelError = internal.LevelError
	LevelFatal = internal.LevelFatal
)

type FatalHandler func()

type WriteErrorHandler func(writer io.Writer, err error)

// LevelResolver is a function that determines the effective log level at runtime.
// This allows dynamic log level adjustment based on runtime conditions such as
// system load, error rate, or time of day. The function is called for each log
// entry, so it should be efficient.
//
// Example:
//
//	// Adjust level based on system load
//	resolver := func(ctx context.Context) LogLevel {
//	    if getCPULoad() > 0.8 {
//	        return LevelWarn  // Only log warnings and above under high load
//	    }
//	    return LevelDebug
//	}
//	logger.SetLevelResolver(resolver)
type LevelResolver func(ctx context.Context) LogLevel

// Flusher is an interface for writers that can flush buffered data.
// Writers implementing this interface will have their Flush method called
// during Logger.Flush() to ensure all buffered data is written.
type Flusher interface {
	Flush() error
}

type Logger struct {
	level  atomic.Int32
	closed atomic.Bool

	callerDepth       int
	fatalHandler      FatalHandler
	writeErrorHandler atomic.Value // stores WriteErrorHandler
	formatter         *internal.MessageFormatter

	// levelResolver stores an optional dynamic level resolver function.
	// When set, it is called to determine the effective log level for each entry.
	// If nil or returns LevelDebug, the static level is used.
	levelResolver atomic.Pointer[LevelResolver]

	// fieldValidation stores the field validation configuration.
	// When set, field keys are validated against the configured naming convention.
	fieldValidation atomic.Pointer[FieldValidationConfig]

	// writersPtr stores an immutable slice of writers using atomic pointer.
	// This eliminates slice copying during write operations.
	// The slice is replaced atomically when writers are added/removed.
	writersPtr     atomic.Pointer[[]io.Writer]
	writersMu      sync.Mutex // protects AddWriter/RemoveWriter operations
	securityConfig atomic.Value

	// contextExtractors stores the ContextExtractorRegistry for extracting
	// fields from context. If nil, default extractors are used.
	contextExtractors atomic.Value // stores *ContextExtractorRegistry

	// hooks stores the HookRegistry for lifecycle hooks.
	hooks atomic.Value // stores *HookRegistry
	// hooksMu protects the Clone-Modify-Store sequence in AddHook to prevent race conditions
	hooksMu sync.Mutex

	// sampling stores the sampling configuration and state.
	sampling atomic.Value // stores *samplingState

	// ctx and cancel provide graceful shutdown for background operations.
	// When Close() is called, cancel() signals all background goroutines
	// (compression, cleanup) to stop. This ensures clean shutdown without
	// leaking goroutines. The context is also used by filter timeout goroutines.
	ctx    context.Context
	cancel context.CancelFunc
}

// samplingState holds the runtime state for log sampling.
type samplingState struct {
	config  *SamplingConfig
	counter atomic.Int64 // Atomic counter for thread-safe increment
	start   time.Time
	startMu sync.Mutex // Only protects start time reset during tick
}

var (
	defaultOutput                    = os.Stdout
	defaultFatalHandler FatalHandler = func() {
		os.Exit(1)
	}
)

// New creates a new Logger with the provided configuration.
// If no configuration is provided, default settings are used.
//
// Example:
//
//	// Simple usage with defaults
//	logger, _ := dd.New()
//	logger.Info("hello")
//
//	// With custom configuration
//	cfg := dd.DefaultConfig()
//	cfg.Level = dd.LevelDebug
//	cfg.Format = dd.FormatJSON
//	logger, _ := dd.New(cfg)
func New(cfgs ...*Config) (*Logger, error) {
	// Return error if multiple configs provided - this is likely a developer mistake
	if len(cfgs) > 1 {
		return nil, fmt.Errorf("%w: %d configs provided, expected 0 or 1", ErrMultipleConfigs, len(cfgs))
	}
	if len(cfgs) == 0 {
		return defaultConfig().build()
	}
	if cfgs[0] == nil {
		return nil, ErrNilConfig
	}
	return cfgs[0].build()
}

// Must creates a new Logger and panics on error.
// If no configuration is provided, default settings are used.
//
// Example:
//
//	// Simple usage with defaults
//	logger := dd.Must()
//	defer logger.Close()
//
//	// With custom configuration
//	cfg := dd.DefaultConfig()
//	cfg.Format = dd.FormatJSON
//	logger := dd.Must(cfg)
func Must(cfgs ...*Config) *Logger {
	logger, err := New(cfgs...)
	if err != nil {
		panic("dd: failed to create logger: " + err.Error())
	}
	return logger
}

// newFromInternalConfig creates a Logger from the internal configuration.
func newFromInternalConfig(config *internalConfig) (*Logger, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Pre-allocate writers slice with expected capacity
	initialWriters := make([]io.Writer, 0, len(config.writers))

	// Create formatter config from logger config
	formatterConfig := &internal.FormatterConfig{
		Format:        internal.LogFormat(config.format),
		TimeFormat:    config.timeFormat,
		IncludeTime:   config.includeTime,
		IncludeLevel:  config.includeLevel,
		FullPath:      config.fullPath,
		DynamicCaller: config.dynamicCaller,
		JSON:          config.json,
	}

	l := &Logger{
		callerDepth:  DefaultCallerDepth,
		fatalHandler: config.fatalHandler,
		formatter:    internal.NewMessageFormatter(formatterConfig),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Initialize writers pointer with empty slice
	l.writersPtr.Store(&initialWriters)

	if config.writeErrorHandler != nil {
		l.writeErrorHandler.Store(config.writeErrorHandler)
	}

	l.level.Store(int32(config.level))
	l.securityConfig.Store(config.securityConfig)

	// Initialize field validation
	if config.fieldValidation != nil && config.fieldValidation.Mode != FieldValidationNone {
		l.fieldValidation.Store(config.fieldValidation)
	}

	// Initialize context extractors
	if config.contextExtractors != nil && len(config.contextExtractors) > 0 {
		registry := NewContextExtractorRegistry()
		for _, extractor := range config.contextExtractors {
			registry.Add(extractor)
		}
		l.contextExtractors.Store(registry)
	}

	// Initialize hooks
	if config.hooks != nil && config.hooks.Count() > 0 {
		l.hooks.Store(config.hooks.Clone())
	}

	// Initialize sampling
	if config.sampling != nil && config.sampling.Enabled {
		l.SetSampling(config.sampling)
	}

	if config.writers != nil {
		for _, writer := range config.writers {
			if err := l.AddWriter(writer); err != nil {
				cancel()
				return nil, fmt.Errorf("failed to add writer: %w", err)
			}
		}
	}

	return l, nil
}

// GetLevel returns the current log level (thread-safe).
func (l *Logger) GetLevel() LogLevel {
	return LogLevel(l.level.Load())
}

// SetLevel atomically sets the log level (thread-safe).
func (l *Logger) SetLevel(level LogLevel) error {
	if level < LevelDebug || level > LevelFatal {
		return ErrInvalidLevel
	}
	l.level.Store(int32(level))
	return nil
}

// IsLevelEnabled checks if logging is enabled for the given level (thread-safe).
// Returns true if the logger's level is at or below the specified level.
//
// Example:
//
//	if logger.IsLevelEnabled(dd.LevelDebug) {
//	    // Expensive debug computation only when debug is enabled
//	    logger.DebugWith("Details", dd.Any("data", computeExpensiveDebugInfo()))
//	}
func (l *Logger) IsLevelEnabled(level LogLevel) bool {
	currentLevel := LogLevel(l.level.Load())
	return level >= currentLevel
}

// IsDebugEnabled returns true if debug level logging is enabled.
func (l *Logger) IsDebugEnabled() bool {
	return l.IsLevelEnabled(LevelDebug)
}

// IsInfoEnabled returns true if info level logging is enabled.
func (l *Logger) IsInfoEnabled() bool {
	return l.IsLevelEnabled(LevelInfo)
}

// IsWarnEnabled returns true if warn level logging is enabled.
func (l *Logger) IsWarnEnabled() bool {
	return l.IsLevelEnabled(LevelWarn)
}

// IsErrorEnabled returns true if error level logging is enabled.
func (l *Logger) IsErrorEnabled() bool {
	return l.IsLevelEnabled(LevelError)
}

// IsFatalEnabled returns true if fatal level logging is enabled.
func (l *Logger) IsFatalEnabled() bool {
	return l.IsLevelEnabled(LevelFatal)
}

// SetLevelResolver sets a dynamic level resolver function (thread-safe).
// The resolver is called for each log entry to determine the effective log level.
// This allows runtime adjustment of log levels based on conditions like system load,
// error rates, or request context. Set to nil to disable dynamic resolution.
//
// Example:
//
//	// Adaptive logging based on error rate
//	var errorCount atomic.Int64
//	logger.SetLevelResolver(func(ctx context.Context) LogLevel {
//	    if errorCount.Load() > 100 {
//	        return LevelWarn  // Reduce logging under high error rate
//	    }
//	    return LevelDebug
//	})
func (l *Logger) SetLevelResolver(resolver LevelResolver) {
	if resolver == nil {
		l.levelResolver.Store(nil)
	} else {
		l.levelResolver.Store(&resolver)
	}
}

// GetLevelResolver returns the current level resolver function.
// Returns nil if no resolver is set.
func (l *Logger) GetLevelResolver() LevelResolver {
	if ptr := l.levelResolver.Load(); ptr != nil {
		return *ptr
	}
	return nil
}

// SetWriteErrorHandler sets a callback for handling write errors (thread-safe).
// When a write operation fails, the handler is called with the writer and error.
// If no handler is set, write errors are silently ignored.
func (l *Logger) SetWriteErrorHandler(handler WriteErrorHandler) {
	if handler != nil {
		l.writeErrorHandler.Store(handler)
	} else {
		l.writeErrorHandler.Store(nil)
	}
}

// getWriteErrorHandler returns the current write error handler (thread-safe).
func (l *Logger) getWriteErrorHandler() WriteErrorHandler {
	if v := l.writeErrorHandler.Load(); v != nil {
		return v.(WriteErrorHandler)
	}
	return nil
}

// SetSecurityConfig atomically sets the security configuration (thread-safe).
func (l *Logger) SetSecurityConfig(config *SecurityConfig) {
	if config == nil {
		config = DefaultSecurityConfig()
	}
	l.securityConfig.Store(config)
}

// GetSecurityConfig returns a copy of the current security configuration (thread-safe).
// Returns DefaultSecurityConfig() if no security config has been set.
// The returned config is a clone, so modifications do not affect the logger's config.
// For internal use within the logger, use getSecurityConfig() which returns the original.
func (l *Logger) GetSecurityConfig() *SecurityConfig {
	config := l.securityConfig.Load()
	if config == nil {
		return DefaultSecurityConfig()
	}
	secConfig, ok := config.(*SecurityConfig)
	if !ok {
		return DefaultSecurityConfig()
	}
	return secConfig.Clone()
}

// AddContextExtractor adds a context extractor to the logger (thread-safe).
// Extractors are called in order to extract fields from context during logging.
// If the logger has no extractors, the provided extractor becomes the first one.
// Returns ErrNilExtractor if the extractor is nil, or ErrLoggerClosed if the logger is closed.
func (l *Logger) AddContextExtractor(extractor ContextExtractor) error {
	if extractor == nil {
		return ErrNilExtractor
	}
	if l.closed.Load() {
		return ErrLoggerClosed
	}

	// Load existing registry or create new one
	var registry *ContextExtractorRegistry
	if v := l.contextExtractors.Load(); v != nil {
		registry = v.(*ContextExtractorRegistry).Clone()
	} else {
		registry = NewContextExtractorRegistry()
	}

	registry.Add(extractor)
	l.contextExtractors.Store(registry)
	return nil
}

// MustAddContextExtractor adds a context extractor and panics on error.
// This is useful for initialization code where failure should be fatal.
func (l *Logger) MustAddContextExtractor(extractor ContextExtractor) {
	if err := l.AddContextExtractor(extractor); err != nil {
		panic(fmt.Sprintf("dd: failed to add context extractor: %v", err))
	}
}

// SetContextExtractors replaces all context extractors with the provided list (thread-safe).
// Pass no arguments to clear all extractors (which will use default behavior).
// Returns ErrLoggerClosed if the logger is closed.
func (l *Logger) SetContextExtractors(extractors ...ContextExtractor) error {
	if l.closed.Load() {
		return ErrLoggerClosed
	}

	if len(extractors) == 0 {
		l.contextExtractors.Store(nil)
		return nil
	}

	registry := NewContextExtractorRegistry()
	for _, extractor := range extractors {
		registry.Add(extractor)
	}
	l.contextExtractors.Store(registry)
	return nil
}

// MustSetContextExtractors sets context extractors and panics on error.
// This is useful for initialization code where failure should be fatal.
func (l *Logger) MustSetContextExtractors(extractors ...ContextExtractor) {
	if err := l.SetContextExtractors(extractors...); err != nil {
		panic(fmt.Sprintf("dd: failed to set context extractors: %v", err))
	}
}

// GetContextExtractors returns a copy of the current context extractors (thread-safe).
// Returns nil if no custom extractors are registered.
func (l *Logger) GetContextExtractors() []ContextExtractor {
	if v := l.contextExtractors.Load(); v != nil {
		registry := v.(*ContextExtractorRegistry)
		extractorsPtr := registry.extractorsPtr.Load()
		if extractorsPtr == nil {
			return nil
		}
		extractors := make([]ContextExtractor, len(*extractorsPtr))
		copy(extractors, *extractorsPtr)
		return extractors
	}
	return nil
}

// AddHook registers a hook for a specific event type (thread-safe).
// Hooks are called in order during the logging lifecycle.
// Returns ErrNilHook if the hook is nil, or ErrLoggerClosed if the logger is closed.
func (l *Logger) AddHook(event HookEvent, hook Hook) error {
	if hook == nil {
		return ErrNilHook
	}
	if l.closed.Load() {
		return ErrLoggerClosed
	}

	l.hooksMu.Lock()
	defer l.hooksMu.Unlock()

	// Load existing registry or create new one
	var registry *HookRegistry
	if v := l.hooks.Load(); v != nil {
		registry = v.(*HookRegistry).Clone()
	} else {
		registry = NewHookRegistry()
	}

	registry.Add(event, hook)
	l.hooks.Store(registry)
	return nil
}

// MustAddHook registers a hook and panics on error.
// This is useful for initialization code where failure should be fatal.
func (l *Logger) MustAddHook(event HookEvent, hook Hook) {
	if err := l.AddHook(event, hook); err != nil {
		panic(fmt.Sprintf("dd: failed to add hook: %v", err))
	}
}

// SetHooks replaces the hook registry with the provided one (thread-safe).
// Pass nil to clear all hooks.
// Returns ErrLoggerClosed if the logger is closed.
func (l *Logger) SetHooks(registry *HookRegistry) error {
	if l.closed.Load() {
		return ErrLoggerClosed
	}

	l.hooksMu.Lock()
	defer l.hooksMu.Unlock()

	if registry == nil {
		l.hooks.Store(nil)
		return nil
	}

	l.hooks.Store(registry.Clone())
	return nil
}

// MustSetHooks sets the hook registry and panics on error.
// This is useful for initialization code where failure should be fatal.
func (l *Logger) MustSetHooks(registry *HookRegistry) {
	if err := l.SetHooks(registry); err != nil {
		panic(fmt.Sprintf("dd: failed to set hooks: %v", err))
	}
}

// GetHooks returns a copy of the current hook registry (thread-safe).
// Returns nil if no hooks are registered.
func (l *Logger) GetHooks() *HookRegistry {
	if v := l.hooks.Load(); v != nil {
		return v.(*HookRegistry).Clone()
	}
	return nil
}

// triggerHooks triggers hooks for the given event and context.
// Returns an error if any hook returns an error.
func (l *Logger) triggerHooks(ctx context.Context, hookCtx *HookContext) error {
	if v := l.hooks.Load(); v != nil {
		registry := v.(*HookRegistry)
		return registry.Trigger(ctx, hookCtx.Event, hookCtx)
	}
	return nil
}

// AddWriter adds a writer to the logger in a thread-safe manner.
func (l *Logger) AddWriter(writer io.Writer) error {
	if writer == nil {
		return ErrNilWriter
	}

	if l.closed.Load() {
		return ErrLoggerClosed
	}

	l.writersMu.Lock()
	defer l.writersMu.Unlock()

	// Load current writers slice
	currentWriters := l.writersPtr.Load()
	if currentWriters == nil {
		return ErrLoggerClosed
	}

	if len(*currentWriters) >= MaxWriterCount {
		return ErrMaxWritersExceeded
	}

	// Create new slice with the new writer added
	newWriters := make([]io.Writer, len(*currentWriters)+1)
	copy(newWriters, *currentWriters)
	newWriters[len(*currentWriters)] = writer

	// Atomically swap the pointer
	l.writersPtr.Store(&newWriters)
	return nil
}

// RemoveWriter removes a writer from the logger in a thread-safe manner.
func (l *Logger) RemoveWriter(writer io.Writer) error {
	if writer == nil {
		return ErrNilWriter
	}

	if l.closed.Load() {
		return ErrLoggerClosed
	}

	l.writersMu.Lock()
	defer l.writersMu.Unlock()

	// Load current writers slice
	currentWriters := l.writersPtr.Load()
	if currentWriters == nil {
		return ErrLoggerClosed
	}

	writerCount := len(*currentWriters)
	for i := 0; i < writerCount; i++ {
		if (*currentWriters)[i] == writer {
			// Create new slice without the removed writer
			newWriters := make([]io.Writer, writerCount-1)
			copy(newWriters, (*currentWriters)[:i])
			copy(newWriters[i:], (*currentWriters)[i+1:])

			// Atomically swap the pointer
			l.writersPtr.Store(&newWriters)
			return nil
		}
	}

	return ErrWriterNotFound
}

// Close closes the logger and all associated resources (thread-safe).
// If multiple writers fail to close, all errors are collected and returned.
// Triggers OnClose hooks before closing writers.
func (l *Logger) Close() error {
	if !l.closed.CompareAndSwap(false, true) {
		return nil
	}

	// Trigger OnClose hook
	hookCtx := &HookContext{
		Event:     HookOnClose,
		Timestamp: time.Now(),
	}
	_ = l.triggerHooks(context.Background(), hookCtx)

	l.cancel()

	l.writersMu.Lock()
	defer l.writersMu.Unlock()

	// Load and clear writers atomically
	currentWriters := l.writersPtr.Swap(nil)
	if currentWriters == nil {
		return nil
	}

	var errs []error
	for _, writer := range *currentWriters {
		if closer, ok := writer.(io.Closer); ok {
			if writer != os.Stdout && writer != os.Stderr && writer != os.Stdin {
				if err := closer.Close(); err != nil {
					errs = append(errs, fmt.Errorf("failed to close writer: %w", err))
				}
			}
		}
	}

	return errors.Join(errs...)
}

// shouldLog checks if a message should be logged based on level and logger state
func (l *Logger) shouldLog(level LogLevel) bool {
	// Check dynamic level resolver first
	if resolver := l.getLevelResolver(); resolver != nil {
		// Use context.Background() as default to prevent nil pointer panics
		effectiveLevel := resolver(context.Background())
		if level < effectiveLevel || level > LevelFatal {
			return false
		}
	} else {
		// Use static level
		currentLevel := LogLevel(l.level.Load())
		if level < currentLevel || level > LevelFatal {
			return false
		}
	}
	if l.closed.Load() {
		return false
	}
	return l.shouldSample()
}

// shouldLogCtx checks if a log entry should be logged with context support.
// If a level resolver is set, it uses the context for dynamic level determination.
func (l *Logger) shouldLogCtx(ctx context.Context, level LogLevel) bool {
	// Check dynamic level resolver first
	if resolver := l.getLevelResolver(); resolver != nil {
		effectiveLevel := resolver(ctx)
		if level < effectiveLevel || level > LevelFatal {
			return false
		}
	} else {
		// Use static level
		currentLevel := LogLevel(l.level.Load())
		if level < currentLevel || level > LevelFatal {
			return false
		}
	}
	if l.closed.Load() {
		return false
	}
	return l.shouldSample()
}

// getLevelResolver safely returns the level resolver function.
func (l *Logger) getLevelResolver() LevelResolver {
	if ptr := l.levelResolver.Load(); ptr != nil {
		return *ptr
	}
	return nil
}

// shouldSample determines if a log message should be recorded based on sampling configuration.
// Returns true if:
//   - Sampling is disabled (default)
//   - The counter is less than Initial
//   - The counter modulo Thereafter equals 0
//
// Thread-safe using atomic operations for counter and mutex only for tick reset.
func (l *Logger) shouldSample() bool {
	v := l.sampling.Load()
	if v == nil {
		return true // No sampling configured
	}

	state := v.(*samplingState)
	if state.config == nil || !state.config.Enabled {
		return true
	}

	// Check if tick interval has passed and reset if needed
	// This is the only part that needs mutex protection
	if state.config.Tick > 0 {
		state.startMu.Lock()
		if time.Since(state.start) >= state.config.Tick {
			state.counter.Store(0)
			state.start = time.Now()
		}
		state.startMu.Unlock()
	}

	// Atomic increment - no mutex needed
	count := state.counter.Add(1)

	// Always log the first Initial messages
	if count <= int64(state.config.Initial) {
		return true
	}

	// Log 1 out of every Thereafter messages after Initial
	if state.config.Thereafter > 0 {
		return (count-int64(state.config.Initial))%int64(state.config.Thereafter) == 0
	}

	// If Thereafter is 0 after Initial, don't log anymore
	return false
}

// SetSampling enables or disables log sampling at runtime (thread-safe).
// Pass nil to disable sampling.
func (l *Logger) SetSampling(config *SamplingConfig) {
	if l.closed.Load() {
		return
	}

	if config == nil || !config.Enabled {
		// Don't store nil in atomic.Value - use a disabled state instead
		disabledState := &samplingState{
			config: &SamplingConfig{Enabled: false},
		}
		disabledState.counter.Store(0)
		l.sampling.Store(disabledState)
		return
	}

	// Apply defaults
	if config.Initial < 0 {
		config.Initial = 0
	}
	// Thereafter=0 is valid and means "log nothing after Initial"
	// Thereafter<0 is treated as "log everything" (set to 1)
	if config.Thereafter < 0 {
		config.Thereafter = 1
	}
	if config.Tick <= 0 {
		config.Tick = 0 // No tick reset
	}

	newState := &samplingState{
		config: config,
		start:  time.Now(),
	}
	newState.counter.Store(0)
	l.sampling.Store(newState)
}

// GetSampling returns the current sampling configuration (thread-safe).
// Returns nil if sampling is not enabled.
func (l *Logger) GetSampling() *SamplingConfig {
	v := l.sampling.Load()
	if v == nil {
		return nil
	}
	state := v.(*samplingState)
	if state.config == nil || !state.config.Enabled {
		return nil
	}
	return state.config
}

// getSecurityConfig returns the current security configuration (internal use).
// This returns the original config pointer, not a clone, for efficiency.
// For external access, use GetSecurityConfig() which returns a safe clone.
func (l *Logger) getSecurityConfig() *SecurityConfig {
	if config := l.securityConfig.Load(); config != nil {
		if secConfig, ok := config.(*SecurityConfig); ok {
			return secConfig
		}
	}
	return DefaultSecurityConfig()
}

// Log logs a message at the specified level
func (l *Logger) Log(level LogLevel, args ...any) {
	if !l.shouldLog(level) {
		return
	}

	// Format args to string first, then apply security filtering before adding timestamp/level/caller
	msg := l.formatter.FormatArgsToString(args...)
	msg = l.applyMessageSecurity(msg)

	// Trigger BeforeLog hook
	hookCtx := &HookContext{
		Event:     HookBeforeLog,
		Level:     level,
		Message:   msg,
		Timestamp: time.Now(),
	}
	if err := l.triggerHooks(context.Background(), hookCtx); err != nil {
		return // Hook aborted the log
	}

	message := l.formatter.FormatWithMessage(level, l.callerDepth, msg, nil)
	l.writeMessage(l.applySizeLimit(message))

	// Trigger AfterLog hook
	hookCtx.Event = HookAfterLog
	_ = l.triggerHooks(context.Background(), hookCtx)

	if level == LevelFatal {
		l.handleFatal()
	}
}

// Logf logs a formatted message at the specified level
func (l *Logger) Logf(level LogLevel, format string, args ...any) {
	if !l.shouldLog(level) {
		return
	}

	// Format with sprintf first, then apply security filtering before adding timestamp/level/caller
	msg := fmt.Sprintf(format, args...)
	msg = l.applyMessageSecurity(msg)

	// Trigger BeforeLog hook
	hookCtx := &HookContext{
		Event:     HookBeforeLog,
		Level:     level,
		Message:   msg,
		Timestamp: time.Now(),
	}
	if err := l.triggerHooks(context.Background(), hookCtx); err != nil {
		return // Hook aborted the log
	}

	message := l.formatter.FormatWithMessage(level, l.callerDepth, msg, nil)
	l.writeMessage(l.applySizeLimit(message))

	// Trigger AfterLog hook
	hookCtx.Event = HookAfterLog
	_ = l.triggerHooks(context.Background(), hookCtx)

	if level == LevelFatal {
		l.handleFatal()
	}
}

// LogWith logs a structured message with fields at the specified level
func (l *Logger) LogWith(level LogLevel, msg string, fields ...Field) {
	if !l.shouldLog(level) {
		return
	}

	// Only copy original fields if hooks are registered (they may need them)
	var originalFields []Field
	if l.hooks.Load() != nil && len(fields) > 0 {
		originalFields = make([]Field, len(fields))
		copy(originalFields, fields)
	}

	msg = l.applyMessageSecurity(msg)
	processedFields := l.processFields(fields)

	// Trigger BeforeLog hook
	hookCtx := &HookContext{
		Event:          HookBeforeLog,
		Level:          level,
		Message:        msg,
		Fields:         processedFields,
		OriginalFields: originalFields,
		Timestamp:      time.Now(),
	}
	if err := l.triggerHooks(context.Background(), hookCtx); err != nil {
		return // Hook aborted the log
	}

	message := l.formatter.FormatWithMessage(level, l.callerDepth, msg, toInternalFields(processedFields))
	l.writeMessage(l.applySizeLimit(message))

	// Trigger AfterLog hook
	hookCtx.Event = HookAfterLog
	_ = l.triggerHooks(context.Background(), hookCtx)

	if level == LevelFatal {
		l.handleFatal()
	}
}

// toInternalFields converts dd.Field slice to internal.Field slice.
// Since Field is a type alias for internal.Field, we can use direct slice conversion.
func toInternalFields(fields []Field) []internal.Field {
	return fields
}

// processFields processes and filters structured fields
func (l *Logger) processFields(fields []Field) []Field {
	if len(fields) == 0 {
		return fields
	}

	// Validate field keys if validation is enabled
	l.validateFields(fields)

	secConfig := l.getSecurityConfig()
	if secConfig == nil || secConfig.SensitiveFilter == nil || !secConfig.SensitiveFilter.IsEnabled() {
		return fields // Early return - no allocation
	}

	// Quick check: if no patterns and no sensitive keys, skip processing
	if secConfig.SensitiveFilter.PatternCount() == 0 {
		// Still need to check for sensitive keys
		hasSensitiveKey := false
		for _, field := range fields {
			if internal.IsSensitiveKey(field.Key) {
				hasSensitiveKey = true
				break
			}
		}
		if !hasSensitiveKey {
			return fields // No patterns and no sensitive keys - return original
		}
	}

	// First pass: check if any field actually needs filtering
	// This avoids allocation when all values are non-sensitive
	needsFiltering := false
	for _, field := range fields {
		// Check if key is sensitive
		if internal.IsSensitiveKey(field.Key) {
			needsFiltering = true
			break
		}
		// Check if value is a string that might contain sensitive data
		if _, ok := field.Value.(string); ok && secConfig.SensitiveFilter.PatternCount() > 0 {
			needsFiltering = true
			break
		}
		// Check for complex types that might need recursive filtering
		if internal.IsComplexValue(field.Value) {
			needsFiltering = true
			break
		}
	}

	// If no field needs filtering, return original slice
	if !needsFiltering {
		return fields
	}

	// Only allocate when actually filtering
	filtered := make([]Field, len(fields))
	for i, field := range fields {
		filtered[i] = Field{
			Key:   field.Key,
			Value: secConfig.SensitiveFilter.FilterValueRecursive(field.Key, field.Value),
		}
	}

	return filtered
}

// applyMessageSecurity applies sensitive data filtering to the raw message (before formatting)
func (l *Logger) applyMessageSecurity(message string) string {
	secConfig := l.getSecurityConfig()
	if secConfig == nil {
		return internal.SanitizeControlChars(message)
	}

	if secConfig.SensitiveFilter != nil && secConfig.SensitiveFilter.IsEnabled() {
		message = secConfig.SensitiveFilter.Filter(message)
	}

	return internal.SanitizeControlChars(message)
}

// applySizeLimit applies message size limit to the formatted message (after formatting)
func (l *Logger) applySizeLimit(message string) string {
	secConfig := l.getSecurityConfig()
	if secConfig == nil {
		return message
	}

	if secConfig.MaxMessageSize > 0 && len(message) > secConfig.MaxMessageSize {
		message = message[:secConfig.MaxMessageSize] + "..."
	}

	return message
}

// validateFields validates field keys against the configured naming convention.
// In warn mode, validation errors are logged as warnings.
// In strict mode, validation errors are logged as errors.
func (l *Logger) validateFields(fields []Field) {
	fv := l.getFieldValidation()
	if fv == nil || fv.Mode == FieldValidationNone {
		return
	}

	for _, field := range fields {
		if err := fv.ValidateFieldKey(field.Key); err != nil {
			switch fv.Mode {
			case FieldValidationWarn:
				// Log warning without affecting the log output
				fmt.Fprintf(os.Stderr, "dd: field validation warning: %v\n", err)
			case FieldValidationStrict:
				// Log error without affecting the log output
				fmt.Fprintf(os.Stderr, "dd: field validation error: %v\n", err)
			}
		}
	}
}

// getFieldValidation safely returns the field validation configuration.
func (l *Logger) getFieldValidation() *FieldValidationConfig {
	if ptr := l.fieldValidation.Load(); ptr != nil {
		return ptr
	}
	return nil
}

// SetFieldValidation sets the field validation configuration (thread-safe).
// This allows runtime adjustment of field key validation.
//
// Example:
//
//	// Enable strict snake_case validation
//	logger.SetFieldValidation(dd.StrictSnakeCaseConfig())
func (l *Logger) SetFieldValidation(config *FieldValidationConfig) {
	if config == nil || config.Mode == FieldValidationNone {
		l.fieldValidation.Store(nil)
	} else {
		l.fieldValidation.Store(config)
	}
}

// GetFieldValidation returns the current field validation configuration.
// Returns nil if no validation is configured.
func (l *Logger) GetFieldValidation() *FieldValidationConfig {
	return l.getFieldValidation()
}

// handleFatal handles fatal log messages with timeout protection.
// If Close() takes longer than DefaultFatalFlushTimeout, a warning is printed
// and the program exits anyway to prevent indefinite hanging.
func (l *Logger) handleFatal() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = l.Close()
	}()

	select {
	case <-done:
		// Close completed successfully
	case <-time.After(DefaultFatalFlushTimeout):
		fmt.Fprintln(os.Stderr, "[dd] Warning: logger close timed out after 5 seconds")
	}

	if l.fatalHandler != nil {
		l.fatalHandler()
	} else {
		os.Exit(1)
	}
}

// writeMessage writes a message to all configured writers
func (l *Logger) writeMessage(message string) {
	if l.closed.Load() || len(message) == 0 {
		return
	}

	bufPtr := messagePool.Get().(*[]byte)
	buf := *bufPtr
	defer func() {
		if cap(buf) <= MaxBufferSize {
			*bufPtr = buf[:0]
		} else {
			// Reset to default capacity to avoid holding large buffers in the pool
			// This prevents memory leaks while still returning the pointer to the pool
			*bufPtr = make([]byte, 0, DefaultBufferSize)
		}
		messagePool.Put(bufPtr)
	}()

	needed := len(message) + 1
	if cap(buf) < needed {
		buf = make([]byte, 0, max(needed, DefaultBufferSize))
	} else {
		buf = buf[:0]
	}

	buf = append(buf, message...)
	buf = append(buf, '\n')

	// Load writers slice atomically - no mutex needed for reading
	writersPtr := l.writersPtr.Load()
	if writersPtr == nil || len(*writersPtr) == 0 {
		return
	}

	writers := *writersPtr
	writerCount := len(writers)

	if writerCount == 1 {
		w := writers[0]
		if _, err := w.Write(buf); err != nil {
			l.handleWriteError(w, err)
		}
		return
	}

	// Iterate directly over the immutable slice - no copy needed
	for _, writer := range writers {
		if _, err := writer.Write(buf); err != nil {
			l.handleWriteError(writer, err)
		}
	}
}

// handleWriteError handles write errors by calling both legacy handler and hooks.
func (l *Logger) handleWriteError(writer io.Writer, err error) {
	// Call legacy write error handler
	if handler := l.getWriteErrorHandler(); handler != nil {
		handler(writer, err)
	}

	// Trigger OnError hook
	hookCtx := &HookContext{
		Event:     HookOnError,
		Error:     err,
		Writer:    writer,
		Timestamp: time.Now(),
	}
	_ = l.triggerHooks(context.Background(), hookCtx)
}

func (l *Logger) Debug(args ...any) { l.Log(LevelDebug, args...) }
func (l *Logger) Info(args ...any)  { l.Log(LevelInfo, args...) }
func (l *Logger) Warn(args ...any)  { l.Log(LevelWarn, args...) }
func (l *Logger) Error(args ...any) { l.Log(LevelError, args...) }

// Fatal logs a message at FATAL level and terminates the program via os.Exit(1).
// WARNING: defer statements will NOT execute. For graceful shutdown, use Error() with custom logic.
func (l *Logger) Fatal(args ...any) { l.Log(LevelFatal, args...) }

func (l *Logger) Debugf(format string, args ...any) { l.Logf(LevelDebug, format, args...) }
func (l *Logger) Infof(format string, args ...any)  { l.Logf(LevelInfo, format, args...) }
func (l *Logger) Warnf(format string, args ...any)  { l.Logf(LevelWarn, format, args...) }
func (l *Logger) Errorf(format string, args ...any) { l.Logf(LevelError, format, args...) }

// Fatalf logs a formatted message at FATAL level and terminates the program via os.Exit(1).
// WARNING: defer statements will NOT execute. For graceful shutdown, use Errorf() with custom logic.
func (l *Logger) Fatalf(format string, args ...any) { l.Logf(LevelFatal, format, args...) }

func (l *Logger) DebugWith(msg string, fields ...Field) { l.LogWith(LevelDebug, msg, fields...) }
func (l *Logger) InfoWith(msg string, fields ...Field)  { l.LogWith(LevelInfo, msg, fields...) }
func (l *Logger) WarnWith(msg string, fields ...Field)  { l.LogWith(LevelWarn, msg, fields...) }
func (l *Logger) ErrorWith(msg string, fields ...Field) { l.LogWith(LevelError, msg, fields...) }

// extractContextFields extracts tracing fields from context using registered extractors.
// If no custom extractors are registered, uses default extractors for trace_id, span_id, request_id.
func (l *Logger) extractContextFields(ctx context.Context) []Field {
	if ctx == nil {
		return nil
	}

	// Check if custom extractors are registered
	if v := l.contextExtractors.Load(); v != nil {
		registry := v.(*ContextExtractorRegistry)
		return registry.Extract(ctx)
	}

	// Fall back to default extractors
	return DefaultContextExtractorRegistry().Extract(ctx)
}

// LogCtx logs a message at the specified level with context support.
func (l *Logger) LogCtx(ctx context.Context, level LogLevel, args ...any) {
	if !l.shouldLogCtx(ctx, level) {
		return
	}

	msg := l.formatter.FormatArgsToString(args...)
	msg = l.applyMessageSecurity(msg)
	fields := l.extractContextFields(ctx)

	// Trigger BeforeLog hook
	hookCtx := &HookContext{
		Event:     HookBeforeLog,
		Level:     level,
		Message:   msg,
		Fields:    fields,
		Timestamp: time.Now(),
	}
	if err := l.triggerHooks(ctx, hookCtx); err != nil {
		return // Hook aborted the log
	}

	message := l.formatter.FormatWithMessage(level, l.callerDepth, msg, toInternalFields(fields))
	l.writeMessage(l.applySizeLimit(message))

	// Trigger AfterLog hook
	hookCtx.Event = HookAfterLog
	_ = l.triggerHooks(ctx, hookCtx)

	if level == LevelFatal {
		l.handleFatal()
	}
}

// LogfCtx logs a formatted message with context support.
func (l *Logger) LogfCtx(ctx context.Context, level LogLevel, format string, args ...any) {
	if !l.shouldLogCtx(ctx, level) {
		return
	}

	msg := fmt.Sprintf(format, args...)
	msg = l.applyMessageSecurity(msg)
	fields := l.extractContextFields(ctx)

	// Trigger BeforeLog hook
	hookCtx := &HookContext{
		Event:     HookBeforeLog,
		Level:     level,
		Message:   msg,
		Fields:    fields,
		Timestamp: time.Now(),
	}
	if err := l.triggerHooks(ctx, hookCtx); err != nil {
		return // Hook aborted the log
	}

	message := l.formatter.FormatWithMessage(level, l.callerDepth, msg, toInternalFields(fields))
	l.writeMessage(l.applySizeLimit(message))

	// Trigger AfterLog hook
	hookCtx.Event = HookAfterLog
	_ = l.triggerHooks(ctx, hookCtx)

	if level == LevelFatal {
		l.handleFatal()
	}
}

// LogWithCtx logs a structured message with fields and context support.
func (l *Logger) LogWithCtx(ctx context.Context, level LogLevel, msg string, fields ...Field) {
	if !l.shouldLogCtx(ctx, level) {
		return
	}

	// Save original fields before processing
	originalFields := make([]Field, len(fields))
	copy(originalFields, fields)

	msg = l.applyMessageSecurity(msg)
	processedFields := l.processFields(fields)
	contextFields := l.extractContextFields(ctx)
	allFields := append(contextFields, processedFields...)

	// Trigger BeforeLog hook
	hookCtx := &HookContext{
		Event:          HookBeforeLog,
		Level:          level,
		Message:        msg,
		Fields:         allFields,
		OriginalFields: originalFields,
		Timestamp:      time.Now(),
	}
	if err := l.triggerHooks(ctx, hookCtx); err != nil {
		return // Hook aborted the log
	}

	message := l.formatter.FormatWithMessage(level, l.callerDepth, msg, toInternalFields(allFields))
	l.writeMessage(l.applySizeLimit(message))

	// Trigger AfterLog hook
	hookCtx.Event = HookAfterLog
	_ = l.triggerHooks(ctx, hookCtx)

	if level == LevelFatal {
		l.handleFatal()
	}
}

// Context-aware convenience methods
func (l *Logger) DebugCtx(ctx context.Context, args ...any) { l.LogCtx(ctx, LevelDebug, args...) }
func (l *Logger) InfoCtx(ctx context.Context, args ...any)  { l.LogCtx(ctx, LevelInfo, args...) }
func (l *Logger) WarnCtx(ctx context.Context, args ...any)  { l.LogCtx(ctx, LevelWarn, args...) }
func (l *Logger) ErrorCtx(ctx context.Context, args ...any) { l.LogCtx(ctx, LevelError, args...) }

func (l *Logger) DebugfCtx(ctx context.Context, format string, args ...any) {
	l.LogfCtx(ctx, LevelDebug, format, args...)
}
func (l *Logger) InfofCtx(ctx context.Context, format string, args ...any) {
	l.LogfCtx(ctx, LevelInfo, format, args...)
}
func (l *Logger) WarnfCtx(ctx context.Context, format string, args ...any) {
	l.LogfCtx(ctx, LevelWarn, format, args...)
}
func (l *Logger) ErrorfCtx(ctx context.Context, format string, args ...any) {
	l.LogfCtx(ctx, LevelError, format, args...)
}

func (l *Logger) DebugWithCtx(ctx context.Context, msg string, fields ...Field) {
	l.LogWithCtx(ctx, LevelDebug, msg, fields...)
}
func (l *Logger) InfoWithCtx(ctx context.Context, msg string, fields ...Field) {
	l.LogWithCtx(ctx, LevelInfo, msg, fields...)
}
func (l *Logger) WarnWithCtx(ctx context.Context, msg string, fields ...Field) {
	l.LogWithCtx(ctx, LevelWarn, msg, fields...)
}
func (l *Logger) ErrorWithCtx(ctx context.Context, msg string, fields ...Field) {
	l.LogWithCtx(ctx, LevelError, msg, fields...)
}

// FatalCtx logs a message at FATAL level with context and terminates the program via os.Exit(1).
// WARNING: defer statements will NOT execute. For graceful shutdown, use ErrorCtx() with custom logic.
func (l *Logger) FatalCtx(ctx context.Context, args ...any) {
	l.LogCtx(ctx, LevelFatal, args...)
}

// FatalfCtx logs a formatted message at FATAL level with context and terminates the program via os.Exit(1).
// WARNING: defer statements will NOT execute. For graceful shutdown, use ErrorfCtx() with custom logic.
func (l *Logger) FatalfCtx(ctx context.Context, format string, args ...any) {
	l.LogfCtx(ctx, LevelFatal, format, args...)
}

// FatalWithCtx logs a structured message at FATAL level with context and terminates the program via os.Exit(1).
// WARNING: defer statements will NOT execute. For graceful shutdown, use ErrorWithCtx() with custom logic.
func (l *Logger) FatalWithCtx(ctx context.Context, msg string, fields ...Field) {
	l.LogWithCtx(ctx, LevelFatal, msg, fields...)
}

// FatalWith logs a structured message at FATAL level and terminates the program via os.Exit(1).
// WARNING: defer statements will NOT execute. For graceful shutdown, use ErrorWith() with custom logic.
func (l *Logger) FatalWith(msg string, fields ...Field) { l.LogWith(LevelFatal, msg, fields...) }

// IsClosed returns true if the logger has been closed (thread-safe).
func (l *Logger) IsClosed() bool {
	return l.closed.Load()
}

// WriterCount returns the number of registered writers (thread-safe).
func (l *Logger) WriterCount() int {
	writersPtr := l.writersPtr.Load()
	if writersPtr == nil {
		return 0
	}
	return len(*writersPtr)
}

// ActiveFilterGoroutines returns the number of currently active filter goroutines
// in the security filter. This can be used for monitoring and detecting potential
// goroutine leaks in high-concurrency scenarios. A consistently high count may
// indicate that filter operations are timing out frequently.
func (l *Logger) ActiveFilterGoroutines() int32 {
	secConfig := l.getSecurityConfig()
	if secConfig == nil || secConfig.SensitiveFilter == nil {
		return 0
	}
	return secConfig.SensitiveFilter.ActiveGoroutineCount()
}

// WaitForFilterGoroutines waits for all active filter goroutines to complete
// or until the timeout is reached.
//
// IMPORTANT: Call this method before Close() in graceful shutdown scenarios
// to prevent goroutine leaks. The security filter may spawn background goroutines
// for processing large inputs with regex patterns. Failing to wait for these
// goroutines can result in resource leaks.
//
// Example graceful shutdown:
//
//	// 1. Stop accepting new requests/logs
//	// 2. Wait for filter goroutines (use 2-5 seconds typically)
//	if !logger.WaitForFilterGoroutines(3 * time.Second) {
//	    log.Println("Warning: some filter goroutines did not complete in time")
//	}
//	// 3. Close the logger
//	logger.Close()
//
// Returns true if all goroutines completed, false if timeout was reached.
func (l *Logger) WaitForFilterGoroutines(timeout time.Duration) bool {
	secConfig := l.getSecurityConfig()
	if secConfig == nil || secConfig.SensitiveFilter == nil {
		return true
	}
	return secConfig.SensitiveFilter.WaitForGoroutines(timeout)
}

// Flush flushes all buffered writers (thread-safe).
// Writers that implement Flusher interface will be flushed.
func (l *Logger) Flush() error {
	writersPtr := l.writersPtr.Load()
	if writersPtr == nil {
		return nil
	}

	var firstErr error
	for _, w := range *writersPtr {
		if flusher, ok := w.(Flusher); ok {
			if err := flusher.Flush(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// fmt package replacement methods - output via logger's writers with caller info
//
// IMPORTANT: These Logger methods are DIFFERENT from the package-level dd.Print functions!
//
//	logger.Print()  -> writes to configured writers with security filtering
//	dd.Print()      -> writes directly to stdout WITHOUT security filtering (debug only)
//
// Always use logger.Print/Printf/Println for production logging.

// Print writes to configured writers with caller info and newline.
// Uses LevelDebug for filtering. Arguments are joined with spaces.
// Applies sensitive data filtering based on SecurityConfig.
// Note: Both Print() and Println() behave identically because Log() already adds a newline.
func (l *Logger) Print(args ...any) {
	l.Log(LevelDebug, args...)
}

// Println writes to configured writers with caller info, spaces between operands, and a newline.
// Uses LevelDebug for filtering. Applies sensitive data filtering based on SecurityConfig.
// Note: Behaves identically to Print() because Log() already adds a newline.
func (l *Logger) Println(args ...any) {
	l.Log(LevelDebug, args...)
}

// Printf formats according to a format specifier and writes to configured writers with caller info.
// Uses LevelDebug for filtering.
func (l *Logger) Printf(format string, args ...any) {
	l.Logf(LevelDebug, format, args...)
}

// Debug utilities - Text and JSON output for debugging

// Text outputs data as pretty-printed format to stdout for debugging.
//
// SECURITY WARNING: This method does NOT apply sensitive data filtering.
// Do not use with sensitive data in production environments. For secure logging,
// use logger.Info(), logger.Debug(), etc. which apply sensitive data filtering.
func (l *Logger) Text(data ...any) {
	internal.OutputTextData(os.Stdout, data...)
}

func (l *Logger) Textf(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stdout, formatted)
}

func (l *Logger) JSON(data ...any) {
	caller := internal.GetCaller(DebugVisualizationDepth, false)
	internal.OutputJSON(os.Stdout, caller, data...)
}

func (l *Logger) JSONF(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	caller := internal.GetCaller(DebugVisualizationDepth, false)
	internal.OutputJSON(os.Stdout, caller, formatted)
}

var (
	defaultLogger       atomic.Pointer[Logger]
	defaultOnce         sync.Once
	defaultInitErr      atomic.Value // stores error from initialization
	defaultUsedFallback atomic.Bool  // true if fallback logger was created
)

// DefaultInitError returns the error that occurred during default logger initialization.
// Returns nil if initialization was successful or hasn't occurred yet.
// This allows applications to detect if the default logger is running in fallback mode.
//
// Example:
//
//	logger := dd.Default()
//	if err := dd.DefaultInitError(); err != nil {
//	    log.Printf("Warning: default logger initialized with error: %v", err)
//	}
func DefaultInitError() error {
	if v := defaultInitErr.Load(); v != nil {
		return v.(error)
	}
	return nil
}

// DefaultUsedFallback returns true if the default logger was created using
// a fallback configuration due to an initialization error.
// This indicates the default logger may not be configured as expected.
func DefaultUsedFallback() bool {
	return defaultUsedFallback.Load()
}

// DefaultWithErr returns the default logger and any initialization error.
// This is useful when you need to verify the default logger was created correctly.
//
// Example:
//
//	logger, err := dd.DefaultWithErr()
//	if err != nil {
//	    // Handle initialization error
//	    log.Fatalf("Failed to initialize default logger: %v", err)
//	}
//	logger.Info("Application started")
func DefaultWithErr() (*Logger, error) {
	return Default(), DefaultInitError()
}

// Default returns the default global logger (thread-safe).
// The logger is created on first call with default configuration.
// Package-level convenience functions use this logger.
// Note: If SetDefault() is called before Default(), the custom logger is returned.
//
// To check if the default logger was initialized correctly, use DefaultInitError()
// or DefaultWithErr():
//
//	if err := dd.DefaultInitError(); err != nil {
//	    // Logger is running in fallback mode
//	}
func Default() *Logger {
	if logger := defaultLogger.Load(); logger != nil {
		return logger
	}

	defaultOnce.Do(func() {
		// Only create if not already set by SetDefault()
		if defaultLogger.Load() == nil {
			logger, err := New()
			if err != nil {
				// Store the error for later retrieval
				defaultInitErr.Store(err)
				defaultUsedFallback.Store(true)

				// Print warning to stderr about fallback logger creation
				fmt.Fprintf(os.Stderr, "[dd] WARNING: Default logger initialization failed: %v\n", err)
				fmt.Fprintln(os.Stderr, "[dd] WARNING: Using fallback logger with stderr output")

				// Create fallback logger with proper initialization if New() fails
				fallbackCfg := defaultConfig()
				ctx, cancel := context.WithCancel(context.Background())
				fallbackWriters := []io.Writer{os.Stderr}
				formatterConfig := &internal.FormatterConfig{
					Format:        internal.LogFormat(fallbackCfg.Format),
					TimeFormat:    fallbackCfg.TimeFormat,
					IncludeTime:   fallbackCfg.IncludeTime,
					IncludeLevel:  fallbackCfg.IncludeLevel,
					FullPath:      fallbackCfg.FullPath,
					DynamicCaller: fallbackCfg.DynamicCaller,
					JSON:          fallbackCfg.JSON,
				}
				logger = &Logger{
					callerDepth: DefaultCallerDepth,
					formatter:   internal.NewMessageFormatter(formatterConfig),
					ctx:         ctx,
					cancel:      cancel,
				}
				logger.writersPtr.Store(&fallbackWriters)
				logger.level.Store(int32(fallbackCfg.Level))
				logger.securityConfig.Store(fallbackCfg.Security)
			}
			defaultLogger.Store(logger)
		}
	})

	return defaultLogger.Load()
}

// SetDefault sets the default global logger (thread-safe).
// If a previous default logger exists, it is safely closed in background.
// Passing nil is ignored (no change).
func SetDefault(logger *Logger) {
	if logger == nil {
		return
	}

	oldLogger := defaultLogger.Swap(logger)

	if oldLogger != nil {
		go func() {
			time.Sleep(DefaultLoggerCloseDelay)
			_ = oldLogger.Close()
		}()
	}
}

func Debug(args ...any) { Default().Log(LevelDebug, args...) }
func Info(args ...any)  { Default().Log(LevelInfo, args...) }
func Warn(args ...any)  { Default().Log(LevelWarn, args...) }
func Error(args ...any) { Default().Log(LevelError, args...) }

// Fatal logs a message at FATAL level using the default logger and terminates the program via os.Exit(1).
// WARNING: defer statements will NOT execute. For graceful shutdown, use Error() with custom logic.
func Fatal(args ...any) { Default().Log(LevelFatal, args...) }

func Debugf(format string, args ...any) { Default().Logf(LevelDebug, format, args...) }
func Infof(format string, args ...any)  { Default().Logf(LevelInfo, format, args...) }
func Warnf(format string, args ...any)  { Default().Logf(LevelWarn, format, args...) }
func Errorf(format string, args ...any) { Default().Logf(LevelError, format, args...) }

// Fatalf logs a formatted message at FATAL level using the default logger and terminates the program via os.Exit(1).
// WARNING: defer statements will NOT execute. For graceful shutdown, use Errorf() with custom logic.
func Fatalf(format string, args ...any) { Default().Logf(LevelFatal, format, args...) }

// SetLevel sets the log level for the default logger.
func SetLevel(level LogLevel) {
	if err := Default().SetLevel(level); err != nil {
		// Fallback to stderr since logger might not be available
		fmt.Fprintf(os.Stderr, "Failed to set log level: %v\n", err)
	}
}

// GetLevel returns the current log level of the default logger.
func GetLevel() LogLevel {
	return Default().GetLevel()
}

// ============================================================================
// Generic Level Logging Functions
// ============================================================================

// Log logs a message at the specified level using the default logger.
func Log(level LogLevel, args ...any) { Default().Log(level, args...) }

// Logf logs a formatted message at the specified level using the default logger.
func Logf(level LogLevel, format string, args ...any) {
	Default().Logf(level, format, args...)
}

// LogWith logs a structured message at the specified level using the default logger.
func LogWith(level LogLevel, msg string, fields ...Field) {
	Default().LogWith(level, msg, fields...)
}

// LogCtx logs a message at the specified level with context using the default logger.
func LogCtx(ctx context.Context, level LogLevel, args ...any) {
	Default().LogCtx(ctx, level, args...)
}

// LogfCtx logs a formatted message at the specified level with context using the default logger.
func LogfCtx(ctx context.Context, level LogLevel, format string, args ...any) {
	Default().LogfCtx(ctx, level, format, args...)
}

// LogWithCtx logs a structured message at the specified level with context using the default logger.
func LogWithCtx(ctx context.Context, level LogLevel, msg string, fields ...Field) {
	Default().LogWithCtx(ctx, level, msg, fields...)
}

// ============================================================================
// Level Check Functions
// ============================================================================

// IsLevelEnabled checks if the specified log level is enabled for the default logger.
func IsLevelEnabled(level LogLevel) bool { return Default().IsLevelEnabled(level) }

// IsDebugEnabled checks if DEBUG level is enabled for the default logger.
func IsDebugEnabled() bool { return Default().IsDebugEnabled() }

// IsInfoEnabled checks if INFO level is enabled for the default logger.
func IsInfoEnabled() bool { return Default().IsInfoEnabled() }

// IsWarnEnabled checks if WARN level is enabled for the default logger.
func IsWarnEnabled() bool { return Default().IsWarnEnabled() }

// IsErrorEnabled checks if ERROR level is enabled for the default logger.
func IsErrorEnabled() bool { return Default().IsErrorEnabled() }

// IsFatalEnabled checks if FATAL level is enabled for the default logger.
func IsFatalEnabled() bool { return Default().IsFatalEnabled() }

// ============================================================================
// Field Chaining Functions
// ============================================================================

// WithFields returns a LoggerEntry with pre-set fields using the default logger.
// The fields are inherited by all logging calls on the returned entry.
//
// Example:
//
//	dd.WithFields(dd.String("service", "api"), dd.String("version", "1.0")).
//	    Info("request received")
func WithFields(fields ...Field) *LoggerEntry {
	return Default().WithFields(fields...)
}

// WithField returns a LoggerEntry with a single pre-set field using the default logger.
//
// Example:
//
//	dd.WithField("request_id", "abc123").Info("processing request")
func WithField(key string, value any) *LoggerEntry {
	return Default().WithField(key, value)
}

// ============================================================================
// Lifecycle Functions
// ============================================================================

// Flush flushes any buffered data in the default logger.
func Flush() error { return Default().Flush() }

// ============================================================================
// Writer Management Functions
// ============================================================================

// AddWriter adds a writer to the default logger.
func AddWriter(writer io.Writer) error { return Default().AddWriter(writer) }

// RemoveWriter removes a writer from the default logger.
func RemoveWriter(writer io.Writer) error { return Default().RemoveWriter(writer) }

// WriterCount returns the number of writers in the default logger.
func WriterCount() int { return Default().WriterCount() }

// ============================================================================
// Sampling Functions
// ============================================================================

// SetSampling sets the sampling configuration for the default logger.
func SetSampling(config *SamplingConfig) { Default().SetSampling(config) }

// GetSampling returns the sampling configuration for the default logger.
func GetSampling() *SamplingConfig { return Default().GetSampling() }
