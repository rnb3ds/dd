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
// 4. Disable filtering when needed
func main() {
	fmt.Println("=== DD Security Features ===\n")

	example1BasicFiltering()
	example2FullFiltering()
	example3CustomFiltering()
	example4DisableFiltering()

	fmt.Println("\n✅ Security examples completed!")
	fmt.Println("\nSecurity Tips:")
	fmt.Println("  • Basic filtering is enabled by default for security")
	fmt.Println("  • Use full filtering for comprehensive protection")
	fmt.Println("  • Disable filtering only when you need raw data")
	fmt.Println("  • Log injection protection is always enabled")
}

// Example 1: Basic filtering (enabled by default)
func example1BasicFiltering() {
	fmt.Println("1. Basic Filtering (Default)")
	fmt.Println("----------------------------")

	// Basic filtering is enabled by default
	cfg := dd.DefaultConfig()
	cfg.Security = dd.DefaultSecurityConfig() // Enables basic filtering

	logger, _ := dd.New(cfg)
	defer logger.Close()

	// These will be automatically filtered
	logger.Info("password=secret123")
	logger.Info("api_key=sk-1234567890abcdef")
	logger.Info("credit_card=4532015112830366")
	logger.Info("phone=+1-415-555-2671")

	// Structured logging with sensitive fields
	logger.InfoWith("User login",
		dd.String("username", "john_doe"),
		dd.String("password", "secret123"), // Filtered by key name
		dd.String("api_key", "sk-abc123"),  // Filtered by key name
	)

	fmt.Println("✓ Sensitive data filtered\n")
}

// Example 2: Full filtering
func example2FullFiltering() {
	fmt.Println("2. Full Filtering")
	fmt.Println("-----------------")

	cfg := dd.DefaultConfig()
	secConfig := dd.DefaultSecurityConfig()
	secConfig.SensitiveFilter = dd.NewSensitiveDataFilter() // Full filtering
	cfg.Security = secConfig

	logger, _ := dd.New(cfg)
	defer logger.Close()

	// More types of sensitive data filtered
	logger.Info("email=user@example.com")
	logger.Info("ssn=123-45-6789")
	logger.Info("ip=192.168.1.1")
	logger.Info("mysql://user:pass@localhost:3306/db")

	// JWT tokens
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc123"
	logger.Infof("Authorization: Bearer %s", jwt)

	// Private keys
	privateKey := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA1234567890...
-----END RSA PRIVATE KEY-----`
	logger.Info(privateKey)

	fmt.Println("✓ Comprehensive filtering applied\n")
}

// Example 3: Custom filtering patterns
func example3CustomFiltering() {
	fmt.Println("3. Custom Filtering")
	fmt.Println("-------------------")

	// Create empty filter and add custom patterns
	filter := dd.NewEmptySensitiveDataFilter()
	filter.AddPattern(`(?i)(internal[_-]?token[:\s=]+)[^\s]+`)
	filter.AddPattern(`(?i)(session[_-]?id[:\s=]+)[^\s]+`)
	filter.AddPattern(`(?i)(secret[_-]?code[:\s=]+)[^\s]+`)

	cfg := dd.DefaultConfig()
	secConfig := dd.DefaultSecurityConfig()
	secConfig.SensitiveFilter = filter
	cfg.Security = secConfig

	logger, _ := dd.New(cfg)
	defer logger.Close()

	logger.Info("internal_token=abc123")
	logger.Info("session_id=xyz789")
	logger.Info("secret_code=def456")
	logger.Info("public_data=visible") // Not filtered

	fmt.Println("✓ Custom patterns applied\n")
}

// Example 4: Disable filtering when needed
func example4DisableFiltering() {
	fmt.Println("4. Disable Filtering")
	fmt.Println("--------------------")

	// Disable all filtering (use with caution)
	cfg := dd.DefaultConfig()
	cfg.Security = dd.DefaultSecurityConfigDisabled()

	logger, _ := dd.New(cfg)
	defer logger.Close()

	logger.Info("password=secret123") // Not filtered

	fmt.Println("Note: Disable filtering only when you need raw data")
	fmt.Println()
}

// Example 5: Message size limit (commented out to avoid large output)
func example5MessageSizeLimit() {
	fmt.Println("5. Message Size Limit")
	fmt.Println("---------------------")

	// Default limit is 5MB
	logger, _ := dd.New()
	defer logger.Close()

	// Try to log 6MB message (will be truncated)
	largeMsg := strings.Repeat("A", 6*1024*1024)
	logger.Info(largeMsg)
	fmt.Println("✓ 6MB message truncated to 5MB (default limit)")

	// Custom limit: 1MB
	cfg := dd.DefaultConfig()
	secConfig := dd.DefaultSecurityConfig()
	secConfig.MaxMessageSize = 1 * 1024 * 1024 // 1MB
	cfg.Security = secConfig

	logger2, _ := dd.New(cfg)
	defer logger2.Close()

	mediumMsg := strings.Repeat("B", 2*1024*1024)
	logger2.Info(mediumMsg)
	fmt.Println("✓ 2MB message truncated to 1MB (custom limit)")

	fmt.Println()
}
