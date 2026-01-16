package dd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cybergodev/dd/internal"
)

const (
	// DebugVisualizationDepth is the caller depth for debug visualization functions.
	// Value of 2 means: 0 = current function, 1 = caller, 2 = caller's caller.
	DebugVisualizationDepth = 2
)

// Re-export fmt package functions for convenience.
var (
	Sprint       = fmt.Sprint
	Sprintln     = fmt.Sprintln
	Sprintf      = fmt.Sprintf
	Fprint       = fmt.Fprint
	Fprintln     = fmt.Fprintln
	Fprintf      = fmt.Fprintf
	Scan         = fmt.Scan
	Scanf        = fmt.Scanf
	Scanln       = fmt.Scanln
	Fscan        = fmt.Fscan
	Fscanf       = fmt.Fscanf
	Fscanln      = fmt.Fscanln
	Sscan        = fmt.Sscan
	Sscanf       = fmt.Sscanf
	Sscanln      = fmt.Sscanln
	Append       = fmt.Append
	Appendln     = fmt.Appendln
	AppendFormat = fmt.Appendf
)

// Print formats using the default formats and writes to stdout with caller info and newline.
// This is an alias for Println() - both methods add spaces between operands and append a newline.
func Print(args ...any) {
	caller := internal.GetCaller(DebugVisualizationDepth, false)
	if caller != "" {
		fmt.Fprintf(os.Stdout, "%s ", caller)
	}
	fmt.Fprintln(os.Stdout, args...)
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

func NewError(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func NewErrorWith(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	Default().Error(err.Error())
	return err
}

func PrintfWith(format string, args ...any) int {
	formatted := fmt.Sprintf(format, args...)
	n, err := fmt.Fprint(os.Stdout, formatted)
	if err != nil {
		fmt.Println(err)
	}
	Default().Info(strings.TrimSuffix(formatted, "\n"))
	return n
}

func PrintlnWith(args ...any) int {
	n, err := fmt.Fprintln(os.Stdout, args...)
	if err != nil {
		fmt.Println(err)
	}
	Default().Info(fmt.Sprint(args...))
	return n
}
