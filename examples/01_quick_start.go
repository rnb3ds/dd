//go:build examples

package main

import (
	"fmt"

	"github.com/cybergodev/dd"
)

// Quick Start - Master dd logger in 5 minutes
//
// This example covers:
// 1. Creating logger instances
// 2. Using convenience constructors
// 3. All log levels (Debug, Info, Warn, Error, Fatal)
// 4. Formatted logging
// 5. Dynamic level control
func main() {
	fmt.Println("=== DD Logger Quick Start ===\n ")

	example1CreatingLoggers()
	example2ConvenienceConstructors()
	example3LogLevels()
	example4FormattedLogging()
	example5DynamicLevelControl()

	fmt.Println("\n✅ Quick start completed!")
	fmt.Println("\nNext steps:")
	fmt.Println("  • Run 02_structured_logging.go for production patterns")
	fmt.Println("  • Run 03_configuration.go for file output and rotation")
}

// Example 1: Creating logger instances
func example1CreatingLoggers() {
	fmt.Println("1. Creating Logger Instances")
	fmt.Println("----------------------------")

	// Create a logger with default configuration
	logger, _ := dd.New()
	defer logger.Close()

	logger.Info("Logger created with default configuration")
	logger.Info("Application started successfully")

	// Create a logger with custom configuration
	customLogger, _ := dd.New(dd.DefaultConfig().WithLevel(dd.LevelDebug))
	defer customLogger.Close()

	customLogger.Debug("Custom logger with DEBUG level")

	fmt.Println()
}

// Example 2: Convenience constructors
func example2ConvenienceConstructors() {
	fmt.Println("2. Convenience Constructors")
	fmt.Println("-------------------------")

	// Console only
	logger1 := dd.ToConsole()
	defer logger1.Close()
	logger1.Info("Console only output")

	// File only (default: logs/app.log)
	logger2 := dd.ToFile()
	defer logger2.Close()
	logger2.Info("File only output → logs/app.log")

	// JSON file
	logger3 := dd.ToJSONFile()
	defer logger3.Close()
	logger3.Info("Json format → logs/app.log")

	// Console + file
	logger4 := dd.ToAll()
	defer logger4.Close()
	logger4.Info("Both console and file output")

	// Custom filename
	logger5 := dd.ToFile("logs/custom.log")
	defer logger5.Close()
	logger5.Info("Custom file → logs/custom.log")

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

	logger.Debug("DEBUG: Detailed diagnostic information")
	logger.Info("INFO: General informational messages")
	logger.Warn("WARN: Warning messages")
	logger.Error("ERROR: Error messages")
	// logger.Fatal("FATAL: Fatal error - terminates program")

	fmt.Printf("Current level: %s\n\n", logger.GetLevel().String())
}

// Example 4: Formatted logging
func example4FormattedLogging() {
	fmt.Println("4. Formatted Logging")
	fmt.Println("--------------------")

	logger := dd.ToConsole()
	defer logger.Close()

	// Printf-style formatting
	name := "Alice"
	age := 30
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
	fmt.Println("------------------------")

	logger := dd.ToConsole()
	defer logger.Close()

	// Initial level (default INFO)
	logger.Debug("Debug message won't show")
	logger.Info("Info message will show")

	// Change to DEBUG level
	logger.SetLevel(dd.LevelDebug)
	logger.Debug("Now Debug messages are visible!")

	// Change back to INFO
	logger.SetLevel(dd.LevelInfo)
	logger.Debug("Debug messages hidden again")
	logger.Info("Info messages still visible")

	fmt.Println()
}
