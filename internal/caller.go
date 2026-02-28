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

func GetCaller(callerDepth int, fullPath bool) string {
	if callerDepth < 0 {
		callerDepth = 0
	}

	_, file, line, ok := runtime.Caller(callerDepth)
	if !ok {
		return ""
	}

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
	sb.WriteString(strconv.FormatInt(int64(line), 10))

	result := sb.String()
	callerBuilderPool.Put(sb)
	return result
}
