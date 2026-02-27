package dd

import "time"

// LogFormat defines the output format for log messages.
type LogFormat int8

const (
	FormatText LogFormat = iota
	FormatJSON
)

func (f LogFormat) String() string {
	switch f {
	case FormatText:
		return "text"
	case FormatJSON:
		return "json"
	default:
		return "unknown"
	}
}

const (
	DefaultCallerDepth = 3
)

const (
	DefaultBufferSize    = 1024
	MaxBufferSize        = 4 * 1024
	FieldBuilderCapacity = 256
	EstimatedFieldSize   = 24
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
	DefaultFilterTimeout = 50 * time.Millisecond
	EmptyFilterTimeout   = 10 * time.Millisecond
	// MaxConcurrentFilters limits concurrent regex filtering goroutines.
	MaxConcurrentFilters = 100
)

const (
	DefaultTimeFormat = "2006-01-02T15:04:05Z07:00"
	DevTimeFormat     = "15:04:05.000"
)

const (
	DefaultJSONIndent = "  "
	// FastPathThreshold is the input length threshold (bytes) below which
	// regex filtering skips chunking and timeout protection.
	// Small inputs are processed directly for better performance.
	FastPathThreshold = 100
)

const (
	// DebugVisualizationDepth is the caller depth for debug visualization functions.
	// Value of 2 means: 0 = current function, 1 = caller, 2 = caller's caller.
	DebugVisualizationDepth = 2
)
