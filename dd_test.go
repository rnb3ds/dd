package dd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cybergodev/dd/internal"
)

// ============================================================================
// LOGGER CREATION AND CONFIGURATION TESTS
// ============================================================================

func TestLoggerCreation(t *testing.T) {
	var buf bytes.Buffer

	cfg := DefaultConfig()
	cfg.Level = LevelInfo
	cfg.Format = FormatText
	cfg.TimeFormat = DefaultTimeFormat
	cfg.Output = &buf
	cfg.IncludeTime = true

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if logger == nil {
		t.Fatal("Build() returned nil logger")
	}

	logger.Info("test message")
	if buf.Len() == 0 {
		t.Error("Logger should have written to buffer")
	}
}

func TestLoggerSetSecurityConfig(t *testing.T) {
	cfg := DefaultConfig()
	logger, _ := New(cfg)

	secConfig := &SecurityConfig{
		MaxMessageSize:  1000,
		MaxWriters:      10,
		SensitiveFilter: NewBasicSensitiveDataFilter(),
	}

	logger.SetSecurityConfig(secConfig)

	retrieved := logger.GetSecurityConfig()
	if retrieved == nil {
		t.Fatal("GetSecurityConfig() should return config")
	}
	if retrieved.MaxMessageSize != 1000 {
		t.Errorf("MaxMessageSize = %d, want 1000", retrieved.MaxMessageSize)
	}
	if retrieved.SensitiveFilter == nil {
		t.Error("SensitiveFilter should be set")
	}
}

// ============================================================================
// BASIC LOGGING TESTS
// ============================================================================

func TestBasicLogging(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	tests := []struct {
		name   string
		level  LogLevel
		method func(...any)
	}{
		{"Debug", LevelDebug, func(args ...any) { logger.Debug(args...) }},
		{"Info", LevelInfo, func(args ...any) { logger.Info(args...) }},
		{"Warn", LevelWarn, func(args ...any) { logger.Warn(args...) }},
		{"Error", LevelError, func(args ...any) { logger.Error(args...) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.method("test message")
			if tt.level >= LevelInfo && buf.Len() == 0 {
				t.Errorf("%s should write output", tt.name)
			}
		})
	}
}

func TestFormattedLogging(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	logger.Infof("test %s", "message")
	if !strings.Contains(buf.String(), "test message") {
		t.Error("Formatted logging should work")
	}
}

func TestAllFormattedMethods(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)

	tests := []struct {
		name   string
		method func(string, ...any)
	}{
		{"Debugf", logger.Debugf},
		{"Infof", logger.Infof},
		{"Warnf", logger.Warnf},
		{"Errorf", logger.Errorf},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.method("test %s", "value")
			if buf.Len() == 0 {
				t.Errorf("%s should write output", tt.name)
			}
		})
	}
}

func TestStructuredLogging(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	logger.InfoWith("test", String("key", "value"), Int("count", 42))
	output := buf.String()

	if !strings.Contains(output, "key=value") {
		t.Error("Structured logging should include fields")
	}
	if !strings.Contains(output, "count=42") {
		t.Error("Structured logging should include count")
	}
}

func TestAllStructuredLoggingMethods(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)

	tests := []struct {
		name   string
		method func(string, ...Field)
	}{
		{"DebugWith", logger.DebugWith},
		{"InfoWith", logger.InfoWith},
		{"WarnWith", logger.WarnWith},
		{"ErrorWith", logger.ErrorWith},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.method("test", String("key", "value"))
			if buf.Len() == 0 {
				t.Errorf("%s should write output", tt.name)
			}
		})
	}
}

func TestFatalLogging(t *testing.T) {
	t.Run("Fatal", func(t *testing.T) {
		var buf bytes.Buffer
		exited := false

		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelInfo
		cfg.FatalHandler = func() { exited = true }
		logger, _ := New(cfg)

		logger.Fatal("fatal message")

		if !exited {
			t.Error("Fatal handler should be called")
		}
		if !strings.Contains(buf.String(), "fatal message") {
			t.Error("Fatal message should be logged")
		}
	})

	t.Run("Fatalf", func(t *testing.T) {
		var buf bytes.Buffer
		exited := false

		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelInfo
		cfg.FatalHandler = func() { exited = true }
		logger, _ := New(cfg)

		logger.Fatalf("fatal %s", "message")

		if !exited {
			t.Error("Fatalf handler should be called")
		}
		if !strings.Contains(buf.String(), "fatal message") {
			t.Error("Fatalf message should be logged")
		}
	})

	t.Run("FatalWith", func(t *testing.T) {
		var buf bytes.Buffer
		exited := false

		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelInfo
		cfg.FatalHandler = func() { exited = true }
		logger, _ := New(cfg)

		logger.FatalWith("fatal", String("key", "value"))

		if !exited {
			t.Error("FatalWith handler should be called")
		}
	})
}

func TestJSONLogging(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	cfg.Format = FormatJSON
	logger, _ := New(cfg)

	logger.Info("test message")

	if !strings.Contains(buf.String(), `"message":"test message"`) {
		t.Error("JSON logging should format as JSON")
	}
}

// ============================================================================
// LOG LEVEL MANAGEMENT TESTS
// ============================================================================

func TestLoggerLevelManagement(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	// Test level filtering
	logger.Debug("debug message")
	if buf.Len() != 0 {
		t.Error("Debug message should not be logged at Info level")
	}

	logger.Info("info message")
	if buf.Len() == 0 {
		t.Error("Info message should be logged at Info level")
	}

	// Test level change
	buf.Reset()
	logger.SetLevel(LevelDebug)
	logger.Debug("debug message")
	if buf.Len() == 0 {
		t.Error("Debug message should be logged after setting level to Debug")
	}

	// Test invalid level
	err := logger.SetLevel(LogLevel(99))
	if err == nil {
		t.Error("SetLevel() should return error for invalid level")
	}
}

func TestConvenienceFunctions(t *testing.T) {
	oldLevel := Default().GetLevel()
	oldDefault := Default()
	defer func() {
		SetDefault(oldDefault)
		oldDefault.SetLevel(oldLevel)
	}()

	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	SetDefault(logger)

	Info("test info")
	output := buf.String()
	if !strings.Contains(output, "test info") {
		t.Error("Global Info function should work")
	}

	buf.Reset()
	SetLevel(LevelDebug)
	Debug("test debug")
	output = buf.String()
	if !strings.Contains(output, "test debug") {
		t.Error("Global Debug function should work after setting level")
	}
}

func TestGlobalGetLevel(t *testing.T) {
	oldLevel := Default().GetLevel()
	oldDefault := Default()
	defer func() {
		SetDefault(oldDefault)
		oldDefault.SetLevel(oldLevel)
	}()

	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)
	SetDefault(logger)

	// Test initial level
	if GetLevel() != LevelInfo {
		t.Errorf("GetLevel() = %v, want %v", GetLevel(), LevelInfo)
	}

	// Test after setting level
	SetLevel(LevelDebug)
	if GetLevel() != LevelDebug {
		t.Errorf("GetLevel() after SetLevel(Debug) = %v, want %v", GetLevel(), LevelDebug)
	}

	// Test after setting another level
	SetLevel(LevelError)
	if GetLevel() != LevelError {
		t.Errorf("GetLevel() after SetLevel(Error) = %v, want %v", GetLevel(), LevelError)
	}
}

func TestAllGlobalFunctions(t *testing.T) {
	oldDefault := Default()
	defer SetDefault(oldDefault)

	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	SetDefault(logger)

	tests := []struct {
		name   string
		method func(...any)
	}{
		{"Debug", Debug},
		{"Info", Info},
		{"Warn", Warn},
		{"Error", Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.method("test message")
			if buf.Len() == 0 {
				t.Errorf("Global %s should work", tt.name)
			}
		})
	}
}

func TestAllGlobalFormattedFunctions(t *testing.T) {
	oldDefault := Default()
	defer SetDefault(oldDefault)

	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	SetDefault(logger)

	tests := []struct {
		name   string
		method func(string, ...any)
	}{
		{"Debugf", Debugf},
		{"Infof", Infof},
		{"Warnf", Warnf},
		{"Errorf", Errorf},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.method("test %s", "value")
			if buf.Len() == 0 {
				t.Errorf("Global %s should work", tt.name)
			}
		})
	}
}

func TestGlobalStructuredLoggingMethods(t *testing.T) {
	oldDefault := Default()
	defer SetDefault(oldDefault)

	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	SetDefault(logger)

	InfoWith("test", String("key", "value"))
	if buf.Len() == 0 {
		t.Error("Global InfoWith should work")
	}

	DebugWith("test", String("key", "value"))
	if buf.Len() == 0 {
		t.Error("Global DebugWith should work")
	}

	WarnWith("test", String("key", "value"))
	if buf.Len() == 0 {
		t.Error("Global WarnWith should work")
	}

	ErrorWith("test", String("key", "value"))
	if buf.Len() == 0 {
		t.Error("Global ErrorWith should work")
	}
}

// ============================================================================
// WRITER MANAGEMENT TESTS
// ============================================================================

func TestWriterManagement(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf1
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	// Add writer
	err := logger.AddWriter(&buf2)
	if err != nil {
		t.Fatalf("AddWriter() error = %v", err)
	}

	logger.Info("test")
	if buf1.Len() == 0 || buf2.Len() == 0 {
		t.Error("Message should be written to both writers")
	}

	// Remove writer
	err = logger.RemoveWriter(&buf2)
	if err != nil {
		t.Fatalf("RemoveWriter() error = %v", err)
	}

	buf1.Reset()
	buf2.Reset()
	logger.Info("test 2")
	if buf2.Len() != 0 {
		t.Error("Message should not be written to removed writer")
	}
}

func TestFileWriter(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")
	fw, err := NewFileWriter(tmpFile, FileWriterConfig{
		MaxSizeMB:  1,
		MaxBackups: 3,
		MaxAge:     24 * time.Hour,
		Compress:   false,
	})

	if err != nil {
		t.Fatalf("NewFileWriter() error = %v", err)
	}
	defer fw.Close()

	data := []byte("test log message\n")
	n, err := fw.Write(data)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() wrote %d bytes, want %d", n, len(data))
	}
}

func TestBufferedWriter(t *testing.T) {
	var buf bytes.Buffer
	bw, err := NewBufferedWriter(&buf, 4096)
	if err != nil {
		t.Fatalf("NewBufferedWriter() error = %v", err)
	}
	defer bw.Close()

	data := []byte("test message\n")
	n, err := bw.Write(data)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() wrote %d bytes, want %d", n, len(data))
	}

	bw.Flush()
	if buf.Len() == 0 {
		t.Error("BufferedWriter should have written data after flush")
	}
}

func TestBufferedWriterAutoFlush(t *testing.T) {
	var buf bytes.Buffer
	bw, err := NewBufferedWriter(&buf, 4096)
	if err != nil {
		t.Fatalf("NewBufferedWriter() error = %v", err)
	}
	defer bw.Close()

	// Write data that should trigger auto-flush
	largeData := make([]byte, 2048)
	for i := 0; i < 10; i++ {
		bw.Write(largeData)
	}

	// Give time for auto-flush to run
	time.Sleep(100 * time.Millisecond)

	if buf.Len() == 0 {
		t.Error("Auto-flush should have written some data")
	}
}

func TestMultiWriter(t *testing.T) {
	var buf1, buf2, buf3 bytes.Buffer
	mw := NewMultiWriter(&buf1, &buf2, &buf3)

	data := []byte("test message\n")
	n, err := mw.Write(data)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() wrote %d bytes, want %d", n, len(data))
	}

	if buf1.Len() == 0 || buf2.Len() == 0 || buf3.Len() == 0 {
		t.Error("MultiWriter should write to all writers")
	}
}

func TestMultiWriterManagement(t *testing.T) {
	var buf1, buf2, buf3 bytes.Buffer
	mw := NewMultiWriter(&buf1, &buf2)

	// Test AddWriter
	if err := mw.AddWriter(&buf3); err != nil {
		t.Errorf("AddWriter failed: %v", err)
	}
	data := []byte("test\n")
	mw.Write(data)

	if buf3.Len() == 0 {
		t.Error("AddWriter should add writer to MultiWriter")
	}

	// Test RemoveWriter
	buf1.Reset()
	buf2.Reset()
	buf3.Reset()
	mw.RemoveWriter(&buf2)
	mw.Write(data)

	if buf2.Len() != 0 {
		t.Error("RemoveWriter should remove writer from MultiWriter")
	}
	if buf1.Len() == 0 || buf3.Len() == 0 {
		t.Error("Remaining writers should still work")
	}

	// Test Close
	mw.Close()
}

func TestMultiWriterClose(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")
	fw, _ := NewFileWriter(tmpFile)

	var buf bytes.Buffer
	mw := NewMultiWriter(&buf, fw)

	err := mw.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// File should be closed
	_, err = fw.Write([]byte("test"))
	if err == nil {
		t.Error("FileWriter should be closed after MultiWriter.Close()")
	}
}

// ============================================================================
// CONCURRENCY TESTS
// ============================================================================

func TestConcurrentLogging(t *testing.T) {
	safeWriter := &threadSafeWriter{w: &bytes.Buffer{}}
	cfg := DefaultConfig()
	cfg.Level = LevelInfo
	cfg.IncludeTime = false
	cfg.IncludeLevel = false
	cfg.Output = safeWriter
	cfg.Security = &SecurityConfig{SensitiveFilter: nil}
	logger, _ := New(cfg)

	const goroutines = 100
	const messagesPerGoroutine = 10

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Info(fmt.Sprintf("goroutine %d message %d", id, j))
			}
		}(i)
	}
	wg.Wait()

	output := safeWriter.String()
	if len(output) == 0 {
		t.Error("Concurrent logging should produce output")
	}
}

func TestConcurrentLevelChanges(t *testing.T) {
	cfg := DefaultConfig()
	logger, _ := New(cfg)

	const goroutines = 50
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			level := LogLevel(id % 5)
			logger.SetLevel(level)
		}(i)
	}
	wg.Wait()
}

func TestConcurrentWriterOperations(t *testing.T) {
	cfg := DefaultConfig()
	logger, _ := New(cfg)
	const goroutines = 50
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			var buf bytes.Buffer
			logger.AddWriter(&buf)
			logger.RemoveWriter(&buf)
		}(i)
	}
	wg.Wait()
}

// threadSafeWriter wraps a writer with mutex for thread-safe writes in tests
type threadSafeWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (t *threadSafeWriter) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.w.Write(p)
}

func (t *threadSafeWriter) String() string {
	if buf, ok := t.w.(*bytes.Buffer); ok {
		t.mu.Lock()
		defer t.mu.Unlock()
		return buf.String()
	}
	return ""
}

// ============================================================================
// SECURITY AND SANITIZATION TESTS
// ============================================================================

func TestSanitization(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Level = LevelInfo
	cfg.Output = &buf
	cfg.Security = DefaultSecurityConfig()
	logger, _ := New(cfg)

	// Test control character sanitization
	logger.Info("test\x00message\x1f")
	output := buf.String()
	if strings.Contains(output, "\x00") || strings.Contains(output, "\x1f") {
		t.Error("Control characters should be sanitized")
	}
}

func TestMessageSizeLimit(t *testing.T) {
	var buf bytes.Buffer
	secConfig := DefaultSecurityConfig()
	secConfig.MaxMessageSize = 100
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	cfg.Security = secConfig
	logger, _ := New(cfg)

	largeMessage := strings.Repeat("a", 200)
	logger.Info(largeMessage)

	output := buf.String()
	if len(output) > 150 {
		t.Error("Message should be truncated per MaxMessageSize")
	}
}

func TestSensitiveDataFiltering(t *testing.T) {
	var buf bytes.Buffer
	filter := NewSensitiveDataFilter()
	secConfig := &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: filter,
	}

	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	cfg.Security = secConfig
	logger, _ := New(cfg)

	logger.Info("password=secret123 api_key=sk-1234")

	output := buf.String()
	if strings.Contains(output, "secret123") || strings.Contains(output, "sk-1234") {
		t.Error("Sensitive data should be filtered")
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("Should contain redaction marker")
	}
}

// ============================================================================
// EDGE CASES AND ERROR HANDLING TESTS
// ============================================================================

func TestNilWriter(t *testing.T) {
	cfg := DefaultConfig()
	logger, _ := New(cfg)

	err := logger.AddWriter(nil)
	if err == nil {
		t.Error("AddWriter(nil) should return error")
	}

	err = logger.RemoveWriter(nil)
	if err == nil {
		t.Error("RemoveWriter(nil) should return error")
	}
}

func TestMaxWritersExceeded(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	logger, _ := New(cfg)

	// Clear default writer and add 99 more using the atomic pointer
	writers := make([]io.Writer, 0, 100)
	for i := 0; i < 99; i++ {
		var b bytes.Buffer
		writers = append(writers, &b)
	}
	logger.writersPtr.Store(&writers)

	// The 100th writer should work
	var buf100 bytes.Buffer
	err := logger.AddWriter(&buf100)
	if err != nil {
		t.Errorf("Adding 100th writer should succeed, got %v", err)
	}

	// The 101st writer should fail
	var buf101 bytes.Buffer
	err = logger.AddWriter(&buf101)
	if err == nil {
		t.Error("Adding 101st writer should fail")
	}
}

func TestClosedLogger(t *testing.T) {
	cfg := DefaultConfig()
	logger, _ := New(cfg)
	logger.Close()

	var buf bytes.Buffer
	err := logger.AddWriter(&buf)
	if err == nil {
		t.Error("AddWriter on closed logger should fail")
	}

	// Logging should be silently ignored after close
	logger.Info("test") // Should not panic
}

func TestEmptyAndNilInputs(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	// Empty message
	logger.Info()
	logger.Info("")

	// Nil fields
	logger.InfoWith("test")

	// Should not panic
}

func TestLargeMessage(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	largeMsg := strings.Repeat("test", 10000)
	logger.Info(largeMsg)

	if buf.Len() == 0 {
		t.Error("Large message should be logged")
	}
}

func TestManyFields(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	fields := make([]Field, 100)
	for i := 0; i < 100; i++ {
		fields[i] = Int(fmt.Sprintf("field%d", i), i)
	}

	logger.InfoWith("many fields", fields...)

	if buf.Len() == 0 {
		t.Error("Message with many fields should be logged")
	}
}

// ============================================================================
// LOGGER LIFECYCLE TESTS
// ============================================================================

func TestLoggerClose(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	fw, _ := NewFileWriter(tmpFile)
	cfg := DefaultConfig()
	cfg.Output = fw
	logger, _ := New(cfg)

	err := logger.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Double close should be safe
	err = logger.Close()
	if err != nil {
		t.Errorf("Second Close() should not error, got %v", err)
	}

	// Should not close stdout/stderr
	stdoutCfg := DefaultConfig()
	stdoutCfg.Output = os.Stdout
	stdoutLogger, _ := New(stdoutCfg)
	stdoutLogger.Close() // Should not panic or exit
}

// ============================================================================
// FIELD CONSTRUCTORS TESTS
// ============================================================================

func TestFieldConstructors(t *testing.T) {
	tests := []struct {
		name  string
		field Field
		key   string
	}{
		{"String", String("k", "v"), "k"},
		{"Int", Int("k", 42), "k"},
		{"Int64", Int64("k", 42), "k"},
		{"Bool", Bool("k", true), "k"},
		{"Float64", Float64("k", 3.14), "k"},
		{"Any", Any("k", nil), "k"},
		{"Err", Err(nil), "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.field.Key != tt.key {
				t.Errorf("Field key = %q, want %q", tt.field.Key, tt.key)
			}
		})
	}
}

// ============================================================================
// FORMAT TESTS
// ============================================================================

func TestLogFormatString(t *testing.T) {
	tests := []struct {
		format LogFormat
		want   string
	}{
		{FormatText, "text"},
		{FormatJSON, "json"},
		{LogFormat(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.format.String(), func(t *testing.T) {
			if got := tt.format.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level LogLevel
		want  string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultJSONOptions(t *testing.T) {
	opts := DefaultJSONOptions()

	if opts == nil {
		t.Fatal("DefaultJSONOptions() should not return nil")
	}
	if opts.PrettyPrint {
		t.Error("Default should not use pretty print")
	}
	if opts.Indent != DefaultJSONIndent {
		t.Errorf("Default indent = %q, want %q", opts.Indent, DefaultJSONIndent)
	}
	if opts.FieldNames == nil {
		t.Error("Default should have field names")
	}
}

func TestJSONFieldNamesDefaults(t *testing.T) {
	names := internal.DefaultJSONFieldNames()

	if names.Timestamp != "timestamp" {
		t.Errorf("Default timestamp field = %q, want %q", names.Timestamp, "timestamp")
	}
	if names.Level != "level" {
		t.Errorf("Default level field = %q, want %q", names.Level, "level")
	}
	if names.Message != "message" {
		t.Errorf("Default message field = %q, want %q", names.Message, "message")
	}
}

func TestFullLoggingPipeline(t *testing.T) {
	tmpFile := t.TempDir() + "/test.log"

	fw, _ := NewFileWriter(tmpFile)
	defer fw.Close()

	cfg := DefaultConfig()
	cfg.Level = LevelInfo
	cfg.IncludeTime = true
	cfg.IncludeLevel = true
	cfg.DynamicCaller = false
	cfg.Output = fw
	cfg.Security = DefaultSecurityConfig()
	logger, _ := New(cfg)

	// Test various logging methods
	logger.Info("simple message")
	logger.Infof("formatted %s", "message")
	logger.InfoWith("structured", String("key", "value"))

	// Verify file has content
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(data) == 0 {
		t.Error("Log file should contain data")
	}

	content := string(data)
	if !strings.Contains(content, "simple message") {
		t.Error("Log should contain simple message")
	}
	if !strings.Contains(content, "formatted message") {
		t.Error("Log should contain formatted message")
	}
	if !strings.Contains(content, "structured") {
		t.Error("Log should contain structured message")
	}
}

func TestJSONLoggingPipeline(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	cfg.Format = FormatJSON
	logger, _ := New(cfg)

	logger.InfoWith("test", String("key", "value"), Int("count", 42))

	output := buf.String()

	// Should be valid JSON
	var jsonData map[string]any
	if err := json.Unmarshal([]byte(output), &jsonData); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Should have required fields
	if jsonData["message"] == nil {
		t.Error("JSON should have message field")
	}
	if jsonData["level"] == nil {
		t.Error("JSON should have level field")
	}
}

// ============================================================================
// DEBUG VISUALIZATION TESTS
// ============================================================================

func TestDebugVisualization(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Text("test data")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test data") {
		t.Error("Text() should output the data")
	}
}

func TestDebugVisualizationf(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Textf("test: %s", "formatted")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test: formatted") {
		t.Error("Textf() should output formatted data")
	}
}

func TestDebugVisualizationJson(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	JSON(map[string]string{"key": "value"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should be valid JSON
	if !strings.Contains(output, `"key"`) || !strings.Contains(output, `"value"`) {
		t.Error("JSON() should output JSON")
	}
}

func TestDebugVisualizationJsonf(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	JSONF("test: %s", "formatted")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test: formatted") {
		t.Error("JSONF() should output formatted data as JSON")
	}
}

func TestTypeConverter(t *testing.T) {
	// Test simple types
	isSimple := internal.IsSimpleType("test")
	if !isSimple {
		t.Error("String should be simple type")
	}

	isSimple = internal.IsSimpleType(42)
	if !isSimple {
		t.Error("Int should be simple type")
	}

	isSimple = internal.IsSimpleType(map[string]string{})
	if isSimple {
		t.Error("Map should not be simple type")
	}

	// Test format simple value
	formatted := internal.FormatSimpleValue("test")
	if formatted != "test" {
		t.Errorf("formatSimpleValue(string) = %q, want %q", formatted, "test")
	}

	formatted = internal.FormatSimpleValue(nil)
	if formatted != "nil" {
		t.Errorf("formatSimpleValue(nil) = %q, want %q", formatted, "nil")
	}
}

func TestTypeConverterComplexTypes(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test slice
	JSON([]int{1, 2, 3})

	// Test map
	JSON(map[string]int{"one": 1, "two": 2})

	// Test struct
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	JSON(TestStruct{Name: "John", Age: 30})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should be valid JSON output
	if !strings.Contains(output, "[") && !strings.Contains(output, "{") {
		t.Error("Complex types should be converted to JSON")
	}
}

// ============================================================================
// ERROR HANDLING TESTS
// ============================================================================

func TestErrorReturns(t *testing.T) {
	// Test New() with nil config - it returns a default logger
	logger, err := New()
	if logger == nil {
		t.Error("New() should return default logger, not nil")
	}
	_ = err // Use err to avoid lint error

	// Test NewFileWriter with invalid path
	_, err = NewFileWriter("\x00invalid")
	if err == nil {
		t.Error("NewFileWriter() with invalid path should fail")
	}

	// Test NewBufferedWriter with nil writer
	_, err = NewBufferedWriter(nil, 1024)
	if err == nil {
		t.Error("NewBufferedWriter(nil) should fail")
	}
}

func TestLoggerSetLevelInvalid(t *testing.T) {
	cfg := DefaultConfig()
	logger, _ := New(cfg)

	err := logger.SetLevel(LogLevel(99))
	if err == nil {
		t.Error("SetLevel(invalid) should return error")
	}
}

// ============================================================================
// SECURITY FILTER TESTS (Consolidated from security_test.go)
// ============================================================================

func TestSensitiveDataFilter(t *testing.T) {
	filter := NewSensitiveDataFilter()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "password field",
			input:    "password=secret123",
			contains: "[REDACTED]",
		},
		{
			name:     "api key",
			input:    "api_key=sk-1234567890",
			contains: "[REDACTED]",
		},
		{
			name:     "credit card",
			input:    "card number: 4532015112830366",
			contains: "[REDACTED]",
		},
		{
			name:     "email address",
			input:    "email: user@example.com",
			contains: "[REDACTED]",
		},
		{
			name:     "normal text",
			input:    "hello world",
			contains: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Filter(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Expected %q in result, got: %s", tt.contains, result)
			}
		})
	}
}

func TestBasicSensitiveDataFilter(t *testing.T) {
	filter := NewBasicSensitiveDataFilter()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "password",
			input:    "password=secret123",
			contains: "[REDACTED]",
		},
		{
			name:     "token",
			input:    "token=abc123xyz",
			contains: "[REDACTED]",
		},
		{
			name:     "api key",
			input:    "api_key=sk-1234567890",
			contains: "[REDACTED]",
		},
		{
			name:     "normal text",
			input:    "username=john_doe",
			contains: "username=john_doe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Filter(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Expected %q in result, got: %s", tt.contains, result)
			}
		})
	}
}

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	if config == nil {
		t.Fatal("DefaultSecurityConfig should not return nil")
	}

	if config.MaxMessageSize <= 0 {
		t.Error("MaxMessageSize should be positive")
	}

	if config.MaxWriters <= 0 {
		t.Error("MaxWriters should be positive")
	}
}

// ============================================================================
// CONCURRENT DEFAULT LOGGER TESTS
// ============================================================================

func TestDefaultLoggerConcurrent(t *testing.T) {
	// Reset the default logger for this test
	// This test verifies that concurrent access to Default() is safe

	var wg sync.WaitGroup
	const goroutines = 100

	// Concurrent calls to Default()
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger := Default()
			if logger == nil {
				t.Error("Default() should not return nil")
			}
		}()

		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cfg := DefaultConfig()
			logger, err := New(cfg)
			if err != nil {
				return
			}
			defer logger.Close()
			SetDefault(logger)
		}(i)
	}

	wg.Wait()

	// Verify final state is consistent
	logger := Default()
	if logger == nil {
		t.Error("Final Default() should not return nil")
	}
}

func TestDefaultLoggerCompareAndSwap(t *testing.T) {
	// Test that CompareAndSwap semantics work correctly
	cfg1 := DefaultConfig()
	logger1, err := New(cfg1)
	if err != nil {
		t.Fatalf("Failed to create logger1: %v", err)
	}
	defer logger1.Close()

	cfg2 := DefaultConfig()
	logger2, err := New(cfg2)
	if err != nil {
		t.Fatalf("Failed to create logger2: %v", err)
	}
	defer logger2.Close()

	// Set default to logger1
	SetDefault(logger1)

	// Verify Default() returns logger1
	if Default() != logger1 {
		t.Error("Default() should return logger1")
	}

	// Set default to logger2
	SetDefault(logger2)

	// Verify Default() returns logger2
	if Default() != logger2 {
		t.Error("Default() should return logger2")
	}
}

// ============================================================================
// CONFIG BUILD TESTS
// ============================================================================

func TestConfigBuild(t *testing.T) {
	t.Run("BasicBuild", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelInfo

		logger, err := New(cfg)
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if logger == nil {
			t.Fatal("Build() returned nil logger")
		}

		logger.Info("test message")
		if buf.Len() == 0 {
			t.Error("Logger should have written to buffer")
		}
	})

	t.Run("MustBuild", func(t *testing.T) {
		cfg := DefaultConfig()
		logger := Must(cfg)
		if logger == nil {
			t.Error("Must() should not return nil")
		}
	})

	t.Run("MustBuildPanic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Must() with invalid config should panic")
			}
		}()

		cfg := DefaultConfig()
		cfg.Level = LogLevel(99) // Invalid level
		Must(cfg)
	})

	t.Run("WithMultipleOutputs", func(t *testing.T) {
		var buf1, buf2 bytes.Buffer
		cfg := DefaultConfig()
		cfg.Outputs = []io.Writer{&buf1, &buf2}
		cfg.Level = LevelInfo

		logger, _ := New(cfg)
		logger.Info("test")

		if buf1.Len() == 0 || buf2.Len() == 0 {
			t.Error("Message should be written to all outputs")
		}
	})

	t.Run("WithFileOutput", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "test.log")
		cfg := DefaultConfig()
		cfg.File = &FileConfig{
			Path:       tmpFile,
			MaxSizeMB:  1,
			MaxBackups: 3,
		}
		cfg.Level = LevelInfo

		logger, err := New(cfg)
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		defer logger.Close()

		logger.Info("test message")

		data, err := os.ReadFile(tmpFile)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if len(data) == 0 {
			t.Error("File should contain log data")
		}
	})

	t.Run("WithJSONFormat", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := JSONConfig()
		cfg.Output = &buf

		logger, _ := New(cfg)
		logger.Info("test message")

		if !strings.Contains(buf.String(), `"message"`) {
			t.Error("Output should be JSON format")
		}
	})

	t.Run("WithSecurityConfig", func(t *testing.T) {
		var buf bytes.Buffer
		secConfig := DefaultSecurityConfig()
		secConfig.MaxMessageSize = 100

		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelInfo
		cfg.Security = secConfig

		logger, _ := New(cfg)
		logger.Info(strings.Repeat("a", 200))

		if len(buf.String()) > 150 {
			t.Error("Message should be truncated")
		}
	})

	t.Run("WithFatalHandler", func(t *testing.T) {
		var buf bytes.Buffer
		exited := false

		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelInfo
		cfg.FatalHandler = func() { exited = true }

		logger, _ := New(cfg)
		logger.Fatal("fatal message")

		if !exited {
			t.Error("FatalHandler should be called")
		}
	})
}

func TestConfigValidation(t *testing.T) {
	t.Run("InvalidLevel", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Level = LogLevel(99)

		_, err := New(cfg)
		if err == nil {
			t.Error("Build() with invalid level should return error")
		}
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Format = LogFormat(99)

		_, err := New(cfg)
		if err == nil {
			t.Error("Build() with invalid format should return error")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		var cfg *Config
		_, err := New(cfg)
		if err == nil {
			t.Error("New() with nil config should return error")
		}
	})
}

// ============================================================================
// IS LEVEL ENABLED TESTS
// ============================================================================

func TestIsLevelEnabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Level = LevelInfo
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	t.Run("IsLevelEnabled general method", func(t *testing.T) {
		// Debug is below Info, should be false
		if logger.IsLevelEnabled(LevelDebug) {
			t.Error("Debug should not be enabled when level is Info")
		}
		// Info is equal to Info, should be true
		if !logger.IsLevelEnabled(LevelInfo) {
			t.Error("Info should be enabled when level is Info")
		}
		// Warn is above Info, should be true
		if !logger.IsLevelEnabled(LevelWarn) {
			t.Error("Warn should be enabled when level is Info")
		}
		// Error is above Info, should be true
		if !logger.IsLevelEnabled(LevelError) {
			t.Error("Error should be enabled when level is Info")
		}
		// Fatal is above Info, should be true
		if !logger.IsLevelEnabled(LevelFatal) {
			t.Error("Fatal should be enabled when level is Info")
		}
	})

	t.Run("convenience methods at Info level", func(t *testing.T) {
		if logger.IsDebugEnabled() {
			t.Error("IsDebugEnabled() should return false when level is Info")
		}
		if !logger.IsInfoEnabled() {
			t.Error("IsInfoEnabled() should return true when level is Info")
		}
		if !logger.IsWarnEnabled() {
			t.Error("IsWarnEnabled() should return true when level is Info")
		}
		if !logger.IsErrorEnabled() {
			t.Error("IsErrorEnabled() should return true when level is Info")
		}
		if !logger.IsFatalEnabled() {
			t.Error("IsFatalEnabled() should return true when level is Info")
		}
	})

	t.Run("convenience methods at Debug level", func(t *testing.T) {
		logger.SetLevel(LevelDebug)
		if !logger.IsDebugEnabled() {
			t.Error("IsDebugEnabled() should return true when level is Debug")
		}
		if !logger.IsInfoEnabled() {
			t.Error("IsInfoEnabled() should return true when level is Debug")
		}
	})

	t.Run("convenience methods at Error level", func(t *testing.T) {
		logger.SetLevel(LevelError)
		if logger.IsDebugEnabled() {
			t.Error("IsDebugEnabled() should return false when level is Error")
		}
		if logger.IsInfoEnabled() {
			t.Error("IsInfoEnabled() should return false when level is Error")
		}
		if logger.IsWarnEnabled() {
			t.Error("IsWarnEnabled() should return false when level is Error")
		}
		if !logger.IsErrorEnabled() {
			t.Error("IsErrorEnabled() should return true when level is Error")
		}
		if !logger.IsFatalEnabled() {
			t.Error("IsFatalEnabled() should return true when level is Error")
		}
	})

	t.Run("thread safety", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				logger.IsLevelEnabled(LevelInfo)
			}()
			go func() {
				defer wg.Done()
				logger.SetLevel(LevelDebug)
			}()
		}
		wg.Wait()
	})
}

// ============================================================================
// NEW PACKAGE-LEVEL FUNCTIONS TESTS
// ============================================================================

func TestPackageLevelGenericLogFunctions(t *testing.T) {
	oldDefault := Default()
	defer SetDefault(oldDefault)

	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	SetDefault(logger)

	tests := []struct {
		name     string
		fn       func()
		expected string
	}{
		{
			name:     "Log",
			fn:       func() { Log(LevelInfo, "test log") },
			expected: "test log",
		},
		{
			name:     "Logf",
			fn:       func() { Logf(LevelInfo, "test %s", "logf") },
			expected: "test logf",
		},
		{
			name:     "LogWith",
			fn:       func() { LogWith(LevelInfo, "test logwith", String("key", "value")) },
			expected: "test logwith",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.fn()
			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("%s() output = %q, want to contain %q", tt.name, output, tt.expected)
			}
		})
	}
}

func TestPackageLevelGenericLogCtxFunctions(t *testing.T) {
	oldDefault := Default()
	defer SetDefault(oldDefault)

	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	cfg.ContextExtractors = []ContextExtractor{
		func(ctx context.Context) []Field {
			if traceID := ctx.Value("trace_id"); traceID != nil {
				return []Field{String("trace_id", traceID.(string))}
			}
			return nil
		},
	}
	logger, _ := New(cfg)
	SetDefault(logger)

	ctx := context.WithValue(context.Background(), "trace_id", "abc123")

	tests := []struct {
		name     string
		fn       func()
		expected string
	}{
		{
			name:     "LogCtx",
			fn:       func() { LogCtx(ctx, LevelInfo, "test logctx") },
			expected: "test logctx",
		},
		{
			name:     "LogfCtx",
			fn:       func() { LogfCtx(ctx, LevelInfo, "test %s", "logfctx") },
			expected: "test logfctx",
		},
		{
			name:     "LogWithCtx",
			fn:       func() { LogWithCtx(ctx, LevelInfo, "test logwithctx", String("key", "value")) },
			expected: "test logwithctx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.fn()
			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("%s() output = %q, want to contain %q", tt.name, output, tt.expected)
			}
			if !strings.Contains(output, "trace_id") {
				t.Errorf("%s() should contain context field trace_id", tt.name)
			}
		})
	}
}

func TestPackageLevelIsEnabledFunctions(t *testing.T) {
	oldDefault := Default()
	defer SetDefault(oldDefault)

	cfg := DefaultConfig()
	cfg.Level = LevelInfo
	logger, _ := New(cfg)
	SetDefault(logger)

	t.Run("IsLevelEnabled", func(t *testing.T) {
		if IsLevelEnabled(LevelDebug) {
			t.Error("IsLevelEnabled(Debug) should be false when level is Info")
		}
		if !IsLevelEnabled(LevelInfo) {
			t.Error("IsLevelEnabled(Info) should be true when level is Info")
		}
		if !IsLevelEnabled(LevelWarn) {
			t.Error("IsLevelEnabled(Warn) should be true when level is Info")
		}
	})

	t.Run("Convenience functions at Info level", func(t *testing.T) {
		if IsDebugEnabled() {
			t.Error("IsDebugEnabled() should be false when level is Info")
		}
		if !IsInfoEnabled() {
			t.Error("IsInfoEnabled() should be true when level is Info")
		}
		if !IsWarnEnabled() {
			t.Error("IsWarnEnabled() should be true when level is Info")
		}
		if !IsErrorEnabled() {
			t.Error("IsErrorEnabled() should be true when level is Info")
		}
		if !IsFatalEnabled() {
			t.Error("IsFatalEnabled() should be true when level is Info")
		}
	})

	t.Run("Convenience functions at Debug level", func(t *testing.T) {
		SetLevel(LevelDebug)
		if !IsDebugEnabled() {
			t.Error("IsDebugEnabled() should be true when level is Debug")
		}
		if !IsInfoEnabled() {
			t.Error("IsInfoEnabled() should be true when level is Debug")
		}
	})

	t.Run("Convenience functions at Error level", func(t *testing.T) {
		SetLevel(LevelError)
		if IsDebugEnabled() {
			t.Error("IsDebugEnabled() should be false when level is Error")
		}
		if IsInfoEnabled() {
			t.Error("IsInfoEnabled() should be false when level is Error")
		}
		if IsWarnEnabled() {
			t.Error("IsWarnEnabled() should be false when level is Error")
		}
		if !IsErrorEnabled() {
			t.Error("IsErrorEnabled() should be true when level is Error")
		}
		if !IsFatalEnabled() {
			t.Error("IsFatalEnabled() should be true when level is Error")
		}
	})
}

func TestPackageLevelWithFields(t *testing.T) {
	oldDefault := Default()
	defer SetDefault(oldDefault)

	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	SetDefault(logger)

	t.Run("WithFields", func(t *testing.T) {
		buf.Reset()
		entry := WithFields(String("service", "api"), String("version", "1.0"))
		entry.Info("test message")
		output := buf.String()
		if !strings.Contains(output, "test message") {
			t.Error("WithFields entry should log message")
		}
		if !strings.Contains(output, "service") || !strings.Contains(output, "api") {
			t.Error("WithFields entry should contain fields")
		}
		if !strings.Contains(output, "version") || !strings.Contains(output, "1.0") {
			t.Error("WithFields entry should contain fields")
		}
	})

	t.Run("WithField", func(t *testing.T) {
		buf.Reset()
		entry := WithField("request_id", "abc123")
		entry.Info("test message")
		output := buf.String()
		if !strings.Contains(output, "test message") {
			t.Error("WithField entry should log message")
		}
		if !strings.Contains(output, "request_id") || !strings.Contains(output, "abc123") {
			t.Error("WithField entry should contain field")
		}
	})

	t.Run("Chained WithFields", func(t *testing.T) {
		buf.Reset()
		entry := WithFields(String("service", "api")).
			WithFields(String("version", "1.0")).
			WithField("request_id", "xyz789")
		entry.Info("chained message")
		output := buf.String()
		if !strings.Contains(output, "service") {
			t.Error("Chained entry should contain first field")
		}
		if !strings.Contains(output, "version") {
			t.Error("Chained entry should contain second field")
		}
		if !strings.Contains(output, "request_id") {
			t.Error("Chained entry should contain third field")
		}
	})
}

func TestPackageLevelFlush(t *testing.T) {
	oldDefault := Default()
	defer SetDefault(oldDefault)

	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)
	SetDefault(logger)

	Info("test message")

	err := Flush()
	if err != nil {
		t.Errorf("Flush() returned error: %v", err)
	}
}

func TestPackageLevelFormattedCtxFunctions(t *testing.T) {
	oldDefault := Default()
	defer SetDefault(oldDefault)

	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelDebug
	logger, _ := New(cfg)
	SetDefault(logger)

	ctx := context.Background()

	tests := []struct {
		name     string
		fn       func()
		expected string
	}{
		{
			name:     "DebugfCtx",
			fn:       func() { DebugfCtx(ctx, "debug %s", "message") },
			expected: "debug message",
		},
		{
			name:     "InfofCtx",
			fn:       func() { InfofCtx(ctx, "info %s", "message") },
			expected: "info message",
		},
		{
			name:     "WarnfCtx",
			fn:       func() { WarnfCtx(ctx, "warn %s", "message") },
			expected: "warn message",
		},
		{
			name:     "ErrorfCtx",
			fn:       func() { ErrorfCtx(ctx, "error %s", "message") },
			expected: "error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.fn()
			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("%s() output = %q, want to contain %q", tt.name, output, tt.expected)
			}
		})
	}
}

func TestPackageLevelWriterManagement(t *testing.T) {
	oldDefault := Default()
	defer SetDefault(oldDefault)

	var buf1, buf2 bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf1
	cfg.Level = LevelInfo
	logger, _ := New(cfg)
	SetDefault(logger)

	initialCount := WriterCount()
	if initialCount < 1 {
		t.Errorf("Initial WriterCount() = %d, want at least 1", initialCount)
	}

	// Add writer
	err := AddWriter(&buf2)
	if err != nil {
		t.Errorf("AddWriter() returned error: %v", err)
	}

	newCount := WriterCount()
	if newCount != initialCount+1 {
		t.Errorf("WriterCount() after AddWriter = %d, want %d", newCount, initialCount+1)
	}

	// Log to both writers
	Info("test both writers")
	if buf1.String() == "" {
		t.Error("First writer should have output")
	}
	if buf2.String() == "" {
		t.Error("Second writer should have output")
	}

	// Remove writer
	err = RemoveWriter(&buf2)
	if err != nil {
		t.Errorf("RemoveWriter() returned error: %v", err)
	}

	finalCount := WriterCount()
	if finalCount != initialCount {
		t.Errorf("WriterCount() after RemoveWriter = %d, want %d", finalCount, initialCount)
	}
}

func TestPackageLevelSampling(t *testing.T) {
	oldDefault := Default()
	defer SetDefault(oldDefault)

	cfg := DefaultConfig()
	cfg.Level = LevelInfo
	logger, _ := New(cfg)
	SetDefault(logger)

	// Test initial sampling is nil
	initialSampling := GetSampling()
	if initialSampling != nil {
		t.Error("Initial GetSampling() should be nil")
	}

	// Set sampling
	sampling := &SamplingConfig{
		Enabled:    true,
		Initial:    2,
		Thereafter: 5,
	}
	SetSampling(sampling)

	// Verify sampling was set
	newSampling := GetSampling()
	if newSampling == nil {
		t.Fatal("GetSampling() after SetSampling should not be nil")
	}
	if !newSampling.Enabled {
		t.Error("SamplingConfig.Enabled should be true")
	}
	if newSampling.Initial != 2 {
		t.Errorf("SamplingConfig.Initial = %d, want 2", newSampling.Initial)
	}
	if newSampling.Thereafter != 5 {
		t.Errorf("SamplingConfig.Thereafter = %d, want 5", newSampling.Thereafter)
	}
}
