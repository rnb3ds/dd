package dd

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybergodev/dd/internal"
)

// Cached default JSON options to avoid repeated allocations
var defaultJSONOptions = &internal.JSONOptions{
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

	// Slow path: create new entry and atomically swap
	// Multiple goroutines may race here, but that's acceptable -
	// they'll all create the same formatted time for the same second
	newEntry := &cachedTimeEntry{
		sec:       currentSec,
		formatted: now.Format(tc.timeFormat),
	}

	// Compare-and-swap to avoid unnecessary updates
	// If another goroutine already updated to the same second, use their value
	if cached == nil || cached.sec != currentSec {
		tc.current.Store(newEntry)
		return newEntry.formatted
	}

	// Another goroutine won the race, use their cached value
	return cached.formatted
}

type MessageFormatter struct {
	format        LogFormat
	timeFormat    string
	includeTime   bool
	includeLevel  bool
	fullPath      bool
	dynamicCaller bool
	// Cached JSON options to avoid repeated allocations
	jsonOpts *internal.JSONOptions
	// Cached merged field names to avoid allocations during logging
	cachedFieldNames *internal.JSONFieldNames
	// Time cache for reducing time formatting overhead
	timeCache *timeCache
}

func newMessageFormatter(config *LoggerConfig) *MessageFormatter {
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
		mf.jsonOpts = &internal.JSONOptions{
			PrettyPrint: config.JSON.PrettyPrint,
			Indent:      config.JSON.Indent,
			FieldNames:  config.JSON.FieldNames,
		}
		// Pre-merge field names at creation time
		mf.cachedFieldNames = internal.MergeWithDefaults(config.JSON.FieldNames)
	} else {
		mf.jsonOpts = defaultJSONOptions
		// Use default field names when no JSON config provided
		mf.cachedFieldNames = internal.DefaultJSONFieldNames()
	}

	return mf
}

// formatArgsToString converts arguments to a single string for filtering.
// Complex types (slices, maps, structs) are formatted as JSON for better readability.
func (f *MessageFormatter) formatArgsToString(args ...any) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return f.formatArgToString(args[0])
	}

	var parts []string
	for _, arg := range args {
		parts = append(parts, f.formatArgToString(arg))
	}
	return strings.Join(parts, " ")
}

// formatArgToString converts a single argument to string.
func (f *MessageFormatter) formatArgToString(arg any) string {
	if internal.IsComplexValue(arg) {
		if jsonData, err := json.Marshal(convertValue(arg)); err == nil {
			return string(jsonData)
		}
	}
	return fmt.Sprint(arg)
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

	result := buf.String()

	// Return builder to pool
	textBuilderPool.Put(buf)

	return result
}

func (f *MessageFormatter) formatJSON(level LogLevel, callerDepth int, message string, fields []Field) string {
	fieldNames := f.getJSONFieldNames()

	// Create new map directly - for typical log entries (4-5 fields),
	// this is faster than pool + clear due to:
	// 1. No pool get/put overhead
	// 2. No clear loop overhead
	// 3. Better cache locality with exact size
	// Pre-allocate with expected capacity to avoid growth
	entry := make(map[string]any, 8)

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
		if callerInfo := internal.GetCaller(callerDepth, f.fullPath); callerInfo != "" {
			entry[fieldNames.Caller] = callerInfo
		}
	}

	// Add message
	entry[fieldNames.Message] = message

	// Add structured fields if present
	if len(fields) > 0 {
		// Create new fields map with exact capacity
		fieldsMap := make(map[string]any, len(fields))
		for _, field := range fields {
			fieldsMap[field.Key] = field.Value
		}
		entry[fieldNames.Fields] = fieldsMap
	}

	// Format JSON
	result := internal.FormatJSON(entry, f.getJSONOptions())

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
	return internal.DefaultJSONFieldNames()
}

// getJSONOptions returns the JSON formatting options (cached at initialization)
func (f *MessageFormatter) getJSONOptions() *internal.JSONOptions {
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
