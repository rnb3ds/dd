// Package dd provides a high-performance, thread-safe logging library.
package dd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"

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

type Logger struct {
	level  atomic.Int32
	closed atomic.Bool

	callerDepth       int
	fatalHandler      FatalHandler
	writeErrorHandler atomic.Value // stores WriteErrorHandler
	formatter         *MessageFormatter

	writers        []io.Writer
	mu             sync.RWMutex
	securityConfig atomic.Value

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

	l := &Logger{
		callerDepth:  DefaultCallerDepth,
		fatalHandler: config.FatalHandler,
		formatter:    newMessageFormatter(config),
		writers:      make([]io.Writer, 0, len(config.Writers)),
		ctx:          ctx,
		cancel:       cancel,
	}

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

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.writers) >= MaxWriterCount {
		return ErrMaxWritersExceeded
	}

	l.writers = append(l.writers, writer)
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

	l.mu.Lock()
	defer l.mu.Unlock()

	writerCount := len(l.writers)
	for i := 0; i < writerCount; i++ {
		if l.writers[i] == writer {
			l.writers[i] = l.writers[writerCount-1]
			l.writers[writerCount-1] = nil
			l.writers = l.writers[:writerCount-1]
			return nil
		}
	}

	return ErrWriterNotFound
}

// Close closes the logger and all associated resources (thread-safe).
func (l *Logger) Close() error {
	if !l.closed.CompareAndSwap(false, true) {
		return nil
	}

	l.cancel()

	l.mu.Lock()
	defer l.mu.Unlock()

	var closeErr error
	for _, writer := range l.writers {
		if closer, ok := writer.(io.Closer); ok {
			if writer != os.Stdout && writer != os.Stderr && writer != os.Stdin {
				if err := closer.Close(); err != nil && closeErr == nil {
					closeErr = fmt.Errorf("failed to close writer: %w", err)
				}
			}
		}
	}

	l.writers = nil
	return closeErr
}

// shouldLog checks if a message should be logged based on level and logger state
func (l *Logger) shouldLog(level LogLevel) bool {
	currentLevel := LogLevel(l.level.Load())
	if level < currentLevel || level > LevelFatal {
		return false
	}
	return !l.closed.Load()
}

// getSecurityConfig returns the current security configuration
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

	// Format message with space-separated arguments
	message := l.formatter.formatMessage(level, l.callerDepth, args...)
	message = l.applyMessageSecurity(message)
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

	msg := fmt.Sprintf(format, args...)
	msg = l.applyMessageSecurity(msg)
	message := l.formatter.formatMessage(level, l.callerDepth, msg)
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
	message := l.formatter.formatMessageWith(level, l.callerDepth, msg, processedFields)
	l.writeMessage(l.applySizeLimit(message))

	if level == LevelFatal {
		l.handleFatal()
	}
}

// processFields processes and filters structured fields
func (l *Logger) processFields(fields []Field) []Field {
	if len(fields) == 0 {
		return fields
	}

	secConfig := l.getSecurityConfig()
	if secConfig == nil || secConfig.SensitiveFilter == nil || !secConfig.SensitiveFilter.IsEnabled() {
		return fields
	}

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
		return sanitizeControlChars(message)
	}

	if secConfig.SensitiveFilter != nil && secConfig.SensitiveFilter.IsEnabled() {
		message = secConfig.SensitiveFilter.Filter(message)
	}

	return sanitizeControlChars(message)
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

// sanitizeControlChars removes control characters from the message
func sanitizeControlChars(message string) string {
	if len(message) == 0 {
		return message
	}

	// Fast path: check if sanitization is needed
	needsSanitization := false
	for _, r := range message {
		if r == '\x00' || (r < 32 && r != '\n' && r != '\r' && r != '\t') || r == 127 {
			needsSanitization = true
			break
		}
	}

	if !needsSanitization {
		return message
	}

	// Slow path: remove control characters
	var builder strings.Builder
	builder.Grow(len(message))
	for _, r := range message {
		if r != '\x00' && (r >= 32 || r == '\n' || r == '\r' || r == '\t') && r != 127 {
			builder.WriteRune(r)
		}
	}

	return builder.String()
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

	l.mu.RLock()
	writerCount := len(l.writers)
	if writerCount == 0 {
		l.mu.RUnlock()
		return
	}

	if writerCount == 1 {
		w := l.writers[0]
		l.mu.RUnlock()
		if _, err := w.Write(buf); err != nil {
			if handler := l.getWriteErrorHandler(); handler != nil {
				handler(w, err)
			}
		}
		return
	}

	writers := make([]io.Writer, writerCount)
	copy(writers, l.writers)
	l.mu.RUnlock()

	for _, writer := range writers {
		if _, err := writer.Write(buf); err != nil {
			if handler := l.getWriteErrorHandler(); handler != nil {
				handler(writer, err)
			}
		}
	}
}

// Convenience logging methods
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
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.writers)
}

// Flush flushes all buffered writers (thread-safe).
// Writers that implement interface{ Flush() error } will be flushed.
func (l *Logger) Flush() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var firstErr error
	for _, w := range l.writers {
		if flusher, ok := w.(interface{ Flush() error }); ok {
			if err := flusher.Flush(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// fmt package replacement methods - output via logger's writers with caller info

// Print writes to configured writers with caller info and newline.
// Uses LevelDebug for filtering. Arguments are joined with spaces.
// Note: Both Print() and Println() behave identically because Log() already adds a newline.
func (l *Logger) Print(args ...any) {
	l.Log(LevelDebug, args...)
}

// Println writes to configured writers with caller info, spaces between operands, and a newline.
// Uses LevelDebug for filtering.
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

// Text These methods output directly to stdout with caller information,
// matching the behavior of package-level Text() and JSON() functions.
func (l *Logger) Text(data ...any) {
	if len(data) == 0 {
		fmt.Fprintln(os.Stdout)
		return
	}

	// Simple types output directly
	for i, item := range data {
		if isSimpleType(item) {
			output := formatSimpleValue(item)
			if i < len(data)-1 {
				fmt.Fprintf(os.Stdout, "%s ", output)
			} else {
				fmt.Fprintf(os.Stdout, "%s\n", output)
			}
			continue
		}
		// Complex types - use JSON formatting
		buf := debugBufPool.Get().(*bytes.Buffer)
		defer func() {
			buf.Reset()
			debugBufPool.Put(buf)
		}()

		encoder := json.NewEncoder(buf)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")

		convertedItem := convertValue(item)
		if err := encoder.Encode(convertedItem); err != nil {
			fmt.Fprintf(os.Stdout, "[%d] %v", i, item)
			if i < len(data)-1 {
				fmt.Fprint(os.Stdout, " ")
			} else {
				fmt.Fprintln(os.Stdout)
			}
			continue
		}

		out := buf.Bytes()
		if len(out) > 0 && out[len(out)-1] == '\n' {
			out = out[:len(out)-1]
		}

		if i < len(data)-1 {
			fmt.Fprintf(os.Stdout, "%s ", out)
		} else {
			fmt.Fprintf(os.Stdout, "%s\n", out)
		}
	}
}

func (l *Logger) Textf(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stdout, formatted)
}

func (l *Logger) JSON(data ...any) {
	caller := internal.GetCaller(DebugVisualizationDepth, false)
	outputJSON(caller, data...)
}

func (l *Logger) JSONF(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	caller := internal.GetCaller(DebugVisualizationDepth, false)
	outputJSON(caller, formatted)
}

var defaultLogger atomic.Pointer[Logger]

// Default returns the default global logger (thread-safe).
// The logger is created on first call with default configuration.
// Package-level convenience functions use this logger.
// Note: If SetDefault() is called before Default(), the custom logger is returned.
func Default() *Logger {
	if logger := defaultLogger.Load(); logger != nil {
		return logger
	}

	// Create default logger
	logger, err := New(nil)
	if err != nil {
		// Create fallback logger with proper initialization if New() fails
		defaultConfig := DefaultConfig()
		ctx, cancel := context.WithCancel(context.Background())
		logger = &Logger{
			callerDepth: DefaultCallerDepth,
			formatter:   newMessageFormatter(defaultConfig),
			writers:     []io.Writer{os.Stderr},
			ctx:         ctx,
			cancel:      cancel,
		}
		logger.level.Store(int32(defaultConfig.Level))
		logger.securityConfig.Store(defaultConfig.SecurityConfig)
	}

	// Use CompareAndSwap for atomic initialization
	// If another goroutine already set it, use their value
	if defaultLogger.CompareAndSwap(nil, logger) {
		return logger
	}
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
