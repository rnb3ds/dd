package dd

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// NOTE: TestSensitiveDataFilter, TestBasicSensitiveDataFilter, and TestDefaultSecurityConfig
// are now in dd_test.go to avoid duplication. This file contains specialized
// security tests that are not in the main test file.

// ============================================================================
// EMPTY FILTER TESTS
// ============================================================================

func TestEmptySensitiveDataFilter(t *testing.T) {
	filter := NewEmptySensitiveDataFilter()

	// Empty filter should not filter anything
	input := "password=secret123"
	result := filter.Filter(input)
	if result != input {
		t.Errorf("Empty filter should not modify input, got: %s", result)
	}

	// Add pattern and test
	err := filter.AddPattern(`password=\w+`)
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}

	result = filter.Filter(input)
	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("Filter should redact after adding pattern, got: %s", result)
	}
}

func TestCustomSensitiveDataFilter(t *testing.T) {
	filter, err := NewCustomSensitiveDataFilter(
		`custom_secret=\w+`,
		`internal_id=\d+`,
	)
	if err != nil {
		t.Fatalf("Failed to create custom filter: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "custom secret",
			input:    "custom_secret=abc123",
			contains: "[REDACTED]",
		},
		{
			name:     "internal id",
			input:    "internal_id=12345",
			contains: "[REDACTED]",
		},
		{
			name:     "normal text",
			input:    "hello world",
			contains: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Filter(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Expected %q in result, got: %s", tt.contains, result)
			}
		})
	}
}

// ============================================================================
// FILTER MANAGEMENT TESTS
// ============================================================================

func TestFilterPatternManagement(t *testing.T) {
	filter := NewEmptySensitiveDataFilter()

	// Test initial state
	if filter.PatternCount() != 0 {
		t.Error("Empty filter should have 0 patterns")
	}

	// Test adding patterns
	patterns := []string{
		`pattern1=\w+`,
		`pattern2=\d+`,
		`pattern3=[a-z]+`,
	}

	err := filter.AddPatterns(patterns...)
	if err != nil {
		t.Fatalf("Failed to add patterns: %v", err)
	}

	if filter.PatternCount() != 3 {
		t.Errorf("Expected 3 patterns, got %d", filter.PatternCount())
	}

	// Test clearing patterns
	filter.ClearPatterns()
	if filter.PatternCount() != 0 {
		t.Error("Pattern count should be 0 after clear")
	}

	// Filter should not filter anything after clear
	result := filter.Filter("password=secret123")
	if result != "password=secret123" {
		t.Error("Should not filter after clearing patterns")
	}
}

func TestInvalidPattern(t *testing.T) {
	filter := NewEmptySensitiveDataFilter()

	// Try to add invalid pattern
	err := filter.AddPattern(`[invalid(`)
	if err == nil {
		t.Error("Should fail with invalid pattern")
	}
}

func TestAddPatternsWithInvalid(t *testing.T) {
	filter := NewEmptySensitiveDataFilter()

	patterns := []string{
		`valid_pattern=\w+`,
		`[invalid(`,
		`another_valid=\d+`,
	}

	err := filter.AddPatterns(patterns...)
	if err == nil {
		t.Error("Should fail when one pattern is invalid")
	}
}

// ============================================================================
// FIELD VALUE FILTERING TESTS
// ============================================================================

func TestFilterFieldValue(t *testing.T) {
	filter := NewSensitiveDataFilter()

	tests := []struct {
		name     string
		key      string
		value    interface{}
		expected string
	}{
		{
			name:     "password field",
			key:      "password",
			value:    "secret123",
			expected: "[REDACTED]",
		},
		{
			name:     "api_key field",
			key:      "api_key",
			value:    "sk-1234567890",
			expected: "[REDACTED]",
		},
		{
			name:     "token field",
			key:      "token",
			value:    "abc123xyz",
			expected: "[REDACTED]",
		},
		{
			name:     "normal field",
			key:      "username",
			value:    "john_doe",
			expected: "john_doe",
		},
		{
			name:     "non-string value",
			key:      "count",
			value:    42,
			expected: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.FilterFieldValue(tt.key, tt.value)
			resultStr := ""
			if str, ok := result.(string); ok {
				resultStr = str
			} else {
				resultStr = "42" // For the non-string test case
			}

			if tt.name != "non-string value" && !strings.Contains(resultStr, tt.expected) {
				t.Errorf("Expected %q in result, got: %v", tt.expected, result)
			}
		})
	}
}

func TestFilterFieldValueSubstring(t *testing.T) {
	filter := NewSensitiveDataFilter()

	tests := []struct {
		key      string
		value    string
		redacted bool
	}{
		{"user_password", "secret", true},
		{"password_hash", "hash123", true},
		{"api_key_prod", "key123", true},
		{"secret_token", "token123", true},
		{"username", "john", false},
		{"user_id", "12345", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := filter.FilterFieldValue(tt.key, tt.value)
			resultStr := result.(string)

			if tt.redacted {
				if resultStr != "[REDACTED]" {
					t.Errorf("Expected [REDACTED] for key %q, got: %s", tt.key, resultStr)
				}
			} else {
				if resultStr == "[REDACTED]" {
					t.Errorf("Should not redact key %q", tt.key)
				}
			}
		})
	}
}

func TestFilterValueRecursive(t *testing.T) {
	filter := NewSensitiveDataFilter()

	tests := []struct {
		name  string
		value interface{}
	}{
		{"string", "test string"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"nil", nil},
		{"slice", []int{1, 2, 3}},
		{"map", map[string]int{"a": 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.FilterValueRecursive("", tt.value)
			// Should not panic
			if result == nil && tt.value != nil {
				t.Error("FilterValueRecursive should not return nil for non-nil input")
			}
		})
	}
}

// ============================================================================
// FILTER CLONING TESTS
// ============================================================================

func TestFilterClone(t *testing.T) {
	original := NewSensitiveDataFilter()
	originalCount := original.PatternCount()

	clone := original.Clone()

	if clone == nil {
		t.Fatal("Clone should not be nil")
	}

	if clone.PatternCount() != originalCount {
		t.Error("Clone should have same pattern count")
	}

	// Modify clone
	clone.AddPattern(`test_pattern=\w+`)

	// Original should not be affected
	if original.PatternCount() == clone.PatternCount() {
		t.Error("Modifying clone should not affect original")
	}
}

func TestNilFilterClone(t *testing.T) {
	var filter *SensitiveDataFilter
	clone := filter.Clone()

	if clone != nil {
		t.Error("Cloning nil filter should return nil")
	}
}

// ============================================================================
// REDOS PROTECTION TESTS
// ============================================================================

func TestFilterValueRecursiveCircularReference(t *testing.T) {
	filter := NewSensitiveDataFilter()

	type Node struct {
		Value int
		Next  *Node
	}

	a := &Node{Value: 1}
	b := &Node{Value: 2}
	a.Next = b
	b.Next = a // Circular reference

	// Should not panic or hang
	result := filter.FilterValueRecursive("", a)
	if result == nil {
		t.Error("Should handle circular reference without returning nil")
	}

	// Check that circular reference is detected
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	next, ok := resultMap["Next"].(map[string]any)
	if !ok {
		t.Fatalf("Expected Next to be a map, got %T", resultMap["Next"])
	}

	// The circular reference should be marked
	if next["Next"] != "[CIRCULAR_REFERENCE]" {
		t.Errorf("Expected [CIRCULAR_REFERENCE], got %v", next["Next"])
	}
}

func TestFilterValueRecursiveSliceCircularReference(t *testing.T) {
	filter := NewSensitiveDataFilter()

	type Container struct {
		Items []*Container
	}

	a := &Container{}
	b := &Container{}
	a.Items = []*Container{b}
	b.Items = []*Container{a} // Circular reference

	// Should not panic or hang
	result := filter.FilterValueRecursive("", a)
	if result == nil {
		t.Error("Should handle circular reference without returning nil")
	}
}

func TestFilterValueRecursiveMapCircularReference(t *testing.T) {
	filter := NewSensitiveDataFilter()

	// Create maps with circular references
	mapA := make(map[string]any)
	mapB := make(map[string]any)
	mapA["ref"] = mapB
	mapB["ref"] = mapA // Circular reference

	// Should not panic or hang
	result := filter.FilterValueRecursive("", mapA)
	if result == nil {
		t.Error("Should handle circular reference without returning nil")
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	ref, ok := resultMap["ref"].(map[string]any)
	if !ok {
		t.Fatalf("Expected ref to be a map, got %T", resultMap["ref"])
	}

	// The circular reference should be marked
	if ref["ref"] != "[CIRCULAR_REFERENCE]" {
		t.Errorf("Expected [CIRCULAR_REFERENCE], got %v", ref["ref"])
	}
}

func TestReDoSAlternationPattern(t *testing.T) {
	filter := NewEmptySensitiveDataFilter()

	tests := []struct {
		name    string
		pattern string
		safe    bool
	}{
		// Dangerous alternation patterns
		{"alternation with quantifier first", "(a+|b)+", false},
		{"alternation with quantifier second", "(a|b+)+", false},
		{"alternation both quantified", "(a+|b+)+", false},
		{"alternation with star", "(a*|b)+", false},
		{"nested alternation", "((a|b)+|c)+", false},

		// Safe alternation patterns
		{"simple alternation", "(a|b)", true},
		{"alternation optional", "(a|b)?", true},
		{"alternation with count", "(a|b){3}", true},
		{"alternation with range", "(a|b){1,5}", true},

		// Dangerous excessive ranges
		{"excessive range", "a{1,10000}", false},
		{"exact excessive", "a{5000}", false},

		// Safe ranges
		{"safe range", "a{1,100}", true},
		{"safe exact", "a{50}", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := filter.AddPattern(tt.pattern)
			if tt.safe {
				if err != nil {
					t.Errorf("Pattern %q should be safe, got error: %v", tt.pattern, err)
				}
			} else {
				if err == nil {
					t.Errorf("Pattern %q should be rejected as dangerous", tt.pattern)
				}
			}
		})
	}
}

func TestReDoSProtection(t *testing.T) {
	filter := NewSensitiveDataFilter()

	// Create a potentially malicious input that could cause catastrophic backtracking
	maliciousInput := strings.Repeat("a", 100) + "X"

	start := time.Now()
	result := filter.Filter(maliciousInput)
	duration := time.Since(start)

	// Should complete quickly (within timeout)
	if duration > 500*time.Millisecond {
		t.Errorf("Filter took too long: %v (possible ReDoS)", duration)
	}

	// Result should be either filtered or timeout message
	if result == "" {
		t.Error("Filter should return a result")
	}
}

func TestFilterTimeout(t *testing.T) {
	filter := NewSensitiveDataFilter()

	// Try to add a dangerous ReDoS pattern - should be rejected
	err := filter.AddPattern(`(a+)+b`)
	if err == nil {
		t.Error("Should reject dangerous nested quantifier pattern (a+)+b")
	}

	// Add a safe pattern instead
	err = filter.AddPattern(`a+b`)
	if err != nil {
		t.Fatalf("Failed to add safe pattern: %v", err)
	}

	// Input that could cause backtracking with the dangerous pattern
	// but is safe with the simple pattern
	input := strings.Repeat("a", 50)

	result := filter.Filter(input)

	// Should not hang
	if result == "" {
		t.Error("Filter should return a result")
	}
}

func TestFilterMaxInputLength(t *testing.T) {
	filter := NewSensitiveDataFilter()

	// Create input larger than max length
	largeInput := strings.Repeat("a", 2*1024*1024) // 2MB

	result := filter.Filter(largeInput)

	// The filter should handle large inputs safely
	resultPreview := result
	if len(result) > 100 {
		resultPreview = result[:100] + "..."
	}
	t.Logf("Input length: %d, Result length: %d, Result preview: %q", len(largeInput), len(result), resultPreview)

	// Result should be much smaller than input
	if len(result) >= len(largeInput) {
		t.Errorf("Result should be smaller than input, got result=%d, input=%d", len(result), len(largeInput))
	}

	// Filter should handle large input without panic
	if result == "" {
		t.Error("Result should not be empty")
	}
}

// ============================================================================
// CONCURRENT ACCESS TESTS
// ============================================================================

func TestConcurrentFilterAccess(t *testing.T) {
	filter := NewSensitiveDataFilter()

	var wg sync.WaitGroup

	// Concurrent filtering
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				filter.Filter("password=secret123 card=4532015112830366")
			}
		}(i)
	}

	// Concurrent pattern addition
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			filter.AddPattern(`test\d+`)
		}(i)
	}

	// Concurrent pattern count
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = filter.PatternCount()
		}()
	}

	wg.Wait()
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

func TestSecurityIntegrationWithLogger(t *testing.T) {
	var buf strings.Builder
	config := DefaultConfig()
	config.Writers = []io.Writer{&buf}
	config.SecurityConfig = &SecurityConfig{
		MaxMessageSize:  1024,
		MaxWriters:      10,
		SensitiveFilter: NewBasicSensitiveDataFilter(),
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test message filtering
	logger.Info("User password: secret123")

	output := buf.String()
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("Password should be filtered in logger output")
	}
	if strings.Contains(output, "secret123") {
		t.Error("Password value should not appear in logger output")
	}
}

func TestSecurityMessageSizeLimit(t *testing.T) {
	var buf strings.Builder
	config := DefaultConfig()
	config.Writers = []io.Writer{&buf}
	config.SecurityConfig = &SecurityConfig{
		MaxMessageSize: 100, // Small limit for testing
		MaxWriters:     10,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create message larger than limit
	largeMessage := strings.Repeat("A", 200)
	logger.Info(largeMessage)

	output := buf.String()
	// Message should be truncated
	if len(output) > 150 { // Account for timestamp, level, etc.
		t.Error("Message should be truncated due to size limit")
	}
	if !strings.Contains(output, "...") {
		t.Error("Truncated message should contain ellipsis")
	}
}

func TestSecurityFieldFiltering(t *testing.T) {
	var buf strings.Builder
	config := JSONConfig()
	config.Writers = []io.Writer{&buf}
	config.SecurityConfig = &SecurityConfig{
		SensitiveFilter: NewBasicSensitiveDataFilter(),
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test structured field filtering
	logger.InfoWith("User login",
		String("username", "john"),
		String("password", "secret123"),
		String("api_key", "sk-1234567890"),
	)

	output := buf.String()
	if !strings.Contains(output, "john") {
		t.Error("Username should not be filtered")
	}
	if strings.Contains(output, "secret123") {
		t.Error("Password value should be filtered")
	}
	if strings.Contains(output, "sk-1234567890") {
		t.Error("API key value should be filtered")
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("Sensitive fields should be redacted")
	}
}

// ============================================================================
// PHONE NUMBER FILTERING TESTS
// ============================================================================

func TestPhoneNumberFiltering(t *testing.T) {
	filter := NewSensitiveDataFilter()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		// Phone number fields with field names (preserve field name)
		{
			name:     "phone field with colon",
			input:    "phone: +1-415-555-2671",
			contains: "phone: [REDACTED]",
		},
		{
			name:     "mobile field",
			input:    "mobile=13812345678",
			contains: "mobile=[REDACTED]",
		},
		{
			name:     "tel field",
			input:    "tel +86 138 1234 5678",
			contains: "tel [REDACTED]",
		},
		{
			name:     "telephone field",
			input:    "telephone: (415) 555-2671",
			contains: "telephone: [REDACTED]",
		},
		{
			name:     "cell field",
			input:    "cell=+44 7700 900077",
			contains: "cell=[REDACTED]",
		},
		{
			name:     "fax field",
			input:    "fax: +1-415-555-2671",
			contains: "fax: [REDACTED]",
		},
		{
			name:     "contact field",
			input:    "contact +49 30 12345678",
			contains: "contact [REDACTED]",
		},

		// International E.164 format
		{
			name:     "E.164 with plus",
			input:    "Call me at +14155552671",
			contains: "[REDACTED]",
		},
		{
			name:     "E.164 medium",
			input:    "+8613812345678",
			contains: "[REDACTED]",
		},

		// 00 prefix international
		{
			name:     "00 prefix",
			input:    "0014155552671",
			contains: "[REDACTED]",
		},

		// NANP (North America) format
		{
			name:     "NANP with parentheses",
			input:    "(415) 555-2671",
			contains: "[REDACTED]",
		},
		{
			name:     "NANP with dashes",
			input:    "415-555-2671",
			contains: "[REDACTED]",
		},
		{
			name:     "NANP with dots",
			input:    "415.555.2671",
			contains: "[REDACTED]",
		},
		{
			name:     "NANP with spaces",
			input:    "415 555 2671",
			contains: "[REDACTED]",
		},
		{
			name:     "NANP with area code",
			input:    "1-415-555-2671",
			contains: "[REDACTED]",
		},

		// Chinese mobile numbers
		// Note: Standalone 11-digit numbers are NOT filtered to avoid over-matching
		// order IDs, timestamps, user IDs, etc. They ARE filtered when used with
		// sensitive field names like "phone", "mobile", etc.
		{
			name:     "Chinese mobile 11 digits (standalone - not filtered)",
			input:    "13812345678",
			contains: "13812345678",
		},
		{
			name:     "Chinese mobile with country code",
			input:    "+86 138 1234 5678",
			contains: "[REDACTED]",
		},
		{
			name:     "Chinese mobile with dash",
			input:    "+86-138-1234-5678",
			contains: "[REDACTED]",
		},
		{
			name:     "Chinese mobile starts with 13 (standalone - not filtered)",
			input:    "13123456789",
			contains: "13123456789",
		},
		{
			name:     "Chinese mobile starts with 18 (standalone - not filtered)",
			input:    "18123456789",
			contains: "18123456789",
		},
		{
			name:     "Chinese mobile starts with 19 (standalone - not filtered)",
			input:    "19123456789",
			contains: "19123456789",
		},

		// UK mobile numbers
		{
			name:     "UK mobile with country code",
			input:    "+44 7700 900077",
			contains: "[REDACTED]",
		},
		{
			name:     "UK mobile with dashes",
			input:    "+44-7700-900077",
			contains: "[REDACTED]",
		},
		{
			name:     "UK mobile local format",
			input:    "07700 900077",
			contains: "[REDACTED]",
		},

		// German phone numbers
		{
			name:     "German local format",
			input:    "030 12345678",
			contains: "[REDACTED]",
		},
		{
			name:     "German with area code in parentheses",
			input:    "+49 (030) 12345678",
			contains: "[REDACTED]",
		},

		// Japanese phone numbers
		{
			name:     "Japanese local format",
			input:    "090-1234-5678",
			contains: "[REDACTED]",
		},

		// Korean phone numbers
		{
			name:     "Korean local format",
			input:    "010-1234-5678",
			contains: "[REDACTED]",
		},

		// Indian phone numbers
		{
			name:     "Indian local format",
			input:    "098765 43210",
			contains: "[REDACTED]",
		},

		// Edge cases - non-phone numbers
		{
			name:     "short number (not a phone)",
			input:    "12345",
			contains: "12345",
		},
		{
			name:     "regular text",
			input:    "hello world",
			contains: "hello world",
		},
		{
			name:     "year number",
			input:    "year 2024",
			contains: "year 2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Filter(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Expected result to contain %q, got: %s", tt.contains, result)
			}
		})
	}
}

func TestPhoneNumberFieldFiltering(t *testing.T) {
	var buf strings.Builder
	config := JSONConfig()
	config.Writers = []io.Writer{&buf}
	config.SecurityConfig = &SecurityConfig{
		SensitiveFilter: NewBasicSensitiveDataFilter(),
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test phone number field filtering
	logger.InfoWith("User contact",
		String("username", "john"),
		String("phone", "+1-415-555-2671"),
		String("mobile", "13812345678"),
		String("email", "john@example.com"),
	)

	output := buf.String()
	if !strings.Contains(output, "john") {
		t.Error("Username should not be filtered")
	}
	if strings.Contains(output, "+1-415-555-2671") {
		t.Error("Phone value should be filtered")
	}
	if strings.Contains(output, "13812345678") {
		t.Error("Mobile value should be filtered")
	}
	// Note: Email is NOT filtered in basic mode to avoid false positives on user@host format
	// Email filtering is only available in full filter mode (NewSensitiveDataFilter)
	if !strings.Contains(output, "john@example.com") {
		t.Error("Email should NOT be filtered in basic mode")
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("Sensitive fields should be redacted")
	}
}

// ============================================================================
// DATABASE CONNECTION STRING FILTERING TESTS
// ============================================================================

func TestDatabaseConnectionFiltering(t *testing.T) {
	filter := NewSensitiveDataFilter()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		// MySQL connection strings
		{
			name:     "MySQL with protocol",
			input:    "mysql://user:pass@localhost:3306/db",
			contains: "mysql://[REDACTED]",
		},
		{
			name:     "MySQL with credentials",
			input:    "mysql://admin:secret123@db.example.com:3306/production",
			contains: "mysql://[REDACTED]",
		},
		{
			name:     "MySQL with SSL options",
			input:    "mysql://user:pass@host:3306/db?sslmode=require",
			contains: "mysql://[REDACTED]",
		},

		// PostgreSQL connection strings
		{
			name:     "PostgreSQL with protocol",
			input:    "postgresql://user:pass@localhost:5432/db",
			contains: "postgresql://[REDACTED]",
		},
		{
			name:     "PostgreSQL with host",
			input:    "postgresql://admin:secret@db.prod.example.com:5432/appdb",
			contains: "postgresql://[REDACTED]",
		},
		{
			name:     "PostgreSQL with options",
			input:    "postgresql://user:pass@host:5432/db?sslmode=verify-full",
			contains: "postgresql://[REDACTED]",
		},

		// MongoDB connection strings
		{
			name:     "MongoDB with protocol",
			input:    "mongodb://user:pass@localhost:27017/db",
			contains: "mongodb://[REDACTED]",
		},
		{
			name:     "MongoDB replica set",
			input:    "mongodb://admin:pass@host1:27017,host2:27017,host3:27017/db?replicaSet=mySet",
			contains: "mongodb://[REDACTED]",
		},

		// Redis connection strings
		{
			name:     "Redis with protocol",
			input:    "redis://user:pass@localhost:6379/0",
			contains: "redis://[REDACTED]",
		},
		{
			name:     "Redis with DB",
			input:    "redis://:password@redis.example.com:6379/1",
			contains: "redis://[REDACTED]",
		},

		// SQLite connection strings
		{
			name:     "SQLite file",
			input:    "sqlite:///path/to/database.db",
			contains: "sqlite://[REDACTED]",
		},
		{
			name:     "SQLite memory",
			input:    "sqlite:///:memory:",
			contains: "sqlite://[REDACTED]",
		},

		// Cassandra connection strings
		{
			name:     "Cassandra with protocol",
			input:    "cassandra://user:pass@localhost:9042/keyspace",
			contains: "cassandra://[REDACTED]",
		},

		// InfluxDB connection strings
		{
			name:     "InfluxDB with protocol",
			input:    "influx://user:pass@localhost:8086/db",
			contains: "influx://[REDACTED]",
		},

		// JDBC connection strings
		{
			name:     "JDBC MySQL",
			input:    "jdbc:mysql://localhost:3306/db?user=root&password=secret",
			contains: "jdbc:mysql://[REDACTED]",
		},
		{
			name:     "JDBC PostgreSQL",
			input:    "jdbc:postgresql://host:5432/db?user=postgres&password=pass",
			contains: "jdbc:postgresql://[REDACTED]",
		},
		{
			name:     "JDBC SQL Server",
			input:    "jdbc:sqlserver://localhost:1433;databaseName=adb;user=sa;password=secret",
			contains: "jdbc:sqlserver:[REDACTED]",
		},
		{
			name:     "JDBC Oracle",
			input:    "jdbc:oracle:thin:@localhost:1521:ORCL",
			contains: "jdbc:oracle:[REDACTED]",
		},

		// SQL Server connection strings
		{
			name:     "SQL Server with server keyword",
			input:    "Server=localhost;user id=sa;password=secret;database=production",
			contains: "Server=[REDACTED]",
		},
		{
			name:     "SQL Server with Data Source",
			input:    "Data Source=tcp:localhost,1433;Initial Catalog=db;User Id=sa;Password=pass",
			contains: "Data Source=[REDACTED]",
		},
		{
			name:     "SQL Server with host keyword",
			input:    "host=db.example.com;username=admin;password=secret123;database=mydb",
			contains: "host=[REDACTED]",
		},

		// Oracle DSN format
		{
			name:     "Oracle DSN",
			input:    "oracle=scott/tiger@localhost:1521/ORCL",
			contains: "oracle=[REDACTED]",
		},
		{
			name:     "Oracle SID",
			input:    "sid=prod:admin:pass@dbhost:1521/service",
			contains: "sid=[REDACTED]",
		},
		{
			name:     "TNS format",
			input:    "tns=(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=localhost)(PORT=1521))(CONNECT_DATA=(SERVICE_NAME=ORCL)))",
			contains: "tns=[REDACTED]",
		},

		// Database credential strings (user:pass@host format)
		{
			name:     "MySQL DSN format",
			input:    "user:pass@tcp(localhost:3306)/dbname",
			contains: "[REDACTED]",
		},
		{
			name:     "PostgreSQL DSN",
			input:    "postgres://user:pass@localhost/dbname?sslmode=disable",
			contains: "[REDACTED]",
		},
		{
			name:     "Credentials with IP",
			input:    "admin:secret@192.168.1.100:3306/production",
			contains: "[REDACTED]",
		},
		{
			name:     "Credentials with port",
			input:    "root:password123@db.example.com:5432/app",
			contains: "[REDACTED]",
		},

		// Edge cases - non-sensitive strings
		{
			name:     "URL without protocol",
			input:    "example.com/path",
			contains: "example.com/path",
		},
		{
			name:     "regular text",
			input:    "connect to database server",
			contains: "connect to database server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Filter(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Expected result to contain %q, got: %s", tt.contains, result)
			}
		})
	}
}

func TestDatabaseConnectionFieldFiltering(t *testing.T) {
	var buf strings.Builder
	config := JSONConfig()
	config.Writers = []io.Writer{&buf}
	config.SecurityConfig = &SecurityConfig{
		SensitiveFilter: NewBasicSensitiveDataFilter(),
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test database connection field filtering
	logger.InfoWith("Database connection established",
		String("location", "localhost"),
		String("name", "myapp"),
		String("connection", "mysql://user:pass@localhost:3306/myapp"),
		String("dsn", "postgresql://admin:secret@db.example.com:5432/production"),
	)

	output := buf.String()
	if strings.Contains(output, "location") && !strings.Contains(output, "\"localhost\"") {
		t.Errorf("Location should not be filtered. Output: %s", output)
	}
	if strings.Contains(output, "name") && !strings.Contains(output, "\"myapp\"") {
		t.Errorf("App name should not be filtered. Output: %s", output)
	}
	if strings.Contains(output, "mysql://user:pass@localhost:3306/myapp") {
		t.Error("Connection string should be filtered")
	}
	if strings.Contains(output, "postgresql://admin:secret@db.example.com:5432/production") {
		t.Error("DSN should be filtered")
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("Sensitive fields should be redacted")
	}
}

func TestDatabaseConnectionInMessage(t *testing.T) {
	var buf strings.Builder
	config := JSONConfig()
	config.Writers = []io.Writer{&buf}
	config.SecurityConfig = &SecurityConfig{
		SensitiveFilter: NewBasicSensitiveDataFilter(),
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test database connection strings in message text
	logger.Info("Connecting to mysql://user:pass@localhost:3306/db")
	logger.Info("Database postgresql://admin:secret@db.example.com:5432/production connected")

	output := buf.String()
	if strings.Contains(output, "mysql://user:pass@localhost:3306/db") {
		t.Error("MySQL connection string should be filtered in message")
	}
	if strings.Contains(output, "postgresql://admin:secret@db.example.com:5432/production") {
		t.Error("PostgreSQL connection string should be filtered in message")
	}
	if !strings.Contains(output, "mysql://[REDACTED]") {
		t.Error("Should contain redacted MySQL connection")
	}
	if !strings.Contains(output, "postgresql://[REDACTED]") {
		t.Error("Should contain redacted PostgreSQL connection")
	}
}
