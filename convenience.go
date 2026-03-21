// Package dd provides convenience constructors for quick logger setup.
package dd

import (
	"io"
	"os"
)

// DefaultLogPath is the default path for log files.
const DefaultLogPath = "logs/app.log"

// ============================================================================
// File Output Constructors (return error)
// ============================================================================

// ToFile creates a logger that outputs to a file only.
// If no filename is provided, uses the default path "logs/app.log".
// The format is text (human-readable).
//
// Example:
//
//	logger, err := dd.ToFile()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer logger.Close()
//
//	// Custom filename
//	logger, err := dd.ToFile("logs/myapp.log")
func ToFile(filename ...string) (*Logger, error) {
	path := DefaultLogPath
	if len(filename) > 0 && filename[0] != "" {
		path = filename[0]
	}

	cfg := DefaultConfig()
	cfg.File = &FileConfig{Path: path}
	cfg.Output = nil
	cfg.Outputs = nil
	return New(cfg)
}

// ToJSONFile creates a logger that outputs to a file in JSON format only.
// If no filename is provided, uses the default path "logs/app.log".
//
// Example:
//
//	logger, err := dd.ToJSONFile()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer logger.Close()
func ToJSONFile(filename ...string) (*Logger, error) {
	path := DefaultLogPath
	if len(filename) > 0 && filename[0] != "" {
		path = filename[0]
	}

	cfg := JSONConfig()
	cfg.File = &FileConfig{Path: path}
	cfg.Output = nil
	cfg.Outputs = nil
	return New(cfg)
}

// ToConsole creates a logger that outputs to stdout only.
// The format is text (human-readable).
//
// Example:
//
//	logger, err := dd.ToConsole()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer logger.Close()
func ToConsole() (*Logger, error) {
	cfg := DefaultConfig()
	cfg.Output = os.Stdout
	cfg.Outputs = nil
	cfg.File = nil
	return New(cfg)
}

// ToAll creates a logger that outputs to both console and file.
// If no filename is provided, uses the default path "logs/app.log".
// The format is text (human-readable).
//
// Example:
//
//	logger, err := dd.ToAll()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer logger.Close()
func ToAll(filename ...string) (*Logger, error) {
	path := DefaultLogPath
	if len(filename) > 0 && filename[0] != "" {
		path = filename[0]
	}

	cfg := DefaultConfig()
	cfg.Output = os.Stdout
	cfg.File = &FileConfig{Path: path}
	return New(cfg)
}

// ToAllJSON creates a logger that outputs to both console and file in JSON format.
// If no filename is provided, uses the default path "logs/app.log".
//
// Example:
//
//	logger, err := dd.ToAllJSON()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer logger.Close()
func ToAllJSON(filename ...string) (*Logger, error) {
	path := DefaultLogPath
	if len(filename) > 0 && filename[0] != "" {
		path = filename[0]
	}

	cfg := JSONConfig()
	cfg.Output = os.Stdout
	cfg.File = &FileConfig{Path: path}
	return New(cfg)
}

// ToWriter creates a logger that outputs to the provided writer.
//
// Example:
//
//	var buf bytes.Buffer
//	logger, err := dd.ToWriter(&buf)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer logger.Close()
func ToWriter(w io.Writer) (*Logger, error) {
	cfg := DefaultConfig()
	cfg.Output = w
	cfg.Outputs = nil
	cfg.File = nil
	return New(cfg)
}

// ToWriters creates a logger that outputs to multiple writers.
//
// Example:
//
//	logger, err := dd.ToWriters(os.Stdout, fileWriter)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer logger.Close()
func ToWriters(writers ...io.Writer) (*Logger, error) {
	cfg := DefaultConfig()
	cfg.Output = nil
	cfg.Outputs = writers
	cfg.File = nil
	return New(cfg)
}
