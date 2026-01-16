package internal

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	FilePermissions = 0600
	RetryAttempts   = 3
	RetryDelay      = 10 * time.Millisecond
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
) string {
	if opts == nil {
		opts = &JSONOptions{
			PrettyPrint: false,
			Indent:      "  ",
			FieldNames:  DefaultJSONFieldNames(),
		}
	}

	fieldNames := MergeWithDefaults(opts.FieldNames)

	capacity := 1
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

	if includeTime {
		entry[fieldNames.Timestamp] = time.Now().Format(timeFormat)
	}
	if includeLevel {
		entry[fieldNames.Level] = level.String()
	}
	if includeCaller {
		if callerInfo := GetCaller(callerDepth, fullPath); callerInfo != "" {
			entry[fieldNames.Caller] = callerInfo
		}
	}

	entry[fieldNames.Message] = message

	if len(fields) > 0 {
		entry[fieldNames.Fields] = fields
	}

	var data []byte
	var err error
	if opts.PrettyPrint {
		data, err = json.MarshalIndent(entry, "", opts.Indent)
	} else {
		data, err = json.Marshal(entry)
	}

	if err != nil {
		return message
	}

	return string(data)
}

func FormatJSON(entry map[string]any, opts *JSONOptions) string {
	if opts == nil {
		opts = &JSONOptions{PrettyPrint: false, Indent: "  "}
	}

	var data []byte
	var err error
	if opts.PrettyPrint {
		data, err = json.MarshalIndent(entry, "", opts.Indent)
	} else {
		data, err = json.Marshal(entry)
	}

	if err != nil {
		return fmt.Sprintf(`{"error":"json marshal failed: %v"}`, err)
	}

	return string(data)
}
