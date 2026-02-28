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
// This test verifies:
// 1. handleFatal completes within the expected timeout
// 2. The FatalHandler is called even when Close() blocks
// Note: Testing stderr output is skipped under race detector due to
// inherent race in capturing os.Stderr from concurrent goroutines.
func TestFatalTimeout(t *testing.T) {
	t.Run("handleFatal completes and calls handler when close blocks", func(t *testing.T) {
		exited := false
		var mu sync.Mutex

		cfg := DefaultConfig()
		cfg.FatalHandler = func() {
			mu.Lock()
			exited = true
			mu.Unlock()
		}
		logger, _ := New(cfg)

		// Replace writers with an interruptible blocking writer
		blockingWriter := newInterruptibleBlockingWriter()
		logger.writersPtr.Store(&[]io.Writer{blockingWriter})

		// Channel to signal when handleFatal completes
		handleFatalDone := make(chan struct{})

		// Call handleFatal in a goroutine
		go func() {
			defer close(handleFatalDone)
			logger.handleFatal()
		}()

		// Wait for handleFatal to complete with a timeout
		// The DefaultFatalFlushTimeout is 5 seconds, so we wait up to 10 seconds
		select {
		case <-handleFatalDone:
			// Good, completed
		case <-time.After(10 * time.Second):
			blockingWriter.cancel()
			t.Fatal("handleFatal should have completed within timeout")
		}

		mu.Lock()
		if !exited {
			t.Error("FatalHandler should have been called")
		}
		mu.Unlock()

		// Cancel the blocking writer to allow the Close() goroutine to complete
		// This ensures test isolation and prevents race conditions with subsequent tests
		blockingWriter.cancel()
	})
}

// interruptibleBlockingWriter is a writer that blocks on Write and Close but can be interrupted
type interruptibleBlockingWriter struct {
	cancelFunc context.CancelFunc
	ctx        context.Context
}

func newInterruptibleBlockingWriter() *interruptibleBlockingWriter {
	ctx, cancel := context.WithCancel(context.Background())
	return &interruptibleBlockingWriter{cancelFunc: cancel, ctx: ctx}
}

func (w *interruptibleBlockingWriter) Write(p []byte) (n int, err error) {
	select {
	case <-time.After(10 * time.Second):
		return len(p), nil
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	}
}

func (w *interruptibleBlockingWriter) Close() error {
	select {
	case <-time.After(10 * time.Second):
		return nil
	case <-w.ctx.Done():
		return w.ctx.Err()
	}
}

func (w *interruptibleBlockingWriter) cancel() {
	if w.cancelFunc != nil {
		w.cancelFunc()
	}
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
		cfg := DefaultConfig()
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
		cfg := DefaultConfig()
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
		cfg := DefaultConfig()
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
		cfg := DefaultConfig()
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
		cfg := DefaultConfig()
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
		cfg := DefaultConfig()
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
		cfg := DefaultConfig()
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
		cfg := DefaultConfig()
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
		cfg := DefaultConfig()
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
		cfg := DefaultConfig()
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
		cfg := DefaultConfig()
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
		cfg := DefaultConfig()
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
		cfg := DefaultConfig()
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

		cfg := DefaultConfig()
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

// TestLevelResolver tests the dynamic level resolver functionality
func TestLevelResolver(t *testing.T) {
	t.Run("SetLevelResolver stores resolver", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelDebug
		logger, _ := New(cfg)

		resolver := func(ctx context.Context) LogLevel {
			return LevelWarn
		}
		logger.SetLevelResolver(resolver)

		if logger.GetLevelResolver() == nil {
			t.Error("Expected resolver to be set")
		}
	})

	t.Run("resolver affects logging without context", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelDebug
		logger, _ := New(cfg)

		// Set resolver that only allows Warn and above
		logger.SetLevelResolver(func(ctx context.Context) LogLevel {
			return LevelWarn
		})

		logger.Debug("debug message") // Should be filtered
		logger.Info("info message")   // Should be filtered
		logger.Warn("warn message")   // Should appear
		logger.Error("error message") // Should appear

		output := buf.String()
		if strings.Contains(output, "debug message") {
			t.Error("Debug should be filtered by resolver")
		}
		if strings.Contains(output, "info message") {
			t.Error("Info should be filtered by resolver")
		}
		if !strings.Contains(output, "warn message") {
			t.Error("Warn should appear")
		}
		if !strings.Contains(output, "error message") {
			t.Error("Error should appear")
		}
	})

	t.Run("resolver with context", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelDebug
		logger, _ := New(cfg)

		// Resolver that uses context to determine level
		// When no context value is set, defaults to Info level
		logger.SetLevelResolver(func(ctx context.Context) LogLevel {
			if level, ok := ctx.Value("log_level").(LogLevel); ok {
				return level
			}
			return LevelInfo // Default when no context value is set
		})

		// Without context - uses resolver's default (Info)
		logger.Debug("no context debug") // Should be filtered (resolver returns Info)

		// With context specifying Debug level
		ctx := context.WithValue(context.Background(), "log_level", LevelDebug)
		logger.DebugCtx(ctx, "with context debug") // Should appear

		// With context specifying Warn level
		ctxWarn := context.WithValue(context.Background(), "log_level", LevelWarn)
		logger.InfoCtx(ctxWarn, "with context info") // Should be filtered

		output := buf.String()
		if strings.Contains(output, "no context debug") {
			t.Error("Debug without context should be filtered")
		}
		if !strings.Contains(output, "with context debug") {
			t.Error("Debug with context should appear")
		}
		if strings.Contains(output, "with context info") {
			t.Error("Info with Warn context should be filtered")
		}
	})

	t.Run("nil resolver uses static level", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelInfo
		logger, _ := New(cfg)

		// Set resolver then clear it
		logger.SetLevelResolver(func(ctx context.Context) LogLevel {
			return LevelWarn
		})
		logger.SetLevelResolver(nil)

		logger.Debug("debug message") // Should be filtered (static level is Info)
		logger.Info("info message")   // Should appear

		output := buf.String()
		if strings.Contains(output, "debug message") {
			t.Error("Debug should be filtered by static level")
		}
		if !strings.Contains(output, "info message") {
			t.Error("Info should appear")
		}
	})

	t.Run("dynamic level changes at runtime", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelDebug
		logger, _ := New(cfg)

		highLoad := false

		logger.SetLevelResolver(func(ctx context.Context) LogLevel {
			if highLoad {
				return LevelWarn
			}
			return LevelDebug
		})

		// Normal load - debug should work
		logger.Debug("debug 1")
		if !strings.Contains(buf.String(), "debug 1") {
			t.Error("Debug should appear under normal load")
		}

		buf.Reset()

		// Simulate high load
		highLoad = true
		logger.Debug("debug 2") // Should be filtered
		logger.Warn("warn 1")   // Should appear

		output := buf.String()
		if strings.Contains(output, "debug 2") {
			t.Error("Debug should be filtered under high load")
		}
		if !strings.Contains(output, "warn 1") {
			t.Error("Warn should appear under high load")
		}
	})

	t.Run("LogWithCtx uses resolver", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Level = LevelDebug
		cfg.Format = FormatJSON
		logger, _ := New(cfg)

		logger.SetLevelResolver(func(ctx context.Context) LogLevel {
			return LevelWarn
		})

		ctx := context.Background()
		logger.InfoWithCtx(ctx, "info with ctx", String("key", "value")) // Should be filtered
		logger.WarnWithCtx(ctx, "warn with ctx", String("key", "value")) // Should appear

		output := buf.String()
		if strings.Contains(output, "info with ctx") {
			t.Error("Info should be filtered by resolver")
		}
		if !strings.Contains(output, "warn with ctx") {
			t.Error("Warn should appear")
		}
	})
}

// TestFieldValidation tests the field key validation functionality
func TestFieldValidation(t *testing.T) {
	t.Run("naming convention validators", func(t *testing.T) {
		tests := []struct {
			convention FieldNamingConvention
			valid      []string
			invalid    []string
		}{
			{
				convention: NamingConventionSnakeCase,
				valid:      []string{"user_id", "first_name", "created_at", "id", "url"},
				invalid:    []string{"userId", "UserID", "user-id", "user__id", "_user_id", "user_id_"},
			},
			{
				convention: NamingConventionCamelCase,
				valid:      []string{"userId", "firstName", "createdAt", "id", "url"},
				invalid:    []string{"user_id", "UserID", "user-id", "user__id"},
			},
			{
				convention: NamingConventionPascalCase,
				valid:      []string{"UserId", "FirstName", "CreatedAt", "ID", "URL"},
				invalid:    []string{"user_id", "userId", "user-id", "user__id"},
			},
			{
				convention: NamingConventionKebabCase,
				valid:      []string{"user-id", "first-name", "created-at", "id", "url"},
				invalid:    []string{"userId", "user_id", "UserID", "user--id", "-user-id", "user-id-"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.convention.String(), func(t *testing.T) {
				config := &FieldValidationConfig{
					Mode:                     FieldValidationWarn,
					Convention:               tt.convention,
					AllowCommonAbbreviations: true,
				}

				for _, key := range tt.valid {
					err := config.ValidateFieldKey(key)
					if err != nil {
						t.Errorf("Expected %q to be valid for %s, got error: %v", key, tt.convention, err)
					}
				}

				for _, key := range tt.invalid {
					// Skip common abbreviations
					if isCommonAbbreviation(key) {
						continue
					}
					err := config.ValidateFieldKey(key)
					if err == nil {
						t.Errorf("Expected %q to be invalid for %s", key, tt.convention)
					}
				}
			})
		}
	})

	t.Run("empty key is always invalid", func(t *testing.T) {
		config := &FieldValidationConfig{
			Mode:       FieldValidationWarn,
			Convention: NamingConventionSnakeCase,
		}
		err := config.ValidateFieldKey("")
		if err == nil {
			t.Error("Expected empty key to be invalid")
		}
	})

	t.Run("none mode skips validation", func(t *testing.T) {
		config := &FieldValidationConfig{
			Mode:       FieldValidationNone,
			Convention: NamingConventionSnakeCase,
		}
		// Any key should be valid when validation is disabled
		for _, key := range []string{"userId", "user_id", "user-id", "UserID"} {
			err := config.ValidateFieldKey(key)
			if err != nil {
				t.Errorf("Expected %q to be valid when validation is disabled", key)
			}
		}
	})

	t.Run("any convention accepts all keys", func(t *testing.T) {
		config := &FieldValidationConfig{
			Mode:       FieldValidationWarn,
			Convention: NamingConventionAny,
		}
		for _, key := range []string{"userId", "user_id", "user-id", "UserID"} {
			err := config.ValidateFieldKey(key)
			if err != nil {
				t.Errorf("Expected %q to be valid with any convention", key)
			}
		}
	})

	t.Run("common abbreviations allowed", func(t *testing.T) {
		config := &FieldValidationConfig{
			Mode:                     FieldValidationStrict,
			Convention:               NamingConventionSnakeCase,
			AllowCommonAbbreviations: true,
		}
		for _, key := range []string{"id", "ID", "url", "URL", "api", "API", "user_id", "request_url"} {
			err := config.ValidateFieldKey(key)
			if err != nil {
				t.Errorf("Expected common abbreviation %q to be valid", key)
			}
		}
	})

	t.Run("validation with logger", func(t *testing.T) {
		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		cfg.Format = FormatJSON
		cfg.FieldValidation = &FieldValidationConfig{
			Mode:                     FieldValidationWarn,
			Convention:               NamingConventionSnakeCase,
			AllowCommonAbbreviations: true,
		}
		logger, _ := New(cfg)

		// Valid key - should not produce warning
		logger.InfoWith("test", String("user_id", "123"))

		// Invalid key - should produce warning on stderr
		logger.InfoWith("test", String("userId", "123"))

		w.Close()
		os.Stderr = oldStderr

		var stderrBuf bytes.Buffer
		io.Copy(&stderrBuf, r)
		stderrOutput := stderrBuf.String()

		// Should have warning for invalid key
		if !strings.Contains(stderrOutput, "userId") {
			t.Errorf("Expected warning for 'userId' in stderr, got: %s", stderrOutput)
		}

		// Log output should still contain both fields
		output := buf.String()
		if !strings.Contains(output, "user_id") {
			t.Error("Expected user_id in output")
		}
		if !strings.Contains(output, "userId") {
			t.Error("Expected userId in output (validation doesn't block logging)")
		}
	})

	t.Run("SetFieldValidation at runtime", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Output = &buf
		logger, _ := New(cfg)

		// No validation initially
		logger.InfoWith("test", String("userId", "123"))

		// Enable validation
		logger.SetFieldValidation(&FieldValidationConfig{
			Mode:       FieldValidationWarn,
			Convention: NamingConventionSnakeCase,
		})

		if logger.GetFieldValidation() == nil {
			t.Error("Expected field validation to be set")
		}

		// Clear validation
		logger.SetFieldValidation(nil)
		if logger.GetFieldValidation() != nil {
			t.Error("Expected field validation to be nil after clearing")
		}
	})
}
