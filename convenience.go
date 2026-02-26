package dd

import (
	"fmt"
	"io"
	"os"

	"github.com/cybergodev/dd/internal"
)

type Options struct {
	Level             LogLevel
	Format            LogFormat
	Console           bool
	File              string
	FileConfig        FileWriterConfig
	FullPath          bool
	DynamicCaller     bool
	TimeFormat        string
	FilterLevel       FilterLevel
	CustomFilter      *SensitiveDataFilter
	JSONOptions       *JSONOptions
	AdditionalWriters []io.Writer
}

func NewWithOptions(opts Options) (*Logger, error) {
	if opts.Level < LevelDebug || opts.Level > LevelFatal {
		opts.Level = LevelInfo
	}
	if opts.Format != FormatText && opts.Format != FormatJSON {
		opts.Format = FormatText
	}
	if opts.TimeFormat == "" {
		opts.TimeFormat = DefaultTimeFormat
	}

	writerCap := 0
	if opts.Console {
		writerCap++
	}
	if opts.File != "" {
		writerCap++
	}
	additionalCount := len(opts.AdditionalWriters)
	if additionalCount > 0 {
		if writerCap+additionalCount > MaxWriterCount {
			return nil, fmt.Errorf("%w: requested %d writers (max %d)",
				ErrMaxWritersExceeded, writerCap+additionalCount, MaxWriterCount)
		}
		writerCap += additionalCount
	}
	if writerCap == 0 {
		writerCap = 1
	}

	config := &LoggerConfig{
		Level:         opts.Level,
		Format:        opts.Format,
		TimeFormat:    opts.TimeFormat,
		FullPath:      opts.FullPath,
		DynamicCaller: opts.DynamicCaller,
		IncludeTime:   true,
		IncludeLevel:  true,
		Writers:       make([]io.Writer, 0, writerCap),
	}

	if opts.Console {
		config.Writers = append(config.Writers, os.Stdout)
	}

	if opts.File != "" {
		fileWriter, err := NewFileWriter(opts.File, opts.FileConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create file writer: %w", err)
		}
		config.Writers = append(config.Writers, fileWriter)
	}

	if len(opts.AdditionalWriters) > 0 {
		for _, w := range opts.AdditionalWriters {
			if w != nil {
				config.Writers = append(config.Writers, w)
			}
		}
	}

	if len(config.Writers) == 0 {
		config.Writers = []io.Writer{os.Stdout}
	}

	config.SecurityConfig = DefaultSecurityConfig()
	if opts.CustomFilter != nil {
		config.SecurityConfig.SensitiveFilter = opts.CustomFilter
	} else {
		switch opts.FilterLevel {
		case FilterNone:
			config.SecurityConfig.SensitiveFilter = nil
		case FilterBasic:
			config.SecurityConfig.SensitiveFilter = NewBasicSensitiveDataFilter()
		case FilterFull:
			config.SecurityConfig.SensitiveFilter = NewSensitiveDataFilter()
		default:
			// Default is already set by DefaultSecurityConfig() which uses Basic
		}
	}

	if opts.Format == FormatJSON {
		if opts.JSONOptions != nil {
			config.JSON = opts.JSONOptions
		} else {
			config.JSON = &internal.JSONOptions{
				PrettyPrint: false,
				Indent:      DefaultJSONIndent,
				FieldNames:  internal.DefaultJSONFieldNames(),
			}
		}
	}

	return New(config)
}

func getFilename(filename []string) string {
	if len(filename) > 0 && filename[0] != "" {
		return filename[0]
	}
	return DefaultLogFile
}

// FileLogger creates a logger that writes to a file.
// Returns an error if the file cannot be created or opened.
func FileLogger(filename ...string) (*Logger, error) {
	return NewWithOptions(Options{
		Console: false,
		File:    getFilename(filename),
	})
}

// ConsoleLogger creates a logger that writes to stdout.
// Returns an error if the logger cannot be created.
func ConsoleLogger() (*Logger, error) {
	return NewWithOptions(Options{Console: true})
}

// JSONFileLogger creates a logger that writes JSON format to a file.
// Returns an error if the file cannot be created or opened.
func JSONFileLogger(filename ...string) (*Logger, error) {
	return NewWithOptions(Options{
		Format:  FormatJSON,
		Console: false,
		File:    getFilename(filename),
	})
}

// MultiLogger creates a logger that writes to both console and file.
// Returns an error if the file cannot be created or opened.
func MultiLogger(filename ...string) (*Logger, error) {
	return NewWithOptions(Options{
		Console: true,
		File:    getFilename(filename),
	})
}
