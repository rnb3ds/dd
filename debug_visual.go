package dd

// Debug Visualization Functions
//
// SECURITY WARNING: These functions output directly to stdout WITHOUT sensitive
// data filtering. For production logging, use Logger methods (Info, Debug, etc.).
// Never use these with passwords, tokens, or other sensitive data.

import (
	"fmt"
	"os"

	"github.com/cybergodev/dd/internal"
)

// JSON outputs data as compact JSON to stdout with caller info for debugging.
func JSON(data ...any) {
	internal.OutputJSON(os.Stdout, internal.GetCaller(DebugVisualizationDepth, false), data...)
}

// JSONF outputs formatted data as compact JSON to stdout with caller info for debugging.
func JSONF(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	internal.OutputJSON(os.Stdout, internal.GetCaller(DebugVisualizationDepth, false), formatted)
}

// Text outputs data as pretty-printed format to stdout for debugging.
func Text(data ...any) {
	internal.OutputTextData(os.Stdout, data...)
}

// Textf outputs formatted data as pretty-printed format to stdout for debugging.
func Textf(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stdout, formatted)
}

// Exit outputs data as pretty-printed JSON to stdout and exits with code 0.
func Exit(data ...any) {
	internal.OutputText(os.Stdout, internal.GetCaller(DebugVisualizationDepth, false), data...)
	os.Exit(0)
}

// Exitf outputs formatted data to stdout with caller info and exits with code 0.
func Exitf(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "%s %s\n", internal.GetCaller(DebugVisualizationDepth, false), formatted)
	os.Exit(0)
}
