package dd

import "errors"

var (
	ErrNilConfig          = errors.New("config cannot be nil")
	ErrNilWriter          = errors.New("writer cannot be nil")
	ErrNilFilter          = errors.New("filter cannot be nil")
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
