//go:build examples

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/cybergodev/dd"
)

// Convenience Constructors - Quick Logger Setup
//
// Topics covered:
// 1. ToFile/ToJSONFile - File output
// 2. ToConsole - Console only
// 3. ToAll/ToAllJSON - Dual output
// 4. ToWriter/ToWriters - Custom writers
// 5. Proper error handling patterns
func main() {
	fmt.Println("=== DD Convenience Constructors ===")

	section1FileOutput()
	section2ConsoleOutput()
	section3DualOutput()
	section4CustomWriters()
	section5ConstructorErrors()

	fmt.Println("\n✅ Convenience examples completed!")
	fmt.Println("\nCheck logs/ directory for output files")
}

// Section 1: File output
func section1FileOutput() {
	fmt.Println("1. File Output")
	fmt.Println("---------------")

	// Default file: logs/app.log
	logger, _ := dd.ToFile()
	defer logger.Close()
	logger.Info("Text format to logs/app.log")

	// Custom path
	logger2, _ := dd.ToFile("logs/custom.log")
	defer logger2.Close()
	logger2.Info("Text format to logs/custom.log")

	// JSON format
	logger3, _ := dd.ToJSONFile("logs/json.log")
	defer logger3.Close()
	logger3.InfoWith("JSON format",
		dd.String("format", "json"),
		dd.Bool("structured", true),
	)

	fmt.Println("✓ Files: logs/app.log, logs/custom.log, logs/json.log")
}

// Section 2: Console output
func section2ConsoleOutput() {
	fmt.Println("2. Console Output")
	fmt.Println("------------------")

	// Console only (stdout)
	logger, _ := dd.ToConsole()
	defer logger.Close()

	logger.Info("Console only - no file")
	logger.InfoWith("Structured console output",
		dd.String("source", "console"),
	)

	fmt.Println()
}

// Section 3: Dual output (console + file)
func section3DualOutput() {
	fmt.Println("3. Dual Output (Console + File)")
	fmt.Println("--------------------------------")

	// Text format to both
	logger, _ := dd.ToAll("logs/dual.log")
	defer logger.Close()
	logger.Info("Appears in BOTH console and file")

	// JSON format to both
	logger2, _ := dd.ToAllJSON("logs/dual-json.log")
	defer logger2.Close()
	logger2.InfoWith("JSON to both outputs",
		dd.String("format", "json"),
	)

	fmt.Println()
}

// Section 4: Custom writers
func section4CustomWriters() {
	fmt.Println("4. Custom Writers")
	fmt.Println("------------------")

	// Single custom writer
	var buf bytes.Buffer
	logger, _ := dd.ToWriter(&buf)
	defer logger.Close()

	logger.Info("Written to buffer")
	fmt.Printf("  Buffer content: %s", buf.String()[:50])

	// Multiple writers
	file, _ := os.Create("logs/multi-writer.log")
	defer file.Close()

	logger2, _ := dd.ToWriters(os.Stdout, file)
	defer logger2.Close()

	logger2.Info("Goes to stdout AND file")

	fmt.Println()
}

// Section 5: Constructor error handling patterns
func section5ConstructorErrors() {
	fmt.Println("5. Constructor Error Patterns")
	fmt.Println("------------------------------")

	// Pattern 1: Explicit error handling with log.Fatal
	logger, err := dd.New(dd.DevelopmentConfig())
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()
	logger.Debug("Created with explicit error handling")

	// Pattern 2: Using ToFile with error handling
	logger2, err := dd.ToFile("logs/safe.log")
	if err != nil {
		log.Printf("warning: could not create file logger: %v", err)
		// Fall back to console
		logger2, _ = dd.ToConsole()
	}
	defer logger2.Close()
	logger2.Info("Created with fallback handling")

	// Pattern 3: Using ToConsole (rarely fails)
	logger3, err := dd.ToConsole()
	if err != nil {
		// Console creation rarely fails, but handle it anyway
		fmt.Fprintf(os.Stderr, "failed to create console logger: %v\n", err)
		return
	}
	defer logger3.Close()
	logger3.Info("Created with console fallback")

	// Pattern 4: Using ToAll for dual output
	logger4, err := dd.ToAll("logs/safe-dual.log")
	if err != nil {
		log.Printf("warning: could not create dual logger: %v", err)
		return
	}
	defer logger4.Close()
	logger4.Info("Created with dual output")

	fmt.Println("\n✓ Always handle errors explicitly for robust code")
}
