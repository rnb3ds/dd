//go:build examples

package main

import (
	"fmt"

	"github.com/cybergodev/dd"
)

// Quick Start - Master dd logger in 5 minutes
//
// This example covers:
// 1. Package-level functions (simplest)
// 2. Convenience constructors
// 3. All log levels
// 4. Formatted and structured logging
// 5. Dynamic level control
func main() {
	fmt.Println("=== DD Logger Quick Start ===\n")

	example1PackageLevelFunctions()
	example2ConvenienceConstructors()
	example3LogLevels()
	example4FormattedLogging()
	example5DynamicLevelControl()

	fmt.Println("\n✅ Quick start completed!")
	fmt.Println("\nNext steps:")
	fmt.Println("  • Run 02_structured_logging.go for production patterns")
	fmt.Println("  • Run 03_configuration.go for file output and rotation")
	fmt.Println("  • Run 05_writers.go for advanced output management")
}

// Example 1: Package-level functions (simplest way)
func example1PackageLevelFunctions() {
	fmt.Println("1. Package-Level Functions (Simplest)")
	fmt.Println("-------------------------------------")

	// Use package-level functions directly - zero setup required
	dd.Debug("Debug: Detailed diagnostic information")
	dd.Info("Info: General informational messages")
	dd.Warn("Warn: Warning messages")
	dd.Error("Error: Error messages")
	// dd.Fatal("Fatal: Fatal error - terminates program") // Uncomment to test

	fmt.Println()
}

// Example 2: Convenience constructors
func example2ConvenienceConstructors() {
	fmt.Println("2. Convenience Constructors")
	fmt.Println("---------------------------")

	// Console logger (recommended)
	logger1, err := dd.ConsoleLogger()
	if err != nil {
		fmt.Printf("Failed to create console logger: %v\n", err)
		return
	}
	defer logger1.Close()
	logger1.Info("Console only output")

	// File logger
	logger2, err := dd.FileLogger("logs/custom.log")
	if err != nil {
		fmt.Printf("Failed to create file logger: %v\n", err)
		return
	}
	defer logger2.Close()
	logger2.Info("File only output → logs/custom.log")

	// JSON file logger
	logger3, err := dd.JSONFileLogger()
	if err != nil {
		fmt.Printf("Failed to create JSON logger: %v\n", err)
		return
	}
	defer logger3.Close()
	logger3.Info("JSON format → logs/app.log")

	// Multi-output (console + file)
	logger4, err := dd.MultiLogger()
	if err != nil {
		fmt.Printf("Failed to create multi logger: %v\n", err)
		return
	}
	defer logger4.Close()
	logger4.Info("Both console and file output")

	fmt.Println()
}

// Example 3: All log levels
func example3LogLevels() {
	fmt.Println("3. Log Levels")
	fmt.Println("-------------")

	logger, _ := dd.NewWithOptions(dd.Options{
		Level:   dd.LevelDebug, // Show all levels
		Console: true,
	})
	defer logger.Close()

	// Level hierarchy: Debug < Info < Warn < Error < Fatal
	logger.Debug("DEBUG: Detailed diagnostic information")
	logger.Info("INFO: General informational messages")
	logger.Warn("WARN: Warning messages")
	logger.Error("ERROR: Error messages")
	// logger.Fatal("FATAL: Fatal error - terminates program")

	fmt.Printf("Current level: %s\n", logger.GetLevel().String())
	fmt.Println()
}

// Example 4: Formatted and structured logging
func example4FormattedLogging() {
	fmt.Println("4. Formatted & Structured Logging")
	fmt.Println("---------------------------------")

	logger, err := dd.ConsoleLogger()
	if err != nil {
		fmt.Printf("Failed to create console logger: %v\n", err)
		return
	}
	defer logger.Close()

	name := "Alice"
	age := 30

	// Printf-style formatting
	logger.Infof("User: %s, Age: %d", name, age)

	// Multiple arguments (space-separated)
	logger.Info("User", name, "age", age)

	// Structured logging (recommended for production)
	logger.InfoWith("User information",
		dd.String("name", name),
		dd.Int("age", age),
		dd.Bool("active", true),
	)

	fmt.Println()
}

// Example 5: Dynamic level control
func example5DynamicLevelControl() {
	fmt.Println("5. Dynamic Level Control")
	fmt.Println("-------------------------")

	logger, err := dd.ConsoleLogger()
	if err != nil {
		fmt.Printf("Failed to create console logger: %v\n", err)
		return
	}
	defer logger.Close()

	// Initial level (default INFO)
	logger.Debug("Debug message won't show (INFO level)")
	logger.Info("Info message will show")

	// Change to DEBUG level
	logger.SetLevel(dd.LevelDebug)
	logger.Debug("Now Debug messages are visible!")

	// Change back to INFO
	logger.SetLevel(dd.LevelInfo)
	logger.Debug("Debug messages hidden again")
	logger.Info("Info messages still visible")

	// Get current level
	currentLevel := logger.GetLevel()
	fmt.Printf("Current level: %s\n", currentLevel.String())

	fmt.Println()
}
