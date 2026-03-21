package dd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// CONTEXT HELPER TESTS
// ============================================================================

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	newCtx := WithTraceID(ctx, "trace-123")

	if newCtx == nil {
		t.Fatal("WithTraceID should return non-nil context")
	}

	traceID := GetTraceID(newCtx)
	if traceID != "trace-123" {
		t.Errorf("GetTraceID() = %q, want %q", traceID, "trace-123")
	}
}

func TestWithSpanID(t *testing.T) {
	ctx := context.Background()
	newCtx := WithSpanID(ctx, "span-456")

	if newCtx == nil {
		t.Fatal("WithSpanID should return non-nil context")
	}

	spanID := GetSpanID(newCtx)
	if spanID != "span-456" {
		t.Errorf("GetSpanID() = %q, want %q", spanID, "span-456")
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	newCtx := WithRequestID(ctx, "req-789")

	if newCtx == nil {
		t.Fatal("WithRequestID should return non-nil context")
	}

	requestID := GetRequestID(newCtx)
	if requestID != "req-789" {
		t.Errorf("GetRequestID() = %q, want %q", requestID, "req-789")
	}
}

func TestGetTraceID_Empty(t *testing.T) {
	ctx := context.Background()
	traceID := GetTraceID(ctx)
	if traceID != "" {
		t.Errorf("GetTraceID() on empty context = %q, want empty", traceID)
	}
}

func TestGetSpanID_Empty(t *testing.T) {
	ctx := context.Background()
	spanID := GetSpanID(ctx)
	if spanID != "" {
		t.Errorf("GetSpanID() on empty context = %q, want empty", spanID)
	}
}

func TestGetRequestID_Empty(t *testing.T) {
	ctx := context.Background()
	requestID := GetRequestID(ctx)
	if requestID != "" {
		t.Errorf("GetRequestID() on empty context = %q, want empty", requestID)
	}
}

func TestContextKeys_WithLogger(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	cfg.Format = FormatJSON
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-abc")
	ctx = WithSpanID(ctx, "span-def")
	ctx = WithRequestID(ctx, "req-ghi")

	// Manually extract context values and pass them as fields
	logger.InfoWith("test message with context",
		String("trace_id", GetTraceID(ctx)),
		String("span_id", GetSpanID(ctx)),
		String("request_id", GetRequestID(ctx)),
		String("user", "test"),
	)

	output := buf.String()
	if !strings.Contains(output, "trace-abc") {
		t.Errorf("Output should contain trace_id, got: %s", output)
	}
}

// ============================================================================
// CONVENIENCE CONSTRUCTOR TESTS
// ============================================================================

func TestToConsole(t *testing.T) {
	logger, err := ToConsole()
	if err != nil {
		t.Fatalf("ToConsole() error = %v", err)
	}
	if logger == nil {
		t.Fatal("ToConsole() returned nil logger")
	}
	logger.Close()
}

func TestToWriter(t *testing.T) {
	var buf bytes.Buffer
	logger, err := ToWriter(&buf)
	if err != nil {
		t.Fatalf("ToWriter() error = %v", err)
	}
	if logger == nil {
		t.Fatal("ToWriter() returned nil logger")
	}

	logger.Info("test message")
	if buf.Len() == 0 {
		t.Error("ToWriter() should write to buffer")
	}
	logger.Close()
}

func TestToWriters(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	logger, err := ToWriters(&buf1, &buf2)
	if err != nil {
		t.Fatalf("ToWriters() error = %v", err)
	}
	if logger == nil {
		t.Fatal("ToWriters() returned nil logger")
	}

	logger.Info("test message")
	if buf1.Len() == 0 || buf2.Len() == 0 {
		t.Error("ToWriters() should write to all buffers")
	}
	logger.Close()
}

func TestToFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := ToFile(logPath)
	if err != nil {
		t.Fatalf("ToFile() error = %v", err)
	}
	if logger == nil {
		t.Fatal("ToFile() returned nil logger")
	}

	logger.Info("test message")
	logger.Close()

	// Verify file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("LogFile should be created")
	}
}

func TestToFile_DefaultPath(t *testing.T) {
	// Test with no filename argument (uses default path)
	logger, err := ToFile()
	if err != nil {
		t.Fatalf("ToFile() error = %v", err)
	}
	if logger == nil {
		t.Fatal("ToFile() returned nil logger")
	}
	logger.Close()

	// Clean up default log directory
	os.RemoveAll("logs")
}

func TestToJSONFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.json.log")

	logger, err := ToJSONFile(logPath)
	if err != nil {
		t.Fatalf("ToJSONFile() error = %v", err)
	}
	if logger == nil {
		t.Fatal("ToJSONFile() returned nil logger")
	}

	logger.Info("test message")
	logger.Close()

	// Verify file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("LogFile should be created")
	}
}

func TestToAll(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := ToAll(logPath)
	if err != nil {
		t.Fatalf("ToAll() error = %v", err)
	}
	if logger == nil {
		t.Fatal("ToAll() returned nil logger")
	}

	logger.Info("test message")
	logger.Close()

	// Verify file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("LogFile should be created")
	}
}

func TestToAllJSON(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.json.log")

	logger, err := ToAllJSON(logPath)
	if err != nil {
		t.Fatalf("ToAllJSON() error = %v", err)
	}
	if logger == nil {
		t.Fatal("ToAllJSON() returned nil logger")
	}

	logger.Info("test message")
	logger.Close()

	// Verify file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("LogFile should be created")
	}
}

// ============================================================================
// CONTEXT EXTRACTOR WITH LOGGER TESTS
// ============================================================================

func TestLoggerWithContextExtractors(t *testing.T) {
	var buf bytes.Buffer

	// Define a context extractor
	extractor := func(ctx context.Context) []Field {
		if customField := ctx.Value("custom_field"); customField != nil {
			return []Field{String("custom_field", customField.(string))}
		}
		return nil
	}

	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	cfg.Format = FormatJSON

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create context with custom value
	ctx := context.WithValue(context.Background(), "custom_field", "custom_value")

	// Manually extract context fields using the extractor
	contextFields := extractor(ctx)

	// Combine context fields with regular fields
	logger.InfoWith("test message", append(contextFields, String("context", "test"))...)

	output := buf.String()
	if !strings.Contains(output, "custom_field") {
		t.Errorf("Output should contain custom_field, got: %s", output)
	}
	if !strings.Contains(output, "custom_value") {
		t.Errorf("Output should contain custom_value, got: %s", output)
	}
}

// ============================================================================
// DEFAULT FILE WRITER CONFIG TEST
// ============================================================================

func TestDefaultFileWriterConfig(t *testing.T) {
	config := DefaultFileWriterConfig()

	if config.MaxSizeMB != DefaultMaxSizeMB {
		t.Errorf("MaxSizeMB = %d, want %d", config.MaxSizeMB, DefaultMaxSizeMB)
	}
	if config.MaxBackups != DefaultMaxBackups {
		t.Errorf("MaxBackups = %d, want %d", config.MaxBackups, DefaultMaxBackups)
	}
	if config.MaxAge != DefaultMaxAge {
		t.Errorf("MaxAge = %v, want %v", config.MaxAge, DefaultMaxAge)
	}
	if config.Compress != false {
		t.Error("Compress should be false by default")
	}
}

// ============================================================================
// CONTEXT WITH LEGACY STRING KEY TESTS
// ============================================================================

func TestContextKeys_LegacyStringKeys(t *testing.T) {
	// Test that string keys work for backward compatibility
	ctx := context.Background()
	ctx = context.WithValue(ctx, "trace_id", "legacy-trace")
	ctx = context.WithValue(ctx, "span_id", "legacy-span")
	ctx = context.WithValue(ctx, "request_id", "legacy-request")

	// Use the default extractors which should handle both key types
	registry := DefaultContextExtractorRegistry()
	fields := registry.Extract(ctx)

	// Should extract all three IDs
	if len(fields) < 3 {
		t.Errorf("Expected at least 3 fields, got %d", len(fields))
	}
}

// ============================================================================
// CONTEXT EXTRACTOR EDGE CASES
// ============================================================================

func TestContextExtractorRegistry_NilContext(t *testing.T) {
	registry := NewContextExtractorRegistry()
	registry.Add(func(ctx context.Context) []Field {
		if ctx == nil {
			return nil
		}
		return []Field{String("key", "value")}
	})

	// Extract with nil context should not panic
	fields := registry.Extract(nil)
	if fields != nil {
		t.Errorf("Extract(nil) should return nil, got %v", fields)
	}
}

func TestContextExtractorRegistry_EmptyRegistry(t *testing.T) {
	registry := NewContextExtractorRegistry()
	ctx := context.Background()

	fields := registry.Extract(ctx)
	if fields != nil {
		t.Errorf("Empty registry should return nil, got %v", fields)
	}
}

// ============================================================================
// LOGGER ENTRY METHOD TESTS
// ============================================================================

func TestLoggerEntry_LogMethods(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	defer logger.Close()

	entry := logger.WithFields(String("service", "api"))

	tests := []struct {
		name     string
		logFunc  func()
		expected string
	}{
		{"Debug", func() { entry.Debug("debug msg") }, "debug msg"},
		{"Info", func() { entry.Info("info msg") }, "info msg"},
		{"Warn", func() { entry.Warn("warn msg") }, "warn msg"},
		{"Error", func() { entry.Error("error msg") }, "error msg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc()
			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("Entry.%s() should contain %q, got: %s", tt.name, tt.expected, buf.String())
			}
			if !strings.Contains(buf.String(), "service=api") {
				t.Errorf("Entry.%s() should contain entry fields, got: %s", tt.name, buf.String())
			}
		})
	}
}

func TestLoggerEntry_LogfMethods(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	defer logger.Close()

	entry := logger.WithField("env", "test")

	tests := []struct {
		name     string
		logFunc  func()
		expected string
	}{
		{"Debugf", func() { entry.Debugf("debug: %s", "formatted") }, "debug: formatted"},
		{"Infof", func() { entry.Infof("info: %d", 42) }, "info: 42"},
		{"Warnf", func() { entry.Warnf("warn: %v", true) }, "warn: true"},
		{"Errorf", func() { entry.Errorf("error: %s", "test") }, "error: test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc()
			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("Entry.%s() should contain %q, got: %s", tt.name, tt.expected, buf.String())
			}
		})
	}
}

func TestLoggerEntry_LogWithMethods(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	defer logger.Close()

	entry := logger.WithField("base", "value")

	tests := []struct {
		name     string
		logFunc  func()
		expected string
	}{
		{"DebugWith", func() { entry.DebugWith("debug msg", String("extra", "debug")) }, "debug msg"},
		{"InfoWith", func() { entry.InfoWith("info msg", String("extra", "info")) }, "info msg"},
		{"WarnWith", func() { entry.WarnWith("warn msg", String("extra", "warn")) }, "warn msg"},
		{"ErrorWith", func() { entry.ErrorWith("error msg", String("extra", "error")) }, "error msg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc()
			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("Entry.%s() should contain %q, got: %s", tt.name, tt.expected, buf.String())
			}
			if !strings.Contains(buf.String(), "base=value") {
				t.Errorf("Entry.%s() should contain base field, got: %s", tt.name, buf.String())
			}
		})
	}
}

func TestLoggerEntry_LogLevel(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	defer logger.Close()

	entry := logger.WithField("test", "value")

	// Test Log method with specific level
	entry.Log(LevelInfo, "info message")
	if !strings.Contains(buf.String(), "info message") {
		t.Errorf("Entry.Log(LevelInfo) should contain message, got: %s", buf.String())
	}
}

func TestLoggerEntry_LogfLevel(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	defer logger.Close()

	entry := logger.WithField("key", "value")

	buf.Reset()
	entry.Logf(LevelWarn, "warning: %s", "test")
	if !strings.Contains(buf.String(), "warning: test") {
		t.Errorf("Entry.Logf(LevelWarn) should contain message, got: %s", buf.String())
	}
}

func TestLoggerEntry_LogWithLevel(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	defer logger.Close()

	entry := logger.WithField("base", "field")

	buf.Reset()
	entry.LogWith(LevelError, "error message", String("extra", "data"))
	output := buf.String()
	if !strings.Contains(output, "error message") {
		t.Errorf("Entry.LogWith should contain message, got: %s", output)
	}
}
