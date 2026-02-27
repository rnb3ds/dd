//go:build examples

package main

import (
	"fmt"

	"github.com/cybergodev/dd"
)

// Global Logger - SetDefault, GetLevel, and Package-Level Functions
//
// This example demonstrates:
// 1. SetDefault() - Customize package-level functions
// 2. GetLevel() / SetLevel() - Dynamic level control
// 3. Best practices for global logger usage
// 4. Common pitfalls and how to avoid them
func main() {
	fmt.Println("=== DD Global Logger Management ===\n")

	example1BasicUsage()
	example2SetDefault()
	example3GetLevelSetLevel()
	example4BestPractices()

	fmt.Println("\n✅ Global logger examples completed!")
	fmt.Println("\nKey Points:")
	fmt.Println("  • Call SetDefault() BEFORE any package-level functions")
	fmt.Println("  • Use GetLevel() to check current log level")
	fmt.Println("  • SetDefault() affects ALL package-level functions")
	fmt.Println("  • Best place for SetDefault(): init() or start of main()")
}

// Example 1: Basic package-level functions
func example1BasicUsage() {
	fmt.Println("1. Basic Package-Level Functions")
	fmt.Println("---------------------------------")

	// Package-level functions use the global default logger
	dd.Debug("Debug message (may not show - default is INFO)")
	dd.Info("Info message")
	dd.Warn("Warning message")
	dd.Error("Error message")

	dd.InfoWith("Structured logging",
		dd.String("key", "value"),
		dd.Int("count", 42),
	)

	fmt.Println()
}

// Example 2: SetDefault() - Customize package-level functions
func example2SetDefault() {
	fmt.Println("2. SetDefault() - Custom Configuration")
	fmt.Println("---------------------------------------")

	// Create custom logger with DEBUG level
	customLogger, err := dd.New(dd.DefaultConfig().WithLevel(dd.LevelDebug))
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}

	// Set as default - affects ALL package-level functions
	dd.SetDefault(customLogger)

	// Now package-level functions use DEBUG level
	dd.Debug("This debug message is now visible!")
	dd.Info("Info message with custom logger")

	// Create logger with security filtering
	filterLogger, err := dd.New(dd.DefaultConfig().EnableBasicFiltering())
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}
	dd.SetDefault(filterLogger)

	// Package-level functions now use filtered logger
	dd.Info("password=secret123")
	dd.Info("api_key=sk-1234567890")

	fmt.Println()
}

// Example 3: GetLevel() and SetLevel() - Dynamic level control
func example3GetLevelSetLevel() {
	fmt.Println("3. GetLevel() & SetLevel() - Dynamic Control")
	fmt.Println("---------------------------------------------")

	// Get current level of default logger
	currentLevel := dd.GetLevel()
	fmt.Printf("Current level: %s\n", currentLevel.String())

	// Set new level for default logger
	dd.SetLevel(dd.LevelWarn)
	fmt.Printf("After SetLevel(Warn): %s\n", dd.GetLevel().String())

	dd.Debug("This won't show (WARN level)")
	dd.Info("This won't show either (WARN level)")
	dd.Warn("This will show!")
	dd.Error("This will show too!")

	// Set to DEBUG level
	dd.SetLevel(dd.LevelDebug)
	fmt.Printf("After SetLevel(Debug): %s\n", dd.GetLevel().String())

	dd.Debug("Debug messages are back!")

	// Practical usage: Check level before expensive operations
	if dd.GetLevel() <= dd.LevelDebug {
		// Only compute expensive debug info when needed
		expensiveDebugInfo := computeExpensiveDebugInfo()
		dd.Debug(expensiveDebugInfo)
	}

	fmt.Println()
}

// Example 4: Best practices and common pitfalls
func example4BestPractices() {
	fmt.Println("4. Best Practices & Common Pitfalls")
	fmt.Println("------------------------------------")

	// ✅ CORRECT: Set default BEFORE any package-level calls
	setupGlobalLogger()
	dd.Info("This uses the configured logger")
	dd.Info("password=secret123") // Will be filtered

	// ✅ CORRECT: Use instance logger for specific needs
	specialLogger, _ := dd.New(dd.DefaultConfig().DisableFiltering())
	specialLogger.Info("password=raw123") // Not filtered
	specialLogger.Close()

	// ⚠️ PITFALL: SetDefault after first call
	// If you call dd.Info() BEFORE SetDefault(), the first logger is used
	// This example works because we called setupGlobalLogger() first

	fmt.Println("\nCommon Pitfalls:")
	fmt.Println("  ❌ Calling dd.Info() before SetDefault()")
	fmt.Println("  ❌ Forgetting to call Close() on loggers")
	fmt.Println("\nRecommended Patterns:")
	fmt.Println("  ✅ SetDefault() in init() or start of main()")
	fmt.Println("  ✅ Use New() with config for production")
	fmt.Println("  ✅ Use defer logger.Close() for cleanup")
	fmt.Println()
}

// setupGlobalLogger is the recommended way to configure global logger
func setupGlobalLogger() {
	// Create custom logger
	logger, err := dd.New(dd.DefaultConfig().EnableBasicFiltering())
	if err != nil {
		// Fallback to default if creation fails
		return
	}

	// Set as default - call this BEFORE any package-level functions
	dd.SetDefault(logger)
}

// computeExpensiveDebugInfo simulates expensive debug computation
func computeExpensiveDebugInfo() string {
	return "Expensive debug information computed only when needed"
}

// init function is a good place to set up global logger
// Uncomment to use:
/*
func init() {
	logger, _ := dd.New(dd.DefaultConfig().WithLevel(dd.LevelDebug))
	dd.SetDefault(logger)
}
*/
