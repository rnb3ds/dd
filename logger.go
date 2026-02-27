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

	// writersPtr stores an immutable slice of writers using atomic pointer.
	// This eliminates slice copying during write operations.
	// The slice is replaced atomically when writers are added/removed.
	writersPtr     atomic.Pointer[[]io.Writer]
	writersMu      sync.Mutex // protects AddWriter/RemoveWriter operations
	securityConfig atomic.Value

	// ctx and cancel provide graceful shutdown for background operations.
	// When Close() is called, cancel() signals all background goroutines
	// (compression, cleanup) to stop. This ensures clean shutdown without
	// leaking goroutines. The context is also used by filter timeout goroutines.
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new Logger with the given configuration.
// All public methods of the returned Logger are goroutine-safe.
func New(configs ...*LoggerConfig) (*Logger, error) {
	var config *LoggerConfig
	if len(configs) == 0 || configs[0] == nil {
		config = DefaultConfig()
	} else {
		config = configs[0]
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid logger configuration: %w", err)
	}

	config.ApplyDefaults()

	ctx, cancel := context.WithCancel(context.Background())

	// Pre-allocate writers slice with expected capacity
	initialWriters := make([]io.Writer, 0, len(config.Writers))

	// Create formatter config from logger config
	formatterConfig := &internal.FormatterConfig{
		Format:        internal.LogFormat(config.Format),
		TimeFormat:    config.TimeFormat,
		IncludeTime:   config.IncludeTime,
		IncludeLevel:  config.IncludeLevel,
		FullPath:      config.FullPath,
		DynamicCaller: config.DynamicCaller,
		JSON:          config.JSON,
	}

	l := &Logger{
		callerDepth:  DefaultCallerDepth,
		fatalHandler: config.FatalHandler,
		formatter:    internal.NewMessageFormatter(formatterConfig),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Initialize writers pointer with empty slice
	l.writersPtr.Store(&initialWriters)

	if config.WriteErrorHandler != nil {
		l.writeErrorHandler.Store(config.WriteErrorHandler)
	}

	l.level.Store(int32(config.Level))
	l.securityConfig.Store(config.SecurityConfig)

	if config.Writers != nil {
		for _, writer := range config.Writers {
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
func (l *Logger) Close() error {
	if !l.closed.CompareAndSwap(false, true) {
		return nil
	}

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
	currentLevel := LogLevel(l.level.Load())
	if level < currentLevel || level > LevelFatal {
		return false
	}
	return !l.closed.Load()
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
	message := l.formatter.FormatWithMessage(level, l.callerDepth, msg, nil)
	l.writeMessage(l.applySizeLimit(message))

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
	message := l.formatter.FormatWithMessage(level, l.callerDepth, msg, nil)
	l.writeMessage(l.applySizeLimit(message))

	if level == LevelFatal {
		l.handleFatal()
	}
}

// LogWith logs a structured message with fields at the specified level
func (l *Logger) LogWith(level LogLevel, msg string, fields ...Field) {
	if !l.shouldLog(level) {
		return
	}

	msg = l.applyMessageSecurity(msg)
	processedFields := l.processFields(fields)
	message := l.formatter.FormatWithMessage(level, l.callerDepth, msg, toInternalFields(processedFields))
	l.writeMessage(l.applySizeLimit(message))

	if level == LevelFatal {
		l.handleFatal()
	}
}

// toInternalFields converts dd.Field slice to internal.Field slice
// Uses stack allocation for small field counts (<=8) to avoid heap allocation
func toInternalFields(fields []Field) []internal.Field {
	if len(fields) == 0 {
		return nil
	}

	// For small field counts (90% of cases), use stack-allocated array
	// This avoids heap allocation for typical logging scenarios
	if len(fields) <= 8 {
		var result [8]internal.Field
		for i, f := range fields {
			result[i] = internal.Field{Key: f.Key, Value: f.Value}
		}
		return result[:len(fields)]
	}

	// For larger counts, allocate on heap
	result := make([]internal.Field, len(fields))
	for i, f := range fields {
		result[i] = internal.Field{Key: f.Key, Value: f.Value}
	}
	return result
}

// processFields processes and filters structured fields
func (l *Logger) processFields(fields []Field) []Field {
	if len(fields) == 0 {
		return fields
	}

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

// handleFatal handles fatal log messages
func (l *Logger) handleFatal() {
	_ = l.Close()
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
			messagePool.Put(bufPtr)
		}
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
			if handler := l.getWriteErrorHandler(); handler != nil {
				handler(w, err)
			}
		}
		return
	}

	// Iterate directly over the immutable slice - no copy needed
	for _, writer := range writers {
		if _, err := writer.Write(buf); err != nil {
			if handler := l.getWriteErrorHandler(); handler != nil {
				handler(writer, err)
			}
		}
	}
}

func (l *Logger) Debug(args ...any) { l.Log(LevelDebug, args...) }
func (l *Logger) Info(args ...any)  { l.Log(LevelInfo, args...) }
func (l *Logger) Warn(args ...any)  { l.Log(LevelWarn, args...) }
func (l *Logger) Error(args ...any) { l.Log(LevelError, args...) }
func (l *Logger) Fatal(args ...any) { l.Log(LevelFatal, args...) }

func (l *Logger) Debugf(format string, args ...any) { l.Logf(LevelDebug, format, args...) }
func (l *Logger) Infof(format string, args ...any)  { l.Logf(LevelInfo, format, args...) }
func (l *Logger) Warnf(format string, args ...any)  { l.Logf(LevelWarn, format, args...) }
func (l *Logger) Errorf(format string, args ...any) { l.Logf(LevelError, format, args...) }
func (l *Logger) Fatalf(format string, args ...any) { l.Logf(LevelFatal, format, args...) }

func (l *Logger) DebugWith(msg string, fields ...Field) { l.LogWith(LevelDebug, msg, fields...) }
func (l *Logger) InfoWith(msg string, fields ...Field)  { l.LogWith(LevelInfo, msg, fields...) }
func (l *Logger) WarnWith(msg string, fields ...Field)  { l.LogWith(LevelWarn, msg, fields...) }
func (l *Logger) ErrorWith(msg string, fields ...Field) { l.LogWith(LevelError, msg, fields...) }
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
// or until the timeout is reached. This is useful for graceful shutdown to
// ensure all background filtering operations have finished.
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
	defaultLogger atomic.Pointer[Logger]
	defaultOnce   sync.Once
)

// Default returns the default global logger (thread-safe).
// The logger is created on first call with default configuration.
// Package-level convenience functions use this logger.
// Note: If SetDefault() is called before Default(), the custom logger is returned.
func Default() *Logger {
	if logger := defaultLogger.Load(); logger != nil {
		return logger
	}

	defaultOnce.Do(func() {
		// Only create if not already set by SetDefault()
		if defaultLogger.Load() == nil {
			logger, err := New(nil)
			if err != nil {
				// Create fallback logger with proper initialization if New() fails
				defaultConfig := DefaultConfig()
				ctx, cancel := context.WithCancel(context.Background())
				fallbackWriters := []io.Writer{os.Stderr}
				formatterConfig := &internal.FormatterConfig{
					Format:        internal.LogFormat(defaultConfig.Format),
					TimeFormat:    defaultConfig.TimeFormat,
					IncludeTime:   defaultConfig.IncludeTime,
					IncludeLevel:  defaultConfig.IncludeLevel,
					FullPath:      defaultConfig.FullPath,
					DynamicCaller: defaultConfig.DynamicCaller,
					JSON:          defaultConfig.JSON,
				}
				logger = &Logger{
					callerDepth: DefaultCallerDepth,
					formatter:   internal.NewMessageFormatter(formatterConfig),
					ctx:         ctx,
					cancel:      cancel,
				}
				logger.writersPtr.Store(&fallbackWriters)
				logger.level.Store(int32(defaultConfig.Level))
				logger.securityConfig.Store(defaultConfig.SecurityConfig)
			}
			defaultLogger.Store(logger)
		}
	})

	return defaultLogger.Load()
}

// SetDefault sets the default global logger (thread-safe).
// This affects all subsequent calls to package-level convenience functions.
// Passing nil is ignored (no change).
func SetDefault(logger *Logger) {
	if logger != nil {
		defaultLogger.Store(logger)
	}
}

func Debug(args ...any) { Default().Log(LevelDebug, args...) }
func Info(args ...any)  { Default().Log(LevelInfo, args...) }
func Warn(args ...any)  { Default().Log(LevelWarn, args...) }
func Error(args ...any) { Default().Log(LevelError, args...) }
func Fatal(args ...any) { Default().Log(LevelFatal, args...) }

func Debugf(format string, args ...any) { Default().Logf(LevelDebug, format, args...) }
func Infof(format string, args ...any)  { Default().Logf(LevelInfo, format, args...) }
func Warnf(format string, args ...any)  { Default().Logf(LevelWarn, format, args...) }
func Errorf(format string, args ...any) { Default().Logf(LevelError, format, args...) }
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
