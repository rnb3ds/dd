//go:build examples

package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/cybergodev/dd"
)

// Writers - Advanced Output Management
//
// This example demonstrates:
// 1. BufferedWriter for high-performance scenarios
// 2. MultiWriter for multiple outputs
// 3. Dynamic writer management (AddWriter, RemoveWriter)
// 4. WriteErrorHandler for error handling
// 5. Flush and logger state inspection
func main() {
	fmt.Println("=== DD Writers Management ===\n")

	example1BufferedWriter()
	example2MultiWriter()
	example3DynamicWriterManagement()
	example4WriteErrorHandler()
	example5LoggerStateInspection()

	fmt.Println("\n✅ Writers examples completed!")
	fmt.Println("\nKey Points:")
	fmt.Println("  • BufferedWriter reduces system calls for high-throughput")
	fmt.Println("  • MultiWriter enables simultaneous multi-output")
	fmt.Println("  • AddWriter/RemoveWriter allow dynamic output management")
	fmt.Println("  • WriteErrorHandler captures write errors gracefully")
}

// Example 1: BufferedWriter for high-performance scenarios
func example1BufferedWriter() {
	fmt.Println("1. BufferedWriter (High Performance)")
	fmt.Println("-------------------------------------")

	// Create file writer
	fileWriter, err := dd.NewFileWriter("logs/buffered.log", dd.FileWriterConfig{
		MaxSizeMB:  100,
		MaxBackups: 5,
	})
	if err != nil {
		fmt.Printf("Failed to create file writer: %v\n", err)
		return
	}

	// Wrap with buffered writer (4KB buffer)
	bufferedWriter, err := dd.NewBufferedWriter(fileWriter, 4*1024)
	if err != nil {
		fmt.Printf("Failed to create buffered writer: %v\n", err)
		return
	}
	defer bufferedWriter.Close() // IMPORTANT: Always call Close() to flush!

	// Create logger with buffered writer
	cfg := dd.DefaultConfig()
	cfg.Output = bufferedWriter

	logger, _ := dd.New(cfg)
	defer logger.Close()

	// High-throughput logging
	start := time.Now()
	for i := 0; i < 1000; i++ {
		logger.InfoWith("Buffered log entry",
			dd.Int("sequence", i),
			dd.String("data", "example"),
		)
	}
	duration := time.Since(start)

	fmt.Printf("✓ 1000 messages logged in %v\n", duration)
	fmt.Println("  Note: Always call Close() to flush buffered data!\n")
}

// Example 2: MultiWriter for multiple outputs
func example2MultiWriter() {
	fmt.Println("2. MultiWriter (Multiple Outputs)")
	fmt.Println("----------------------------------")

	// Create multiple writers
	fileWriter, _ := dd.NewFileWriter("logs/multi.log", dd.FileWriterConfig{})

	// Create MultiWriter combining console and file
	multiWriter := dd.NewMultiWriter(os.Stdout, fileWriter)

	// Create logger with MultiWriter
	cfg := dd.DefaultConfig()
	cfg.Output = multiWriter

	logger, _ := dd.New(cfg)
	defer logger.Close()

	logger.Info("This message goes to both console and file")
	logger.InfoWith("Structured data",
		dd.String("source", "multiwriter"),
		dd.Int("count", 1),
	)

	fmt.Println()
}

// Example 3: Dynamic writer management
func example3DynamicWriterManagement() {
	fmt.Println("3. Dynamic Writer Management")
	fmt.Println("-----------------------------")

	logger, _ := dd.New()
	defer logger.Close()

	fmt.Printf("Initial writer count: %d\n", logger.WriterCount())

	// Add console writer dynamically
	err := logger.AddWriter(os.Stdout)
	if err != nil {
		fmt.Printf("Failed to add writer: %v\n", err)
	}
	fmt.Printf("After adding stdout: %d writers\n", logger.WriterCount())

	// Add file writer dynamically
	fileWriter, _ := dd.NewFileWriter("logs/dynamic.log", dd.FileWriterConfig{})
	err = logger.AddWriter(fileWriter)
	if err != nil {
		fmt.Printf("Failed to add file writer: %v\n", err)
	}
	fmt.Printf("After adding file: %d writers\n", logger.WriterCount())

	logger.Info("This goes to all writers")

	// Remove a writer
	err = logger.RemoveWriter(os.Stdout)
	if err != nil {
		fmt.Printf("Failed to remove writer: %v\n", err)
	}
	fmt.Printf("After removing stdout: %d writers\n", logger.WriterCount())

	logger.Info("This only goes to file now")

	fmt.Println()
}

// Example 4: WriteErrorHandler for error handling
func example4WriteErrorHandler() {
	fmt.Println("4. WriteErrorHandler")
	fmt.Println("--------------------")

	var errorCount int
	var mu sync.Mutex

	// Create config with custom error handler
	handler := func(writer io.Writer, err error) {
		mu.Lock()
		errorCount++
		mu.Unlock()
		fmt.Printf("  [Write Error] Writer: %T, Error: %v\n", writer, err)
	}

	cfg := dd.DefaultConfig()
	cfg.WriteErrorHandler = handler

	logger, _ := dd.New(cfg)
	defer logger.Close()

	// Set error handler at runtime
	logger.SetWriteErrorHandler(func(writer io.Writer, err error) {
		mu.Lock()
		errorCount++
		mu.Unlock()
		fmt.Printf("  [Runtime Handler] Error: %v\n", err)
	})

	logger.Info("Normal logging works fine")

	fmt.Println("  Write errors are now captured by the handler")
	fmt.Println()
}

// Example 5: Logger state inspection
func example5LoggerStateInspection() {
	fmt.Println("5. Logger State Inspection")
	fmt.Println("---------------------------")

	logger, _ := dd.New()
	defer logger.Close()

	// Check logger state
	fmt.Printf("  Is closed: %v\n", logger.IsClosed())
	fmt.Printf("  Writer count: %d\n", logger.WriterCount())
	fmt.Printf("  Current level: %s\n", logger.GetLevel().String())

	// Flush buffered writers (if any)
	err := logger.Flush()
	if err != nil {
		fmt.Printf("  Flush error: %v\n", err)
	} else {
		fmt.Println("  Flush: success")
	}

	// Change level
	logger.SetLevel(dd.LevelDebug)
	fmt.Printf("  After SetLevel(Debug): %s\n", logger.GetLevel().String())

	// After close
	logger.Close()
	fmt.Printf("  After Close(): Is closed = %v\n", logger.IsClosed())

	fmt.Println()
}
