package dd

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/cybergodev/dd/internal"
)

type MessageFormatter struct {
	format        LogFormat
	timeFormat    string
	includeTime   bool
	includeLevel  bool
	fullPath      bool
	dynamicCaller bool
	jsonConfig    *JSONOptions
}

func newMessageFormatter(config *LoggerConfig) *MessageFormatter {
	return &MessageFormatter{
		format:        config.Format,
		timeFormat:    config.TimeFormat,
		includeTime:   config.IncludeTime,
		includeLevel:  config.IncludeLevel,
		fullPath:      config.FullPath,
		dynamicCaller: config.DynamicCaller,
		jsonConfig:    config.JSON,
	}
}

func (f *MessageFormatter) formatMessage(level LogLevel, callerDepth int, args ...any) string {
	// Join multiple arguments with spaces
	// Complex types (slices, maps, structs) are formatted as JSON for better readability
	var parts []string
	for _, arg := range args {
		if internal.IsComplexValue(arg) {
			// Use JSON formatting for complex types
			if jsonData, err := json.Marshal(convertValue(arg)); err == nil {
				parts = append(parts, string(jsonData))
			} else {
				// Fallback to fmt.Sprint if JSON formatting fails
				parts = append(parts, fmt.Sprint(arg))
			}
		} else {
			// Use simple formatting for basic types
			parts = append(parts, fmt.Sprint(arg))
		}
	}
	message := strings.Join(parts, " ")
	return f.formatWithMessage(level, callerDepth, message, nil)
}

func (f *MessageFormatter) formatMessageWith(level LogLevel, callerDepth int, msg string, fields []Field) string {
	return f.formatWithMessage(level, callerDepth, msg, fields)
}

func (f *MessageFormatter) formatWithMessage(level LogLevel, callerDepth int, message string, fields []Field) string {
	// Adjust caller depth if dynamic detection is enabled
	if f.dynamicCaller {
		callerDepth = f.adjustCallerDepth(callerDepth)
	}

	switch f.format {
	case FormatJSON:
		return f.formatJSON(level, callerDepth, message, fields)
	default:
		return f.formatText(level, callerDepth, message, fields)
	}
}

func (f *MessageFormatter) formatText(level LogLevel, callerDepth int, message string, fields []Field) string {
	var buf strings.Builder

	// Add timestamp and level with brackets
	if f.includeTime || f.includeLevel {
		buf.WriteByte('[')

		// Add timestamp
		if f.includeTime {
			buf.WriteString(time.Now().Format(f.timeFormat))
		}

		// Add level with alignment (5 character width, left-padded with spaces)
		if f.includeLevel {
			if f.includeTime {
				buf.WriteString(" ") // Two spaces before level for alignment
			}
			levelStr := level.String()
			// Pad to 5 characters for alignment (DEBUG, INFO, WARN, ERROR, FATAL)
			// Shorter levels (INFO, WARN) get leading spaces
			for i := len(levelStr); i < 5; i++ {
				buf.WriteByte(' ')
			}
			buf.WriteString(levelStr)
		}

		buf.WriteByte(']')
	}

	// Add caller
	if f.dynamicCaller {
		if callerInfo := internal.GetCaller(callerDepth, f.fullPath); callerInfo != "" {
			if buf.Len() > 0 {
				buf.WriteByte(' ')
			}
			buf.WriteString(callerInfo)
		}
	}

	// Add message
	if buf.Len() > 0 {
		buf.WriteByte(' ')
	}
	buf.WriteString(message)

	// Add fields
	if len(fields) > 0 {
		if fieldsStr := formatFields(fields); fieldsStr != "" {
			buf.WriteByte(' ')
			buf.WriteString(fieldsStr)
		}
	}

	return buf.String()
}

func (f *MessageFormatter) formatJSON(level LogLevel, callerDepth int, message string, fields []Field) string {
	fieldNames := f.getJSONFieldNames()

	// Pre-calculate capacity for better performance
	capacity := 1 // message always included
	if f.includeTime {
		capacity++
	}
	if f.includeLevel {
		capacity++
	}
	if f.dynamicCaller {
		capacity++
	}
	if len(fields) > 0 {
		capacity++
	}

	entry := make(map[string]any, capacity)

	// Add timestamp if enabled
	if f.includeTime {
		entry[fieldNames.Timestamp] = time.Now().Format(f.timeFormat)
	}

	// Add level if enabled
	if f.includeLevel {
		entry[fieldNames.Level] = level.String()
	}

	// Add caller if enabled
	if f.dynamicCaller {
		if callerInfo := internal.GetCaller(callerDepth, f.fullPath); callerInfo != "" {
			entry[fieldNames.Caller] = callerInfo
		}
	}

	// Add message
	entry[fieldNames.Message] = message

	// Add structured fields if present
	if len(fields) > 0 {
		fieldsMap := make(map[string]any, len(fields))
		for _, field := range fields {
			fieldsMap[field.Key] = field.Value
		}
		entry[fieldNames.Fields] = fieldsMap
	}

	return internal.FormatJSON(entry, f.getJSONOptions())
}

// getJSONFieldNames returns the JSON field names configuration (thread-safe)
func (f *MessageFormatter) getJSONFieldNames() *JSONFieldNames {
	if f.jsonConfig != nil && f.jsonConfig.FieldNames != nil {
		return internal.MergeWithDefaults((*internal.JSONFieldNames)(f.jsonConfig.FieldNames))
	}
	return internal.DefaultJSONFieldNames()
}

// getJSONOptions returns the JSON formatting options (thread-safe)
func (f *MessageFormatter) getJSONOptions() *internal.JSONOptions {
	if f.jsonConfig == nil {
		return &internal.JSONOptions{
			PrettyPrint: false,
			Indent:      DefaultJSONIndent,
		}
	}

	return &internal.JSONOptions{
		PrettyPrint: f.jsonConfig.PrettyPrint,
		Indent:      f.jsonConfig.Indent,
	}
}

// adjustCallerDepth adjusts the caller depth based on dynamic caller detection.
// This method looks for the first non-dd package in the call stack.
// Returns the depth relative to formatText (not relative to this function).
func (f *MessageFormatter) adjustCallerDepth(baseDepth int) int {
	// Validate base depth
	if baseDepth < 0 {
		baseDepth = 0
	}

	// Find the first non-dd package
	var userCodeDepth int

	maxDepth := baseDepth + 20
	for depth := baseDepth; depth < maxDepth; depth++ {
		pc, _, _, ok := runtime.Caller(depth)
		if !ok {
			break // No more stack frames
		}

		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}

		pkgName := fn.Name()

		// Check if caller is outside the dd package
		isDDPackage := strings.HasPrefix(pkgName, "github.com/cybergodev/dd.") ||
			strings.HasPrefix(pkgName, "github.com/cybergodev/dd/") ||
			strings.HasPrefix(pkgName, "github.com/cybergodev/dd(") ||
			pkgName == "github.com/cybergodev/dd"

		if !isDDPackage && userCodeDepth == 0 {
			userCodeDepth = depth
			break // Found user code, no need to continue
		}
	}

	if userCodeDepth == 0 {
		return baseDepth
	}

	// The userCodeDepth is relative to adjustCallerDepth's call stack.
	// When formatText calls GetCaller, the call stack is slightly different because GetCaller adds 1 frame.
	// We need to add 1 to account for the GetCaller frame.
	adjustedDepth := userCodeDepth + 1
	if adjustedDepth < 0 {
		adjustedDepth = 0
	}

	return adjustedDepth
}
