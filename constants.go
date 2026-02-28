package dd

import (
	"time"

	"github.com/cybergodev/dd/internal"
)

// LogFormat defines the output format for log messages.
type LogFormat = internal.LogFormat

const (
	FormatText LogFormat = internal.LogFormatText
	FormatJSON LogFormat = internal.LogFormatJSON
)

const (
	// DefaultCallerDepth is the number of stack frames to skip when
	// determining the caller of a log function.
	// Value 3 accounts for: runtime.Caller -> GetCaller -> Log method -> user code
	DefaultCallerDepth = 3
)

const (
	DefaultBufferSize = 1024
	MaxBufferSize     = 4 * 1024
	// FieldBuilderCapacity and EstimatedFieldSize are defined in internal/fields.go
)

const (
	MaxPathLength      = 4096
	MaxMessageSize     = 5 * 1024 * 1024
	MaxInputLength     = 256 * 1024
	MaxWriterCount     = 100
	MaxPatternLength   = 1000
	MaxQuantifierRange = 1000
	MaxRecursionDepth  = 100
)

const (
	MaxBackupCount = 1000
	MaxFileSizeMB  = 10240
)

const (
	DefaultMaxSizeMB    = 100
	DefaultMaxBackups   = 10
	DefaultMaxAge       = 30 * 24 * time.Hour
	DefaultBufferSizeKB = 1
	MaxBufferSizeKB     = 10 * 1024
	AutoFlushThreshold  = 2
	AutoFlushInterval   = 100 * time.Millisecond
)

// File system permission constants.
const (
	// DirPermissions is the permission mode for creating directories (rwx------).
	DirPermissions = 0700
)

// FilePermissions (0600) for log files is defined in the internal package.

const (
	// DefaultFilterTimeout is the maximum time for sensitive data filtering.
	// This timeout protects against ReDoS (Regular Expression Denial of Service)
	// attacks on malicious input with complex regex patterns.
	// 50ms provides reasonable protection while allowing thorough filtering.
	DefaultFilterTimeout = 50 * time.Millisecond

	// EmptyFilterTimeout is used for filters with no patterns.
	// Shorter timeout since minimal processing is needed.
	EmptyFilterTimeout = 10 * time.Millisecond

	// MaxConcurrentFilters limits concurrent regex filtering goroutines.
	// Prevents resource exhaustion in high-concurrency scenarios.
	MaxConcurrentFilters = 100
)

const (
	DefaultTimeFormat = "2006-01-02T15:04:05Z07:00"
	DevTimeFormat     = "15:04:05.000"
)

const (
	// DefaultFatalFlushTimeout is the maximum time to wait for logger flush
	// during fatal log handling. This ensures the program can exit even if
	// the underlying writer is blocked or unresponsive.
	DefaultFatalFlushTimeout = 5 * time.Second
)

const (
	DefaultJSONIndent = "  "
	// FastPathThreshold is the input length threshold (bytes) below which
	// regex filtering skips chunking and timeout protection.
	// Small inputs are processed directly for better performance.
	FastPathThreshold = 100

	// BoundaryCheckSize is the size of the boundary region to check for sensitive data
	// when truncating input. This ensures patterns spanning the truncation boundary
	// are still detected and redacted.
	// Set to 512 to cover most sensitive data patterns (credit cards, SSNs, API keys, etc.)
	BoundaryCheckSize = 512

	// ChunkOverlapSize is the overlap size between chunks during chunked filtering.
	// This ensures sensitive data patterns that span chunk boundaries are still detected.
	// Must be >= maximum expected sensitive pattern length.
	ChunkOverlapSize = 512
)

const (
	// DebugVisualizationDepth is the caller depth for debug visualization functions.
	// Value of 2 means: 0 = current function, 1 = caller, 2 = caller's caller.
	DebugVisualizationDepth = 2
)
