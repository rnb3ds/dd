package dd

import (
	"fmt"
	"strings"
	"sync"
)

type Field struct {
	Key   string
	Value any
}

func Any(key string, value any) Field {
	return Field{Key: key, Value: value}
}

func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

func Err(err error) Field {
	if err == nil {
		return Field{Key: "error", Value: nil}
	}
	return Field{Key: "error", Value: err.Error()}
}

var (
	fieldPool = sync.Pool{
		New: func() any {
			sb := &strings.Builder{}
			sb.Grow(FieldBuilderCapacity)
			return sb
		},
	}
)

func formatFields(fields []Field) string {
	fieldCount := len(fields)
	if fieldCount == 0 {
		return ""
	}

	sb := fieldPool.Get().(*strings.Builder)
	sb.Reset()
	defer fieldPool.Put(sb)

	// Pre-allocate capacity to reduce reallocations
	estimatedSize := fieldCount * EstimatedFieldSize
	if sb.Cap() < estimatedSize {
		sb.Grow(estimatedSize - sb.Cap())
	}

	// Track if we've written any fields (for spacing)
	written := false

	// Optimized field formatting with type switch
	for _, field := range fields {
		// Skip empty keys
		if field.Key == "" {
			continue
		}

		if written {
			sb.WriteByte(' ')
		}
		written = true

		sb.WriteString(field.Key)
		sb.WriteByte('=')

		// Optimized type handling
		switch v := field.Value.(type) {
		case string:
			if needsQuoting(v) {
				sb.WriteByte('"')
				// Simple escaping to avoid fmt.Fprintf overhead
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
			sb.WriteString(fmt.Sprintf("%d", v))
		case int64:
			sb.WriteString(fmt.Sprintf("%d", v))
		case float64:
			sb.WriteString(fmt.Sprintf("%g", v))
		case bool:
			if v {
				sb.WriteString("true")
			} else {
				sb.WriteString("false")
			}
		case nil:
			sb.WriteString("<nil>")
		default:
			sb.WriteString(fmt.Sprintf("%v", v))
		}
	}

	return sb.String()
}

// Fast check if string needs quoting
func needsQuoting(s string) bool {
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

// Package-level convenience functions for structured logging
func DebugWith(msg string, fields ...Field) { Default().LogWith(LevelDebug, msg, fields...) }
func InfoWith(msg string, fields ...Field)  { Default().LogWith(LevelInfo, msg, fields...) }
func WarnWith(msg string, fields ...Field)  { Default().LogWith(LevelWarn, msg, fields...) }
func ErrorWith(msg string, fields ...Field) { Default().LogWith(LevelError, msg, fields...) }
func FatalWith(msg string, fields ...Field) { Default().LogWith(LevelFatal, msg, fields...) }
