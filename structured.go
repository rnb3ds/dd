package dd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cybergodev/dd/internal"
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

func Int8(key string, value int8) Field {
	return Field{Key: key, Value: value}
}

func Int16(key string, value int16) Field {
	return Field{Key: key, Value: value}
}

func Int32(key string, value int32) Field {
	return Field{Key: key, Value: value}
}

func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

func Uint(key string, value uint) Field {
	return Field{Key: key, Value: value}
}

func Uint8(key string, value uint8) Field {
	return Field{Key: key, Value: value}
}

func Uint16(key string, value uint16) Field {
	return Field{Key: key, Value: value}
}

func Uint32(key string, value uint32) Field {
	return Field{Key: key, Value: value}
}

func Uint64(key string, value uint64) Field {
	return Field{Key: key, Value: value}
}

func Float32(key string, value float32) Field {
	return Field{Key: key, Value: value}
}

func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

func Time(key string, value time.Time) Field {
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
			if internal.IsComplexValue(v) {
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
