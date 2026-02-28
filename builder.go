package dd

import (
	"fmt"
	"io"
	"time"

	"github.com/cybergodev/dd/internal"
)

// FileConfig configures file output with rotation settings.
type FileConfig struct {
	Path       string        // Log file path
	MaxSizeMB  int           // Max file size in MB before rotation (default: 100)
	MaxBackups int           // Max number of old log files to retain (default: 10)
	MaxAge     time.Duration // Max duration to retain old log files (default: 30 days)
	Compress   bool          // Enable gzip compression for rotated files (default: false)
}

// Config provides a struct-based configuration API for creating loggers.
// Direct field modification with IDE autocomplete support.
//
// Example:
//
//	cfg := dd.DefaultConfig()
//	cfg.Format = dd.FormatJSON
//	cfg.Level = dd.LevelDebug
//	logger, _ := dd.New(cfg)
type Config struct {
	// Log level
	Level LogLevel

	// Output format
	Format LogFormat

	// Time settings
	TimeFormat   string
	IncludeTime  bool
	IncludeLevel bool

	// Caller information
	DynamicCaller bool
	FullPath      bool

	// Output targets
	Output  io.Writer   // Single output writer
	Outputs []io.Writer // Multiple output writers
	File    *FileConfig // File output configuration

	// JSON configuration
	JSON *JSONOptions

	// Security configuration
	Security *SecurityConfig

	// Field validation configuration
	FieldValidation *FieldValidationConfig

	// Lifecycle handlers
	FatalHandler      FatalHandler
	WriteErrorHandler WriteErrorHandler

	// Extensibility
	ContextExtractors []ContextExtractor
	Hooks             *HookRegistry
	Sampling          *SamplingConfig
}

// NewConfig creates a new Config with default settings.
// Deprecated: Use DefaultConfig() instead. This alias will be removed in a future version.
func NewConfig() *Config {
	return defaultConfig()
}

// DefaultConfig creates a new Config with default settings.
//
// Example:
//
//	cfg := dd.DefaultConfig()
//	cfg.Level = dd.LevelDebug
//	cfg.Format = dd.FormatJSON
//	logger, _ := dd.New(cfg)
func DefaultConfig() *Config {
	return defaultConfig()
}

func defaultConfig() *Config {
	return &Config{
		Level:         LevelInfo,
		Format:        FormatText,
		TimeFormat:    DefaultTimeFormat,
		IncludeTime:   true,
		IncludeLevel:  true,
		FullPath:      false,
		DynamicCaller: false,
		Security:      DefaultSecurityConfigDisabled(),
		FatalHandler:  defaultFatalHandler,
	}
}

// DevelopmentConfig creates a Config with development-friendly settings.
// Enables DEBUG level and dynamic caller detection.
//
// Example:
//
//	cfg := dd.DevelopmentConfig()
//	cfg.File = &dd.FileConfig{Path: "dev.log"}
//	logger, _ := dd.New(cfg)
func DevelopmentConfig() *Config {
	return &Config{
		Level:         LevelDebug,
		Format:        FormatText,
		TimeFormat:    DevTimeFormat,
		IncludeTime:   true,
		IncludeLevel:  true,
		FullPath:      false,
		DynamicCaller: true,
		Security:      DefaultSecurityConfigDisabled(),
		FatalHandler:  defaultFatalHandler,
	}
}

// JSONConfig creates a Config with JSON output settings.
//
// Example:
//
//	cfg := dd.JSONConfig()
//	cfg.Level = dd.LevelInfo
//	logger, _ := dd.New(cfg)
func JSONConfig() *Config {
	return &Config{
		Level:         LevelDebug,
		Format:        FormatJSON,
		TimeFormat:    time.RFC3339,
		IncludeTime:   true,
		IncludeLevel:  true,
		FullPath:      false,
		DynamicCaller: true,
		Security:      DefaultSecurityConfigDisabled(),
		FatalHandler:  defaultFatalHandler,
		JSON: &internal.JSONOptions{
			PrettyPrint: false,
			Indent:      DefaultJSONIndent,
			FieldNames:  internal.DefaultJSONFieldNames(),
		},
	}
}

// build creates a new Logger from the configuration.
// This is an internal method used by dd.New().
func (c *Config) build() (*Logger, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	// Build internal config
	loggerConfig := &internalConfig{
		level:             c.Level,
		format:            c.Format,
		timeFormat:        c.TimeFormat,
		includeTime:       c.IncludeTime,
		includeLevel:      c.IncludeLevel,
		fullPath:          c.FullPath,
		dynamicCaller:     c.DynamicCaller,
		securityConfig:    c.Security,
		fieldValidation:   c.FieldValidation,
		fatalHandler:      c.FatalHandler,
		writeErrorHandler: c.WriteErrorHandler,
		contextExtractors: c.ContextExtractors,
		hooks:             c.Hooks,
		sampling:          c.Sampling,
	}

	// Handle JSON options
	if c.Format == FormatJSON && c.JSON != nil {
		loggerConfig.json = c.JSON
	} else if c.Format == FormatJSON {
		loggerConfig.json = &internal.JSONOptions{
			PrettyPrint: false,
			Indent:      DefaultJSONIndent,
			FieldNames:  internal.DefaultJSONFieldNames(),
		}
	}

	// Collect writers
	var writers []io.Writer

	// Add single output writer
	if c.Output != nil {
		writers = append(writers, c.Output)
	}

	// Add multiple output writers
	for _, w := range c.Outputs {
		if w != nil {
			writers = append(writers, w)
		}
	}

	// Handle file output
	if c.File != nil && c.File.Path != "" {
		fileWriter, err := c.createFileWriter()
		if err != nil {
			return nil, err
		}
		writers = append(writers, fileWriter)
	}

	// Default to stdout if no writers configured
	if len(writers) == 0 {
		writers = []io.Writer{defaultOutput}
	}

	loggerConfig.writers = writers

	return newFromInternalConfig(loggerConfig)
}

// createFileWriter creates a FileWriter from FileConfig.
func (c *Config) createFileWriter() (*FileWriter, error) {
	if c.File == nil || c.File.Path == "" {
		return nil, nil
	}

	config := FileWriterConfig{
		MaxSizeMB:  c.File.MaxSizeMB,
		MaxBackups: c.File.MaxBackups,
		MaxAge:     c.File.MaxAge,
		Compress:   c.File.Compress,
	}

	return NewFileWriter(c.File.Path, config)
}

// Clone creates a copy of the configuration.
//
// Example:
//
//	base := dd.DefaultConfig()
//	base.Format = dd.FormatJSON
//
//	appCfg := base.Clone()
//	appCfg.File = &dd.FileConfig{Path: "app.log"}
//	logger, _ := dd.New(appCfg)
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	clone := &Config{
		Level:             c.Level,
		Format:            c.Format,
		TimeFormat:        c.TimeFormat,
		IncludeTime:       c.IncludeTime,
		IncludeLevel:      c.IncludeLevel,
		FullPath:          c.FullPath,
		DynamicCaller:     c.DynamicCaller,
		Output:            c.Output,
		Security:          c.Security,
		FieldValidation:   c.FieldValidation,
		FatalHandler:      c.FatalHandler,
		WriteErrorHandler: c.WriteErrorHandler,
		Sampling:          c.Sampling,
	}

	// Copy Outputs slice
	if c.Outputs != nil {
		clone.Outputs = make([]io.Writer, len(c.Outputs))
		copy(clone.Outputs, c.Outputs)
	}

	// Copy File config
	if c.File != nil {
		clone.File = &FileConfig{
			Path:       c.File.Path,
			MaxSizeMB:  c.File.MaxSizeMB,
			MaxBackups: c.File.MaxBackups,
			MaxAge:     c.File.MaxAge,
			Compress:   c.File.Compress,
		}
	}

	// Copy JSON options
	if c.JSON != nil {
		clone.JSON = &internal.JSONOptions{
			PrettyPrint: c.JSON.PrettyPrint,
			Indent:      c.JSON.Indent,
		}
		if c.JSON.FieldNames != nil {
			clone.JSON.FieldNames = &internal.JSONFieldNames{
				Timestamp: c.JSON.FieldNames.Timestamp,
				Level:     c.JSON.FieldNames.Level,
				Caller:    c.JSON.FieldNames.Caller,
				Message:   c.JSON.FieldNames.Message,
				Fields:    c.JSON.FieldNames.Fields,
			}
		}
	}

	// Copy ContextExtractors
	if c.ContextExtractors != nil {
		clone.ContextExtractors = make([]ContextExtractor, len(c.ContextExtractors))
		copy(clone.ContextExtractors, c.ContextExtractors)
	}

	// Copy Hooks
	if c.Hooks != nil {
		clone.Hooks = c.Hooks.Clone()
	}

	// Copy Security config
	if c.Security != nil {
		clone.Security = c.Security.Clone()
	}

	// Copy Sampling config
	if c.Sampling != nil {
		clone.Sampling = &SamplingConfig{
			Enabled:    c.Sampling.Enabled,
			Initial:    c.Sampling.Initial,
			Thereafter: c.Sampling.Thereafter,
			Tick:       c.Sampling.Tick,
		}
	}

	return clone
}

// validate validates the configuration.
func (c *Config) validate() error {
	if c == nil {
		return ErrNilConfig
	}

	// Validate log level
	if c.Level < LevelDebug || c.Level > LevelFatal {
		return fmt.Errorf("%w: %d (valid range: %d-%d)", ErrInvalidLevel, c.Level, LevelDebug, LevelFatal)
	}

	// Validate format
	if c.Format != FormatText && c.Format != FormatJSON {
		return fmt.Errorf("%w: %d (valid: %d=Text, %d=JSON)", ErrInvalidFormat, c.Format, FormatText, FormatJSON)
	}

	// Validate time format
	if c.IncludeTime && c.TimeFormat != "" {
		if err := internal.ValidateTimeFormat(c.TimeFormat); err != nil {
			return err
		}
	}

	// Count total writers
	writerCount := 0
	if c.Output != nil {
		writerCount++
	}
	writerCount += len(c.Outputs)
	if c.File != nil && c.File.Path != "" {
		writerCount++
	}

	// Validate writer count
	if writerCount > MaxWriterCount {
		return fmt.Errorf("%w: %d writers configured, maximum is %d", ErrMaxWritersExceeded, writerCount, MaxWriterCount)
	}

	// Check for nil writers in Outputs slice
	for i, w := range c.Outputs {
		if w == nil {
			return fmt.Errorf("writer at Outputs[%d] is nil", i)
		}
	}

	return nil
}

// internalConfig is used internally to create a logger.
type internalConfig struct {
	level             LogLevel
	format            LogFormat
	timeFormat        string
	includeTime       bool
	includeLevel      bool
	fullPath          bool
	dynamicCaller     bool
	writers           []io.Writer
	json              *JSONOptions
	securityConfig    *SecurityConfig
	fieldValidation   *FieldValidationConfig
	fatalHandler      FatalHandler
	writeErrorHandler WriteErrorHandler
	contextExtractors []ContextExtractor
	hooks             *HookRegistry
	sampling          *SamplingConfig
}
