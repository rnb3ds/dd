package internal

import (
	"path/filepath"
	"runtime"
	"strconv"
)

// GetCaller returns the caller information at the specified depth.
// Returns empty string if caller information is not available.
// callerDepth: stack depth (0 = caller of GetCaller, 1 = caller's caller, etc.)
// fullPath: if true, returns full file path; if false, returns only filename
func GetCaller(callerDepth int, fullPath bool) string {
	// Normalize negative depth to 0
	if callerDepth < 0 {
		callerDepth = 0
	}

	// Get caller information from runtime
	_, file, line, ok := runtime.Caller(callerDepth)
	if !ok {
		return ""
	}

	// Extract filename if full path not requested
	if !fullPath {
		file = filepath.Base(file)
	}

	// Optimize: use byte slice to avoid fmt.Sprintf allocation
	// Calculate accurate capacity: filename + ":" + max 10 digits for line number
	result := make([]byte, 0, len(file)+11)
	result = append(result, file...)
	result = append(result, ':')
	result = strconv.AppendInt(result, int64(line), 10)

	return string(result)
}
