//go:build examples

package main

import (
	"fmt"

	"github.com/cybergodev/dd"
)

// Correct Usage of SetDefault() - Configure package-level functions
//
// This example demonstrates the CORRECT way to use SetDefault()
// to customize package-level logging behavior.
func main() {
	fmt.Println("=== Correct SetDefault() Usage ===\n ")

	correctUsageInInit()
	correctUsageBeforeFirstCall()
	incorrectUsageAfterFirstCall()

	fmt.Println("\n✅ Examples completed!")
	fmt.Println("\nKey Point:")
	fmt.Println("  ⚠️  Call SetDefault() BEFORE any package-level functions!")
	fmt.Println("  ✅ Best place: init() function or at very start of main()")
}

func correctUsageInInit() {
	fmt.Println("Example 1: SetDefault() in init() - RECOMMENDED")
	fmt.Println("---------------------------------------------------")

	// This is the recommended approach
	setupLogger()

	// Now all package-level functions use filtered logger
	dd.Info("password=secret123")
	dd.Info("api_key=sk-1234567890")

	fmt.Println("Notice: All sensitive data is redacted! ✓")
}

func setupLogger() {
	// Create custom logger with security filtering
	logger, _ := dd.New(dd.DefaultConfig().EnableBasicFiltering())

	// Set as default - this affects ALL package-level functions
	dd.SetDefault(logger)
}

func correctUsageBeforeFirstCall() {
	fmt.Println("\nExample 2: SetDefault() Before First Call - CORRECT")
	fmt.Println("-------------------------------------------------------")

	// Create and set custom logger BEFORE any dd.Info() calls
	logger, _ := dd.New(dd.DefaultConfig().EnableBasicFiltering())
	dd.SetDefault(logger)

	// Now safe to use package-level functions
	dd.Info("password=secret123")

	fmt.Println("Notice: Sensitive data is redacted! ✓")
}

func incorrectUsageAfterFirstCall() {
	fmt.Println("\nExample 3: Calling dd.Info() Before SetDefault() - WRONG")
	fmt.Println("--------------------------------------------------------")

	// Create another logger (simulating a new module)
	anotherLogger, _ := dd.New(dd.DefaultConfig().EnableBasicFiltering())

	// Try to set it as default
	dd.SetDefault(anotherLogger)

	// But this won't work if dd.Info() was already called before!
	// The first logger is still being used
	dd.Info("password=secret123")

	fmt.Println("Notice: Still using the first logger!")
	fmt.Println("⚠️  SetDefault() only works if called BEFORE first Default() call")
}
