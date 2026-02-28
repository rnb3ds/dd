package dd

import (
	"errors"
	"testing"
)

func TestLoggerError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *LoggerError
		expected string
	}{
		{
			name: "error without cause",
			err: &LoggerError{
				Code:    ErrCodeInvalidLevel,
				Message: "level must be between 0 and 4",
			},
			expected: "[INVALID_LEVEL] level must be between 0 and 4",
		},
		{
			name: "error with cause",
			err: &LoggerError{
				Code:    ErrCodeConfigValidation,
				Message: "configuration validation failed",
				Cause:   errors.New("underlying error"),
			},
			expected: "[CONFIG_VALIDATION] configuration validation failed: underlying error",
		},
		{
			name: "error with context",
			err: &LoggerError{
				Code:    ErrCodeInvalidLevel,
				Message: "invalid level",
				Context: map[string]any{"level": 10},
			},
			expected: "[INVALID_LEVEL] invalid level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestLoggerError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &LoggerError{
		Code:    ErrCodeConfigValidation,
		Message: "validation failed",
		Cause:   cause,
	}

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}

	// Test nil cause
	errNoCause := &LoggerError{
		Code:    ErrCodeInvalidLevel,
		Message: "no cause",
	}
	if errNoCause.Unwrap() != nil {
		t.Errorf("Unwrap() for nil cause should return nil")
	}
}

func TestLoggerError_Is(t *testing.T) {
	tests := []struct {
		name        string
		err         *LoggerError
		target      error
		shouldMatch bool
	}{
		{
			name: "match ErrInvalidLevel",
			err: &LoggerError{
				Code:    ErrCodeInvalidLevel,
				Message: "invalid level provided",
			},
			target:      ErrInvalidLevel,
			shouldMatch: true,
		},
		{
			name: "match ErrLoggerClosed",
			err: &LoggerError{
				Code:    ErrCodeLoggerClosed,
				Message: "logger is closed",
			},
			target:      ErrLoggerClosed,
			shouldMatch: true,
		},
		{
			name: "no match different error",
			err: &LoggerError{
				Code:    ErrCodeInvalidLevel,
				Message: "invalid level",
			},
			target:      ErrLoggerClosed,
			shouldMatch: false,
		},
		{
			name: "no match non-sentinel error",
			err: &LoggerError{
				Code:    ErrCodeInvalidLevel,
				Message: "invalid level",
			},
			target:      errors.New("random error"),
			shouldMatch: false,
		},
		{
			name: "match ErrNilConfig",
			err: &LoggerError{
				Code:    ErrCodeNilConfig,
				Message: "config is nil",
			},
			target:      ErrNilConfig,
			shouldMatch: true,
		},
		{
			name: "match ErrNilWriter",
			err: &LoggerError{
				Code:    ErrCodeNilWriter,
				Message: "writer is nil",
			},
			target:      ErrNilWriter,
			shouldMatch: true,
		},
		{
			name: "match ErrPatternTooLong",
			err: &LoggerError{
				Code:    ErrCodePatternTooLong,
				Message: "pattern exceeds maximum length",
			},
			target:      ErrPatternTooLong,
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if errors.Is(tt.err, tt.target) != tt.shouldMatch {
				t.Errorf("errors.Is(%v, %v) should be %v", tt.err, tt.target, tt.shouldMatch)
			}
		})
	}
}

func TestNewLoggerError(t *testing.T) {
	err := NewError(ErrCodeInvalidLevel, "invalid level value")

	if err.Code != ErrCodeInvalidLevel {
		t.Errorf("Code = %q, want %q", err.Code, ErrCodeInvalidLevel)
	}
	if err.Message != "invalid level value" {
		t.Errorf("Message = %q, want %q", err.Message, "invalid level value")
	}
	if err.Cause != nil {
		t.Errorf("Cause should be nil")
	}
	if err.Context != nil {
		t.Errorf("Context should be nil")
	}
}

func TestWrapError(t *testing.T) {
	cause := errors.New("underlying error")
	err := WrapError(ErrCodeConfigValidation, "validation failed", cause)

	if err.Code != ErrCodeConfigValidation {
		t.Errorf("Code = %q, want %q", err.Code, ErrCodeConfigValidation)
	}
	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}

	// Test nil cause returns nil
	nilErr := WrapError(ErrCodeInvalidLevel, "test", nil)
	if nilErr != nil {
		t.Errorf("WrapError with nil cause should return nil")
	}
}

func TestLoggerError_WithContext(t *testing.T) {
	err := &LoggerError{
		Code:    ErrCodeInvalidLevel,
		Message: "invalid level",
	}

	// Add context
	errWithContext := err.WithContext("level", 10)

	if errWithContext.Context == nil {
		t.Fatal("Context should not be nil")
	}
	if errWithContext.Context["level"] != 10 {
		t.Errorf("Context[level] = %v, want 10", errWithContext.Context["level"])
	}

	// Original error should not be modified
	if err.Context != nil {
		t.Errorf("Original error context should be nil")
	}

	// Test multiple context values
	errWithContext2 := errWithContext.WithContext("max", 4)
	if errWithContext2.Context["level"] != 10 {
		t.Errorf("Context[level] should still be 10")
	}
	if errWithContext2.Context["max"] != 4 {
		t.Errorf("Context[max] = %v, want 4", errWithContext2.Context["max"])
	}

	// Test nil receiver
	var nilErr *LoggerError
	if nilErr.WithContext("key", "value") != nil {
		t.Errorf("WithContext on nil should return nil")
	}
}

func TestErrorsIsWithWrappedError(t *testing.T) {
	// Test that errors.Is works through the chain
	cause := errors.New("root cause")
	wrapped := WrapError(ErrCodeConfigValidation, "config error", cause)

	// The wrapped error should match ErrConfigValidation sentinel (if we had one)
	// But we can test that Unwrap returns the cause
	unwrapped := errors.Unwrap(wrapped)
	if unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestErrorsAs(t *testing.T) {
	err := &LoggerError{
		Code:    ErrCodeInvalidLevel,
		Message: "invalid level provided",
		Context: map[string]any{
			"provided": 10,
			"max":      4,
		},
	}

	var loggerErr *LoggerError
	if !errors.As(err, &loggerErr) {
		t.Fatal("errors.As should return true")
	}

	if loggerErr.Code != ErrCodeInvalidLevel {
		t.Errorf("Code = %q, want %q", loggerErr.Code, ErrCodeInvalidLevel)
	}
	if loggerErr.Context["provided"] != 10 {
		t.Errorf("Context[provided] = %v, want 10", loggerErr.Context["provided"])
	}
}

func TestAllErrorCodesMatchSentinels(t *testing.T) {
	// Test that all error codes correctly match their sentinel errors
	codeToSentinel := map[string]error{
		ErrCodeNilConfig:          ErrNilConfig,
		ErrCodeNilWriter:          ErrNilWriter,
		ErrCodeNilFilter:          ErrNilFilter,
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

	for code, sentinel := range codeToSentinel {
		t.Run(code, func(t *testing.T) {
			err := &LoggerError{
				Code:    code,
				Message: "test message",
			}
			if !errors.Is(err, sentinel) {
				t.Errorf("errors.Is(LoggerError{Code: %q}, %v) = false, want true", code, sentinel)
			}
		})
	}
}
