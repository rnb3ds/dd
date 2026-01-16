package internal

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

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

	var sb strings.Builder
	sb.Grow(len(file) + 11)
	sb.WriteString(file)
	sb.WriteByte(':')
	sb.WriteString(strconv.FormatInt(int64(line), 10))

	return sb.String()
}
