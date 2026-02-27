package dd

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
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
	{`(?i)((?:password|passwd|pwd|secret)[\s:=]+)[^\s]{1,128}\b`, true},
	{`(?i)((?:token|api[_-]?key|bearer)[\s:=]+)[^\s]{1,256}\b`, true},
	{`\beyJ[A-Za-z0-9_-]{10,100}\.eyJ[A-Za-z0-9_-]{10,100}\.[A-Za-z0-9_-]{10,100}\b`, false},
	{`-----BEGIN[^-]{1,20}PRIVATE\s+KEY-----[A-Za-z0-9+/=\s]{1,4000}-----END[^-]{1,20}PRIVATE\s+KEY-----`, true},
	// API keys
	{`\bAKIA[0-9A-Z]{16}\b`, false},
	{`\bAIza[A-Za-z0-9_-]{35}\b`, false},
	{`\bsk-[A-Za-z0-9]{16,48}\b`, true},
	// Email - only in full filter mode to avoid false positives on user@host format
	{`\b[A-Za-z0-9._%+-]{1,64}@[A-Za-z0-9.-]{1,253}\.[A-Za-z]{2,6}\b`, false},
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
	{`\b\d{3,5}[- ]\d{4,8}\b`, false},                           // Phone numbers with separators (7-13 digits total) - moved to full filter to avoid false positives on dates
	{`\b0\d{3,5}[- ]?\d{4,8}\b`, true},                          // Starting with 0 and separators (10+ digits total)
}

// Pre-compiled regex cache to avoid repeated compilation
var (
	compiledFullPatterns  []*regexp.Regexp
	compiledBasicPatterns []*regexp.Regexp
	patternsOnce          sync.Once
)

// initPatterns initializes the pre-compiled regex patterns.
// This is called once on first use to avoid startup overhead.
func initPatterns() {
	patternsOnce.Do(func() {
		compiledFullPatterns = make([]*regexp.Regexp, 0, len(allPatterns))
		compiledBasicPatterns = make([]*regexp.Regexp, 0, len(allPatterns))

		for _, pd := range allPatterns {
			// Skip ReDoS check for built-in patterns (already validated)
			re, err := regexp.Compile(pd.pattern)
			if err != nil {
				// Output warning in debug/test mode for built-in pattern compilation failures
				if os.Getenv("DD_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "dd: warning: failed to compile pattern %q: %v\n", pd.pattern, err)
				}
				continue
			}
			compiledFullPatterns = append(compiledFullPatterns, re)
			if pd.basic {
				compiledBasicPatterns = append(compiledBasicPatterns, re)
			}
		}
	})
}

type SensitiveDataFilter struct {
	// patternsPtr stores an immutable slice of patterns using atomic pointer.
	// This eliminates slice copying during filter operations (hot path).
	// The slice is replaced atomically when patterns are added/removed.
	patternsPtr    atomic.Pointer[[]*regexp.Regexp]
	mu             sync.RWMutex // protects pattern modifications
	maxInputLength int
	timeout        time.Duration
	enabled        atomic.Bool
	// semaphore limits concurrent regex filtering goroutines to prevent resource exhaustion
	semaphore chan struct{}
	// activeGoroutines tracks the number of currently running filter goroutines
	activeGoroutines atomic.Int32
}

func NewSensitiveDataFilter() *SensitiveDataFilter {
	// Ensure patterns are initialized (only happens once)
	initPatterns()

	filter := &SensitiveDataFilter{
		maxInputLength: MaxInputLength,
		timeout:        DefaultFilterTimeout,
		semaphore:      make(chan struct{}, MaxConcurrentFilters),
	}
	filter.enabled.Store(true)

	// Create and store patterns slice atomically
	patterns := make([]*regexp.Regexp, len(compiledFullPatterns))
	copy(patterns, compiledFullPatterns)
	filter.patternsPtr.Store(&patterns)

	return filter
}

func NewEmptySensitiveDataFilter() *SensitiveDataFilter {
	filter := &SensitiveDataFilter{
		maxInputLength: MaxInputLength,
		timeout:        EmptyFilterTimeout,
		semaphore:      make(chan struct{}, MaxConcurrentFilters),
	}
	filter.enabled.Store(true)
	// Initialize with empty patterns slice
	emptyPatterns := make([]*regexp.Regexp, 0)
	filter.patternsPtr.Store(&emptyPatterns)
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
	if len(pattern) > MaxPatternLength {
		return fmt.Errorf("%w: %d exceeds maximum %d", ErrPatternTooLong, len(pattern), MaxPatternLength)
	}

	if hasNestedQuantifiers(pattern) {
		return ErrReDoSPattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPattern, err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Load current patterns, create new slice with added pattern, store atomically
	currentPatterns := f.patternsPtr.Load()
	newPatterns := make([]*regexp.Regexp, len(*currentPatterns)+1)
	copy(newPatterns, *currentPatterns)
	newPatterns[len(*currentPatterns)] = re
	f.patternsPtr.Store(&newPatterns)

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
	return strconv.Atoi(s)
}

func (f *SensitiveDataFilter) AddPattern(pattern string) error {
	if f == nil {
		return ErrNilFilter
	}
	if pattern == "" {
		return ErrEmptyPattern
	}
	return f.addPattern(pattern)
}

func (f *SensitiveDataFilter) AddPatterns(patterns ...string) error {
	if f == nil {
		return ErrNilFilter
	}
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		if err := f.addPattern(pattern); err != nil {
			return fmt.Errorf("%w: %q", ErrPatternFailed, pattern)
		}
	}
	return nil
}

func (f *SensitiveDataFilter) ClearPatterns() {
	f.mu.Lock()
	defer f.mu.Unlock()
	emptyPatterns := make([]*regexp.Regexp, 0)
	f.patternsPtr.Store(&emptyPatterns)
}

func (f *SensitiveDataFilter) PatternCount() int {
	patterns := f.patternsPtr.Load()
	if patterns == nil {
		return 0
	}
	return len(*patterns)
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

// ActiveGoroutineCount returns the number of currently active filter goroutines.
// This can be used for monitoring and detecting potential goroutine leaks in
// high-concurrency scenarios. A consistently high count may indicate that
// filter operations are timing out frequently.
func (f *SensitiveDataFilter) ActiveGoroutineCount() int32 {
	if f == nil {
		return 0
	}
	return f.activeGoroutines.Load()
}

// WaitForGoroutines waits for all active filter goroutines to complete or until
// the timeout is reached. This is useful for graceful shutdown to ensure all
// background filtering operations have finished.
// Returns true if all goroutines completed, false if timeout was reached.
func (f *SensitiveDataFilter) WaitForGoroutines(timeout time.Duration) bool {
	if f == nil {
		return true
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if f.activeGoroutines.Load() == 0 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return f.activeGoroutines.Load() == 0
}

// Clone creates a copy of the SensitiveDataFilter.
//
// Deep copy:
//   - patterns slice (but the compiled *regexp.Regexp instances are shared)
//
// New instances:
//   - semaphore channel (new channel with same capacity)
//
// Returns nil if the receiver is nil.
func (f *SensitiveDataFilter) Clone() *SensitiveDataFilter {
	if f == nil {
		return nil
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	currentPatterns := f.patternsPtr.Load()
	clone := &SensitiveDataFilter{
		maxInputLength: f.maxInputLength,
		timeout:        f.timeout,
		semaphore:      make(chan struct{}, MaxConcurrentFilters),
	}
	clone.enabled.Store(f.enabled.Load())

	// Copy patterns slice and store atomically
	patterns := make([]*regexp.Regexp, len(*currentPatterns))
	copy(patterns, *currentPatterns)
	clone.patternsPtr.Store(&patterns)

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

	// Truncate input to prevent resource exhaustion attacks.
	// Note: Truncation happens BEFORE filtering for performance reasons.
	// This is a deliberate trade-off: processing extremely large inputs
	// could cause ReDoS or memory issues. The truncation limit (256KB)
	// is large enough that most legitimate messages won't be affected.
	// Sensitive data patterns spanning the truncation boundary may not
	// be fully redacted, but this edge case is acceptable given the
	// security benefits of input size limiting.
	if f.maxInputLength > 0 && inputLen > f.maxInputLength {
		input = input[:f.maxInputLength] + "... [TRUNCATED FOR SECURITY]"
	}

	// Fast path: atomic load of patterns pointer (lock-free read)
	patternsPtr := f.patternsPtr.Load()
	if patternsPtr == nil || len(*patternsPtr) == 0 {
		return input
	}

	patterns := *patternsPtr
	timeout := f.timeout

	result := input
	for i := range patterns {
		result = f.filterWithTimeout(result, patterns[i], timeout)
		// Early exit if result becomes empty or redacted
		if result == "" || result == "[REDACTED]" {
			break
		}
	}

	return result
}

// filterWithTimeout applies regex filtering with timeout protection for large inputs.
//
// The function uses a tiered approach based on input size:
// - Small inputs (< FastPathThreshold): Direct synchronous processing
// - Medium inputs (< 10*FastPathThreshold): Synchronous chunked processing with context
// - Large inputs: Async processing with timeout and semaphore-based concurrency limiting
//
// For large inputs, a goroutine is spawned for regex processing. The context is passed
// to allow early termination on timeout. The semaphore limits concurrent goroutines
// to prevent resource exhaustion.
func (f *SensitiveDataFilter) filterWithTimeout(input string, pattern *regexp.Regexp, timeout time.Duration) string {
	inputLen := len(input)

	if inputLen < FastPathThreshold {
		return f.replaceWithPattern(input, pattern)
	}

	if inputLen < 10*FastPathThreshold {
		return f.filterInChunksWithContext(context.Background(), input, pattern)
	}

	// Try to acquire semaphore with timeout to limit concurrent goroutines
	select {
	case f.semaphore <- struct{}{}:
		defer func() { <-f.semaphore }()
	case <-time.After(timeout / 2):
		// Could not acquire semaphore within half the timeout, return [REDACTED] for safety
		return "[REDACTED]"
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		output string
	}
	done := make(chan result, 1)

	f.activeGoroutines.Add(1)
	go func() {
		defer f.activeGoroutines.Add(-1)
		defer func() {
			if r := recover(); r != nil {
				select {
				case done <- result{output: "[REDACTED]"}:
				default:
				}
			}
		}()

		output := f.filterInChunksWithContext(ctx, input, pattern)
		select {
		case done <- result{output: output}:
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

// filterInChunksWithContext processes input in chunks with context support for early termination.
func (f *SensitiveDataFilter) filterInChunksWithContext(ctx context.Context, input string, pattern *regexp.Regexp) string {
	const chunkSize = 4096

	inputLen := len(input)

	if inputLen <= chunkSize {
		return f.replaceWithPattern(input, pattern)
	}

	var result strings.Builder
	result.Grow(inputLen)

	for i := 0; i < inputLen; i += chunkSize {
		select {
		case <-ctx.Done():
			return result.String()
		default:
		}

		end := min(i+chunkSize, inputLen)
		chunk := input[i:end]
		filtered := f.replaceWithPattern(chunk, pattern)
		result.WriteString(filtered)
	}

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

	// Fast path: try exact match with inline ASCII lowercase comparison
	// This avoids strings.ToLower allocation for exact matches
	keyLen := len(key)

	// Check exact match in both maps using inline lowercase comparison
	// For short keys (< 64 bytes), use stack-allocated buffer
	if keyLen <= 64 {
		var lowerBuf [64]byte
		for i := 0; i < keyLen; i++ {
			c := key[i]
			if c >= 'A' && c <= 'Z' {
				c += 32 // ASCII lowercase conversion
			}
			lowerBuf[i] = c
		}
		lowerKey := string(lowerBuf[:keyLen])

		// Check exact match for all keywords
		if _, exists := sensitiveKeywords[lowerKey]; exists {
			return true
		}
		if _, exists := exactMatchOnlyKeywords[lowerKey]; exists {
			return true
		}

		// Substring match for compound keys like "user_password", "api_key_secret", etc.
		for keyword := range sensitiveKeywords {
			if strings.Contains(lowerKey, keyword) {
				return true
			}
		}
		return false
	}

	// Slow path: for long keys, use strings.ToLower
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

// Clone creates a copy of the SecurityConfig.
//
// Deep copy:
//   - SensitiveFilter (via SensitiveDataFilter.Clone())
//
// Returns nil if the receiver is nil.
func (sc *SecurityConfig) Clone() *SecurityConfig {
	if sc == nil {
		return nil
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
	// Ensure patterns are initialized (only happens once)
	initPatterns()

	filter := &SensitiveDataFilter{
		maxInputLength: MaxInputLength,
		timeout:        DefaultFilterTimeout,
		semaphore:      make(chan struct{}, MaxConcurrentFilters),
	}
	filter.enabled.Store(true)

	// Create and store patterns slice atomically
	patterns := make([]*regexp.Regexp, len(compiledBasicPatterns))
	copy(patterns, compiledBasicPatterns)
	filter.patternsPtr.Store(&patterns)

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
