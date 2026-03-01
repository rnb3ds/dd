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
	// DefaultBufferSize is the initial capacity for message buffers.
	// 1024 bytes covers most typical log messages without reallocation.
	DefaultBufferSize = 1024

	// MaxBufferSize is the maximum buffer capacity returned to the pool.
	// Buffers larger than 4KB are replaced with default-sized buffers to
	// prevent memory bloat from occasional large messages. This value
	// balances memory efficiency with performance for typical workloads.
	MaxBufferSize = 4 * 1024

	// FieldBuilderCapacity and EstimatedFieldSize are defined in internal/fields.go
)

const (
	// MaxPathLength limits file paths to 4096 bytes (POSIX PATH_MAX).
	// Prevents path traversal attacks and memory exhaustion from malicious paths.
	MaxPathLength = 4096

	// MaxMessageSize limits formatted log messages to 5MB.
	// This prevents memory exhaustion from extremely large log entries while
	// allowing substantial content (e.g., stack traces with context).
	MaxMessageSize = 5 * 1024 * 1024

	// MaxInputLength limits input for sensitive data filtering to 256KB.
	// Beyond this size, filtering becomes CPU-intensive. The boundary-aware
	// truncation ensures sensitive data is still detected at the edges.
	MaxInputLength = 256 * 1024

	// MaxWriterCount limits concurrent writers to 100.
	// This prevents resource exhaustion from misconfigured loggers while
	// allowing reasonable multi-output scenarios (file + stdout + network).
	MaxWriterCount = 100

	// MaxPatternLength limits regex patterns to 1000 characters.
	// Longer patterns are rarely needed and may indicate ReDoS attempts.
	MaxPatternLength = 1000

	// MaxQuantifierRange limits regex quantifier ranges (e.g., {1,1000}).
	// Prevents ReDoS attacks with patterns like (a{1,1000000})+.
	MaxQuantifierRange = 1000

	// MaxRecursionDepth limits recursive filtering of nested structures.
	// Prevents stack overflow from deeply nested or circular data.
	MaxRecursionDepth = 100
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

	// FilterMediumInputThreshold is the input size threshold for synchronous chunked processing.
	// Inputs between FastPathThreshold and this value use synchronous chunked processing.
	// Inputs larger than this use async processing with timeout protection.
	FilterMediumInputThreshold = 100 * FastPathThreshold // 10KB

	// FilterDirectProcessThreshold is the maximum input size for direct processing
	// without chunking during timeout-protected filtering.
	FilterDirectProcessThreshold = 32 * 1024 // 32KB

	// FilterChunkSize is the size of chunks for processing large inputs.
	FilterChunkSize = 4096 // 4KB
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

	// DefaultLoggerCloseDelay is the delay before closing an old logger
	// when SetDefault() is called with a new logger. This allows in-flight
	// log operations to complete before the old logger is closed.
	DefaultLoggerCloseDelay = 100 * time.Millisecond
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

// Rate limiting constants for DoS protection.
const (
	// DefaultMaxMessagesPerSecond is the default rate limit for log messages.
	// This prevents log flooding attacks while allowing reasonable throughput.
	DefaultMaxMessagesPerSecond = 10000

	// DefaultMaxBytesPerSecond is the default byte rate limit (10MB).
	// This prevents memory exhaustion from large log messages.
	DefaultMaxBytesPerSecond = 10 * 1024 * 1024

	// DefaultBurstSize is the default burst capacity for the token bucket.
	// Allows temporary spikes in log volume without dropping messages.
	DefaultBurstSize = 1000

	// DefaultSamplingRate is the default sampling rate when rate limiting.
	// Keeps 1 in 100 messages when rate limited.
	DefaultSamplingRate = 100
)
