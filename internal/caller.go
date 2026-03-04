package internal

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// callerBuilderPool pools strings.Builder objects for caller formatting
// to reduce memory allocations during high-frequency logging.
var callerBuilderPool = sync.Pool{
	New: func() any {
		var sb strings.Builder
		sb.Grow(64) // typical caller info size: "filename.go:12345"
		return &sb
	},
}

// GetCaller retrieves the caller information at the specified depth.
func GetCaller(callerDepth int, fullPath bool) string {
	if callerDepth < 0 {
		callerDepth = 0
	}

	_, file, line, ok := runtime.Caller(callerDepth)
	if !ok {
		return ""
	}

	return formatCaller(file, line, fullPath)
}

// formatCaller formats the caller file and line into a string.
// Uses pooled strings.Builder to reduce allocations.
func formatCaller(file string, line int, fullPath bool) string {
	if !fullPath {
		file = filepath.Base(file)
	}

	sb := callerBuilderPool.Get().(*strings.Builder)
	sb.Reset()

	// Grow if needed (file path can vary significantly)
	estimatedLen := len(file) + 11 // file + ":" + max line number (5 digits) + safety
	if sb.Cap() < estimatedLen {
		sb.Grow(estimatedLen - sb.Cap())
	}

	sb.WriteString(file)
	sb.WriteByte(':')
	sb.WriteString(FormatInt(line))

	result := sb.String()
	callerBuilderPool.Put(sb)
	return result
}

// smallInts caches string representations of small integers (0-99)
// to avoid repeated strconv.FormatInt calls for line numbers.
var smallInts = [100]string{
	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
	"10", "11", "12", "13", "14", "15", "16", "17", "18", "19",
	"20", "21", "22", "23", "24", "25", "26", "27", "28", "29",
	"30", "31", "32", "33", "34", "35", "36", "37", "38", "39",
	"40", "41", "42", "43", "44", "45", "46", "47", "48", "49",
	"50", "51", "52", "53", "54", "55", "56", "57", "58", "59",
	"60", "61", "62", "63", "64", "65", "66", "67", "68", "69",
	"70", "71", "72", "73", "74", "75", "76", "77", "78", "79",
	"80", "81", "82", "83", "84", "85", "86", "87", "88", "89",
	"90", "91", "92", "93", "94", "95", "96", "97", "98", "99",
}

// FormatInt formats an integer to string using cached values for small integers.
func FormatInt(n int) string {
	if n >= 0 && n < 100 {
		return smallInts[n]
	}
	return strconv.FormatInt(int64(n), 10)
}
