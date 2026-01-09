package dd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// WRITER TESTS
// ============================================================================

func TestFileWriter(t *testing.T) {
	tmpFile := "test_file_writer.log"
	defer os.Remove(tmpFile)

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

	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}
}

func TestFileWriterInvalidPath(t *testing.T) {
	_, err := NewFileWriter("\x00invalid", FileWriterConfig{})
	if err == nil {
		t.Error("NewFileWriter() should fail with invalid path")
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

	err = bw.Flush()
	if err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	if buf.Len() == 0 {
		t.Error("BufferedWriter should have written data after flush")
	}
}

func TestBufferedWriterNilWriter(t *testing.T) {
	_, err := NewBufferedWriter(nil, 4096)
	if err == nil {
		t.Error("NewBufferedWriter() should fail with nil writer")
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

	if buf1.String() != string(data) {
		t.Error("Writer 1 did not receive data")
	}
	if buf2.String() != string(data) {
		t.Error("Writer 2 did not receive data")
	}
	if buf3.String() != string(data) {
		t.Error("Writer 3 did not receive data")
	}
}

func TestMultiWriterNoWriters(t *testing.T) {
	mw := NewMultiWriter()
	n, err := mw.Write([]byte("test"))
	if err != nil {
		t.Errorf("Write() with no writers should not error, got: %v", err)
	}
	if n != 4 {
		t.Errorf("Write() should return data length, got %d", n)
	}
}

// ============================================================================
// FIELD CONSTRUCTOR TESTS
// ============================================================================

func TestFieldConstructors(t *testing.T) {
	t.Run("Any", func(t *testing.T) {
		field := Any("key", "value")
		if field.Key != "key" || field.Value != "value" {
			t.Error("Any() should create field correctly")
		}
	})

	t.Run("String", func(t *testing.T) {
		field := String("name", "test")
		if field.Key != "name" || field.Value != "test" {
			t.Error("String() should create field correctly")
		}
	})

	t.Run("Int", func(t *testing.T) {
		field := Int("count", 42)
		if field.Key != "count" || field.Value != 42 {
			t.Error("Int() should create field correctly")
		}
	})

	t.Run("Int64", func(t *testing.T) {
		field := Int64("bignum", int64(9999999999))
		if field.Key != "bignum" || field.Value != int64(9999999999) {
			t.Error("Int64() should create field correctly")
		}
	})

	t.Run("Bool", func(t *testing.T) {
		field := Bool("flag", true)
		if field.Key != "flag" || field.Value != true {
			t.Error("Bool() should create field correctly")
		}
	})

	t.Run("Float64", func(t *testing.T) {
		field := Float64("pi", 3.14159)
		if field.Key != "pi" || field.Value != 3.14159 {
			t.Error("Float64() should create field correctly")
		}
	})

	t.Run("Err", func(t *testing.T) {
		err := io.EOF
		field := Err(err)
		if field.Key != "error" || field.Value != "EOF" {
			t.Errorf("Err() should create error field, got key=%s value=%v", field.Key, field.Value)
		}
	})

	t.Run("ErrNil", func(t *testing.T) {
		field := Err(nil)
		if field.Key != "error" || field.Value != nil {
			t.Error("Err(nil) should create field with nil value")
		}
	})
}

// ============================================================================
// CORE LOGGER TESTS
// ============================================================================

func TestLoggerCreation(t *testing.T) {
	tests := []struct {
		name    string
		config  *LoggerConfig
		wantErr bool
	}{
		{
			name:    "default config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: false, // Should use default
		},
		{
			name: "invalid level",
			config: &LoggerConfig{
				Level:  LogLevel(99),
				Format: FormatText,
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			config: &LoggerConfig{
				Level:  LevelInfo,
				Format: LogFormat(99),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if logger != nil {
				logger.Close()
			}
		})
	}
}

func TestBasicLogging(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.Writers = []io.Writer{&buf}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected output to contain 'test message', got: %s", output)
	}
	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected output to contain 'INFO', got: %s", output)
	}
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.Level = LevelWarn
	config.Writers = []io.Writer{&buf}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()

	// Debug and Info should be filtered out
	if strings.Contains(output, "debug message") {
		t.Errorf("Debug message should be filtered out")
	}
	if strings.Contains(output, "info message") {
		t.Errorf("Info message should be filtered out")
	}

	// Warn and Error should be present
	if !strings.Contains(output, "warn message") {
		t.Errorf("Warn message should be present")
	}
	if !strings.Contains(output, "error message") {
		t.Errorf("Error message should be present")
	}
}

func TestStructuredLogging(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.Writers = []io.Writer{&buf}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.InfoWith("test message", String("key", "value"), Int("number", 42))

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected output to contain 'test message', got: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("Expected output to contain 'key=value', got: %s", output)
	}
	if !strings.Contains(output, "number=42") {
		t.Errorf("Expected output to contain 'number=42', got: %s", output)
	}
}

func TestJSONLogging(t *testing.T) {
	var buf bytes.Buffer
	config := JSONConfig()
	config.Writers = []io.Writer{&buf}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, `"message":"test message"`) {
		t.Errorf("Expected JSON output to contain message field, got: %s", output)
	}
	if !strings.Contains(output, `"level":"INFO"`) {
		t.Errorf("Expected JSON output to contain level field, got: %s", output)
	}
}

func TestFormattedLogging(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.Writers = []io.Writer{&buf}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Infof("User %s has %d items", "john", 42)

	output := buf.String()
	if !strings.Contains(output, "User john has 42 items") {
		t.Errorf("Expected formatted output, got: %s", output)
	}
}

// ============================================================================
// LOGGER STATE MANAGEMENT TESTS
// ============================================================================

func TestLoggerLevelManagement(t *testing.T) {
	logger, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test initial level
	if logger.GetLevel() != LevelInfo {
		t.Errorf("Expected initial level Info, got %v", logger.GetLevel())
	}

	// Test level change
	err = logger.SetLevel(LevelWarn)
	if err != nil {
		t.Errorf("SetLevel failed: %v", err)
	}

	if logger.GetLevel() != LevelWarn {
		t.Errorf("Expected level Warn, got %v", logger.GetLevel())
	}

	// Test invalid level
	err = logger.SetLevel(LogLevel(99))
	if err == nil {
		t.Error("Expected error for invalid level")
	}
}

func TestLoggerWriterManagement(t *testing.T) {
	logger, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	var buf bytes.Buffer

	// Test add writer
	err = logger.AddWriter(&buf)
	if err != nil {
		t.Errorf("AddWriter failed: %v", err)
	}

	logger.Info("test")
	if buf.Len() == 0 {
		t.Error("Writer should have received message")
	}

	// Test remove writer
	err = logger.RemoveWriter(&buf)
	if err != nil {
		t.Errorf("RemoveWriter failed: %v", err)
	}

	buf.Reset()
	logger.Info("test2")
	if buf.Len() > 0 {
		t.Error("Removed writer should not receive messages")
	}

	// Test nil writer
	err = logger.AddWriter(nil)
	if err == nil {
		t.Error("Expected error for nil writer")
	}
}

func TestLoggerClose(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.Writers = []io.Writer{&buf}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.Info("before close")
	initialLen := buf.Len()

	err = logger.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	logger.Info("after close")
	if buf.Len() != initialLen {
		t.Error("Should not log after close")
	}

	// Test operations after close
	err = logger.AddWriter(&bytes.Buffer{})
	if err == nil {
		t.Error("Should return error when adding writer after close")
	}

	// Multiple closes should not panic
	logger.Close()
	logger.Close()
}

// ============================================================================
// CONVENIENCE FUNCTIONS TESTS
// ============================================================================

func TestConvenienceFunctions(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.Writers = []io.Writer{&buf}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Store original default
	originalDefault := Default()
	defer SetDefault(originalDefault)

	// Set test logger as default
	SetDefault(logger)

	Info("convenience test")

	output := buf.String()
	if !strings.Contains(output, "convenience test") {
		t.Errorf("Expected output to contain 'convenience test', got: %s", output)
	}
}

func TestGlobalLevelSetting(t *testing.T) {
	originalDefault := Default()
	defer SetDefault(originalDefault)

	var buf bytes.Buffer
	config := DefaultConfig()
	config.Writers = []io.Writer{&buf}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	SetDefault(logger)
	SetLevel(LevelWarn)

	Debug("debug message")
	Info("info message")
	Warn("warn message")

	output := buf.String()
	if strings.Contains(output, "debug message") || strings.Contains(output, "info message") {
		t.Error("Debug and Info messages should be filtered")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("Warn message should be present")
	}
}

// ============================================================================
// CONCURRENCY TESTS
// ============================================================================

func TestConcurrentLogging(t *testing.T) {
	config := DefaultConfig()
	config.Writers = []io.Writer{io.Discard}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	numGoroutines := 100
	messagesPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Infof("goroutine %d message %d", id, j)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentWriterOperations(t *testing.T) {
	config := DefaultConfig()
	config.Writers = []io.Writer{io.Discard}
	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup

	// Concurrent add/remove writers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.AddWriter(io.Discard)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.RemoveWriter(io.Discard)
		}()
	}

	// Concurrent logging
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info("test message")
		}()
	}

	wg.Wait()
}

func TestConcurrentLevelChanges(t *testing.T) {
	config := DefaultConfig()
	config.Writers = []io.Writer{io.Discard}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup

	// Rapidly change levels
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			logger.SetLevel(LogLevel(i % 5))
		}
	}()

	// Log while levels are changing
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			logger.Info("test message")
		}
	}()

	wg.Wait()
}

// ============================================================================
// EDGE CASES AND ERROR CONDITIONS
// ============================================================================

func TestEdgeCases(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.Writers = []io.Writer{&buf}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	t.Run("empty_message", func(t *testing.T) {
		buf.Reset()
		logger.Info("")
		if buf.Len() == 0 {
			t.Error("Should log empty message")
		}
	})

	t.Run("nil_fields", func(t *testing.T) {
		buf.Reset()
		logger.InfoWith("test", Any("key", nil))
		if buf.Len() == 0 {
			t.Error("Should log message with nil field")
		}
	})

	t.Run("special_characters", func(t *testing.T) {
		buf.Reset()
		logger.Info("test\nmessage\rwith\tspecial\x00chars")
		output := buf.String()
		if strings.Contains(output, "\x00") {
			t.Error("Should sanitize null bytes")
		}
	})

	t.Run("unicode_message", func(t *testing.T) {
		buf.Reset()
		logger.Info("测试消息 🚀 тест")
		if buf.Len() == 0 {
			t.Error("Should log unicode message")
		}
	})

	t.Run("very_long_field_key", func(t *testing.T) {
		buf.Reset()
		longKey := strings.Repeat("a", 1000)
		logger.InfoWith("test", String(longKey, "value"))
		if buf.Len() == 0 {
			t.Error("Should log message with long field key")
		}
	})
}

func TestLargeMessages(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.Writers = []io.Writer{&buf}
	config.SecurityConfig = &SecurityConfig{
		MaxMessageSize: 10 * 1024 * 1024, // 10MB
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create 1MB message
	largeMsg := strings.Repeat("A", 1024*1024)
	logger.Info(largeMsg)

	if buf.Len() == 0 {
		t.Error("Logger should handle large messages")
	}
}

func TestManyFields(t *testing.T) {
	var buf bytes.Buffer
	config := JSONConfig()
	config.Writers = []io.Writer{&buf}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	fields := make([]Field, 100)
	for i := 0; i < 100; i++ {
		fields[i] = Int("field"+string(rune(i)), i)
	}

	logger.InfoWith("test message", fields...)

	if buf.Len() == 0 {
		t.Error("Logger should handle many fields")
	}
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

func TestLoggerWithFileWriter(t *testing.T) {
	tmpFile := "test_logger_file.log"
	defer os.Remove(tmpFile)

	fw, err := NewFileWriter(tmpFile, FileWriterConfig{MaxSizeMB: 1})
	if err != nil {
		t.Fatalf("NewFileWriter() error = %v", err)
	}
	defer fw.Close()

	config := DefaultConfig()
	config.Writers = []io.Writer{fw}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Close()

	logger.Info("test message to file")

	time.Sleep(100 * time.Millisecond)

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test message to file") {
		t.Errorf("Log file should contain message, got: %s", string(content))
	}
}

func TestLoggerWithBufferedWriter(t *testing.T) {
	var buf bytes.Buffer
	bw, err := NewBufferedWriter(&buf, 4096)
	if err != nil {
		t.Fatalf("NewBufferedWriter() error = %v", err)
	}
	defer bw.Close()

	config := DefaultConfig()
	config.Writers = []io.Writer{bw}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer logger.Close()

	logger.Info("buffered message")

	bw.Flush()

	if !strings.Contains(buf.String(), "buffered message") {
		t.Errorf("Buffer should contain message, got: %s", buf.String())
	}
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// failingWriter is a writer that fails after N writes
type failingWriter struct {
	failAfter int
	count     int
	mu        sync.Mutex
}

func (fw *failingWriter) Write(p []byte) (int, error) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	fw.count++
	if fw.count > fw.failAfter {
		return 0, errors.New("write failed")
	}
	return len(p), nil
}
