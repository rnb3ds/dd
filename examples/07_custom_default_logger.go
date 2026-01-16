//go:build examples

package main

import (
	"fmt"

	"github.com/cybergodev/dd"
)

// Custom Default Logger - Configure package-level functions with custom settings
//
// This example demonstrates how to:
// 1. Create a custom logger with security filtering
// 2. Set it as the default logger
// 3. Use package-level functions with the custom configuration
func main() {
	fmt.Println("=== Custom Default Logger Example ===\n ")

	example1BasicCustomDefault()
	example2WithSecurityFiltering()
	example3DynamicConfiguration()

	fmt.Println("\n✅ Examples completed!")
	fmt.Println("\nKey Points:")
	fmt.Println("  • SetDefault() affects all package-level functions")
	fmt.Println("  • Call SetDefault() before using package-level functions")
	fmt.Println("  • Can change default logger at any time")
}

func example1BasicCustomDefault() {
	fmt.Println("Example 1: Basic Custom Default Logger")
	fmt.Println("---------------------------------------")

	// Create custom logger
	customLogger, _ := dd.NewWithOptions(dd.Options{
		Level:   dd.LevelDebug, // Show all levels including debug
		Console: true,
	})

	// Set as default
	dd.SetDefault(customLogger)

	// All package-level functions now use this logger
	dd.Debug("This debug message is visible because of custom level")
	dd.Info("This is an info message")
	dd.Warn("This is a warning")
	dd.Error("This is an error")

	fmt.Println()
}

func example2WithSecurityFiltering() {
	fmt.Println("Example 2: Security Filtering with Package-Level Functions")
	fmt.Println("----------------------------------------------------------")

	// Create logger with basic security filtering
	filterLogger, _ := dd.New(dd.DefaultConfig().EnableBasicFiltering())

	// Set as default
	dd.SetDefault(filterLogger)

	// Package-level functions now use filtered logger
	fmt.Println("Output:")
	dd.Info("password=secret123")
	dd.Info("api_key=sk-1234567890")
	dd.Info("credit_card=4532015112830366")

	fmt.Println("Notice: All sensitive data is redacted!")
	fmt.Println()
}

func example3DynamicConfiguration() {
	fmt.Println("Example 3: Change Default Logger Dynamically")
	fmt.Println("--------------------------------------------")

	// Start with INFO level
	infoLogger, _ := dd.New(dd.DefaultConfig())
	dd.SetDefault(infoLogger)

	dd.Debug("This debug message won't show (INFO level)")
	dd.Info("This info message will show")

	// Change to DEBUG level
	fmt.Println("\nChanging to DEBUG level...")
	debugLogger, _ := dd.New(dd.DefaultConfig().WithLevel(dd.LevelDebug))
	dd.SetDefault(debugLogger)

	dd.Debug("Now debug messages are visible!")
	dd.Info("Info messages still visible")

	// Change to filtered logger
	fmt.Println("\nAdding security filtering...")
	filterLogger, _ := dd.New(dd.DefaultConfig().EnableBasicFiltering())
	dd.SetDefault(filterLogger)

	dd.Info("password=secret123")
	fmt.Println()
}

// Advanced Example: Multiple Loggers + Custom Default
func advancedExample() {
	fmt.Println("\nAdvanced Example: Multiple Loggers")
	fmt.Println("-----------------------------------")

	// Create different loggers for different purposes
	// appLogger := dd.ToConsole()
	dbLogger, _ := dd.New(dd.DefaultConfig().EnableBasicFiltering())

	// Use specific logger for database
	dbLogger.Info("database password=secret123") // This is NOT filtered

	// Set filtered logger as default
	dd.SetDefault(dbLogger)

	// Package-level functions now use filtered logger
	dd.Info("password=secret123") // This IS filtered

	fmt.Println("\nNotice:")
	fmt.Println("  • dbLogger.Info() - not filtered (specific logger)")
	fmt.Println("  • dd.Info() - filtered (uses default logger)")
}
