package dd

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
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
			if needsQuoting(v) {
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
		case int64:
			sb.WriteString(strconv.FormatInt(v, 10))
		case float64:
			sb.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
		case bool:
			if v {
				sb.WriteString("true")
			} else {
				sb.WriteString("false")
			}
		case nil:
			sb.WriteString("<nil>")
		default:
			// For complex types (slices, maps, structs), use JSON formatting
			if isComplexFieldValue(v) {
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

func DebugWith(msg string, fields ...Field) { Default().LogWith(LevelDebug, msg, fields...) }
func InfoWith(msg string, fields ...Field)  { Default().LogWith(LevelInfo, msg, fields...) }
func WarnWith(msg string, fields ...Field)  { Default().LogWith(LevelWarn, msg, fields...) }
func ErrorWith(msg string, fields ...Field) { Default().LogWith(LevelError, msg, fields...) }
func FatalWith(msg string, fields ...Field) { Default().LogWith(LevelFatal, msg, fields...) }

// isComplexFieldValue checks if a field value is a complex type that should be JSON-formatted.
// This is used in formatFields to determine if a value needs JSON marshaling.
func isComplexFieldValue(v any) bool {
	if v == nil {
		return false
	}

	val := reflect.ValueOf(v)
	kind := val.Kind()

	// Handle pointers
	if kind == reflect.Ptr {
		if val.IsNil() {
			return false
		}
		val = val.Elem()
		kind = val.Kind()
	}

	// Check for complex types that benefit from JSON formatting
	switch kind {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		// Special case: time.Time and time.Duration have good String() methods
		if _, ok := v.(interface{ String() string }); ok {
			return false
		}
		return true
	default:
		return false
	}
}
