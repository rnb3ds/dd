package internal

import (
	"fmt"
	"reflect"
	"time"
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
// Uses type switch fast paths to avoid reflection for common types.
func IsComplexValue(v any) bool {
	if v == nil {
		return false
	}

	// Fast path: type switch for common simple types (avoids reflection)
	switch v := v.(type) {
	// Primitive types - not complex
	case string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64,
		complex64, complex128:
		return false

	// Time types have good String() methods
	case time.Time, time.Duration:
		return false

	// Types with String() method - use their formatting
	case interface{ String() string }:
		return false

	// Error interface - use Error() method
	case error:
		return false

	// fmt.Stringer interface - use String() method
	case fmt.Stringer:
		return false

	// Pointer types - dereference and check
	case *string, *bool,
		*int, *int8, *int16, *int32, *int64,
		*uint, *uint8, *uint16, *uint32, *uint64,
		*float32, *float64:
		return false

	case *time.Time:
		return false

	case *time.Duration:
		return false

	// Pointer to Stringer - use String() method
	case *interface{ String() string }:
		return false

	// nil pointer check for interface types
	case *error:
		if v == nil {
			return false
		}
		return IsComplexValue(*v)

	case *fmt.Stringer:
		if v == nil {
			return false
		}
		return IsComplexValue(*v)

	// byte slice is common and can be represented as hex/base64
	case []byte:
		return true

		// For all other types, fall through to reflection
	}

	// Slow path: use reflection for unknown types
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
		// Special case: types with String() method
		if _, ok := v.(interface{ String() string }); ok {
			return false
		}
		return true
	default:
		return false
	}
}
