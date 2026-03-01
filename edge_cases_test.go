package dd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// EDGE CASE TESTS FOR FIELDS
// ============================================================================

func TestEmptyMessageWithFields(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	// Empty message with fields should still log
	logger.InfoWith("", String("key", "value"))

	output := buf.String()
	if !strings.Contains(output, "key=value") {
		t.Errorf("Expected 'key=value' in output, got: %s", output)
	}
}

func TestVeryLargeFieldCount(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	// Create 1000 fields
	fields := make([]Field, 1000)
	for i := 0; i < 1000; i++ {
		fields[i] = Int("field_"+strings.Repeat("a", 20), i)
	}

	logger.InfoWith("many fields", fields...)

	output := buf.String()
	if !strings.Contains(output, "many fields") {
		t.Error("Should contain message")
	}
	// Check that at least some fields are present
	if !strings.Contains(output, "field_") {
		t.Error("Should contain field data")
	}
}

func TestMaximumMessageSize(t *testing.T) {
	t.Run("message under limit", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelInfo
		cfg.Security = &SecurityConfig{
			MaxMessageSize: 1000,
		}
		logger, _ := New(cfg)

		// Message under limit should be logged
		smallMsg := strings.Repeat("a", 500)
		logger.Info(smallMsg)

		output := buf.String()
		if !strings.Contains(output, smallMsg) {
			t.Error("Small message should be logged")
		}
	})

	t.Run("message over limit", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelInfo
		cfg.Security = &SecurityConfig{
			MaxMessageSize: 100,
		}
		logger, _ := New(cfg)

		// Message over limit should be truncated or rejected
		largeMsg := strings.Repeat("a", 500)
		logger.Info(largeMsg)

		output := buf.String()
		// The message should either be truncated or not appear in full
		if len(output) > 0 && len(output) < len(largeMsg)+100 {
			// Truncated - this is expected
		} else if output == "" {
			// Rejected - this is also acceptable
		} else {
			// Check if message was truncated
			if strings.Contains(output, largeMsg) {
				t.Error("Large message should be truncated or rejected")
			}
		}
	})
}

// ============================================================================
// CONCURRENCY EDGE CASE TESTS
// ============================================================================

func TestConcurrentCloseWhileLogging(t *testing.T) {
	const goroutines = 20
	const messagesPerGoroutine = 50

	var wg sync.WaitGroup
	var closeOnce sync.Once

	logger, _ := New()

	// Start goroutines that log
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Info("message from goroutine", "id", id, "msg", j)

				// Close the logger partway through
				if id == 0 && j == 25 {
					closeOnce.Do(func() {
						logger.Close()
					})
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentAddRemoveWriter(t *testing.T) {
	logger, _ := New()
	const goroutines = 50
	const iterations = 20
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			buf := &threadSafeBuffer{Buffer: &bytes.Buffer{}}
			for j := 0; j < iterations; j++ {
				logger.AddWriter(buf)
				logger.Info("test message")
				logger.RemoveWriter(buf)
			}
		}(i)
	}
	wg.Wait()
}

// ============================================================================
// WRITER ERROR TESTS
// ============================================================================

// errorWriter is a writer that always returns an error
type errorWriter struct {
	err error
}

func (w *errorWriter) Write(p []byte) (n int, err error) {
	return 0, w.err
}

func TestWriterWriteErrors(t *testing.T) {
	t.Run("error writer", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Output = &errorWriter{err: errors.New("write error")}
		cfg.Level = LevelInfo
		logger, _ := New(cfg)

		// Should not panic when writer returns error
		logger.Info("test message")
		// If we get here without panic, the test passes
	})

	t.Run("partial write", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Output = &partialWriter{}
		cfg.Level = LevelInfo
		logger, _ := New(cfg)

		// Should handle partial writes gracefully
		logger.Info("test message that is longer than the buffer")
		// If we get here without panic, the test passes
	})
}

// partialWriter writes only half the data
type partialWriter struct{}

func (w *partialWriter) Write(p []byte) (n int, err error) {
	return len(p) / 2, nil
}

// ============================================================================
// SAMPLING TESTS
// ============================================================================

func TestSamplingIntegration(t *testing.T) {
	t.Run("sampling disabled allows all", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelInfo
		cfg.Sampling = nil // No sampling
		logger, _ := New(cfg)

		// Log many messages
		for i := 0; i < 100; i++ {
			logger.Info("test message")
		}

		// All should be logged
		lines := strings.Count(buf.String(), "test message")
		if lines != 100 {
			t.Errorf("Expected 100 messages, got %d", lines)
		}
	})

	t.Run("sampling enabled", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelInfo
		cfg.Sampling = &SamplingConfig{
			Enabled:    true,
			Initial:    10,
			Thereafter: 100, // Keep 1 in 100 after initial
			Tick:       time.Minute,
		}
		logger, _ := New(cfg)

		// Log many messages
		for i := 0; i < 200; i++ {
			logger.Info("test message")
		}

		// Should have sampled some messages
		lines := strings.Count(buf.String(), "test message")
		if lines == 0 {
			t.Error("Should have logged at least some messages")
		}
		if lines > 200 {
			t.Errorf("Too many messages logged: %d", lines)
		}
	})
}

// ============================================================================
// NIL AND EMPTY VALUE TESTS
// ============================================================================

func TestNilFieldValues(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	logger.InfoWith("nil test",
		Any("nil_value", nil),
		String("string_value", "test"),
	)

	output := buf.String()
	if !strings.Contains(output, "nil test") {
		t.Error("Should contain message")
	}
	if !strings.Contains(output, "string_value=test") {
		t.Error("Should contain string field")
	}
}

func TestEmptyFieldKey(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	// Empty key should not cause panic
	logger.InfoWith("empty key test",
		String("", "value_with_empty_key"),
		String("valid_key", "valid_value"),
	)

	output := buf.String()
	if !strings.Contains(output, "empty key test") {
		t.Error("Should contain message")
	}
}

// ============================================================================
// TIME FORMAT EDGE CASES
// ============================================================================

func TestCustomTimeFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	cfg.TimeFormat = "2006-01-02"
	cfg.IncludeTime = true
	logger, _ := New(cfg)

	logger.Info("test message")

	output := buf.String()
	// Should contain a date-like format (YYYY-MM-DD)
	if !strings.Contains(output, "-") {
		t.Error("Should contain custom time format")
	}
}

func TestDisabledTimeFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	cfg.IncludeTime = false
	logger, _ := New(cfg)

	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("Should contain message")
	}
}

// ============================================================================
// LOGGER CLOSE EDGE CASES
// ============================================================================

func TestDoubleClose(t *testing.T) {
	logger, _ := New()

	// First close
	logger.Close()

	// Second close should not panic
	logger.Close()
	// If we get here, test passes
}

func TestLogAfterClose(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	// Log before close
	logger.Info("before close")
	if buf.Len() == 0 {
		t.Error("Should log before close")
	}

	// Close
	logger.Close()

	// Clear buffer
	buf.Reset()

	// Log after close should not panic, but may not log
	logger.Info("after close")
	// Either it logs or it doesn't - the important thing is no panic
}

// ============================================================================
// HOOK ERROR TESTS
// ============================================================================

func TestHookError(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &buf
	cfg.Level = LevelInfo
	cfg.Hooks = NewHookRegistry()
	cfg.Hooks.Add(HookBeforeLog, func(ctx context.Context, h *HookContext) error {
		return errors.New("hook error")
	})
	logger, _ := New(cfg)

	// Should handle hook error gracefully
	logger.Info("test message")
	// The message may or may not be logged, but no panic
}

// slowWriter is a writer that introduces delay
type slowWriter struct {
	buf bytes.Buffer
}

func (w *slowWriter) Write(p []byte) (n int, err error) {
	time.Sleep(10 * time.Millisecond)
	return w.buf.Write(p)
}

func TestSlowWriter(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.Output = &slowWriter{buf: buf}
	cfg.Level = LevelInfo
	logger, _ := New(cfg)

	// Log multiple messages with slow writer
	for i := 0; i < 10; i++ {
		logger.Info("test message")
	}

	logger.Close()
	// Should complete without timeout
}
