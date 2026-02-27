//go:build examples

package main

import (
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/cybergodev/dd"
)

// Advanced Features - Caller Detection, Cloud Logging, Performance, Debug Utilities
//
// This example demonstrates:
// 1. Caller detection (static and dynamic)
// 2. Cloud logging formats (ELK, CloudWatch)
// 3. Performance benchmarking
// 4. Debug utilities (Text, JSON, Exit)
func main() {
	fmt.Println("=== DD Advanced Features ===\n")

	example1CallerDetection()
	example2CloudLogging()
	example3Performance()
	example4DebugUtilities()

	fmt.Println("\n✅ Advanced features completed!")
}

// Example 1: Caller detection
func example1CallerDetection() {
	fmt.Println("1. Caller Detection")
	fmt.Println("-------------------")

	// No caller (default)
	logger1, _ := dd.New(dd.DefaultConfig())
	defer logger1.Close()
	logger1.Info("No caller: caller information is not shown")

	// Dynamic caller (auto-detects through wrappers)
	logger2, _ := dd.New(dd.DefaultConfig().WithDynamicCaller(true))
	defer logger2.Close()

	// Direct call
	logger2.Info("Dynamic caller: direct call")

	// Through wrapper function
	logWrapper := func(msg string) {
		logger2.Info(msg)
	}
	logWrapper("Dynamic caller: shows caller of logWrapper, not inside it")

	fmt.Println()
}

// Example 2: Cloud logging formats
func example2CloudLogging() {
	fmt.Println("2. Cloud Logging Formats")
	fmt.Println("------------------------")

	// ELK Stack format
	elkConfig := dd.JSONConfig()
	elkConfig.JSON.FieldNames = &dd.JSONFieldNames{
		Timestamp: "@timestamp", // ELK standard
		Level:     "level",
		Message:   "message",
	}
	elkLogger, _ := dd.New(elkConfig)
	defer elkLogger.Close()

	elkLogger.InfoWith("ELK format",
		dd.String("service.name", "user-api"),
		dd.String("service.version", "1.0.0"),
		dd.String("http.method", "POST"),
		dd.String("http.url", "/api/users"),
		dd.Int("http.status", 201),
	)

	// Distributed tracing format
	traceConfig, err := dd.JSONConfig().WithFileOnly("logs/trace.json", dd.FileWriterConfig{})
	if err != nil {
		fmt.Printf("Failed to create config: %v\n", err)
		return
	}
	traceLogger, err := dd.New(traceConfig)
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}
	defer traceLogger.Close()

	traceLogger.InfoWith("Distributed trace",
		dd.String("trace.id", "1234567890abcdef"),
		dd.String("span.id", "abcdef1234567890"),
		dd.String("span.name", "user.create"),
		dd.Float64("duration_ms", 100.5),
	)

	fmt.Println()
}

// Example 3: Performance benchmarking
func example3Performance() {
	fmt.Println("3. Performance Benchmarking")
	fmt.Println("---------------------------")

	// Basic throughput test
	config := dd.DefaultConfig()
	config.Writers = []io.Writer{io.Discard} // Avoid I/O overhead
	logger, _ := dd.New(config)
	defer logger.Close()

	iterations := 10000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		logger.Info("Performance test message")
	}

	duration := time.Since(start)
	opsPerSec := float64(iterations) / duration.Seconds()

	fmt.Printf("  Simple logging: %d messages in %v\n", iterations, duration)
	fmt.Printf("  Throughput: %.0f ops/sec\n", opsPerSec)

	// Concurrent throughput test
	numGoroutines := runtime.NumCPU()
	messagesPerGoroutine := 5000
	totalMessages := numGoroutines * messagesPerGoroutine

	var wg sync.WaitGroup
	start = time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.InfoWith("Concurrent message",
					dd.Int("goroutine", id),
					dd.Int("message", j),
				)
			}
		}(i)
	}

	wg.Wait()
	duration = time.Since(start)
	opsPerSec = float64(totalMessages) / duration.Seconds()

	fmt.Printf("  Concurrent: %d goroutines × %d = %d messages in %v\n",
		numGoroutines, messagesPerGoroutine, totalMessages, duration)
	fmt.Printf("  Throughput: %.0f ops/sec\n", opsPerSec)

	fmt.Println()
}

// Example 4: Debug utilities
func example4DebugUtilities() {
	fmt.Println("4. Debug Utilities")
	fmt.Println("------------------")

	// Text() - Quick debug output (no quotes for simple types)
	fmt.Println("Text() output:")
	dd.Text("Simple:", "hello", 42, 3.14, true)
	dd.Text("Complex:", map[string]any{"name": "Alice", "age": 30})

	// JSON() - JSON format output
	fmt.Println("\nJSON() output:")
	dd.JSON("user", 123, map[string]string{"status": "active"})

	// Textf() and JSONF() - Formatted output
	fmt.Println("\nFormatted output:")
	dd.Textf("User: %s, Age: %d", "Bob", 25)
	dd.JSONF("Request from %s", "192.168.1.1")

	// Logger methods
	logger, err := dd.New(dd.DefaultConfig())
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}
	defer logger.Close()

	fmt.Println("\nLogger methods:")
	logger.Text("Processing", "item", 42)
	logger.JSON("result", true, "count", 100)

	// Exit() - Debug and exit (commented out to avoid terminating)
	// dd.Exit("Program terminated here")
	// dd.Exitf("Fatal error: %s", "critical")

	fmt.Println("\nNote: Exit() and Exitf() output debug info and call os.Exit(0)")
	fmt.Println()
}
