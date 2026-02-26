package dd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cybergodev/dd/internal"
)

type JSONOptions = internal.JSONOptions
type JSONFieldNames = internal.JSONFieldNames

func DefaultJSONOptions() *JSONOptions {
	return &JSONOptions{
		PrettyPrint: false,
		Indent:      DefaultJSONIndent,
		FieldNames:  internal.DefaultJSONFieldNames(),
	}
}

type LoggerConfig struct {
	Level             LogLevel
	Format            LogFormat
	TimeFormat        string
	IncludeTime       bool
	IncludeLevel      bool
	FullPath          bool
	DynamicCaller     bool
	Writers           []io.Writer
	SecurityConfig    *SecurityConfig
	FatalHandler      FatalHandler
	WriteErrorHandler WriteErrorHandler
	JSON              *JSONOptions
}

func DefaultConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:          LevelInfo,
		Format:         FormatText,
		TimeFormat:     DefaultTimeFormat,
		IncludeTime:    true,
		IncludeLevel:   true,
		FullPath:       false,
		DynamicCaller:  false,
		SecurityConfig: DefaultSecurityConfig(),
	}
}

func DevelopmentConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:         LevelDebug,
		Format:        FormatText,
		TimeFormat:    DevTimeFormat,
		IncludeTime:   true,
		IncludeLevel:  true,
		FullPath:      false,
		DynamicCaller: true,
	}
}

func JSONConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:         LevelDebug,
		Format:        FormatJSON,
		TimeFormat:    time.RFC3339,
		IncludeTime:   true,
		IncludeLevel:  true,
		FullPath:      false,
		DynamicCaller: true,
		JSON: &internal.JSONOptions{
			PrettyPrint: false,
			Indent:      DefaultJSONIndent,
			FieldNames:  internal.DefaultJSONFieldNames(),
		},
	}
}

// Clone creates a shallow copy of the configuration.
// Note: The Writers slice is copied but the Writer instances themselves are shared.
// SecurityConfig is deep-copied.
func (c *LoggerConfig) Clone() *LoggerConfig {
	if c == nil {
		return DefaultConfig()
	}

	clone := &LoggerConfig{
		Level:             c.Level,
		Format:            c.Format,
		TimeFormat:        c.TimeFormat,
		IncludeTime:       c.IncludeTime,
		IncludeLevel:      c.IncludeLevel,
		FullPath:          c.FullPath,
		DynamicCaller:     c.DynamicCaller,
		FatalHandler:      c.FatalHandler,
		WriteErrorHandler: c.WriteErrorHandler,
		Writers:           make([]io.Writer, len(c.Writers)),
	}
	copy(clone.Writers, c.Writers)

	if c.SecurityConfig != nil {
		clone.SecurityConfig = c.SecurityConfig.Clone()
	}

	if c.JSON != nil {
		clone.JSON = &JSONOptions{
			PrettyPrint: c.JSON.PrettyPrint,
			Indent:      c.JSON.Indent,
		}
		if c.JSON.FieldNames != nil {
			clone.JSON.FieldNames = &JSONFieldNames{
				Timestamp: c.JSON.FieldNames.Timestamp,
				Level:     c.JSON.FieldNames.Level,
				Caller:    c.JSON.FieldNames.Caller,
				Message:   c.JSON.FieldNames.Message,
				Fields:    c.JSON.FieldNames.Fields,
			}
		}
	}

	return clone
}

func (c *LoggerConfig) Validate() error {
	if c == nil {
		return ErrNilConfig
	}

	if c.Level < LevelDebug || c.Level > LevelFatal {
		return fmt.Errorf("%w: %d (valid range: %d-%d)", ErrInvalidLevel, c.Level, LevelDebug, LevelFatal)
	}

	if c.Format != FormatText && c.Format != FormatJSON {
		return fmt.Errorf("%w: %d (valid: %d=Text, %d=JSON)", ErrInvalidFormat, c.Format, FormatText, FormatJSON)
	}

	return nil
}

// ApplyDefaults sets default values for uninitialized config fields.
// This should be called after validation and before creating the logger.
func (c *LoggerConfig) ApplyDefaults() {
	if c == nil {
		return
	}

	if c.IncludeTime && c.TimeFormat == "" {
		c.TimeFormat = time.RFC3339
	}

	if len(c.Writers) == 0 {
		c.Writers = []io.Writer{os.Stdout}
	}

	if c.SecurityConfig == nil {
		c.SecurityConfig = DefaultSecurityConfig()
	} else {
		if c.SecurityConfig.MaxMessageSize <= 0 {
			c.SecurityConfig.MaxMessageSize = MaxMessageSize
		}
		if c.SecurityConfig.MaxWriters <= 0 {
			c.SecurityConfig.MaxWriters = MaxWriterCount
		}
	}
}

func (c *LoggerConfig) WithFile(filename string, config FileWriterConfig) (*LoggerConfig, error) {
	if c == nil {
		return nil, ErrNilConfig
	}
	if filename == "" {
		return nil, ErrEmptyFilePath
	}

	fileWriter, err := NewFileWriter(filename, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create file writer: %w", err)
	}

	clone := c.Clone()
	if clone.Writers == nil {
		clone.Writers = []io.Writer{}
	}

	if len(clone.Writers) >= MaxWriterCount {
		return nil, ErrMaxWritersExceeded
	}

	clone.Writers = append(clone.Writers, fileWriter)
	return clone, nil
}

func (c *LoggerConfig) WithFileOnly(filename string, config FileWriterConfig) (*LoggerConfig, error) {
	if c == nil {
		return nil, ErrNilConfig
	}
	if filename == "" {
		return nil, ErrEmptyFilePath
	}

	fileWriter, err := NewFileWriter(filename, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create file writer: %w", err)
	}

	clone := c.Clone()
	clone.Writers = []io.Writer{fileWriter}
	return clone, nil
}

func (c *LoggerConfig) WithWriter(writer io.Writer) *LoggerConfig {
	if c == nil || writer == nil {
		return c
	}

	clone := c.Clone()
	if clone.Writers == nil {
		clone.Writers = []io.Writer{}
	}

	if len(clone.Writers) >= MaxWriterCount {
		return c
	}

	clone.Writers = append(clone.Writers, writer)
	return clone
}

func (c *LoggerConfig) WithLevel(level LogLevel) *LoggerConfig {
	if c == nil {
		return nil
	}
	if level < LevelDebug || level > LevelFatal {
		return c
	}
	clone := c.Clone()
	clone.Level = level
	return clone
}

func (c *LoggerConfig) WithFormat(format LogFormat) *LoggerConfig {
	if c == nil {
		return nil
	}
	if format != FormatText && format != FormatJSON {
		return c
	}
	clone := c.Clone()
	clone.Format = format
	return clone
}

func (c *LoggerConfig) WithDynamicCaller(enabled bool) *LoggerConfig {
	if c == nil {
		return nil
	}
	clone := c.Clone()
	clone.DynamicCaller = enabled
	return clone
}

// DisableFiltering disables all sensitive data filtering.
// Note: Basic filtering is enabled by default for security.
// Use this method if you need to log raw data without any filtering.
func (c *LoggerConfig) DisableFiltering() *LoggerConfig {
	if c == nil {
		return nil
	}
	clone := c.Clone()
	clone.ensureSecurityConfig()
	clone.SecurityConfig.SensitiveFilter = nil
	return clone
}

func (c *LoggerConfig) EnableBasicFiltering() *LoggerConfig {
	if c == nil {
		return nil
	}
	clone := c.Clone()
	clone.ensureSecurityConfig()
	clone.SecurityConfig.SensitiveFilter = NewBasicSensitiveDataFilter()
	return clone
}

func (c *LoggerConfig) EnableFullFiltering() *LoggerConfig {
	if c == nil {
		return nil
	}
	clone := c.Clone()
	clone.ensureSecurityConfig()
	clone.SecurityConfig.SensitiveFilter = NewSensitiveDataFilter()
	return clone
}

func (c *LoggerConfig) WithFilter(filter *SensitiveDataFilter) *LoggerConfig {
	if c == nil {
		return nil
	}
	clone := c.Clone()
	clone.ensureSecurityConfig()
	clone.SecurityConfig.SensitiveFilter = filter
	return clone
}

func (c *LoggerConfig) WithWriteErrorHandler(handler WriteErrorHandler) *LoggerConfig {
	if c == nil {
		return nil
	}
	clone := c.Clone()
	clone.WriteErrorHandler = handler
	return clone
}

func (c *LoggerConfig) ensureSecurityConfig() {
	if c.SecurityConfig == nil {
		c.SecurityConfig = &SecurityConfig{
			MaxMessageSize: MaxMessageSize,
			MaxWriters:     MaxWriterCount,
		}
	}
}
