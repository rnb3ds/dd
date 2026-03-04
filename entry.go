package dd

import (
	"context"
	"fmt"
)

// LoggerEntry represents a logger with pre-set fields.
// Fields are inherited and merged with additional fields passed to logging methods.
// LoggerEntry is immutable - each WithFields call returns a new entry.
type LoggerEntry struct {
	logger *Logger
	fields []Field
}

// newLoggerEntry creates a new LoggerEntry with the given logger and fields.
func newLoggerEntry(logger *Logger, fields []Field) *LoggerEntry {
	// Copy fields to ensure immutability
	copiedFields := make([]Field, len(fields))
	copy(copiedFields, fields)
	return &LoggerEntry{
		logger: logger,
		fields: copiedFields,
	}
}

// mergeFieldSlices combines two field slices, with newFields overriding existingFields.
// This is a shared utility function used by both WithFields and mergeFields.
func mergeFieldSlices(existingFields, newFields []Field) []Field {
	// Fast path: no existing fields
	if len(existingFields) == 0 {
		return newFields
	}
	// Fast path: no new fields
	if len(newFields) == 0 {
		return existingFields
	}

	// Merge fields: start with existing, add new (allowing override)
	merged := make([]Field, 0, len(existingFields)+len(newFields))

	// Track which keys have been set by new fields
	newKeys := make(map[string]struct{}, len(newFields))
	for _, f := range newFields {
		newKeys[f.Key] = struct{}{}
	}

	// Add existing fields that aren't overridden
	for _, f := range existingFields {
		if _, exists := newKeys[f.Key]; !exists {
			merged = append(merged, f)
		}
	}

	// Add all new fields
	merged = append(merged, newFields...)

	return merged
}

// WithFields returns a new LoggerEntry with additional fields.
// Fields are merged with existing fields, with new fields overriding existing ones.
//
// Example:
//
//	entry := logger.WithFields(dd.String("service", "api"))
//	entry2 := entry.WithFields(dd.String("version", "1.0"))
//	entry2.Info("request received") // Contains both service and version fields
func (e *LoggerEntry) WithFields(fields ...Field) *LoggerEntry {
	if len(fields) == 0 {
		return e
	}

	// Fast path: no existing fields
	if len(e.fields) == 0 {
		return newLoggerEntry(e.logger, fields)
	}

	return newLoggerEntry(e.logger, mergeFieldSlices(e.fields, fields))
}

// WithField returns a new LoggerEntry with a single additional field.
// This is a convenience method equivalent to WithFields with a single field.
//
// Example:
//
//	entry := logger.WithField("request_id", "abc123")
func (e *LoggerEntry) WithField(key string, value any) *LoggerEntry {
	return e.WithFields(Field{Key: key, Value: value})
}

// mergeFields combines entry fields with method fields.
// Method fields can override entry fields with the same key.
func (e *LoggerEntry) mergeFields(fields []Field) []Field {
	return mergeFieldSlices(e.fields, fields)
}

// Log logs a message at the specified level with the entry's fields.
func (e *LoggerEntry) Log(level LogLevel, args ...any) {
	e.logger.LogWith(level, e.logger.formatter.FormatArgsToString(args...), e.fields...)
}

// Logf logs a formatted message at the specified level with the entry's fields.
func (e *LoggerEntry) Logf(level LogLevel, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	e.logger.LogWith(level, msg, e.fields...)
}

// LogWith logs a structured message with the entry's fields plus additional fields.
func (e *LoggerEntry) LogWith(level LogLevel, msg string, fields ...Field) {
	e.logger.LogWith(level, msg, e.mergeFields(fields)...)
}

// LogCtx logs a message at the specified level with context and the entry's fields.
func (e *LoggerEntry) LogCtx(ctx context.Context, level LogLevel, args ...any) {
	if !e.logger.shouldLog(level) {
		return
	}
	msg := e.logger.formatter.FormatArgsToString(args...)
	msg = e.logger.applyMessageSecurity(msg)
	contextFields := e.logger.extractContextFields(ctx)
	allFields := append(contextFields, e.fields...)
	e.logger.LogWith(level, msg, allFields...)
}

// LogfCtx logs a formatted message with context and the entry's fields.
func (e *LoggerEntry) LogfCtx(ctx context.Context, level LogLevel, format string, args ...any) {
	if !e.logger.shouldLog(level) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	msg = e.logger.applyMessageSecurity(msg)
	contextFields := e.logger.extractContextFields(ctx)
	allFields := append(contextFields, e.fields...)
	e.logger.LogWith(level, msg, allFields...)
}

// LogWithCtx logs a structured message with context, the entry's fields, and additional fields.
func (e *LoggerEntry) LogWithCtx(ctx context.Context, level LogLevel, msg string, fields ...Field) {
	if !e.logger.shouldLog(level) {
		return
	}
	msg = e.logger.applyMessageSecurity(msg)
	contextFields := e.logger.extractContextFields(ctx)
	mergedFields := e.mergeFields(fields)
	allFields := append(contextFields, mergedFields...)
	e.logger.LogWith(level, msg, allFields...)
}

// Convenience methods for each log level

func (e *LoggerEntry) Debug(args ...any) { e.Log(LevelDebug, args...) }
func (e *LoggerEntry) Info(args ...any)  { e.Log(LevelInfo, args...) }
func (e *LoggerEntry) Warn(args ...any)  { e.Log(LevelWarn, args...) }
func (e *LoggerEntry) Error(args ...any) { e.Log(LevelError, args...) }
func (e *LoggerEntry) Fatal(args ...any) { e.Log(LevelFatal, args...) }

func (e *LoggerEntry) Debugf(format string, args ...any) { e.Logf(LevelDebug, format, args...) }
func (e *LoggerEntry) Infof(format string, args ...any)  { e.Logf(LevelInfo, format, args...) }
func (e *LoggerEntry) Warnf(format string, args ...any)  { e.Logf(LevelWarn, format, args...) }
func (e *LoggerEntry) Errorf(format string, args ...any) { e.Logf(LevelError, format, args...) }
func (e *LoggerEntry) Fatalf(format string, args ...any) { e.Logf(LevelFatal, format, args...) }

func (e *LoggerEntry) DebugWith(msg string, fields ...Field) { e.LogWith(LevelDebug, msg, fields...) }
func (e *LoggerEntry) InfoWith(msg string, fields ...Field)  { e.LogWith(LevelInfo, msg, fields...) }
func (e *LoggerEntry) WarnWith(msg string, fields ...Field)  { e.LogWith(LevelWarn, msg, fields...) }
func (e *LoggerEntry) ErrorWith(msg string, fields ...Field) { e.LogWith(LevelError, msg, fields...) }
func (e *LoggerEntry) FatalWith(msg string, fields ...Field) { e.LogWith(LevelFatal, msg, fields...) }

func (e *LoggerEntry) DebugCtx(ctx context.Context, args ...any) { e.LogCtx(ctx, LevelDebug, args...) }
func (e *LoggerEntry) InfoCtx(ctx context.Context, args ...any)  { e.LogCtx(ctx, LevelInfo, args...) }
func (e *LoggerEntry) WarnCtx(ctx context.Context, args ...any)  { e.LogCtx(ctx, LevelWarn, args...) }
func (e *LoggerEntry) ErrorCtx(ctx context.Context, args ...any) { e.LogCtx(ctx, LevelError, args...) }
func (e *LoggerEntry) FatalCtx(ctx context.Context, args ...any) { e.LogCtx(ctx, LevelFatal, args...) }

func (e *LoggerEntry) DebugfCtx(ctx context.Context, format string, args ...any) {
	e.LogfCtx(ctx, LevelDebug, format, args...)
}
func (e *LoggerEntry) InfofCtx(ctx context.Context, format string, args ...any) {
	e.LogfCtx(ctx, LevelInfo, format, args...)
}
func (e *LoggerEntry) WarnfCtx(ctx context.Context, format string, args ...any) {
	e.LogfCtx(ctx, LevelWarn, format, args...)
}
func (e *LoggerEntry) ErrorfCtx(ctx context.Context, format string, args ...any) {
	e.LogfCtx(ctx, LevelError, format, args...)
}
func (e *LoggerEntry) FatalfCtx(ctx context.Context, format string, args ...any) {
	e.LogfCtx(ctx, LevelFatal, format, args...)
}

func (e *LoggerEntry) DebugWithCtx(ctx context.Context, msg string, fields ...Field) {
	e.LogWithCtx(ctx, LevelDebug, msg, fields...)
}
func (e *LoggerEntry) InfoWithCtx(ctx context.Context, msg string, fields ...Field) {
	e.LogWithCtx(ctx, LevelInfo, msg, fields...)
}
func (e *LoggerEntry) WarnWithCtx(ctx context.Context, msg string, fields ...Field) {
	e.LogWithCtx(ctx, LevelWarn, msg, fields...)
}
func (e *LoggerEntry) ErrorWithCtx(ctx context.Context, msg string, fields ...Field) {
	e.LogWithCtx(ctx, LevelError, msg, fields...)
}
func (e *LoggerEntry) FatalWithCtx(ctx context.Context, msg string, fields ...Field) {
	e.LogWithCtx(ctx, LevelFatal, msg, fields...)
}

// Print methods - output via logger's writers with caller info and entry's fields.
// These methods use LevelInfo for filtering and apply sensitive data filtering.

// Print writes to configured writers with caller info and the entry's fields.
// Uses LevelInfo for filtering. Arguments are joined with spaces.
func (e *LoggerEntry) Print(args ...any) {
	e.Log(LevelInfo, args...)
}

// Println writes to configured writers with caller info and the entry's fields.
// Uses LevelInfo for filtering. Note: Behaves identically to Print() because Log() already adds a newline.
func (e *LoggerEntry) Println(args ...any) {
	e.Log(LevelInfo, args...)
}

// Printf formats according to a format specifier and writes to configured writers
// with caller info and the entry's fields. Uses LevelInfo for filtering.
func (e *LoggerEntry) Printf(format string, args ...any) {
	e.Logf(LevelInfo, format, args...)
}

// Logger methods for WithFields

// WithFields returns a LoggerEntry with pre-set fields.
// The fields are inherited by all logging calls on the returned entry.
//
// Example:
//
//	entry := logger.WithFields(dd.String("service", "api"), dd.String("version", "1.0"))
//	entry.Info("request received") // Contains service and version fields
//	entry.WithFields(dd.String("user", "john")).Info("user action") // Contains all three fields
func (l *Logger) WithFields(fields ...Field) *LoggerEntry {
	return newLoggerEntry(l, fields)
}

// WithField returns a LoggerEntry with a single pre-set field.
// This is a convenience method equivalent to WithFields with a single field.
//
// Example:
//
//	entry := logger.WithField("request_id", "abc123")
func (l *Logger) WithField(key string, value any) *LoggerEntry {
	return newLoggerEntry(l, []Field{{Key: key, Value: value}})
}
