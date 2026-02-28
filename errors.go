package dd

import (
	"errors"
	"fmt"
	"io"
)

// Error codes for structured error handling.
// These codes enable programmatic error matching using errors.Is() and errors.As().
const (
	ErrCodeNilConfig          = "NIL_CONFIG"
	ErrCodeNilWriter          = "NIL_WRITER"
	ErrCodeNilFilter          = "NIL_FILTER"
	ErrCodeNilHook            = "NIL_HOOK"
	ErrCodeNilExtractor       = "NIL_EXTRACTOR"
	ErrCodeLoggerClosed       = "LOGGER_CLOSED"
	ErrCodeWriterNotFound     = "WRITER_NOT_FOUND"
	ErrCodeInvalidLevel       = "INVALID_LEVEL"
	ErrCodeInvalidFormat      = "INVALID_FORMAT"
	ErrCodeMaxWritersExceeded = "MAX_WRITERS_EXCEEDED"
	ErrCodeEmptyFilePath      = "EMPTY_FILE_PATH"
	ErrCodePathTooLong        = "PATH_TOO_LONG"
	ErrCodePathTraversal      = "PATH_TRAVERSAL"
	ErrCodeNullByte           = "NULL_BYTE"
	ErrCodeInvalidPath        = "INVALID_PATH"
	ErrCodeSymlinkNotAllowed  = "SYMLINK_NOT_ALLOWED"
	ErrCodeMaxSizeExceeded    = "MAX_SIZE_EXCEEDED"
	ErrCodeMaxBackupsExceeded = "MAX_BACKUPS_EXCEEDED"
	ErrCodeBufferSizeTooLarge = "BUFFER_SIZE_TOO_LARGE"
	ErrCodeInvalidPattern     = "INVALID_PATTERN"
	ErrCodeEmptyPattern       = "EMPTY_PATTERN"
	ErrCodePatternTooLong     = "PATTERN_TOO_LONG"
	ErrCodeReDoSPattern       = "REDOS_PATTERN"
	ErrCodePatternFailed      = "PATTERN_FAILED"
	ErrCodeConfigValidation   = "CONFIG_VALIDATION"
	ErrCodeWriterAdd          = "WRITER_ADD"
)

// LoggerError represents a structured error with additional context.
// It implements error, Unwrap(), and Is() interfaces for fine-grained error matching.
//
// Example usage:
//
//	logger, err := dd.New(config)
//	if err != nil {
//	    var loggerErr *dd.LoggerError
//	    if errors.As(err, &loggerErr) {
//	        fmt.Printf("Error code: %s\n", loggerErr.Code)
//	        fmt.Printf("Context: %v\n", loggerErr.Context)
//	    }
//	    if errors.Is(err, dd.ErrInvalidLevel) {
//	        // Handle invalid level specifically
//	    }
//	}
type LoggerError struct {
	Code    string         // Machine-readable error code (e.g., "INVALID_LEVEL")
	Message string         // Human-readable message
	Cause   error          // Underlying error (for wrapping)
	Context map[string]any // Additional context for debugging
}

// Error implements the error interface.
func (e *LoggerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause for use with errors.Is() and errors.As().
func (e *LoggerError) Unwrap() error {
	return e.Cause
}

// errorCodeToSentinel maps error codes to their corresponding sentinel errors.
var errorCodeToSentinel = map[string]error{
	ErrCodeNilConfig:          ErrNilConfig,
	ErrCodeNilWriter:          ErrNilWriter,
	ErrCodeNilFilter:          ErrNilFilter,
	ErrCodeNilHook:            ErrNilHook,
	ErrCodeNilExtractor:       ErrNilExtractor,
	ErrCodeLoggerClosed:       ErrLoggerClosed,
	ErrCodeWriterNotFound:     ErrWriterNotFound,
	ErrCodeInvalidLevel:       ErrInvalidLevel,
	ErrCodeInvalidFormat:      ErrInvalidFormat,
	ErrCodeMaxWritersExceeded: ErrMaxWritersExceeded,
	ErrCodeEmptyFilePath:      ErrEmptyFilePath,
	ErrCodePathTooLong:        ErrPathTooLong,
	ErrCodePathTraversal:      ErrPathTraversal,
	ErrCodeNullByte:           ErrNullByte,
	ErrCodeInvalidPath:        ErrInvalidPath,
	ErrCodeSymlinkNotAllowed:  ErrSymlinkNotAllowed,
	ErrCodeMaxSizeExceeded:    ErrMaxSizeExceeded,
	ErrCodeMaxBackupsExceeded: ErrMaxBackupsExceeded,
	ErrCodeBufferSizeTooLarge: ErrBufferSizeTooLarge,
	ErrCodeInvalidPattern:     ErrInvalidPattern,
	ErrCodeEmptyPattern:       ErrEmptyPattern,
	ErrCodePatternTooLong:     ErrPatternTooLong,
	ErrCodeReDoSPattern:       ErrReDoSPattern,
	ErrCodePatternFailed:      ErrPatternFailed,
}

// Is enables matching against sentinel errors using errors.Is().
// This allows LoggerError instances to match their corresponding sentinel errors.
func (e *LoggerError) Is(target error) bool {
	if sentinel, ok := errorCodeToSentinel[e.Code]; ok {
		return target == sentinel
	}
	return false
}

// NewError creates a new LoggerError with the given code and message.
func NewError(code, message string) *LoggerError {
	return &LoggerError{
		Code:    code,
		Message: message,
	}
}

// WrapError wraps an existing error with a code and message.
// If the error is nil, returns nil.
func WrapError(code, message string, cause error) *LoggerError {
	if cause == nil {
		return nil
	}
	return &LoggerError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// WithContext adds context to a LoggerError.
// Returns a new LoggerError with the context added.
func (e *LoggerError) WithContext(key string, value any) *LoggerError {
	if e == nil {
		return nil
	}
	newContext := make(map[string]any, len(e.Context)+1)
	for k, v := range e.Context {
		newContext[k] = v
	}
	newContext[key] = value
	return &LoggerError{
		Code:    e.Code,
		Message: e.Message,
		Cause:   e.Cause,
		Context: newContext,
	}
}

// Sentinel errors for backward compatibility.
// These can be used with errors.Is() for simple error matching.
var (
	ErrNilConfig          = errors.New("config cannot be nil")
	ErrNilWriter          = errors.New("writer cannot be nil")
	ErrNilFilter          = errors.New("filter cannot be nil")
	ErrNilHook            = errors.New("hook cannot be nil")
	ErrNilExtractor       = errors.New("context extractor cannot be nil")
	ErrLoggerClosed       = errors.New("logger is closed")
	ErrWriterNotFound     = errors.New("writer not found")
	ErrInvalidLevel       = errors.New("invalid log level")
	ErrInvalidFormat      = errors.New("invalid log format")
	ErrMaxWritersExceeded = errors.New("maximum writer count exceeded")
	ErrEmptyFilePath      = errors.New("file path cannot be empty")
	ErrPathTooLong        = errors.New("file path too long")
	ErrPathTraversal      = errors.New("path traversal detected")
	ErrNullByte           = errors.New("null byte in input")
	ErrInvalidPath        = errors.New("invalid file path")
	ErrSymlinkNotAllowed  = errors.New("symlinks not allowed")
	ErrMaxSizeExceeded    = errors.New("maximum size exceeded")
	ErrMaxBackupsExceeded = errors.New("maximum backup count exceeded")
	ErrBufferSizeTooLarge = errors.New("buffer size too large")
	ErrInvalidPattern     = errors.New("invalid regex pattern")
	ErrEmptyPattern       = errors.New("pattern cannot be empty")
	ErrPatternTooLong     = errors.New("pattern length exceeds maximum")
	ErrReDoSPattern       = errors.New("pattern contains dangerous nested quantifiers that may cause ReDoS")
	ErrPatternFailed      = errors.New("failed to add pattern")
)

// WriterError represents an error from a single writer in a MultiWriter.
type WriterError struct {
	Index  int       // Index of the writer in the MultiWriter
	Writer io.Writer // The writer that encountered the error
	Err    error     // The error that occurred
}

// Error implements the error interface.
func (e *WriterError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("writer[%d]: %v", e.Index, e.Err)
	}
	return fmt.Sprintf("writer[%d]: unknown error", e.Index)
}

// Unwrap returns the underlying error.
func (e *WriterError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// MultiWriterError collects errors from multiple writers.
// This is returned by MultiWriter.Write() when one or more writers fail.
type MultiWriterError struct {
	Errors []WriterError // Collection of writer errors
}

// Error implements the error interface.
func (e *MultiWriterError) Error() string {
	if e == nil || len(e.Errors) == 0 {
		return ""
	}

	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("multiple writer errors: %v", msgs)
}

// Unwrap returns all underlying errors for use with errors.As().
// Note: errors.Is() will check against each wrapped error.
func (e *MultiWriterError) Unwrap() []error {
	if e == nil || len(e.Errors) == 0 {
		return nil
	}

	errs := make([]error, len(e.Errors))
	for i, we := range e.Errors {
		errs[i] = we.Err
	}
	return errs
}

// HasErrors returns true if any errors were collected.
func (e *MultiWriterError) HasErrors() bool {
	return e != nil && len(e.Errors) > 0
}

// ErrorCount returns the number of errors collected.
func (e *MultiWriterError) ErrorCount() int {
	if e == nil {
		return 0
	}
	return len(e.Errors)
}

// FirstError returns the first error that occurred.
func (e *MultiWriterError) FirstError() error {
	if e == nil || len(e.Errors) == 0 {
		return nil
	}
	return &e.Errors[0]
}

// AddError adds a writer error to the collection.
func (e *MultiWriterError) AddError(index int, writer io.Writer, err error) {
	e.Errors = append(e.Errors, WriterError{
		Index:  index,
		Writer: writer,
		Err:    err,
	})
}
