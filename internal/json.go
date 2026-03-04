package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
)

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

// jsonBuilderPool pools bytes.Buffer objects for fast JSON building.
var jsonBuilderPool = sync.Pool{
	New: func() any {
		buf := &bytes.Buffer{}
		buf.Grow(512)
		return buf
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

// FormatJSON formats a map as JSON using a fast path for simple types
// and falling back to encoding/json for complex types.
func FormatJSON(entry map[string]any, opts *JSONOptions) string {
	if opts == nil {
		opts = &JSONOptions{PrettyPrint: false, Indent: "  "}
	}

	// Use standard encoder for pretty print
	if opts.PrettyPrint {
		return formatJSONStandard(entry, opts)
	}

	// Try fast path for simple entries
	if result, ok := formatJSONFast(entry); ok {
		return result
	}

	// Fall back to standard encoder for complex entries
	return formatJSONStandard(entry, opts)
}

// formatJSONFast attempts to build JSON without reflection.
// Returns (json string, true) if successful, or ("", false) if fallback needed.
func formatJSONFast(entry map[string]any) (string, bool) {
	buf := jsonBuilderPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer jsonBuilderPool.Put(buf)

	buf.WriteByte('{')
	first := true

	for k, v := range entry {
		if !first {
			buf.WriteByte(',')
		}
		first = false

		// Write key
		writeJSONString(buf, k)
		buf.WriteByte(':')

		// Write value - fast path for common types
		if !writeJSONValueFast(buf, v) {
			return "", false // Need fallback for complex type
		}
	}

	buf.WriteByte('}')
	// Use bytesToString to avoid allocation in hot path
	// Note: The returned string shares memory with the buffer, but since
	// we return it immediately and the buffer is returned to pool after,
	// this is safe for the caller's immediate use.
	return bytesToString(buf.Bytes()), true
}

// bytesToString converts a byte slice to string without allocation.
// This is safe because the caller immediately uses the result and doesn't
// retain it beyond the log operation.
func bytesToString(b []byte) string {
	return string(b)
}

// writeJSONValueFast writes a JSON value without reflection for common types.
// Returns true if successful, false if the type needs standard encoding.
func writeJSONValueFast(buf *bytes.Buffer, v any) bool {
	switch val := v.(type) {
	case string:
		writeJSONString(buf, val)
		return true
	case int:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
		return true
	case int64:
		buf.WriteString(strconv.FormatInt(val, 10))
		return true
	case int32:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
		return true
	case int16:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
		return true
	case int8:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
		return true
	case uint:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
		return true
	case uint64:
		buf.WriteString(strconv.FormatUint(val, 10))
		return true
	case uint32:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
		return true
	case uint16:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
		return true
	case uint8:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
		return true
	case float64:
		buf.WriteString(strconv.FormatFloat(val, 'g', -1, 64))
		return true
	case float32:
		buf.WriteString(strconv.FormatFloat(float64(val), 'g', -1, 32))
		return true
	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return true
	case nil:
		buf.WriteString("null")
		return true
	case time.Time:
		writeJSONString(buf, val.Format(time.RFC3339))
		return true
	case time.Duration:
		writeJSONString(buf, val.String())
		return true
	case map[string]any:
		// Nested map - recurse
		buf.WriteByte('{')
		first := true
		for k2, v2 := range val {
			if !first {
				buf.WriteByte(',')
			}
			first = false
			writeJSONString(buf, k2)
			buf.WriteByte(':')
			if !writeJSONValueFast(buf, v2) {
				return false
			}
		}
		buf.WriteByte('}')
		return true
	default:
		// Complex type - need standard encoder
		return false
	}
}

// writeJSONString writes a JSON-escaped string.
func writeJSONString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if c < 0x20 {
				buf.WriteString(`\u00`)
				buf.WriteByte(hexChars[c>>4])
				buf.WriteByte(hexChars[c&0xf])
			} else {
				buf.WriteByte(c)
			}
		}
	}
	buf.WriteByte('"')
}

// hexChars lookup table for hex encoding
var hexChars = []byte("0123456789abcdef")

// formatJSONStandard uses the standard library encoder.
func formatJSONStandard(entry map[string]any, opts *JSONOptions) string {
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

	// Get bytes and convert to string
	// json.Encoder adds a trailing newline, remove it
	data := pe.buf.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}

	// Convert to string - this is the only allocation in the hot path
	return string(data)
}
