//go:build examples

package main

import (
	"fmt"
	"strings"

	"github.com/cybergodev/dd"
)

// Security - Sensitive Data Filtering and Protection
//
// This example demonstrates:
// 1. Basic filtering (passwords, API keys, credit cards)
// 2. Full filtering (emails, IPs, SSNs, database URLs)
// 3. Custom filtering patterns
// 4. Message size limits
func main() {
	fmt.Println("=== DD Security Features ===\n ")

	example1BasicFiltering()
	example2FullFiltering()
	example3CustomFiltering()
	// example4MessageSizeLimit()

	fmt.Println("\n✅ Security examples completed!")
	fmt.Println("\nSecurity Tips:")
	fmt.Println("  • Filtering is disabled by default - enable manually when needed")
	fmt.Println("  • Use EnableBasicFiltering() for common sensitive data")
	fmt.Println("  • Use EnableFullFiltering() for comprehensive protection")
	fmt.Println("  • Log injection protection is always enabled")
}

// Example 1: Basic filtering
func example1BasicFiltering() {
	fmt.Println("1. Basic Filtering")
	fmt.Println("------------------")

	config := dd.DefaultConfig().EnableBasicFiltering()
	logger, _ := dd.New(config)
	defer logger.Close()

	// These will be automatically filtered
	logger.Info("password=secret123")
	logger.Info("api_key=sk-1234567890abcdef")
	logger.Info("credit_card=4532015112830366")
	logger.Info("token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")

	// Structured logging with sensitive fields
	logger.InfoWith("User login",
		dd.String("username", "john_doe"),
		dd.String("password", "secret123"), // Filtered
		dd.String("api_key", "sk-abc123"),  // Filtered
	)

	fmt.Println("✓ Sensitive data filtered\n ")
}

// Example 2: Full filtering
func example2FullFiltering() {
	fmt.Println("2. Full Filtering")
	fmt.Println("-----------------")

	config := dd.DefaultConfig().EnableFullFiltering()
	logger, _ := dd.New(config)
	defer logger.Close()

	// More types of sensitive data filtered
	logger.Info("email=user@example.com")
	logger.Info("ssn=123-45-6789")
	logger.Info("ip=192.168.1.1")
	logger.Info("phone=+8613800000000")
	logger.Info("mysql://user:pass@localhost:3306/db")

	// JWT tokens
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc123"
	logger.Infof("Authorization: Bearer %s", jwt)

	// Private keys
	privateKey := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA1234567890...
-----END RSA PRIVATE KEY-----`
	logger.Info(privateKey)

	fmt.Println("✓ Comprehensive filtering applied\n ")
}

// Example 3: Custom filtering patterns
func example3CustomFiltering() {
	fmt.Println("3. Custom Filtering")
	fmt.Println("-------------------")

	// Create empty filter and add custom patterns
	filter := dd.NewEmptySensitiveDataFilter()
	filter.AddPattern(`(?i)(internal[_-]?token[:\s=])+[^\s]+`)
	filter.AddPattern(`(?i)(session[_-]?id[:\s=])+[^\s]+`)
	filter.AddPattern(`(?i)(secret[_-]?code[:\s=])+[^\s]+`)

	config := dd.DefaultConfig().WithFilter(filter)
	logger, _ := dd.New(config)
	defer logger.Close()

	logger.Info("internal_token=abc123")
	logger.Info("session_id=xyz789")
	logger.Info("secret_code=def456")
	logger.Info("public_data=visible") // Not filtered

	fmt.Println("✓ Custom patterns applied\n ")
}

// Example 4: Message size limit
func example4MessageSizeLimit() {
	fmt.Println("4. Message Size Limit")
	fmt.Println("---------------------")

	// Default limit is 5MB
	config1 := dd.DefaultConfig()
	logger1, _ := dd.New(config1)
	defer logger1.Close()

	// Try to log 6MB message (will be truncated)
	largeMsg := strings.Repeat("A", 6*1024*1024)
	logger1.Info(largeMsg)
	fmt.Println("✓ 6MB message truncated to 5MB (default limit)")

	// Custom limit: 1MB
	config2 := dd.DefaultConfig()
	config2.SecurityConfig = &dd.SecurityConfig{
		MaxMessageSize: 1 * 1024 * 1024, // 1MB
	}
	logger2, _ := dd.New(config2)
	defer logger2.Close()

	mediumMsg := strings.Repeat("B", 2*1024*1024)
	logger2.Info(mediumMsg)
	fmt.Println("✓ 2MB message truncated to 1MB (custom limit)")

	// Custom limit: 10MB
	config3 := dd.DefaultConfig()
	config3.SecurityConfig = &dd.SecurityConfig{
		MaxMessageSize: 10 * 1024 * 1024, // 10MB
	}
	logger3, _ := dd.New(config3)
	defer logger3.Close()

	smallMsg := strings.Repeat("C", 6*1024*1024)
	logger3.Info(smallMsg)
	fmt.Println("✓ 6MB message not truncated (10MB limit)")

	fmt.Println()
}
