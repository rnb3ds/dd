package dd

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type patternDefinition struct {
	pattern string
	basic   bool // Included in basic filter
}

// allPatterns is the centralized registry of all security patterns
var allPatterns = []patternDefinition{
	// Credit card and SSN patterns
	{`\b[0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{3,7}\b`, true},
	{`\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b`, true},
	{`(?i)((?:credit[_-]?card|card)[\s:=]+)[0-9]{13,19}\b`, true},
	// Credentials and secrets
	{`(?i)((?:password|passwd|pwd|secret)[\s:=]+)[^\s]{1,32}\b`, true},
	{`(?i)((?:token|api[_-]?key|bearer)[\s:=]+)[^\s]{1,128}\b`, true},
	{`\beyJ[A-Za-z0-9_-]{10,100}\.eyJ[A-Za-z0-9_-]{10,100}\.[A-Za-z0-9_-]{10,100}\b`, false},
	{`-----BEGIN[^-]{1,20}PRIVATE\s+KEY-----[A-Za-z0-9+/=\s]{1,4000}-----END[^-]{1,20}PRIVATE\s+KEY-----`, true},
	// API keys
	{`\bAKIA[0-9A-Z]{16}\b`, false},
	{`\bAIza[A-Za-z0-9_-]{35}\b`, false},
	{`\bsk-[A-Za-z0-9]{16,48}\b`, true},
	// Email
	{`\b[A-Za-z0-9._%+-]{1,64}@[A-Za-z0-9.-]{1,253}\.[A-Za-z]{2,6}\b`, true},
	// IP addresses
	{`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`, false},
	// Database connection strings - preserve protocol name
	{`(?i)((?:mysql|postgresql|mongodb|redis|sqlite|cassandra|influx|cockroach|timescale|postgres)://)[^\s]{1,200}\b`, true},
	// JDBC connection strings - preserve jdbc:prefix
	{`(?i)((?:jdbc:)(?:mysql|postgresql|sqlserver|oracle|mongodb|redis|cassandra)://)[^\s]{1,200}\b`, false},
	{`(?i)((?:server|data source|host)[\s=:]+)[^\s;]{1,200}(?:;|\s|$)`, false},
	{`(?i)((?:oracle|tns|sid)[\s=:]+)[^\s]{1,100}\b`, false},
	{`(?i)(?:[\w.-]+:[\w.-]+@)(?:[\w.-]+|\([^\)]+\))(?::\d+)?(?:/[\w.-]+)?`, false},
	// Phone numbers - global patterns
	// These patterns are designed to be specific enough to avoid false positives
	// on short numeric strings like "12345" or years like "2024".
	// Standalone 11-digit numbers (e.g., Chinese mobile) are NOT filtered to avoid
	// matching order IDs, timestamps, or user IDs. Use field-based filtering
	// (phone:, mobile:, etc.) for those cases.
	{`(?i)((?:phone|mobile|tel|telephone|cell|cellular|fax|contact|number)[\s:=]+)[\+]?[(]?\d{1,4}[)]?[-\s.]?\(?\d{1,4}\)?[-\s.]?\d{1,9}[-\s.]?\d{0,9}\b`, true},
	{`\+\d{1,3}[- ]?\d{6,14}\b`, true},                          // International: +XXXXXXXXXXXX (7-15 digits after +)
	{`\+[\d\s\-\(\)]{7,}\b`, true},                              // International phone with + and formatting (7+ chars total)
	{`\b00[1-9]\d{6,14}\b`, true},                               // 00 prefix international (8-16 digits total)
	{`\b(?:\(\d{3}\)\s?|\d{3}[-.\s])\d{3}[-.\s]?\d{4}\b`, true}, // NANP with required separator: (415) 555-2671 or 415-555-2671
	{`\b\d{3,5}[- ]\d{4,8}\b`, true},                            // Phone numbers with separators (7-13 digits total)
	{`\b0\d{3,5}[- ]?\d{4,8}\b`, true},                          // Starting with 0 and separators (10+ digits total)
}

type SensitiveDataFilter struct {
	patterns       []*regexp.Regexp
	mu             sync.RWMutex
	maxInputLength int
	timeout        time.Duration
	enabled        atomic.Bool
}

func NewSensitiveDataFilter() *SensitiveDataFilter {
	filter := &SensitiveDataFilter{
		patterns:       make([]*regexp.Regexp, 0, len(allPatterns)),
		maxInputLength: MaxInputLength,
		timeout:        DefaultFilterTimeout,
	}
	filter.enabled.Store(true)

	for _, pd := range allPatterns {
		_ = filter.addPattern(pd.pattern)
	}

	return filter
}

func NewEmptySensitiveDataFilter() *SensitiveDataFilter {
	filter := &SensitiveDataFilter{
		patterns:       make([]*regexp.Regexp, 0),
		maxInputLength: MaxInputLength,
		timeout:        EmptyFilterTimeout,
	}
	filter.enabled.Store(true)
	return filter
}

func NewCustomSensitiveDataFilter(patterns ...string) (*SensitiveDataFilter, error) {
	filter := NewEmptySensitiveDataFilter()

	for _, pattern := range patterns {
		if err := filter.AddPattern(pattern); err != nil {
			return nil, err
		}
	}

	return filter, nil
}

func (f *SensitiveDataFilter) addPattern(pattern string) error {
	// Validate pattern length to prevent ReDoS attacks
	if len(pattern) > MaxPatternLength {
		return fmt.Errorf("%w: pattern length %d exceeds maximum %d", ErrInvalidPattern, len(pattern), MaxPatternLength)
	}

	// Check for nested quantifiers that could cause exponential backtracking (ReDoS)
	if hasNestedQuantifiers(pattern) {
		return fmt.Errorf("%w: pattern contains dangerous nested quantifiers that may cause ReDoS", ErrInvalidPattern)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPattern, err)
	}

	f.mu.Lock()
	f.patterns = append(f.patterns, re)
	f.mu.Unlock()

	return nil
}

// hasNestedQuantifiers checks for regex patterns with nested quantifiers
// that can cause exponential backtracking (ReDoS vulnerability).
// Returns true if dangerous patterns like (a+)+, a++, or a{1,10000} are found.
func hasNestedQuantifiers(pattern string) bool {
	// Track consecutive quantifiers
	prevWasQuantifier := false

	// Track if the content inside a group ends with a quantifier
	// This helps detect (a+)+ patterns
	groupEndsWithQuantifier := make(map[int]bool)
	// Track if a group contains alternation with quantified parts
	groupHasQuantifiedAlternation := make(map[int]bool)
	depth := 0

	for i := 0; i < len(pattern); i++ {
		c := pattern[i]

		switch c {
		case '(':
			depth++
			prevWasQuantifier = false
			groupEndsWithQuantifier[depth] = false
			groupHasQuantifiedAlternation[depth] = false
		case ')':
			if depth > 0 {
				// Check if this group is followed by a repeating quantifier (+, *, {n,})
				// AND the group content ends with a quantifier or has quantified alternation
				if i+1 < len(pattern) && (groupEndsWithQuantifier[depth] || groupHasQuantifiedAlternation[depth]) {
					next := pattern[i+1]
					// Only + and * are dangerous when applied to a quantified group
					// ? is safe because it's optional (no repetition)
					if next == '+' || next == '*' {
						return true
					}
					if next == '{' {
						// Check for {0,} or {1,} which are equivalent to * or +
						end := strings.Index(pattern[i+1:], "}")
						if end != -1 {
							rangeContent := pattern[i+2 : i+1+end]
							if strings.HasSuffix(rangeContent, ",") ||
								strings.Contains(rangeContent, ",") && !strings.Contains(rangeContent[len(strings.Split(rangeContent, ",")[0]):], "0") {
								// Patterns like {1,} or {0,} can cause backtracking
								return true
							}
						}
					}
				}
				delete(groupEndsWithQuantifier, depth)
				delete(groupHasQuantifiedAlternation, depth)
				depth--
			}
			prevWasQuantifier = false
		case '|':
			// Alternation - if we have a quantifier before this, mark the group
			if depth > 0 && prevWasQuantifier {
				groupHasQuantifiedAlternation[depth] = true
			}
			prevWasQuantifier = false
		case '+', '*', '?':
			// Check for consecutive quantifiers (e.g., a++, a*?)
			if prevWasQuantifier {
				return true
			}
			// Mark that current depth ends with a quantifier
			if depth > 0 {
				groupEndsWithQuantifier[depth] = true
			}
			prevWasQuantifier = true
		case '{':
			// Find the closing brace
			end := strings.Index(pattern[i:], "}")
			if end != -1 {
				// Check for consecutive quantifier like a{1,2}+
				if prevWasQuantifier {
					return true
				}

				// Check for excessive quantifier range
				rangeContent := pattern[i+1 : i+end]
				if err := validateQuantifierRange(rangeContent); err != nil {
					return true
				}

				// Mark that current depth ends with a quantifier
				if depth > 0 {
					groupEndsWithQuantifier[depth] = true
				}
				prevWasQuantifier = true
				i += end
			}
		default:
			// Reset for non-special characters (but not for \, |, ^, $, ., [, ])
			if c != '\\' && c != '|' && c != '^' && c != '$' && c != '.' {
				prevWasQuantifier = false
			}
		}
	}

	return false
}

// validateQuantifierRange checks if a quantifier range is within safe limits.
func validateQuantifierRange(rangeStr string) error {
	parts := strings.Split(rangeStr, ",")

	// Parse the maximum value
	var maxVal int
	var err error

	if len(parts) == 1 {
		// Exact count: {n}
		maxVal, err = parseInt(parts[0])
	} else if len(parts) == 2 {
		// Range: {n,m} or {n,}
		if parts[1] == "" {
			// Open-ended range {n,} - dangerous, but handled elsewhere
			return nil
		}
		maxVal, err = parseInt(parts[1])
	} else {
		return fmt.Errorf("invalid quantifier range")
	}

	if err != nil {
		return err
	}

	if maxVal > MaxQuantifierRange {
		return fmt.Errorf("quantifier range %d exceeds maximum %d", maxVal, MaxQuantifierRange)
	}

	return nil
}

// parseInt safely parses an integer from a string.
func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}

	var result int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid character in number")
		}
		result = result*10 + int(c-'0')
		// Early exit for very large numbers
		if result > MaxQuantifierRange*10 {
			return result, nil
		}
	}
	return result, nil
}

func (f *SensitiveDataFilter) AddPattern(pattern string) error {
	if f == nil {
		return fmt.Errorf("filter is nil")
	}
	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}
	return f.addPattern(pattern)
}

func (f *SensitiveDataFilter) AddPatterns(patterns ...string) error {
	if f == nil {
		return fmt.Errorf("filter is nil")
	}
	for _, pattern := range patterns {
		if pattern == "" {
			continue // Skip empty patterns
		}
		if err := f.addPattern(pattern); err != nil {
			return fmt.Errorf("failed to add pattern %q: %w", pattern, err)
		}
	}
	return nil
}

func (f *SensitiveDataFilter) ClearPatterns() {
	f.mu.Lock()
	f.patterns = make([]*regexp.Regexp, 0)
	f.mu.Unlock()
}

func (f *SensitiveDataFilter) PatternCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.patterns)
}

func (f *SensitiveDataFilter) Enable() {
	if f != nil {
		f.enabled.Store(true)
	}
}

func (f *SensitiveDataFilter) Disable() {
	if f != nil {
		f.enabled.Store(false)
	}
}

func (f *SensitiveDataFilter) IsEnabled() bool {
	if f == nil {
		return false
	}
	return f.enabled.Load()
}

func (f *SensitiveDataFilter) Clone() *SensitiveDataFilter {
	if f == nil {
		return nil
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	clone := &SensitiveDataFilter{
		patterns:       make([]*regexp.Regexp, len(f.patterns)),
		maxInputLength: f.maxInputLength,
		timeout:        f.timeout,
	}
	clone.enabled.Store(f.enabled.Load())
	copy(clone.patterns, f.patterns)

	return clone
}

func (f *SensitiveDataFilter) Filter(input string) string {
	if f == nil || !f.enabled.Load() {
		return input
	}

	inputLen := len(input)
	if inputLen == 0 {
		return input
	}

	if f.maxInputLength > 0 && inputLen > f.maxInputLength {
		input = input[:f.maxInputLength] + "... [TRUNCATED FOR SECURITY]"
	}

	f.mu.RLock()
	patternCount := len(f.patterns)
	if patternCount == 0 {
		f.mu.RUnlock()
		return input
	}

	patterns := make([]*regexp.Regexp, patternCount)
	copy(patterns, f.patterns)
	timeout := f.timeout
	f.mu.RUnlock()

	result := input
	for i := range patternCount {
		result = f.filterWithTimeout(result, patterns[i], timeout)
		// Early exit if result becomes empty or redacted
		if result == "" || result == "[REDACTED]" {
			break
		}
	}

	return result
}

func (f *SensitiveDataFilter) filterWithTimeout(input string, pattern *regexp.Regexp, timeout time.Duration) string {
	inputLen := len(input)

	if inputLen < FastPathThreshold {
		return f.replaceWithPattern(input, pattern)
	}

	if inputLen < 10*FastPathThreshold {
		return f.filterInChunks(input, pattern)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		output string
		panic  bool
	}
	done := make(chan result, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- result{output: "[REDACTED]", panic: true}
			}
		}()
		output := f.filterInChunks(input, pattern)
		select {
		case done <- result{output: output, panic: false}:
		case <-ctx.Done():
		}
	}()

	select {
	case res := <-done:
		return res.output
	case <-ctx.Done():
		// Note: The goroutine may continue running in the background.
		// For safety, we return [REDACTED] to ensure sensitive data is not leaked.
		return "[REDACTED]"
	}
}

func (f *SensitiveDataFilter) filterInChunks(input string, pattern *regexp.Regexp) string {
	const chunkSize = 4096

	inputLen := len(input)

	if inputLen <= chunkSize {
		return f.replaceWithPattern(input, pattern)
	}

	// Process input in non-overlapping chunks for efficiency.
	// Patterns spanning chunk boundaries will be caught by the final pass.
	var result strings.Builder
	result.Grow(inputLen)

	for i := 0; i < inputLen; i += chunkSize {
		end := min(i+chunkSize, inputLen)
		chunk := input[i:end]
		filtered := f.replaceWithPattern(chunk, pattern)
		result.WriteString(filtered)
	}

	// Final pass to catch any patterns that might have been split across chunk boundaries
	resultStr := result.String()
	return f.replaceWithPattern(resultStr, pattern)
}

func (f *SensitiveDataFilter) replaceWithPattern(input string, pattern *regexp.Regexp) string {
	if pattern.NumSubexp() > 0 {
		return pattern.ReplaceAllString(input, "$1[REDACTED]")
	}
	return pattern.ReplaceAllString(input, "[REDACTED]")
}

// sensitiveKeywords contains field names that indicate sensitive data.
// These keywords support both exact match and substring matching.
// For short keywords that may cause false positives (e.g., "db", "url"),
// use exactMatchOnlyKeywords instead.
//
// Categories:
//   - Credentials: password, passwd, pwd, secret, token, bearer, auth, authorization
//   - API Keys: api_key, apikey, api-key, access_key, accesskey, access-key
//   - Secrets: secret_key, secretkey, secret-key, private_key, privatekey, private-key
//   - PII: credit_card, creditcard, ssn, social_security
//   - Contact: phone, telephone, mobile, cell, cellular, tel, fax, contact
var sensitiveKeywords = map[string]struct{}{
	// Credentials
	"password":      {},
	"passwd":        {},
	"pwd":           {},
	"secret":        {},
	"token":         {},
	"bearer":        {},
	"auth":          {},
	"authorization": {},

	// API Keys
	"api_key":    {},
	"apikey":     {},
	"api-key":    {},
	"access_key": {},
	"accesskey":  {},
	"access-key": {},

	// Secrets
	"secret_key":  {},
	"secretkey":   {},
	"secret-key":  {},
	"private_key": {},
	"privatekey":  {},
	"private-key": {},

	// Personal Identifiable Information (PII)
	"credit_card":     {},
	"creditcard":      {},
	"ssn":             {},
	"social_security": {},

	// Contact Information
	"phone":        {},
	"telephone":    {},
	"mobile":       {},
	"cell":         {},
	"cellular":     {},
	"tel":          {},
	"fax":          {},
	"contact":      {},
	"phonenumber":  {},
	"phone_number": {},

	// Database/Connection Strings (longer forms that are less likely to cause false positives)
	"connection": {},
	"database":   {},
	"hostname":   {},
	"endpoint":   {},
}

// exactMatchOnlyKeywords contains keywords that should only match exactly.
// These are typically short words that could cause false positives with substring matching.
// For example, "db" should not match "mongodb", and "url" should not match "curl".
var exactMatchOnlyKeywords = map[string]struct{}{
	// Short words that need exact matching to avoid false positives
	"conn": {},
	"dsn":  {},
	"db":   {},
	"host": {},
	"uri":  {},
	"url":  {},
}

func isSensitiveKey(key string) bool {
	if key == "" {
		return false
	}
	lowerKey := strings.ToLower(key)

	// Check exact match for all keywords
	if _, exists := sensitiveKeywords[lowerKey]; exists {
		return true
	}
	if _, exists := exactMatchOnlyKeywords[lowerKey]; exists {
		return true
	}

	// Substring match for compound keys like "user_password", "api_key_secret", etc.
	// Only use sensitiveKeywords (not exactMatchOnlyKeywords) for substring matching
	for keyword := range sensitiveKeywords {
		if strings.Contains(lowerKey, keyword) {
			return true
		}
	}
	return false
}

func (f *SensitiveDataFilter) FilterFieldValue(key string, value any) any {
	if f == nil || !f.enabled.Load() {
		return value
	}

	str, ok := value.(string)
	if !ok {
		return value
	}

	if isSensitiveKey(key) {
		return "[REDACTED]"
	}

	return f.Filter(str)
}

// FilterValueRecursive recursively filters sensitive data from nested structures.
// It processes maps, slices, arrays, and structs to filter sensitive values.
// Circular references are detected and replaced with "[CIRCULAR_REFERENCE]".
// Maximum recursion depth is limited to MaxRecursionDepth to prevent stack overflow.
func (f *SensitiveDataFilter) FilterValueRecursive(key string, value any) any {
	return f.filterValueRecursiveInternal(key, value, make(map[uintptr]bool), 0)
}

// filterValueRecursiveInternal is the internal implementation with circular reference detection.
func (f *SensitiveDataFilter) filterValueRecursiveInternal(key string, value any, visited map[uintptr]bool, depth int) any {
	if f == nil || !f.enabled.Load() {
		return value
	}

	// Check recursion depth to prevent stack overflow on deeply nested structures
	if depth > MaxRecursionDepth {
		return "[MAX_DEPTH_EXCEEDED]"
	}

	// Handle nil values
	if value == nil {
		return nil
	}

	// Check if the key itself is sensitive
	if isSensitiveKey(key) {
		return "[REDACTED]"
	}

	// Handle string values directly
	if str, ok := value.(string); ok {
		return f.Filter(str)
	}

	// Use reflection for complex types
	val := reflect.ValueOf(value)
	if !val.IsValid() {
		return value
	}

	kind := val.Kind()

	// Handle pointers - check for circular references
	if kind == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		ptr := val.Pointer()
		if visited[ptr] {
			return "[CIRCULAR_REFERENCE]"
		}
		visited[ptr] = true
		return f.filterValueRecursiveInternal(key, val.Elem().Interface(), visited, depth+1)
	}

	// Handle interfaces
	if kind == reflect.Interface {
		if val.IsNil() {
			return nil
		}
		return f.filterValueRecursiveInternal(key, val.Elem().Interface(), visited, depth+1)
	}

	// Handle slices and arrays
	if kind == reflect.Slice || kind == reflect.Array {
		if val.Len() == 0 {
			if kind == reflect.Slice {
				return []any{}
			}
			return value
		}
		// Check for circular reference in slice pointer
		if kind == reflect.Slice {
			ptr := val.Pointer()
			if visited[ptr] {
				return "[CIRCULAR_REFERENCE]"
			}
			visited[ptr] = true
		}
		result := make([]any, val.Len())
		for i := 0; i < val.Len(); i++ {
			result[i] = f.filterValueRecursiveInternal("", val.Index(i).Interface(), visited, depth+1)
		}
		return result
	}

	// Handle maps
	if kind == reflect.Map {
		if val.IsNil() {
			return nil
		}
		// Check for circular reference in map pointer
		ptr := val.Pointer()
		if visited[ptr] {
			return "[CIRCULAR_REFERENCE]"
		}
		visited[ptr] = true
		result := make(map[string]any, val.Len())
		for _, mapKey := range val.MapKeys() {
			keyStr := fmt.Sprintf("%v", mapKey.Interface())
			mapValue := val.MapIndex(mapKey).Interface()
			result[keyStr] = f.filterValueRecursiveInternal(keyStr, mapValue, visited, depth+1)
		}
		return result
	}

	// Handle structs
	if kind == reflect.Struct {
		result := make(map[string]any)
		typ := val.Type()
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			fieldType := typ.Field(i)

			// Skip unexported fields
			if !field.CanInterface() {
				continue
			}

			fieldName := fieldType.Name
			// Check for json tag
			if tag := fieldType.Tag.Get("json"); tag != "" && tag != "-" {
				if commaIdx := strings.Index(tag, ","); commaIdx >= 0 {
					if tagName := tag[:commaIdx]; tagName != "" {
						fieldName = tagName
					}
				} else if tag != "" {
					fieldName = tag
				}
			}

			result[fieldName] = f.filterValueRecursiveInternal(fieldName, field.Interface(), visited, depth+1)
		}
		return result
	}

	// For other types, return as-is
	return value
}

type SecurityConfig struct {
	MaxMessageSize  int
	MaxWriters      int
	SensitiveFilter *SensitiveDataFilter
}

func (sc *SecurityConfig) Clone() *SecurityConfig {
	if sc == nil {
		return DefaultSecurityConfig()
	}

	clone := &SecurityConfig{
		MaxMessageSize: sc.MaxMessageSize,
		MaxWriters:     sc.MaxWriters,
	}
	if sc.SensitiveFilter != nil {
		clone.SensitiveFilter = sc.SensitiveFilter.Clone()
	}
	return clone
}

func NewBasicSensitiveDataFilter() *SensitiveDataFilter {
	filter := &SensitiveDataFilter{
		patterns:       make([]*regexp.Regexp, 0, 20),
		maxInputLength: MaxInputLength,
		timeout:        DefaultFilterTimeout,
	}
	filter.enabled.Store(true)

	for _, pd := range allPatterns {
		if pd.basic {
			_ = filter.addPattern(pd.pattern)
		}
	}

	return filter
}

// DefaultSecurityConfig returns a security config with basic sensitive data filtering enabled.
// This provides out-of-the-box protection for common sensitive data like passwords,
// API keys, credit cards, and phone numbers.
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: NewBasicSensitiveDataFilter(),
	}
}

// SecureSecurityConfig returns a security config with full sensitive data filtering enabled.
// This includes all patterns from basic filtering plus additional patterns for
// emails, IP addresses, JWT tokens, and database connection strings.
// Use this for maximum security in production environments.
func SecureSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: NewSensitiveDataFilter(),
	}
}
