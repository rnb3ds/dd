package internal

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Cached default JSON options to avoid repeated allocations
var defaultJSONOptions = &JSONOptions{
	PrettyPrint: false,
	Indent:      DefaultJSONIndent,
}

// textBuilderPool pools strings.Builder objects for text formatting
// to reduce memory allocations during high-frequency logging.
var textBuilderPool = sync.Pool{
	New: func() any {
		var buf strings.Builder
		buf.Grow(256) // typical log message size
		return &buf
	},
}

// argsBuilderPool pools strings.Builder objects for argument concatenation
// to reduce memory allocations when formatting multiple arguments.
var argsBuilderPool = sync.Pool{
	New: func() any {
		var buf strings.Builder
		buf.Grow(128) // typical args concatenation size
		return &buf
	},
}

// jsonEntryMapPool pools map[string]any objects for JSON formatting
// to reduce memory allocations during high-frequency JSON logging.
var jsonEntryMapPool = sync.Pool{
	New: func() any {
		m := make(map[string]any, 8)
		return &m
	},
}

// jsonFieldsMapPool pools map[string]any objects for JSON fields
// to reduce memory allocations when logging with structured fields.
var jsonFieldsMapPool = sync.Pool{
	New: func() any {
		m := make(map[string]any, 4)
		return &m
	},
}

// cachedTimeEntry stores a single cached timestamp entry
type cachedTimeEntry struct {
	sec       int64  // Unix timestamp in seconds
	formatted string // Cached formatted timestamp
}

// timeCache stores cached formatted timestamp for high-frequency logging.
// Uses atomic pointer for lock-free reads with better cache locality.
// Caches the formatted string within the same second to reduce time formatting overhead.
type timeCache struct {
	current    atomic.Pointer[cachedTimeEntry] // Atomic pointer to current cache entry
	timeFormat string                          // Time format string (immutable after creation)
}

// newTimeCache creates a new time cache with the given format
func newTimeCache(timeFormat string) *timeCache {
	tc := &timeCache{
		timeFormat: timeFormat,
	}
	// Initialize with zero entry to avoid nil checks
	tc.current.Store(&cachedTimeEntry{sec: -1, formatted: ""})
	return tc
}

// getFormattedTime returns the formatted current time.
// Uses lock-free atomic operations for better concurrency performance.
// Cache hit path is completely lock-free with no mutex contention.
func (tc *timeCache) getFormattedTime() string {
	now := time.Now()
	currentSec := now.Unix()

	// Fast path: atomic load to check cache (completely lock-free)
	cached := tc.current.Load()
	if cached != nil && cached.sec == currentSec {
		return cached.formatted
	}

	// Slow path: format time and atomically swap
	// Multiple goroutines may race here, but that's acceptable -
	// they'll all create the same formatted time for the same second
	formatted := now.Format(tc.timeFormat)

	// Only update if the cache is stale (another goroutine may have updated it)
	// Use Store directly since the value is the same regardless of who wins the race
	tc.current.Store(&cachedTimeEntry{
		sec:       currentSec,
		formatted: formatted,
	})

	return formatted
}

// FormatterConfig holds the configuration for creating a MessageFormatter.
// This is used to pass configuration from the root package without importing it.
type FormatterConfig struct {
	Format        LogFormat
	TimeFormat    string
	IncludeTime   bool
	IncludeLevel  bool
	FullPath      bool
	DynamicCaller bool
	JSON          *JSONOptions
}

// MessageFormatter handles formatting of log messages.
// It supports both text and JSON formats and caches resources for performance.
type MessageFormatter struct {
	format        LogFormat
	timeFormat    string
	includeTime   bool
	includeLevel  bool
	fullPath      bool
	dynamicCaller bool
	// Cached JSON options to avoid repeated allocations
	jsonOpts *JSONOptions
	// Cached merged field names to avoid allocations during logging
	cachedFieldNames *JSONFieldNames
	// Time cache for reducing time formatting overhead
	timeCache *timeCache
}

// NewMessageFormatter creates a new MessageFormatter with the given configuration.
func NewMessageFormatter(config *FormatterConfig) *MessageFormatter {
	mf := &MessageFormatter{
		format:        config.Format,
		timeFormat:    config.TimeFormat,
		includeTime:   config.IncludeTime,
		includeLevel:  config.IncludeLevel,
		fullPath:      config.FullPath,
		dynamicCaller: config.DynamicCaller,
		timeCache:     newTimeCache(config.TimeFormat),
	}

	// Pre-compute JSON options to avoid allocations during logging
	if config.JSON != nil {
		mf.jsonOpts = &JSONOptions{
			PrettyPrint: config.JSON.PrettyPrint,
			Indent:      config.JSON.Indent,
			FieldNames:  config.JSON.FieldNames,
		}
		// Pre-merge field names at creation time
		mf.cachedFieldNames = MergeWithDefaults(config.JSON.FieldNames)
	} else {
		mf.jsonOpts = defaultJSONOptions
		// Use default field names when no JSON config provided
		mf.cachedFieldNames = DefaultJSONFieldNames()
	}

	return mf
}

// FormatArgsToString converts arguments to a single string for filtering.
// Complex types (slices, maps, structs) are formatted as JSON for better readability.
// Uses pooled strings.Builder to reduce allocations.
func (f *MessageFormatter) FormatArgsToString(args ...any) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return f.formatArgToString(args[0])
	}

	// Use pooled builder for multiple arguments
	sb := argsBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	defer argsBuilderPool.Put(sb)

	for i, arg := range args {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(f.formatArgToString(arg))
	}
	return sb.String()
}

// formatArgToString converts a single argument to string.
func (f *MessageFormatter) formatArgToString(arg any) string {
	if IsComplexValue(arg) {
		if jsonData, err := json.Marshal(ConvertValue(arg)); err == nil {
			return string(jsonData)
		}
	}
	return fmt.Sprint(arg)
}

// FormatWithMessage formats a complete log message with level, caller, and fields.
func (f *MessageFormatter) FormatWithMessage(level LogLevel, callerDepth int, message string, fields []Field) string {
	// Adjust caller depth if dynamic detection is enabled
	if f.dynamicCaller {
		callerDepth = f.adjustCallerDepth(callerDepth)
	}

	switch f.format {
	case LogFormatJSON:
		return f.formatJSON(level, callerDepth, message, fields)
	default:
		return f.formatText(level, callerDepth, message, fields)
	}
}

func (f *MessageFormatter) formatText(level LogLevel, callerDepth int, message string, fields []Field) string {
	// Pre-calculate capacity to reduce memory allocations
	// Base: timestamp (~35) + level (7) + brackets (2) + caller (~30) + message + fields
	estimatedLen := 64 + len(message) + len(fields)*EstimatedFieldSize

	// Get strings.Builder from pool
	buf := textBuilderPool.Get().(*strings.Builder)
	buf.Reset()
	// Grow if needed
	if buf.Cap() < estimatedLen {
		buf.Grow(estimatedLen - buf.Cap())
	}

	// Add timestamp and level with brackets
	if f.includeTime || f.includeLevel {
		buf.WriteByte('[')

		// Add timestamp (using cached time for performance)
		if f.includeTime {
			buf.WriteString(f.timeCache.getFormattedTime())
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
		if callerInfo := GetCaller(callerDepth, f.fullPath); callerInfo != "" {
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
		if fieldsStr := FormatFields(fields); fieldsStr != "" {
			buf.WriteByte(' ')
			buf.WriteString(fieldsStr)
		}
	}

	result := buf.String()

	// Return builder to pool
	textBuilderPool.Put(buf)

	return result
}

func (f *MessageFormatter) formatJSON(level LogLevel, callerDepth int, message string, fields []Field) string {
	fieldNames := f.getJSONFieldNames()

	// Use pooled entry map for better performance
	entryPtr := jsonEntryMapPool.Get().(*map[string]any)
	entry := *entryPtr
	// Clear the map for reuse
	for k := range entry {
		delete(entry, k)
	}

	// Add timestamp if enabled (using cached time for performance)
	if f.includeTime {
		entry[fieldNames.Timestamp] = f.timeCache.getFormattedTime()
	}

	// Add level if enabled
	if f.includeLevel {
		entry[fieldNames.Level] = level.String()
	}

	// Add caller if enabled
	if f.dynamicCaller {
		if callerInfo := GetCaller(callerDepth, f.fullPath); callerInfo != "" {
			entry[fieldNames.Caller] = callerInfo
		}
	}

	// Add message
	entry[fieldNames.Message] = message

	// Add structured fields if present
	if len(fields) > 0 {
		// Use pooled fields map
		fieldsMapPtr := jsonFieldsMapPool.Get().(*map[string]any)
		fieldsMap := *fieldsMapPtr
		// Clear the map for reuse
		for k := range fieldsMap {
			delete(fieldsMap, k)
		}
		for _, field := range fields {
			fieldsMap[field.Key] = field.Value
		}
		entry[fieldNames.Fields] = fieldsMap
	}

	// Format JSON
	result := FormatJSON(entry, f.getJSONOptions())

	// Clean up and return maps to pool
	if fields, ok := entry[fieldNames.Fields].(map[string]any); ok {
		// Return fields map to pool
		for k := range fields {
			delete(fields, k)
		}
		jsonFieldsMapPool.Put(&fields)
	}

	// Return entry map to pool
	for k := range entry {
		delete(entry, k)
	}
	jsonEntryMapPool.Put(entryPtr)

	return result
}

// getJSONFieldNames returns the cached JSON field names configuration.
// Field names are pre-merged at formatter creation time to avoid allocations.
func (f *MessageFormatter) getJSONFieldNames() *JSONFieldNames {
	// Return the pre-cached merged field names
	if f.cachedFieldNames != nil {
		return f.cachedFieldNames
	}
	// Fallback (should never happen if properly initialized)
	return DefaultJSONFieldNames()
}

// getJSONOptions returns the JSON formatting options (cached at initialization)
func (f *MessageFormatter) getJSONOptions() *JSONOptions {
	return f.jsonOpts
}

// adjustCallerDepth adjusts the caller depth based on dynamic caller detection.
// This method looks for the first non-dd package in the call stack.
// Returns the depth relative to formatText (not relative to this function).
//
// Performance note: This function iterates through the call stack (up to 20 frames)
// to find user code. The iteration stops as soon as non-dd package is found.
// For most use cases, this overhead is negligible (< 1µs). If performance is critical
// and caller information is not needed, consider disabling DynamicCaller in config.
func (f *MessageFormatter) adjustCallerDepth(baseDepth int) int {
	// Validate base depth
	if baseDepth < 0 {
		baseDepth = 0
	}

	// Find the first non-dd package
	// Use -1 to indicate "not found" since 0 is a valid depth value
	userCodeDepth := -1

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

		if !isDDPackage {
			userCodeDepth = depth
			break // Found user code, no need to continue
		}
	}

	if userCodeDepth < 0 {
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
