package internal

import (
	"encoding/json"
	"fmt"
	"time"
)

// defaultFieldNames provides the standard JSON field names.
var defaultFieldNames = &JSONFieldNames{
	Timestamp: "timestamp",
	Level:     "level",
	Caller:    "caller",
	Message:   "message",
	Fields:    "fields",
}

// DefaultJSONFieldNames returns the default JSON field names configuration.
func DefaultJSONFieldNames() *JSONFieldNames {
	return defaultFieldNames
}

// mergeWithDefaults fills in missing field names with defaults.
func mergeWithDefaults(f *JSONFieldNames) *JSONFieldNames {
	if f == nil {
		return defaultFieldNames
	}

	// Fast path: if all fields are set, return as-is
	if f.IsComplete() {
		return f
	}

	// Merge with defaults - inline coalesce logic for better performance
	result := &JSONFieldNames{
		Timestamp: f.Timestamp,
		Level:     f.Level,
		Caller:    f.Caller,
		Message:   f.Message,
		Fields:    f.Fields,
	}

	if result.Timestamp == "" {
		result.Timestamp = defaultFieldNames.Timestamp
	}
	if result.Level == "" {
		result.Level = defaultFieldNames.Level
	}
	if result.Caller == "" {
		result.Caller = defaultFieldNames.Caller
	}
	if result.Message == "" {
		result.Message = defaultFieldNames.Message
	}
	if result.Fields == "" {
		result.Fields = defaultFieldNames.Fields
	}

	return result
}

// FormatMessage formats a log message as JSON without structured fields.
// This function provides backward compatibility for callers that don't need custom options.
func FormatMessage(
	level LogLevel,
	includeTime bool,
	timeFormat string,
	includeLevel bool,
	includeCaller bool,
	callerDepth int,
	fullPath bool,
	message string,
) (string, error) {
	return FormatMessageWithOptions(
		level,
		includeTime,
		timeFormat,
		includeLevel,
		includeCaller,
		callerDepth,
		fullPath,
		message,
		nil,
		nil,
	)
}

// FormatMessageWithOptions formats a log message as JSON with custom options.
// This is the primary formatting function used by the logger.
func FormatMessageWithOptions(
	level LogLevel,
	includeTime bool,
	timeFormat string,
	includeLevel bool,
	includeCaller bool,
	callerDepth int,
	fullPath bool,
	message string,
	fields map[string]any,
	opts *JSONOptions,
) (string, error) {
	// Apply default options if not provided
	if opts == nil {
		opts = &JSONOptions{
			PrettyPrint: false,
			Indent:      "  ",
			FieldNames:  defaultFieldNames,
		}
	}

	// Merge field names with defaults
	fieldNames := mergeWithDefaults(opts.FieldNames)

	// Pre-calculate map capacity for optimal allocation
	capacity := 1 // message is always included
	if includeTime {
		capacity++
	}
	if includeLevel {
		capacity++
	}
	if includeCaller {
		capacity++
	}
	if len(fields) > 0 {
		capacity++
	}

	entry := make(map[string]any, capacity)

	// Add timestamp if enabled
	if includeTime {
		entry[fieldNames.Timestamp] = time.Now().Format(timeFormat)
	}

	// Add log level if enabled
	if includeLevel {
		entry[fieldNames.Level] = level.String()
	}

	// Add caller information if enabled
	if includeCaller {
		if callerInfo := GetCaller(callerDepth, fullPath); callerInfo != "" {
			entry[fieldNames.Caller] = callerInfo
		}
	}

	// Add message (always included)
	entry[fieldNames.Message] = message

	// Add structured fields if present
	if len(fields) > 0 {
		entry[fieldNames.Fields] = fields
	}

	// Marshal to JSON
	var data []byte
	var err error

	if opts.PrettyPrint {
		data, err = json.MarshalIndent(entry, "", opts.Indent)
	} else {
		data, err = json.Marshal(entry)
	}

	if err != nil {
		return "", fmt.Errorf("failed to marshal log entry: %w", err)
	}

	return string(data), nil
}

// FormatJSON formats a map as JSON with the given options.
// Returns a JSON error object if marshaling fails.
func FormatJSON(entry map[string]any, opts *JSONOptions) string {
	// Apply default options if not provided
	if opts == nil {
		opts = &JSONOptions{
			PrettyPrint: false,
			Indent:      "  ",
		}
	}

	// Marshal to JSON
	var data []byte
	var err error

	if opts.PrettyPrint {
		data, err = json.MarshalIndent(entry, "", opts.Indent)
	} else {
		data, err = json.Marshal(entry)
	}

	// Return error as JSON if marshaling fails
	if err != nil {
		return fmt.Sprintf(`{"error":"json marshal failed: %v"}`, err)
	}

	return string(data)
}
