package dd

import (
	"fmt"
	"os"

	"github.com/cybergodev/dd/internal"
)

const (
	// DebugVisualizationDepth is the caller depth for debug visualization functions.
	// Value of 2 means: 0 = current function, 1 = caller, 2 = caller's caller.
	DebugVisualizationDepth = 2
)

// Re-export fmt package functions for convenience.
var (
	Sprint   = fmt.Sprint
	Sprintln = fmt.Sprintln
	Sprintf  = fmt.Sprintf
	Fprint   = fmt.Fprint
	Fprintln = fmt.Fprintln
	Fprintf  = fmt.Fprintf
	Scan     = fmt.Scan
	Scanf    = fmt.Scanf
	Scanln   = fmt.Scanln
	Fscan    = fmt.Fscan
	Fscanf   = fmt.Fscanf
	Fscanln  = fmt.Fscanln
	Sscan    = fmt.Sscan
	Sscanf   = fmt.Sscanf
	Sscanln  = fmt.Sscanln
	Append   = fmt.Append
	Appendln = fmt.Appendln
	Appendf  = fmt.Appendf
)

// Print formats using the default formats and writes to stdout with caller info and newline.
// This is a debug utility that always outputs to stdout, not through the logger's writers.
// For logging through configured writers, use logger.Print() or dd.Debug().
// Note: Both Print() and Println() behave identically because Println() adds a newline.
func Print(args ...any) {
	Println(args...)
}

// Println formats using the default formats and writes to stdout with caller info.
// Spaces are always added between operands and a newline is appended.
func Println(args ...any) {
	caller := internal.GetCaller(DebugVisualizationDepth, false)
	if caller != "" {
		fmt.Fprintf(os.Stdout, "%s ", caller)
	}
	fmt.Fprintln(os.Stdout, args...)
}

// Printf formats according to a format specifier and writes to stdout with caller info.
func Printf(format string, args ...any) {
	caller := internal.GetCaller(DebugVisualizationDepth, false)
	if caller != "" {
		fmt.Fprintf(os.Stdout, "%s ", caller)
	}
	fmt.Printf(format, args...)
}
