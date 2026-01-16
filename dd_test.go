package dd

import (
	"bytes"
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
// CONFIG CHAIN METHODS TESTS
// ============================================================================

func TestConfigWithFile(t *testing.T) {
	t.Run("WithFile", func(t *testing.T) {
		config := DefaultConfig()
		tmpFile := filepath.Join(t.TempDir(), "test.log")

		result, err := config.WithFile(tmpFile, FileWriterConfig{})
		if err != nil {
			t.Fatalf("WithFile() error = %v", err)
		}
		if result == nil {
			t.Fatal("WithFile() should return config")
		}
		if len(result.Writers) == 0 {
			t.Error("WithFile() should add writer")
		}

		// Close the file writer
		if len(result.Writers) > 0 {
			if fw, ok := result.Writers[len(result.Writers)-1].(*FileWriter); ok {
				fw.Close()
			}
		}
	})

	t.Run("WithFileOnly", func(t *testing.T) {
		config := DefaultConfig()
		tmpFile := filepath.Join(t.TempDir(), "test.log")

		result, err := config.WithFileOnly(tmpFile, FileWriterConfig{})
		if err != nil {
			t.Fatalf("WithFileOnly() error = %v", err)
		}
		if result == nil {
			t.Fatal("WithFileOnly() should return config")
		}
		if len(result.Writers) != 1 {
			t.Errorf("WithFileOnly() should have 1 writer, got %d", len(result.Writers))
		}

		if len(result.Writers) > 0 {
			if fw, ok := result.Writers[0].(*FileWriter); ok {
				fw.Close()
			}
		}
	})

	t.Run("WithFileNilConfig", func(t *testing.T) {
		var config *LoggerConfig
		tmpFile := filepath.Join(t.TempDir(), "test.log")

		_, err := config.WithFile(tmpFile, FileWriterConfig{})
		if err == nil {
			t.Error("WithFile() on nil config should return error")
		}
	})

	t.Run("WithFileOnlyNilConfig", func(t *testing.T) {
		var config *LoggerConfig
		tmpFile := filepath.Join(t.TempDir(), "test.log")

		_, err := config.WithFileOnly(tmpFile, FileWriterConfig{})
		if err == nil {
			t.Error("WithFileOnly() on nil config should return error")
		}
	})

	t.Run("WithFileEmptyPath", func(t *testing.T) {
		config := DefaultConfig()

		_, err := config.WithFile("", FileWriterConfig{})
		if err == nil {
			t.Error("WithFile() with empty path should return error")
		}
	})

	t.Run("WithFileOnlyEmptyPath", func(t *testing.T) {
		config := DefaultConfig()

		_, err := config.WithFileOnly("", FileWriterConfig{})
		if err == nil {
			t.Error("WithFileOnly() with empty path should return error")
		}
	})
}

func TestConfigWithWriter(t *testing.T) {
	var buf bytes.Buffer

	t.Run("ValidWriter", func(t *testing.T) {
		config := DefaultConfig()
		result := config.WithWriter(&buf)

		if result == nil {
			t.Fatal("WithWriter() should return config")
		}
		if len(result.Writers) == 0 {
			t.Error("WithWriter() should add writer")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		var config *LoggerConfig
		result := config.WithWriter(&buf)

		if result != nil {
			t.Error("WithWriter() on nil config should return nil")
		}
	})

	t.Run("NilWriter", func(t *testing.T) {
		config := DefaultConfig()
		result := config.WithWriter(nil)

		if result == nil {
			t.Fatal("WithWriter() with nil writer should return config")
		}
	})
}

func TestConfigWithFilter(t *testing.T) {
	filter := NewSensitiveDataFilter()

	t.Run("ValidFilter", func(t *testing.T) {
		config := DefaultConfig()
		result := config.WithFilter(filter)

		if result == nil {
			t.Fatal("WithFilter() should return config")
		}
		if result.SecurityConfig == nil {
			t.Error("WithFilter() should create security config")
		}
		if result.SecurityConfig.SensitiveFilter == nil {
			t.Error("WithFilter() should set filter")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		var config *LoggerConfig
		result := config.WithFilter(filter)

		if result != nil {
			t.Error("WithFilter() on nil config should return nil")
		}
	})

	t.Run("NilFilter", func(t *testing.T) {
		config := DefaultConfig()
		result := config.WithFilter(nil)

		if result == nil {
			t.Fatal("WithFilter() with nil filter should return config")
		}
		if result.SecurityConfig == nil {
			t.Error("WithFilter() should create security config")
		}
	})
}

func TestConfigClone(t *testing.T) {
	t.Run("FullClone", func(t *testing.T) {
		original := DefaultConfig()
		original.SecurityConfig.SensitiveFilter = NewSensitiveDataFilter()
		original.JSON = DefaultJSONOptions()
		clone := original.Clone()

		if clone == nil {
			t.Fatal("Clone() returned nil")
		}

		// Modify clone
		clone.Level = LevelDebug
		if original.Level == LevelDebug {
			t.Error("Clone should not affect original")
		}

		// Modify security filter
		clone.SecurityConfig.SensitiveFilter.AddPattern(`test=\w+`)
		if original.SecurityConfig.SensitiveFilter.PatternCount() == clone.SecurityConfig.SensitiveFilter.PatternCount() {
			t.Error("Modifying clone filter should not affect original")
		}
	})

	t.Run("NilClone", func(t *testing.T) {
		var config *LoggerConfig
		clone := config.Clone()

		if clone == nil {
			t.Error("Clone() of nil should return default config")
		}
	})
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *LoggerConfig
		wantErr bool
	}{
		{"nil config", nil, true},
		{"valid", DefaultConfig(), false},
		{"invalid level", &LoggerConfig{Level: LogLevel(99), Format: FormatText}, true},
		{"invalid format", &LoggerConfig{Level: LevelInfo, Format: LogFormat(99)}, true},
		{"no writers", &LoggerConfig{Level: LevelInfo, Format: FormatText}, false},
		{"no time format with include time", &LoggerConfig{Level: LevelInfo, Format: FormatText, IncludeTime: true}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ============================================================================
// LOGGER CREATION AND CONFIGURATION TESTS
// ============================================================================

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Level != LevelInfo {
		t.Errorf("Default level = %v, want %v", config.Level, LevelInfo)
	}
	if config.Format != FormatText {
		t.Errorf("Default format = %v, want %v", config.Format, FormatText)
	}
	if config.SecurityConfig == nil {
		t.Error("Default should have security config")
	}
}

func TestDevelopmentConfig(t *testing.T) {
	config := DevelopmentConfig()

	if config.Level != LevelDebug {
		t.Errorf("Dev level = %v, want %v", config.Level, LevelDebug)
	}
	if !config.DynamicCaller {
		t.Error("Dev config should enable dynamic caller")
	}
}

func TestJSONConfig(t *testing.T) {
	config := JSONConfig()

	if config.Format != FormatJSON {
		t.Errorf("Json config format = %v, want %v", config.Format, FormatJSON)
	}
	if config.JSON == nil {
		t.Error("Json config should have Json options")
	}
}

func TestLoggerCreation(t *testing.T) {
	var buf bytes.Buffer

	config := &LoggerConfig{
		Level:       LevelInfo,
		Format:      FormatText,
		TimeFormat:  DefaultTimeFormat,
		Writers:     []io.Writer{&buf},
		IncludeTime: true,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if logger == nil {
		t.Fatal("New() returned nil logger")
	}

	logger.Info("test message")
	if buf.Len() == 0 {
		t.Error("Logger should have written to buffer")
	}
}

func TestLoggerSetSecurityConfig(t *testing.T) {
	logger, _ := New(DefaultConfig())

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
	logger, _ := New(&LoggerConfig{
		Level:        LevelInfo,
		IncludeLevel: true,
		Writers:      []io.Writer{&buf},
	})

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
	logger, _ := New(&LoggerConfig{
		Level:   LevelInfo,
		Writers: []io.Writer{&buf},
	})

	logger.Infof("test %s", "message")
	if !strings.Contains(buf.String(), "test message") {
		t.Error("Formatted logging should work")
	}
}

func TestAllFormattedMethods(t *testing.T) {
	var buf bytes.Buffer
	logger, _ := New(&LoggerConfig{
		Level:   LevelDebug,
		Writers: []io.Writer{&buf},
	})

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
	logger, _ := New(&LoggerConfig{
		Level:   LevelInfo,
		Writers: []io.Writer{&buf},
	})

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
	logger, _ := New(&LoggerConfig{
		Level:   LevelDebug,
		Writers: []io.Writer{&buf},
	})

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

		logger, _ := New(&LoggerConfig{
			Level:        LevelInfo,
			Writers:      []io.Writer{&buf},
			FatalHandler: func() { exited = true },
		})

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

		logger, _ := New(&LoggerConfig{
			Level:        LevelInfo,
			Writers:      []io.Writer{&buf},
			FatalHandler: func() { exited = true },
		})

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

		logger, _ := New(&LoggerConfig{
			Level:        LevelInfo,
			Writers:      []io.Writer{&buf},
			FatalHandler: func() { exited = true },
		})

		logger.FatalWith("fatal", String("key", "value"))

		if !exited {
			t.Error("FatalWith handler should be called")
		}
	})
}

func TestJSONLogging(t *testing.T) {
	var buf bytes.Buffer
	logger, _ := New(&LoggerConfig{
		Level:   LevelInfo,
		Format:  FormatJSON,
		Writers: []io.Writer{&buf},
	})

	logger.Info("test message")

	if !strings.Contains(buf.String(), `"message":"test message"`) {
		t.Error("Json logging should format as Json")
	}
}

// ============================================================================
// LOG LEVEL MANAGEMENT TESTS
// ============================================================================

func TestLoggerLevelManagement(t *testing.T) {
	var buf bytes.Buffer
	logger, _ := New(&LoggerConfig{
		Level:   LevelInfo,
		Writers: []io.Writer{&buf},
	})

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
	logger, _ := New(&LoggerConfig{Level: LevelDebug, Writers: []io.Writer{&buf}})
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

func TestAllGlobalFunctions(t *testing.T) {
	oldDefault := Default()
	defer SetDefault(oldDefault)

	var buf bytes.Buffer
	logger, _ := New(&LoggerConfig{Level: LevelDebug, Writers: []io.Writer{&buf}})
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
	logger, _ := New(&LoggerConfig{Level: LevelDebug, Writers: []io.Writer{&buf}})
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
	logger, _ := New(&LoggerConfig{
		Level:   LevelDebug,
		Writers: []io.Writer{&buf},
	})
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
	logger, _ := New(&LoggerConfig{
		Level:   LevelInfo,
		Writers: []io.Writer{&buf1},
	})

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
	mw.AddWriter(&buf3)
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
	fw, _ := NewFileWriter(tmpFile, FileWriterConfig{})

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
	logger, _ := New(&LoggerConfig{
		Level:          LevelInfo,
		IncludeTime:    false,
		IncludeLevel:   false,
		Writers:        []io.Writer{safeWriter},
		SecurityConfig: &SecurityConfig{SensitiveFilter: nil},
	})

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
	logger, _ := New(DefaultConfig())

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
	logger, _ := New(DefaultConfig())
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
	logger, _ := New(&LoggerConfig{
		Level:          LevelInfo,
		Writers:        []io.Writer{&buf},
		SecurityConfig: DefaultSecurityConfig(),
	})

	// Test control character sanitization
	logger.Info("test\x00message\x1f")
	output := buf.String()
	if strings.Contains(output, "\x00") || strings.Contains(output, "\x1f") {
		t.Error("Control characters should be sanitized")
	}
}

func TestMessageSizeLimit(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultSecurityConfig()
	config.MaxMessageSize = 100

	logger, _ := New(&LoggerConfig{
		Level:          LevelInfo,
		Writers:        []io.Writer{&buf},
		SecurityConfig: config,
	})

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
	config := &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: filter,
	}

	logger, _ := New(&LoggerConfig{
		Level:          LevelInfo,
		Writers:        []io.Writer{&buf},
		SecurityConfig: config,
	})

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
	logger, _ := New(DefaultConfig())

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
	logger, _ := New(&LoggerConfig{Writers: []io.Writer{&buf}})

	// Clear default writer and add 99 more
	logger.writers = nil
	for i := 0; i < 99; i++ {
		var b bytes.Buffer
		logger.writers = append(logger.writers, &b)
	}

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
	logger, _ := New(DefaultConfig())
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
	logger, _ := New(&LoggerConfig{
		Level:   LevelInfo,
		Writers: []io.Writer{&buf},
	})

	// Empty message
	logger.Info()
	logger.Info("")

	// Nil fields
	logger.InfoWith("test")

	// Should not panic
}

func TestLargeMessage(t *testing.T) {
	var buf bytes.Buffer
	logger, _ := New(&LoggerConfig{
		Level:   LevelInfo,
		Writers: []io.Writer{&buf},
	})

	largeMsg := strings.Repeat("test", 10000)
	logger.Info(largeMsg)

	if buf.Len() == 0 {
		t.Error("Large message should be logged")
	}
}

func TestManyFields(t *testing.T) {
	var buf bytes.Buffer
	logger, _ := New(&LoggerConfig{
		Level:   LevelInfo,
		Writers: []io.Writer{&buf},
	})

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

	fw, _ := NewFileWriter(tmpFile, FileWriterConfig{})
	logger, _ := New(&LoggerConfig{Writers: []io.Writer{fw}})

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
	stdoutLogger, _ := New(&LoggerConfig{Writers: []io.Writer{os.Stdout}})
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
// CONVENIENCE CONSTRUCTORS TESTS
// ============================================================================

func TestConvenienceConstructors(t *testing.T) {
	t.Run("ToFile", func(t *testing.T) {
		logger := ToFile()
		if logger == nil {
			t.Fatal("ToFile should return logger")
		}
		logger.Close()
	})

	t.Run("ToFileWithPath", func(t *testing.T) {
		logger := ToFile(t.TempDir() + "/test.log")
		if logger == nil {
			t.Fatal("ToFile should return logger")
		}
		logger.Close()
	})

	t.Run("ToJSONFile", func(t *testing.T) {
		logger := ToJSONFile()
		if logger == nil {
			t.Fatal("ToJSONFile should return logger")
		}
		logger.Close()
	})

	t.Run("ToConsole", func(t *testing.T) {
		logger := ToConsole()
		if logger == nil {
			t.Fatal("ToConsole should return logger")
		}
		logger.Close()
	})

	t.Run("ToAll", func(t *testing.T) {
		logger := ToAll()
		if logger == nil {
			t.Fatal("ToAll should return logger")
		}
		logger.Close()
	})
}

// ============================================================================
// OPTIONS CONSTRUCTOR TESTS
// ============================================================================

func TestNewWithOptions(t *testing.T) {
	var buf bytes.Buffer

	t.Run("BasicOptions", func(t *testing.T) {
		logger, err := NewWithOptions(Options{
			Level:   LevelDebug,
			Format:  FormatJSON,
			Console: false,
		})
		if err != nil {
			t.Fatalf("NewWithOptions should not error: %v", err)
		}
		if logger == nil {
			t.Fatal("Logger should be created")
		}
		logger.Close()
	})

	t.Run("WithAdditionalWriters", func(t *testing.T) {
		logger, err := NewWithOptions(Options{
			Level:             LevelInfo,
			AdditionalWriters: []io.Writer{&buf},
		})
		if err != nil {
			t.Fatalf("NewWithOptions should not error: %v", err)
		}
		if logger == nil {
			t.Fatal("Logger should be created")
		}
		logger.Close()
	})

	t.Run("WithFilterLevel", func(t *testing.T) {
		logger, err := NewWithOptions(Options{
			FilterLevel: "basic",
		})
		if err != nil {
			t.Fatalf("NewWithOptions should not error: %v", err)
		}
		if logger == nil {
			t.Fatal("Logger should be created")
		}
		logger.Close()
	})
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

// ============================================================================
// FMT REPLACEMENT TESTS
// ============================================================================

func TestPrintfReplacement(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Printf("test %s", "message")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test message") {
		t.Error("Printf() should work")
	}
}

func TestPrintReplacement(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Print("test", "message")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test message") {
		t.Error("Print() should work")
	}
}

func TestPrintlnReplacement(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Println("test", "message")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test message") {
		t.Error("Println() should work")
	}
}

func TestSprintfReplacement(t *testing.T) {
	result := Sprintf("test %s", "formatted")

	if result != "test formatted" {
		t.Errorf("Sprintf() = %q, want %q", result, "test formatted")
	}
}

func TestSprintReplacement(t *testing.T) {
	result := Sprint("test", "message")

	if result != "testmessage" {
		t.Errorf("Sprint() = %q, want %q", result, "testmessage")
	}
}

func TestSprintlnReplacement(t *testing.T) {
	result := Sprintln("test", "message")

	expected := "test message\n"
	if result != expected {
		t.Errorf("Sprintln() = %q, want %q", result, expected)
	}
}

func TestFprintReplacement(t *testing.T) {
	var buf bytes.Buffer

	n, _ := Fprint(&buf, "test", "message")
	if n <= 0 {
		t.Error("Fprint() should write bytes")
	}
	if !strings.Contains(buf.String(), "testmessage") {
		t.Error("Fprint() should write to buffer")
	}
}

func TestFprintlnReplacement(t *testing.T) {
	var buf bytes.Buffer

	n, _ := Fprintln(&buf, "test", "message")
	if n <= 0 {
		t.Error("Fprintln() should write bytes")
	}
	if !strings.Contains(buf.String(), "test message\n") {
		t.Error("Fprintln() should write with newline")
	}
}

func TestFprintfReplacement(t *testing.T) {
	var buf bytes.Buffer

	n, _ := Fprintf(&buf, "test %s", "formatted")
	if n <= 0 {
		t.Error("Fprintf() should write bytes")
	}
	if !strings.Contains(buf.String(), "test formatted") {
		t.Error("Fprintf() should write formatted text")
	}
}

func TestAppendReplacement(t *testing.T) {
	dst := []byte("prefix ")
	result := Append(dst, "test", "message")

	if !bytes.Contains(result, []byte("prefix testmessage")) {
		t.Error("Append() should append to buffer")
	}
}

func TestAppendlnReplacement(t *testing.T) {
	dst := []byte("prefix ")
	result := Appendln(dst, "test", "message")

	if !bytes.Contains(result, []byte("prefix test message\n")) {
		t.Error("Appendln() should append with newline")
	}
}

func TestAppendFormatReplacement(t *testing.T) {
	dst := []byte("prefix ")
	result := AppendFormat(dst, "test %s", "formatted")

	if !bytes.Contains(result, []byte("prefix test formatted")) {
		t.Error("AppendFormat() should append formatted text")
	}
}

func TestNewError(t *testing.T) {
	err := NewError("test error: %s", "context")

	if err == nil {
		t.Fatal("NewError() should return error")
	}
	if !strings.Contains(err.Error(), "test error") {
		t.Error("NewError() should contain message")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Error("NewError() should contain formatted context")
	}
}

func TestScanReplacement(t *testing.T) {
	// Just verify it compiles and doesn't panic
	var s string
	n, _ := Scan(&s)
	if n < 0 {
		t.Error("Scan() should return non-negative count")
	}
}

func TestSscanReplacement(t *testing.T) {
	var s string
	n, _ := Sscan("hello world", &s)
	if n != 1 {
		t.Errorf("Sscan() should scan 1 item, got %d", n)
	}
	if s != "hello" {
		t.Errorf("Sscan() result = %q, want %q", s, "hello")
	}
}

func TestSscanfReplacement(t *testing.T) {
	var s string
	n, _ := Sscanf("hello world", "%s", &s)
	if n != 1 {
		t.Errorf("Sscanf() should scan 1 item, got %d", n)
	}
	if s != "hello" {
		t.Errorf("Sscanf() result = %q, want %q", s, "hello")
	}
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

func TestFullLoggingPipeline(t *testing.T) {
	tmpFile := t.TempDir() + "/test.log"

	fw, _ := NewFileWriter(tmpFile, FileWriterConfig{})
	defer fw.Close()

	logger, _ := New(&LoggerConfig{
		Level:          LevelInfo,
		IncludeTime:    true,
		IncludeLevel:   true,
		DynamicCaller:  false,
		Writers:        []io.Writer{fw},
		SecurityConfig: DefaultSecurityConfig(),
	})

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
	logger, _ := New(&LoggerConfig{
		Level:        LevelInfo,
		Format:       FormatJSON,
		IncludeTime:  true,
		IncludeLevel: true,
		Writers:      []io.Writer{&buf},
	})

	logger.InfoWith("test", String("key", "value"), Int("count", 42))

	output := buf.String()

	// Should be valid Json
	var jsonData map[string]any
	if err := json.Unmarshal([]byte(output), &jsonData); err != nil {
		t.Fatalf("Output is not valid Json: %v", err)
	}

	// Should have required fields
	if jsonData["message"] == nil {
		t.Error("Json should have message field")
	}
	if jsonData["level"] == nil {
		t.Error("Json should have level field")
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

	Json(map[string]string{"key": "value"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should be valid Json
	if !strings.Contains(output, `"key"`) || !strings.Contains(output, `"value"`) {
		t.Error("Json() should output Json")
	}
}

func TestDebugVisualizationJsonf(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Jsonf("test: %s", "formatted")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test: formatted") {
		t.Error("Jsonf() should output formatted data as Json")
	}
}

func TestTypeConverter(t *testing.T) {
	// Test simple types
	isSimple := isSimpleType("test")
	if !isSimple {
		t.Error("String should be simple type")
	}

	isSimple = isSimpleType(42)
	if !isSimple {
		t.Error("Int should be simple type")
	}

	isSimple = isSimpleType(map[string]string{})
	if isSimple {
		t.Error("Map should not be simple type")
	}

	// Test format simple value
	formatted := formatSimpleValue("test")
	if formatted != "test" {
		t.Errorf("formatSimpleValue(string) = %q, want %q", formatted, "test")
	}

	formatted = formatSimpleValue(nil)
	if formatted != "nil" {
		t.Errorf("formatSimpleValue(nil) = %q, want %q", formatted, "nil")
	}
}

func TestTypeConverterComplexTypes(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test slice
	Json([]int{1, 2, 3})

	// Test map
	Json(map[string]int{"one": 1, "two": 2})

	// Test struct
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	Json(TestStruct{Name: "John", Age: 30})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should be valid Json output
	if !strings.Contains(output, "[") && !strings.Contains(output, "{") {
		t.Error("Complex types should be converted to Json")
	}
}

// ============================================================================
// ERROR HANDLING TESTS
// ============================================================================

func TestErrorReturns(t *testing.T) {
	// Test New() with nil config - it returns a default logger
	logger, err := New(nil)
	if logger == nil {
		t.Error("New(nil) should return default logger, not nil")
	}
	_ = err // Use err to avoid lint error

	// Test NewFileWriter with invalid path
	_, err = NewFileWriter("\x00invalid", FileWriterConfig{})
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
	logger, _ := New(DefaultConfig())

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
