//go:build examples

package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/cybergodev/dd"
)

// Structured Logging - Production-Ready Patterns
//
// This example demonstrates:
// 1. Core field types (String, Int, Float64, Bool, Err, Any)
// 2. Real-world logging templates
// 3. Best practices for structured logging
func main() {
	fmt.Println("=== DD Structured Logging ===\n")

	example1CoreFieldTypes()
	example2HTTPRequestLogging()
	example3ErrorLogging()

	fmt.Println("\n✅ Structured logging examples completed!")
	fmt.Println("\nBest Practices:")
	fmt.Println("  • Use type-safe fields (String, Int, Bool) over Any for better performance")
	fmt.Println("  • Keep 5-10 fields per log entry")
	fmt.Println("  • Use consistent field names across your application")
	fmt.Println("  • JSON format for production, Text format for development")
}

// Example 1: Core field types
func example1CoreFieldTypes() {
	fmt.Println("1. Core Field Types")
	fmt.Println("-------------------")

	config, err := dd.JSONConfig().WithFileOnly("logs/structured.json", dd.FileWriterConfig{})
	if err != nil {
		fmt.Printf("Failed to create config: %v\n", err)
		return
	}
	logger, err := dd.New(config)
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}
	defer logger.Close()

	logger.InfoWith("All field types",
		// Type-safe fields (recommended - best performance)
		dd.String("name", "John Doe"),
		dd.Int("age", 30),
		dd.Int64("user_id", 9876543210),
		dd.Float64("score", 98.5),
		dd.Bool("active", true),
		dd.Err(errors.New("example error")),

		// Complex types (use Any)
		dd.Any("tags", []string{"vip", "premium"}),
		dd.Any("metadata", map[string]int{"count": 100}),
		dd.Any("timestamp", time.Now()),
	)

	fmt.Println("✓ Logged with all field types\n")
}

// Example 2: HTTP request logging template
func example2HTTPRequestLogging() {
	fmt.Println("2. HTTP Request Logging")
	fmt.Println("-----------------------")

	config, err := dd.JSONConfig().WithFileOnly("logs/http.json", dd.FileWriterConfig{})
	if err != nil {
		fmt.Printf("Failed to create config: %v\n", err)
		return
	}
	logger, err := dd.New(config)
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}
	defer logger.Close()

	// Request received
	logger.InfoWith("HTTP request",
		dd.String("request_id", "req-abc-123"),
		dd.String("method", "POST"),
		dd.String("path", "/api/v1/users"),
		dd.String("client_ip", "192.168.1.100"),
		dd.Int("user_id", 12345),
	)

	// Response sent
	logger.InfoWith("HTTP response",
		dd.String("request_id", "req-abc-123"),
		dd.Int("status", 201),
		dd.Float64("duration_ms", 125.7),
		dd.Int("response_size", 512),
	)

	fmt.Println("✓ HTTP request/response logged\n")
}

// Example 3: Error logging template
func example3ErrorLogging() {
	fmt.Println("3. Error & Warning Logging")
	fmt.Println("--------------------------")

	config, err := dd.JSONConfig().WithFileOnly("logs/errors.json", dd.FileWriterConfig{})
	if err != nil {
		fmt.Printf("Failed to create config: %v\n", err)
		return
	}
	logger, err := dd.New(config)
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}
	defer logger.Close()

	// Application error with context
	err = errors.New("connection timeout")
	logger.ErrorWith("Operation failed",
		dd.Err(err),
		dd.String("operation", "user_query"),
		dd.String("host", "db.example.com"),
		dd.Int("retry_count", 3),
	)

	// Resource warning
	logger.WarnWith("Resource alert",
		dd.String("alert_type", "high_memory"),
		dd.Float64("memory_percent", 85.5),
		dd.Float64("threshold", 80.0),
		dd.String("host", "app-server-01"),
	)

	fmt.Println("✓ Errors and alerts logged\n")
}
