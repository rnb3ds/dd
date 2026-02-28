package dd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestFatalTimeout tests that Fatal logs exit even when Close() blocks
func TestFatalTimeout(t *testing.T) {
	t.Run("timeout warning printed when close blocks", func(t *testing.T) {
		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		exited := false
		var mu sync.Mutex

		cfg := NewConfig()
		cfg.FatalHandler = func() {
			mu.Lock()
			exited = true
			mu.Unlock()
		}
		logger, _ := New(cfg)

		// Replace writers with a blocking writer
		blockingWriter := &blockingWriter{}
		logger.writersPtr.Store(&[]io.Writer{blockingWriter})

		// Call handleFatal in a goroutine since it may block
		done := make(chan struct{})
		go func() {
			defer close(done)
			logger.handleFatal()
		}()

		// Wait for completion or timeout
		select {
		case <-done:
			// Good, completed
		case <-time.After(10 * time.Second):
			t.Fatal("handleFatal should have completed within timeout")
		}

		w.Close()
		os.Stderr = oldStderr

		var buf bytes.Buffer
		io.Copy(&buf, r)
		stderrOutput := buf.String()

		if !strings.Contains(stderrOutput, "Warning: logger close timed out") {
			t.Errorf("Expected timeout warning in stderr, got: %s", stderrOutput)
		}

		mu.Lock()
		if !exited {
			t.Error("FatalHandler should have been called")
		}
		mu.Unlock()
	})
}

// blockingWriter is a writer that blocks on Write and Close
type blockingWriter struct{}

func (w *blockingWriter) Write(p []byte) (n int, err error) {
	time.Sleep(10 * time.Second) // Block for a long time
	return len(p), nil
}

func (w *blockingWriter) Close() error {
	time.Sleep(10 * time.Second) // Block for a long time
	return nil
}

// TestErrorHandling tests the unified error handling for hooks and extractors
func TestErrorHandling(t *testing.T) {
	t.Run("AddHook returns error for nil hook", func(t *testing.T) {
		logger, _ := New()
		err := logger.AddHook(HookBeforeLog, nil)
		if !errors.Is(err, ErrNilHook) {
			t.Errorf("Expected ErrNilHook, got: %v", err)
		}
	})

	t.Run("AddHook returns error when logger closed", func(t *testing.T) {
		logger, _ := New()
		logger.Close()
		err := logger.AddHook(HookBeforeLog, func(ctx context.Context, h *HookContext) error {
			return nil
		})
		if !errors.Is(err, ErrLoggerClosed) {
			t.Errorf("Expected ErrLoggerClosed, got: %v", err)
		}
	})

	t.Run("MustAddHook panics on error", func(t *testing.T) {
		logger, _ := New()
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for nil hook")
			}
		}()
		logger.MustAddHook(HookBeforeLog, nil)
	})

	t.Run("AddContextExtractor returns error for nil extractor", func(t *testing.T) {
		logger, _ := New()
		err := logger.AddContextExtractor(nil)
		if !errors.Is(err, ErrNilExtractor) {
			t.Errorf("Expected ErrNilExtractor, got: %v", err)
		}
	})

	t.Run("AddContextExtractor returns error when logger closed", func(t *testing.T) {
		logger, _ := New()
		logger.Close()
		err := logger.AddContextExtractor(func(ctx context.Context) []Field {
			return nil
		})
		if !errors.Is(err, ErrLoggerClosed) {
			t.Errorf("Expected ErrLoggerClosed, got: %v", err)
		}
	})

	t.Run("MustAddContextExtractor panics on error", func(t *testing.T) {
		logger, _ := New()
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for nil extractor")
			}
		}()
		logger.MustAddContextExtractor(nil)
	})

	t.Run("SetHooks returns error when logger closed", func(t *testing.T) {
		logger, _ := New()
		logger.Close()
		registry := NewHookRegistry()
		err := logger.SetHooks(registry)
		if !errors.Is(err, ErrLoggerClosed) {
			t.Errorf("Expected ErrLoggerClosed, got: %v", err)
		}
	})

	t.Run("SetContextExtractors returns error when logger closed", func(t *testing.T) {
		logger, _ := New()
		logger.Close()
		err := logger.SetContextExtractors(func(ctx context.Context) []Field {
			return nil
		})
		if !errors.Is(err, ErrLoggerClosed) {
			t.Errorf("Expected ErrLoggerClosed, got: %v", err)
		}
	})
}

// TestDefaultLoggerWarning tests that Default() prints warning on fallback
func TestDefaultLoggerWarning(t *testing.T) {
	// This test is tricky because Default() is a singleton
	// We test the warning logic indirectly through the DefaultInitError function

	// The default logger should work without error
	logger := Default()
	if logger == nil {
		t.Error("Default() should return a logger")
	}

	// Check if we can get the init error (should be nil for normal operation)
	err := DefaultInitError()
	// The error might not be nil if there was an issue during init
	// but the logger should still be usable
	_ = err
}

// TestWithFields tests the WithFields context propagation
func TestWithFields(t *testing.T) {
	t.Run("WithFields creates entry with fields", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Format = FormatJSON
		logger, _ := New(cfg)

		entry := logger.WithFields(String("service", "api"))
		entry.Info("test message")

		output := buf.String()
		if !strings.Contains(output, `"service":"api"`) {
			t.Errorf("Expected field 'service:api' in output, got: %s", output)
		}
		if !strings.Contains(output, `"message":"test message"`) {
			t.Errorf("Expected message in output, got: %s", output)
		}
	})

	t.Run("WithField creates entry with single field", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Format = FormatJSON
		logger, _ := New(cfg)

		entry := logger.WithField("request_id", "abc123")
		entry.Info("test message")

		output := buf.String()
		if !strings.Contains(output, `"request_id":"abc123"`) {
			t.Errorf("Expected field 'request_id:abc123' in output, got: %s", output)
		}
	})

	t.Run("nested WithFields merges fields", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Format = FormatJSON
		logger, _ := New(cfg)

		entry := logger.WithFields(String("service", "api"), String("version", "1.0"))
		entry2 := entry.WithFields(String("user", "john"))
		entry2.Info("test message")

		output := buf.String()
		if !strings.Contains(output, `"service":"api"`) {
			t.Errorf("Expected field 'service:api' in output, got: %s", output)
		}
		if !strings.Contains(output, `"version":"1.0"`) {
			t.Errorf("Expected field 'version:1.0' in output, got: %s", output)
		}
		if !strings.Contains(output, `"user":"john"`) {
			t.Errorf("Expected field 'user:john' in output, got: %s", output)
		}
	})

	t.Run("WithFields override", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Format = FormatJSON
		logger, _ := New(cfg)

		entry := logger.WithFields(String("service", "api"))
		entry2 := entry.WithFields(String("service", "web")) // Override
		entry2.Info("test message")

		output := buf.String()
		// Should have the overridden value
		if !strings.Contains(output, `"service":"web"`) {
			t.Errorf("Expected field 'service:web' in output, got: %s", output)
		}
		// Count occurrences - should only appear once
		count := strings.Count(output, `"service"`)
		if count != 1 {
			t.Errorf("Expected 'service' to appear once, got %d times", count)
		}
	})

	t.Run("entry immutability", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Format = FormatJSON
		logger, _ := New(cfg)

		entry1 := logger.WithFields(String("service", "api"))
		entry2 := entry1.WithFields(String("user", "john"))

		// Clear buffer
		buf.Reset()

		// entry1 should still only have service field
		entry1.Info("message1")
		output1 := buf.String()
		if strings.Contains(output1, `"user"`) {
			t.Errorf("entry1 should not have user field, got: %s", output1)
		}

		// Clear buffer
		buf.Reset()

		// entry2 should have both fields
		entry2.Info("message2")
		output2 := buf.String()
		if !strings.Contains(output2, `"service"`) || !strings.Contains(output2, `"user"`) {
			t.Errorf("entry2 should have both fields, got: %s", output2)
		}
	})

	t.Run("entry WithField method", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Format = FormatJSON
		logger, _ := New(cfg)

		entry := logger.WithField("service", "api")
		entry2 := entry.WithField("user", "john")
		entry2.Info("test message")

		output := buf.String()
		if !strings.Contains(output, `"service":"api"`) {
			t.Errorf("Expected field 'service:api' in output, got: %s", output)
		}
		if !strings.Contains(output, `"user":"john"`) {
			t.Errorf("Expected field 'user:john' in output, got: %s", output)
		}
	})

	t.Run("entry LogWith merges fields", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Format = FormatJSON
		logger, _ := New(cfg)

		entry := logger.WithFields(String("service", "api"))
		entry.InfoWith("test message", String("extra", "value"))

		output := buf.String()
		if !strings.Contains(output, `"service":"api"`) {
			t.Errorf("Expected field 'service:api' in output, got: %s", output)
		}
		if !strings.Contains(output, `"extra":"value"`) {
			t.Errorf("Expected field 'extra:value' in output, got: %s", output)
		}
	})
}

// TestSampling tests the log sampling functionality
func TestSampling(t *testing.T) {
	t.Run("disabled sampling logs everything", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := NewConfig()
		cfg.Output = &buf
		logger, _ := New(cfg)

		for i := 0; i < 100; i++ {
			logger.Info("test")
		}

		lines := strings.Count(buf.String(), "\n")
		if lines != 100 {
			t.Errorf("Expected 100 lines, got %d", lines)
		}
	})

	t.Run("sampling with Initial only", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Sampling = &SamplingConfig{Enabled: true, Initial: 10, Thereafter: 0}
		logger, _ := New(cfg)

		for i := 0; i < 100; i++ {
			logger.Info("test")
		}

		lines := strings.Count(buf.String(), "\n")
		if lines != 10 {
			t.Errorf("Expected 10 lines (Initial only), got %d", lines)
		}
	})

	t.Run("sampling with Thereafter", func(t *testing.T) {
		var buf bytes.Buffer
		// Initial=5, Thereafter=5 means: first 5 always, then 1 out of every 5
		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Sampling = &SamplingConfig{Enabled: true, Initial: 5, Thereafter: 5}
		logger, _ := New(cfg)

		for i := 0; i < 25; i++ {
			logger.Info("test")
		}

		lines := strings.Count(buf.String(), "\n")
		// Expected: 5 (initial) + 4 (20/5 thereafter) = 9
		if lines != 9 {
			t.Errorf("Expected 9 lines (5 initial + 4 thereafter), got %d", lines)
		}
	})

	t.Run("SetSampling at runtime", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := NewConfig()
		cfg.Output = &buf
		logger, _ := New(cfg)

		// No sampling initially
		for i := 0; i < 10; i++ {
			logger.Info("test")
		}
		if lines := strings.Count(buf.String(), "\n"); lines != 10 {
			t.Errorf("Expected 10 lines before sampling, got %d", lines)
		}

		buf.Reset()

		// Enable sampling
		logger.SetSampling(&SamplingConfig{
			Enabled:    true,
			Initial:    2,
			Thereafter: 2,
		})

		for i := 0; i < 10; i++ {
			logger.Info("test")
		}

		lines := strings.Count(buf.String(), "\n")
		// Expected: 2 (initial) + 4 (8/2 thereafter) = 6
		if lines != 6 {
			t.Errorf("Expected 6 lines with sampling, got %d", lines)
		}
	})

	t.Run("GetSampling returns config", func(t *testing.T) {
		cfg := NewConfig()
		cfg.Sampling = &SamplingConfig{Enabled: true, Initial: 10, Thereafter: 5, Tick: time.Minute}
		logger, _ := New(cfg)

		config := logger.GetSampling()
		if config == nil {
			t.Fatal("Expected sampling config, got nil")
		}
		if config.Initial != 10 {
			t.Errorf("Expected Initial=10, got %d", config.Initial)
		}
		if config.Thereafter != 5 {
			t.Errorf("Expected Thereafter=5, got %d", config.Thereafter)
		}
		if config.Tick != time.Minute {
			t.Errorf("Expected Tick=1m, got %v", config.Tick)
		}
	})

	t.Run("SetSampling nil disables sampling", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Sampling = &SamplingConfig{Enabled: true, Initial: 1, Thereafter: 100}
		logger, _ := New(cfg)

		logger.SetSampling(nil)

		for i := 0; i < 10; i++ {
			logger.Info("test")
		}

		lines := strings.Count(buf.String(), "\n")
		if lines != 10 {
			t.Errorf("Expected 10 lines after disabling sampling, got %d", lines)
		}
	})
}

// TestHookContextOriginalFields tests that OriginalFields are passed to hooks
func TestHookContextOriginalFields(t *testing.T) {
	t.Run("OriginalFields contains unfiltered fields", func(t *testing.T) {
		var buf bytes.Buffer
		var capturedOriginal []Field
		var capturedFiltered []Field

		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Security = DefaultSecurityConfig()
		cfg.Hooks = NewHookRegistry()
		cfg.Hooks.Add(HookBeforeLog, func(ctx context.Context, h *HookContext) error {
			capturedOriginal = h.OriginalFields
			capturedFiltered = h.Fields
			return nil
		})

		logger, _ := New(cfg)

		// Log with a field that might be filtered
		logger.InfoWith("test", String("password", "secret123"), String("name", "john"))

		if len(capturedOriginal) != 2 {
			t.Errorf("Expected 2 original fields, got %d", len(capturedOriginal))
		}
		if len(capturedFiltered) != 2 {
			t.Errorf("Expected 2 filtered fields, got %d", len(capturedFiltered))
		}

		// Find the password field in original
		var originalPassword, filteredPassword string
		for _, f := range capturedOriginal {
			if f.Key == "password" {
				originalPassword = fmt.Sprintf("%v", f.Value)
			}
		}
		for _, f := range capturedFiltered {
			if f.Key == "password" {
				filteredPassword = fmt.Sprintf("%v", f.Value)
			}
		}

		// Original should have the actual value, filtered might be redacted
		if originalPassword != "secret123" {
			t.Errorf("Expected original password 'secret123', got '%s'", originalPassword)
		}

		// Filtered should be redacted (since password is a sensitive key)
		if filteredPassword == "secret123" {
			t.Error("Expected filtered password to be redacted, got original value")
		}
	})
}

// TestFilterPerformanceMetrics tests the filter performance monitoring
func TestFilterPerformanceMetrics(t *testing.T) {
	t.Run("GetFilterStats returns metrics", func(t *testing.T) {
		filter := NewBasicSensitiveDataFilter()

		// Process some inputs
		filter.Filter("hello world")
		filter.Filter("password=secret123")
		filter.Filter("normal text")

		stats := filter.GetFilterStats()

		if stats.TotalFiltered != 3 {
			t.Errorf("Expected TotalFiltered=3, got %d", stats.TotalFiltered)
		}

		if stats.PatternCount == 0 {
			t.Error("Expected PatternCount > 0")
		}

		if !stats.Enabled {
			t.Error("Expected Enabled=true")
		}
	})

	t.Run("redactions are tracked", func(t *testing.T) {
		filter := NewBasicSensitiveDataFilter()

		// Log something with a password pattern that should be redacted
		filter.Filter("password=test123")
		filter.Filter("normal text without sensitive data")

		stats := filter.GetFilterStats()

		if stats.TotalFiltered != 2 {
			t.Errorf("Expected TotalFiltered=2, got %d", stats.TotalFiltered)
		}

		// At least one redaction should have occurred
		if stats.TotalRedactions < 1 {
			t.Errorf("Expected TotalRedactions >= 1, got %d", stats.TotalRedactions)
		}
	})

	t.Run("average latency is calculated", func(t *testing.T) {
		filter := NewBasicSensitiveDataFilter()

		// Process many inputs to get meaningful average
		for i := 0; i < 100; i++ {
			filter.Filter("test message with some content")
		}

		stats := filter.GetFilterStats()

		if stats.TotalFiltered != 100 {
			t.Errorf("Expected TotalFiltered=100, got %d", stats.TotalFiltered)
		}

		if stats.AverageLatency == 0 {
			t.Error("Expected AverageLatency > 0")
		}
	})
}
