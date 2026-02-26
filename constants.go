package dd

import "time"

// FilterLevel defines the level of sensitive data filtering.
//
// Deprecated: Use NewBasicSensitiveDataFilter() or NewSensitiveDataFilter() directly
// for more control over filtering behavior.
type FilterLevel int

const (
	// FilterNone disables all sensitive data filtering.
	FilterNone FilterLevel = iota
	// FilterBasic enables basic filtering for common sensitive data
	// (passwords, API keys, credit cards, phone numbers).
	FilterBasic
	// FilterFull enables comprehensive filtering including emails,
	// IP addresses, JWT tokens, and database connection strings.
	FilterFull
)

func (f FilterLevel) String() string {
	switch f {
	case FilterNone:
		return "none"
	case FilterBasic:
		return "basic"
	case FilterFull:
		return "full"
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

const (
	DirPermissions = 0700
)

const (
	DefaultFilterTimeout = 50 * time.Millisecond
	EmptyFilterTimeout   = 10 * time.Millisecond
)

const (
	DefaultLogFile = "logs/app.log"
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
