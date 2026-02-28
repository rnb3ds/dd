//go:build examples

package main

import (
	"bytes"
	"fmt"
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
// 5. Must* variants - Panic on error
// 6. Must helper functions
func main() {
	fmt.Println("=== DD Convenience Constructors ===\n")

	section1FileOutput()
	section2ConsoleOutput()
	section3DualOutput()
	section4CustomWriters()
	section5MustVariants()

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

	fmt.Println("✓ Files: logs/app.log, logs/custom.log, logs/json.log\n")
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

// Section 5: Must* variants (panic on error)
func section5MustVariants() {
	fmt.Println("5. Must* Variants (Panic on Error)")
	fmt.Println("-----------------------------------")

	// These panic if creation fails - use for initialization

	// MustToFile
	logger := dd.MustToFile("logs/must.log")
	defer logger.Close()
	logger.Info("Created with MustToFile")

	// MustToJSONFile
	logger2 := dd.MustToJSONFile("logs/must-json.log")
	defer logger2.Close()
	logger2.Info("Created with MustToJSONFile")

	// MustToConsole
	logger3 := dd.MustToConsole()
	defer logger3.Close()
	logger3.Info("Created with MustToConsole")

	// MustToAll
	logger4 := dd.MustToAll("logs/must-all.log")
	defer logger4.Close()
	logger4.Info("Created with MustToAll")

	// Must (generic)
	logger5 := dd.Must(dd.DevelopmentConfig())
	defer logger5.Close()
	logger5.Debug("Created with Must(DevelopmentConfig())")

	// MustNew (alias for Must)
	logger6 := dd.MustNew(dd.JSONConfig())
	defer logger6.Close()
	logger6.Info("Created with MustNew(JSONConfig())")

	// MustVal - generic helper for any function returning (T, error)
	file := dd.MustVal(os.Create("logs/mustval.log"))
	defer file.Close()
	fmt.Printf("  File created: %s\n", file.Name())

	fmt.Println("\n✓ Must* functions panic on error, return logger otherwise")
}
