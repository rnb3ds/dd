package dd

import (
	"context"
	"fmt"
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
	{`(?i)((?:phone|mobile|tel|telephone|cell|cellular|fax|contact|number)[\s:=]+)[\+]?[(]?\d{1,4}[)]?[-\s.]?\(?\d{1,4}\)?[-\s.]?\d{1,9}[-\s.]?\d{0,9}\b`, true},
	{`\+\d{1,3}[- ]?\d{6,14}\b`, true},                            // International: +XXXXXXXXXXXX (without word boundary)
	{`\+[\d\s\-\(\)]{7,}\b`, true},                                // International phone with + and any format (7+ chars total)
	{`\b00[1-9]\d{6,14}\b`, true},                                 // 00 prefix international
	{`\b(?:\(\d{3}\)\s?|\d{3}[-.\s]?)?\d{3}[-.\s]?\d{4}\b`, true}, // Minimum 7 digits for NANP
	{`\b\d{10,}\b`, true},                                         // Standalone 10+ digit numbers (matches most international formats)
	{`\b\d{3,5}[- ]\d{4,8}\b`, true},                              // Phone numbers with separators (7-13 digits total)
	{`\b0\d{3,5}[- ]?\d{4,8}\b`, true},                            // Starting with 0 and separators (10+ digits total)
	// General international format with various separators (minimum 7 digits)
	{`\b(?:\+\d{1,3}[-\s]?)?\(?\d{1,4}\)?[-\s]?\d{3,4}([-]\s]?\d{0,4})?\b`, true},
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
// that can cause exponential backtracking (ReDoS vulnerability)
func hasNestedQuantifiers(pattern string) bool {
	// Look for patterns like (a+)+, (a*)*, (a+)?+, etc.
	// These can cause catastrophic backtracking
	nestedCount := 0
	inGroup := false

	for i := 0; i < len(pattern); i++ {
		c := pattern[i]

		switch c {
		case '(':
			inGroup = true
		case ')':
			if inGroup {
				inGroup = false
			}
		case '+', '*', '?':
			// Check if this quantifier is followed by another quantifier
			if i+1 < len(pattern) {
				next := pattern[i+1]
				if next == '+' || next == '*' || next == '?' || next == '{' {
					return true // Nested quantifier found
				}
			}
			// Check for quantifier after group
			if inGroup && i > 0 && (pattern[i-1] == ')' || pattern[i-1] == ']' || pattern[i-1] == '}') {
				nestedCount++
				if nestedCount > 1 {
					return true // Multiple nested quantifiers
				}
			}
		}
	}

	return false
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
		return "[REDACTED]"
	}
}

func (f *SensitiveDataFilter) filterInChunks(input string, pattern *regexp.Regexp) string {
	const chunkSize = 1024
	inputLen := len(input)

	if inputLen <= chunkSize {
		return f.replaceWithPattern(input, pattern)
	}

	var result strings.Builder
	result.Grow(inputLen)

	for i := 0; i < inputLen; i += chunkSize {
		end := min(i+chunkSize, inputLen)
		chunk := input[i:end]
		filtered := f.replaceWithPattern(chunk, pattern)
		result.WriteString(filtered)
	}

	return result.String()
}

func (f *SensitiveDataFilter) replaceWithPattern(input string, pattern *regexp.Regexp) string {
	if pattern.NumSubexp() > 0 {
		return pattern.ReplaceAllString(input, "$1[REDACTED]")
	}
	return pattern.ReplaceAllString(input, "[REDACTED]")
}

func (f *SensitiveDataFilter) FilterValue(value any) any {
	if f == nil || !f.enabled.Load() {
		return value
	}
	if str, ok := value.(string); ok {
		return f.Filter(str)
	}
	return value
}

var sensitiveKeywords = map[string]struct{}{
	"password":        {},
	"passwd":          {},
	"pwd":             {},
	"secret":          {},
	"token":           {},
	"bearer":          {},
	"api_key":         {},
	"apikey":          {},
	"api-key":         {},
	"access_key":      {},
	"accesskey":       {},
	"access-key":      {},
	"secret_key":      {},
	"secretkey":       {},
	"secret-key":      {},
	"private_key":     {},
	"privatekey":      {},
	"private-key":     {},
	"auth":            {},
	"authorization":   {},
	"credit_card":     {},
	"creditcard":      {},
	"ssn":             {},
	"social_security": {},
	"phone":           {},
	"telephone":       {},
	"mobile":          {},
	"cell":            {},
	"cellular":        {},
	"tel":             {},
	"fax":             {},
	"contact":         {},
	"phonenumber":     {},
	"phone_number":    {},
	"connection":      {},
	"conn":            {},
	"dsn":             {},
	"database":        {},
	"db":              {},
	"host":            {},
	"hostname":        {},
	"server":          {},
	"uri":             {},
	"url":             {},
	"endpoint":        {},
}

func isSensitiveKey(key string) bool {
	if key == "" {
		return false
	}
	lowerKey := strings.ToLower(key)

	// Direct match check
	if _, exists := sensitiveKeywords[lowerKey]; exists {
		return true
	}

	// Substring match check for compound keys like "api_key", "access_token", etc.
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

func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: nil,
	}
}

func SecureSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: NewSensitiveDataFilter(),
	}
}
