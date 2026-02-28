package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// jsonBufPool pools bytes.Buffer objects for JSON encoding
// to reduce memory allocations during high-frequency logging.
var jsonBufPool = sync.Pool{
	New: func() any {
		buf := &bytes.Buffer{}
		buf.Grow(512) // typical JSON log entry size
		return buf
	},
}

// jsonEncoderPool pools json.Encoder objects for JSON encoding.
// Each encoder is paired with a buffer and reused across calls.
var jsonEncoderPool = sync.Pool{
	New: func() any {
		buf := &bytes.Buffer{}
		buf.Grow(512)
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		return &pooledEncoder{buf: buf, enc: enc}
	},
}

// pooledEncoder holds a buffer and encoder pair for reuse.
type pooledEncoder struct {
	buf *bytes.Buffer
	enc *json.Encoder
}

// File system and retry configuration constants.
const (
	// FilePermissions is the permission mode for creating files (rw-------).
	// Only the owner has read and write permissions. This is more restrictive
	// than DirPermissions (0700) because files don't need execute permission.
	FilePermissions = 0600
	// RetryAttempts is the number of times to retry file operations before giving up.
	RetryAttempts = 3
	// RetryDelay is the duration to wait between retry attempts.
	RetryDelay = 10 * time.Millisecond
)

func DefaultJSONFieldNames() *JSONFieldNames {
	return &JSONFieldNames{
		Timestamp: "timestamp",
		Level:     "level",
		Caller:    "caller",
		Message:   "message",
		Fields:    "fields",
	}
}

func MergeWithDefaults(f *JSONFieldNames) *JSONFieldNames {
	if f == nil {
		return DefaultJSONFieldNames()
	}

	if f.IsComplete() {
		return f
	}

	result := &JSONFieldNames{
		Timestamp: f.Timestamp,
		Level:     f.Level,
		Caller:    f.Caller,
		Message:   f.Message,
		Fields:    f.Fields,
	}

	defaults := DefaultJSONFieldNames()
	if result.Timestamp == "" {
		result.Timestamp = defaults.Timestamp
	}
	if result.Level == "" {
		result.Level = defaults.Level
	}
	if result.Caller == "" {
		result.Caller = defaults.Caller
	}
	if result.Message == "" {
		result.Message = defaults.Message
	}
	if result.Fields == "" {
		result.Fields = defaults.Fields
	}

	return result
}

func FormatJSON(entry map[string]any, opts *JSONOptions) string {
	if opts == nil {
		opts = &JSONOptions{PrettyPrint: false, Indent: "  "}
	}

	// Use pooled encoder (includes buffer) for better performance
	pe := jsonEncoderPool.Get().(*pooledEncoder)
	pe.buf.Reset()
	defer jsonEncoderPool.Put(pe)

	// Reset encoder settings (escape HTML is already false from pool init)
	if opts.PrettyPrint {
		pe.enc.SetIndent("", opts.Indent)
	} else {
		pe.enc.SetIndent("", "") // Reset indent for non-pretty mode
	}

	if err := pe.enc.Encode(entry); err != nil {
		return fmt.Sprintf(`{"error":"json marshal failed: %v"}`, err)
	}

	// json.Encoder adds a trailing newline, remove it
	result := pe.buf.String()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result
}
