package internal

import (
	"reflect"
)

type LogLevel int8

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

func (l LogLevel) IsValid() bool {
	return l >= LevelDebug && l <= LevelFatal
}

type JSONFieldNames struct {
	Timestamp string
	Level     string
	Caller    string
	Message   string
	Fields    string
}

func (j *JSONFieldNames) IsComplete() bool {
	return j != nil &&
		j.Timestamp != "" &&
		j.Level != "" &&
		j.Caller != "" &&
		j.Message != "" &&
		j.Fields != ""
}

type JSONOptions struct {
	PrettyPrint bool
	Indent      string
	FieldNames  *JSONFieldNames
}

// IsComplexValue checks if a field value is a complex type that should be JSON-formatted.
// This is used to determine if a value needs JSON marshaling in structured logging.
func IsComplexValue(v any) bool {
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
