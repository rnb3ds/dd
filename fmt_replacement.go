package dd

import (
	"fmt"
	"os"

	"github.com/cybergodev/dd/internal"
)

// fmt Package Function Re-exports
//
// These are direct aliases to fmt package functions (Sprint, Sprintf, Fprint, etc.).
// They do NOT include caller information or sensitive data filtering.
// For production logging with security filtering, use Logger methods.

var (
	Sprint   = fmt.Sprint
	Sprintln = fmt.Sprintln
	Sprintf  = fmt.Sprintf

	Fprint   = fmt.Fprint
	Fprintln = fmt.Fprintln
	Fprintf  = fmt.Fprintf

	Scan    = fmt.Scan
	Scanf   = fmt.Scanf
	Scanln  = fmt.Scanln
	Fscan   = fmt.Fscan
	Fscanf  = fmt.Fscanf
	Fscanln = fmt.Fscanln
	Sscan   = fmt.Sscan
	Sscanf  = fmt.Sscanf
	Sscanln = fmt.Sscanln

	Append   = fmt.Append
	Appendln = fmt.Appendln
	Appendf  = fmt.Appendf
)

// Debug Print Functions
//
// SECURITY WARNING: These functions output to stdout WITHOUT sensitive data filtering.
// Use logger.Print() or logger.Info() for production logging with security filtering.
//
// Comparison:
//
//	dd.Print()     -> stdout with caller info, NO filtering
//	logger.Print() -> configured writers with caller info, WITH filtering

// Print writes to stdout with caller info. For debugging only.
func Print(args ...any) {
	Println(args...)
}

// Println writes to stdout with caller info and spaces between operands.
func Println(args ...any) {
	caller := internal.GetCaller(DebugVisualizationDepth, false)
	if caller != "" {
		fmt.Fprintf(os.Stdout, "%s ", caller)
	}
	fmt.Fprintln(os.Stdout, args...)
}

// Printf writes formatted output to stdout with caller info.
func Printf(format string, args ...any) {
	caller := internal.GetCaller(DebugVisualizationDepth, false)
	if caller != "" {
		fmt.Fprintf(os.Stdout, "%s ", caller)
	}
	fmt.Printf(format, args...)
}
