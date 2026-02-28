package dd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestNewConfig tests the new unified configuration API
func TestNewConfig(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		cfg := NewConfig()
		if cfg.Level != LevelInfo {
			t.Errorf("Expected LevelInfo, got %v", cfg.Level)
		}
		if cfg.Format != FormatText {
			t.Errorf("Expected FormatText, got %v", cfg.Format)
		}
	})

	t.Run("modify fields directly", func(t *testing.T) {
		cfg := NewConfig()
		cfg.Level = LevelDebug
		cfg.Format = FormatJSON
		cfg.DynamicCaller = true
		cfg.FullPath = true

		if cfg.Level != LevelDebug {
			t.Errorf("Expected LevelDebug, got %v", cfg.Level)
		}
		if cfg.Format != FormatJSON {
			t.Errorf("Expected FormatJSON, got %v", cfg.Format)
		}
		if !cfg.DynamicCaller {
			t.Error("Expected DynamicCaller to be true")
		}
		if !cfg.FullPath {
			t.Error("Expected FullPath to be true")
		}
	})

	t.Run("build logger", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Format = FormatJSON
		cfg.Level = LevelDebug

		logger, err := New(cfg)
		if err != nil {
			t.Fatalf("Failed to build logger: %v", err)
		}
		defer logger.Close()

		logger.Info("test message")

		output := buf.String()
		if !strings.Contains(output, `"message":"test message"`) {
			t.Errorf("Expected JSON output with message, got: %s", output)
		}
	})

	t.Run("MustBuild panics on error", func(t *testing.T) {
		cfg := NewConfig()
		// Use a path with invalid characters on Windows
		cfg.File = &FileConfig{Path: "Z:\\\\nonexistent\\\\drive\\\\app.log"}

		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid file path")
			}
		}()

		Must(cfg)
	})
}

func TestConfigDevelopment(t *testing.T) {
	cfg := ConfigDevelopment()

	if cfg.Level != LevelDebug {
		t.Errorf("Expected LevelDebug, got %v", cfg.Level)
	}
	if cfg.Format != FormatText {
		t.Errorf("Expected FormatText, got %v", cfg.Format)
	}
	if !cfg.DynamicCaller {
		t.Error("Expected DynamicCaller to be true")
	}
}

func TestConfigJSON(t *testing.T) {
	cfg := ConfigJSON()

	if cfg.Format != FormatJSON {
		t.Errorf("Expected FormatJSON, got %v", cfg.Format)
	}
	if cfg.JSON == nil {
		t.Error("Expected JSON options to be initialized")
	}
}

func TestConfigFileOutput(t *testing.T) {
	t.Run("File config sets file path", func(t *testing.T) {
		cfg := NewConfig()
		cfg.File = &FileConfig{Path: "logs/test.log"}

		if cfg.File.Path != "logs/test.log" {
			t.Errorf("Expected File.Path 'logs/test.log', got '%s'", cfg.File.Path)
		}
	})

	t.Run("modify rotation settings", func(t *testing.T) {
		cfg := NewConfig()
		cfg.File = &FileConfig{
			Path:       "logs/test.log",
			MaxSizeMB:  50,
			MaxBackups: 5,
			MaxAge:     7 * 24 * time.Hour,
			Compress:   true,
		}

		if cfg.File.MaxSizeMB != 50 {
			t.Errorf("Expected MaxSizeMB=50, got %d", cfg.File.MaxSizeMB)
		}
		if cfg.File.MaxBackups != 5 {
			t.Errorf("Expected MaxBackups=5, got %d", cfg.File.MaxBackups)
		}
		if cfg.File.MaxAge != 7*24*time.Hour {
			t.Errorf("Expected MaxAge=7d, got %v", cfg.File.MaxAge)
		}
		if !cfg.File.Compress {
			t.Error("Expected Compress to be true")
		}
	})
}

func TestConfigLevelFields(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected LogLevel
	}{
		{LevelDebug, LevelDebug},
		{LevelInfo, LevelInfo},
		{LevelWarn, LevelWarn},
		{LevelError, LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.expected.String(), func(t *testing.T) {
			cfg := NewConfig()
			cfg.Level = tt.level

			if cfg.Level != tt.expected {
				t.Errorf("Expected level %v, got %v", tt.expected, cfg.Level)
			}
		})
	}
}

func TestConfigFormatFields(t *testing.T) {
	t.Run("FormatJSON", func(t *testing.T) {
		cfg := NewConfig()
		cfg.Format = FormatJSON
		if cfg.Format != FormatJSON {
			t.Errorf("Expected FormatJSON, got %v", cfg.Format)
		}
	})

	t.Run("FormatText", func(t *testing.T) {
		cfg := NewConfig()
		cfg.Format = FormatText
		if cfg.Format != FormatText {
			t.Errorf("Expected FormatText, got %v", cfg.Format)
		}
	})
}

func TestBuilderConfigClone(t *testing.T) {
	t.Run("clone preserves settings", func(t *testing.T) {
		original := NewConfig()
		original.Format = FormatJSON
		original.Level = LevelDebug
		original.DynamicCaller = true
		original.File = &FileConfig{MaxSizeMB: 50}

		cloned := original.Clone()

		if cloned.Format != original.Format {
			t.Errorf("Clone Format mismatch")
		}
		if cloned.Level != original.Level {
			t.Errorf("Clone Level mismatch")
		}
		if cloned.DynamicCaller != original.DynamicCaller {
			t.Errorf("Clone DynamicCaller mismatch")
		}
		if cloned.File.MaxSizeMB != original.File.MaxSizeMB {
			t.Errorf("Clone File.MaxSizeMB mismatch")
		}
	})

	t.Run("clone is independent", func(t *testing.T) {
		original := NewConfig()
		original.Level = LevelDebug
		cloned := original.Clone()

		// Modify clone
		cloned.Level = LevelInfo

		// Original should not be affected
		if original.Level != LevelDebug {
			t.Errorf("Original should still be LevelDebug, got %v", original.Level)
		}
		if cloned.Level != LevelInfo {
			t.Errorf("Clone should be LevelInfo, got %v", cloned.Level)
		}
	})

	t.Run("clone for multiple loggers", func(t *testing.T) {
		base := NewConfig()
		base.Format = FormatJSON

		// Create app logger
		appCfg := base.Clone()
		appCfg.File = &FileConfig{Path: "logs/app.log", MaxSizeMB: 100}

		// Create error logger
		errCfg := base.Clone()
		errCfg.File = &FileConfig{Path: "logs/error.log", MaxSizeMB: 50}
		errCfg.Level = LevelError

		// Verify they're independent
		if appCfg.File.MaxSizeMB != 100 {
			t.Errorf("App config MaxSizeMB should be 100")
		}
		if errCfg.File.MaxSizeMB != 50 {
			t.Errorf("Error config MaxSizeMB should be 50")
		}
		if errCfg.Level != LevelError {
			t.Errorf("Error config Level should be LevelError")
		}
	})
}

func TestConfigWithSampling(t *testing.T) {
	cfg := NewConfig()
	cfg.Sampling = &SamplingConfig{
		Enabled:    true,
		Initial:    100,
		Thereafter: 10,
		Tick:       time.Minute,
	}

	if cfg.Sampling == nil {
		t.Fatal("Expected Sampling to be set")
	}
	if !cfg.Sampling.Enabled {
		t.Error("Expected Sampling.Enabled to be true")
	}
	if cfg.Sampling.Initial != 100 {
		t.Errorf("Expected Initial=100, got %d", cfg.Sampling.Initial)
	}
	if cfg.Sampling.Thereafter != 10 {
		t.Errorf("Expected Thereafter=10, got %d", cfg.Sampling.Thereafter)
	}
	if cfg.Sampling.Tick != time.Minute {
		t.Errorf("Expected Tick=1m, got %v", cfg.Sampling.Tick)
	}
}

func TestConfigAddHook(t *testing.T) {
	cfg := NewConfig()
	cfg.Hooks = NewHookRegistry()
	cfg.Hooks.Add(HookBeforeLog, func(ctx context.Context, h *HookContext) error {
		return nil
	})

	if cfg.Hooks == nil {
		t.Fatal("Expected Hooks to be initialized")
	}
	if cfg.Hooks.Count() != 1 {
		t.Errorf("Expected 1 hook, got %d", cfg.Hooks.Count())
	}
}

func TestConfigIntegration(t *testing.T) {
	t.Run("complete configuration example", func(t *testing.T) {
		var buf bytes.Buffer

		cfg := NewConfig()
		cfg.Output = &buf
		cfg.Format = FormatJSON
		cfg.Level = LevelDebug
		cfg.DynamicCaller = true
		cfg.FullPath = false

		logger, err := New(cfg)
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		defer logger.Close()

		logger.InfoWith("test message", String("key", "value"))

		output := buf.String()
		if !strings.Contains(output, `"message":"test message"`) {
			t.Errorf("Expected JSON message, got: %s", output)
		}
		if !strings.Contains(output, `"key":"value"`) {
			t.Errorf("Expected field 'key:value', got: %s", output)
		}
	})

	t.Run("file output configuration", func(t *testing.T) {
		// Create temp directory
		tmpDir := t.TempDir()

		cfg := NewConfig()
		cfg.File = &FileConfig{
			Path:       tmpDir + "/test.log",
			MaxSizeMB:  10,
			MaxBackups: 5,
			Compress:   true,
		}
		cfg.Format = FormatJSON
		cfg.Level = LevelInfo

		logger, err := New(cfg)
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		defer logger.Close()

		logger.Info("test message")

		// Verify file was created
		if _, err := os.Stat(tmpDir + "/test.log"); os.IsNotExist(err) {
			t.Error("Expected log file to be created")
		}
	})
}
