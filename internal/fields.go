package internal

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Field represents a structured log field.
// This is the internal representation used by the formatter.
type Field struct {
	Key   string
	Value any
}

// Constants for field formatting
const (
	// FieldBuilderCapacity is the initial capacity for field builder
	FieldBuilderCapacity = 256
	// EstimatedFieldSize is the estimated size per field in bytes
	EstimatedFieldSize = 24
)

// fieldPool pools strings.Builder objects for field formatting
// to reduce memory allocations during high-frequency logging.
var fieldPool = sync.Pool{
	New: func() any {
		sb := &strings.Builder{}
		sb.Grow(FieldBuilderCapacity)
		return sb
	},
}

// FormatFields formats structured fields into a string representation.
// Uses a sync.Pool for strings.Builder to reduce allocations.
func FormatFields(fields []Field) string {
	fieldCount := len(fields)
	if fieldCount == 0 {
		return ""
	}

	sb := fieldPool.Get().(*strings.Builder)
	sb.Reset()
	defer fieldPool.Put(sb)

	estimatedSize := fieldCount * EstimatedFieldSize
	if sb.Cap() < estimatedSize {
		sb.Grow(estimatedSize - sb.Cap())
	}

	written := false

	for _, field := range fields {
		if field.Key == "" {
			continue
		}

		if written {
			sb.WriteByte(' ')
		}
		written = true

		sb.WriteString(field.Key)
		sb.WriteByte('=')

		switch v := field.Value.(type) {
		case string:
			if NeedsQuoting(v) {
				sb.WriteByte('"')
				vLen := len(v)
				for j := 0; j < vLen; j++ {
					c := v[j]
					if c == '"' || c == '\\' {
						sb.WriteByte('\\')
					}
					sb.WriteByte(c)
				}
				sb.WriteByte('"')
			} else {
				sb.WriteString(v)
			}
		case int:
			sb.WriteString(strconv.FormatInt(int64(v), 10))
		case int8:
			sb.WriteString(strconv.FormatInt(int64(v), 10))
		case int16:
			sb.WriteString(strconv.FormatInt(int64(v), 10))
		case int32:
			sb.WriteString(strconv.FormatInt(int64(v), 10))
		case int64:
			sb.WriteString(strconv.FormatInt(v, 10))
		case uint:
			sb.WriteString(strconv.FormatUint(uint64(v), 10))
		case uint8:
			sb.WriteString(strconv.FormatUint(uint64(v), 10))
		case uint16:
			sb.WriteString(strconv.FormatUint(uint64(v), 10))
		case uint32:
			sb.WriteString(strconv.FormatUint(uint64(v), 10))
		case uint64:
			sb.WriteString(strconv.FormatUint(v, 10))
		case float32:
			sb.WriteString(strconv.FormatFloat(float64(v), 'g', -1, 32))
		case float64:
			sb.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
		case bool:
			if v {
				sb.WriteString("true")
			} else {
				sb.WriteString("false")
			}
		case time.Duration:
			sb.WriteString(v.String())
		case time.Time:
			sb.WriteString(v.Format(time.RFC3339))
		case nil:
			sb.WriteString("<nil>")
		default:
			if IsComplexValue(v) {
				if jsonData, err := json.Marshal(v); err == nil {
					sb.Write(jsonData)
				} else {
					sb.WriteString(fmt.Sprintf("%v", v))
				}
			} else {
				sb.WriteString(fmt.Sprintf("%v", v))
			}
		}
	}

	return sb.String()
}

// NeedsQuoting checks if a string needs to be quoted in log output.
// Strings containing spaces, special characters, or control characters need quoting.
func NeedsQuoting(s string) bool {
	if len(s) == 0 {
		return true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= ' ' || c == '"' || c == '\\' {
			return true
		}
	}
	return false
}
