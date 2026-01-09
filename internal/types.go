package internal

// LogLevel represents the severity level of a log message.
type LogLevel int8

// Log level constants define the severity hierarchy.
const (
	LevelDebug LogLevel = iota // Debug level for detailed diagnostic information
	LevelInfo                   // Info level for general informational messages
	LevelWarn                   // Warn level for warning messages
	LevelError                  // Error level for error messages
	LevelFatal                  // Fatal level for critical errors that require program termination
)

// String returns the string representation of the log level.
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

// IsValid returns true if the log level is within valid range.
func (l LogLevel) IsValid() bool {
	return l >= LevelDebug && l <= LevelFatal
}

// JSONFieldNames allows customization of JSON field names.
// All fields must be non-empty strings when used.
type JSONFieldNames struct {
	Timestamp string // Field name for timestamp (default: "timestamp")
	Level     string // Field name for log level (default: "level")
	Caller    string // Field name for caller info (default: "caller")
	Message   string // Field name for message (default: "message")
	Fields    string // Field name for structured fields (default: "fields")
}

// IsComplete returns true if all field names are non-empty.
func (j *JSONFieldNames) IsComplete() bool {
	return j != nil &&
		j.Timestamp != "" &&
		j.Level != "" &&
		j.Caller != "" &&
		j.Message != "" &&
		j.Fields != ""
}

// JSONOptions holds JSON-specific configuration options.
type JSONOptions struct {
	PrettyPrint bool            // Enable pretty-printed JSON output
	Indent      string          // Indentation string for pretty print (default: "  ")
	FieldNames  *JSONFieldNames // Custom field names (optional)
}
