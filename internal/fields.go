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
	EstimatedFieldSize = 32
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

	for i, field := range fields {
		if field.Key == "" {
			continue
		}

		if i > 0 {
			sb.WriteByte(' ')
		}

		sb.WriteString(field.Key)
		sb.WriteByte('=')

		formatFieldValue(sb, field.Value)
	}

	return sb.String()
}

// formatFieldValue formats a single field value to the builder.
// This is separated to allow for better inlining and reduce code complexity.
func formatFieldValue(sb *strings.Builder, v any) {
	switch val := v.(type) {
	case string:
		if NeedsQuoting(val) {
			sb.WriteByte('"')
			for j := 0; j < len(val); j++ {
				c := val[j]
				if c == '"' || c == '\\' {
					sb.WriteByte('\\')
				}
				sb.WriteByte(c)
			}
			sb.WriteByte('"')
		} else {
			sb.WriteString(val)
		}
	case int:
		sb.WriteString(strconv.FormatInt(int64(val), 10))
	case int64:
		sb.WriteString(strconv.FormatInt(val, 10))
	case int32:
		sb.WriteString(strconv.FormatInt(int64(val), 10))
	case int16:
		sb.WriteString(strconv.FormatInt(int64(val), 10))
	case int8:
		sb.WriteString(strconv.FormatInt(int64(val), 10))
	case uint:
		sb.WriteString(strconv.FormatUint(uint64(val), 10))
	case uint64:
		sb.WriteString(strconv.FormatUint(val, 10))
	case uint32:
		sb.WriteString(strconv.FormatUint(uint64(val), 10))
	case uint16:
		sb.WriteString(strconv.FormatUint(uint64(val), 10))
	case uint8:
		sb.WriteString(strconv.FormatUint(uint64(val), 10))
	case float64:
		sb.WriteString(strconv.FormatFloat(val, 'g', -1, 64))
	case float32:
		sb.WriteString(strconv.FormatFloat(float64(val), 'g', -1, 32))
	case bool:
		if val {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case time.Duration:
		sb.WriteString(val.String())
	case time.Time:
		sb.WriteString(val.Format(time.RFC3339))
	case nil:
		sb.WriteString("<nil>")
	default:
		if IsComplexValue(v) {
			if jsonData, err := json.Marshal(v); err == nil {
				sb.Write(jsonData)
			} else {
				fmt.Fprint(sb, v)
			}
		} else {
			fmt.Fprint(sb, v)
		}
	}
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
