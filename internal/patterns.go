package internal

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// PatternDefinition represents a regex pattern for sensitive data detection.
type PatternDefinition struct {
	Pattern string
	Basic   bool // Included in basic filter
}

// AllPatterns is the centralized registry of all security patterns.
var AllPatterns = []PatternDefinition{
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
	{`(?i)((?:phone|mobile|tel|telephone|cell|cellular|fax|contact|number)[\s:=]+)[\+]?[(]?\d{1,4}[)]?[-\s.]?\(?\d{1,4}\)?[-\s.]?\d{1,9}[-\s.]?\d{0,9}\b`, true},
	{`\+\d{1,3}[- ]?\d{6,14}\b`, true},                          // International: +XXXXXXXXXXXX (7-15 digits after +)
	{`\+[\d\s\-\(\)]{7,}\b`, true},                              // International phone with + and formatting (7+ chars total)
	{`\b00[1-9]\d{6,14}\b`, true},                               // 00 prefix international (8-16 digits total)
	{`\b(?:\(\d{3}\)\s?|\d{3}[-.\s])\d{3}[-.\s]?\d{4}\b`, true}, // NANP with required separator: (415) 555-2671 or 415-555-2671
	{`\b\d{3,5}[- ]\d{4,8}\b`, false},                           // Phone numbers with separators (7-13 digits total) - moved to full filter to avoid false positives on dates
	{`\b0\d{3,5}[- ]?\d{4,8}\b`, true},                          // Starting with 0 and separators (10+ digits total)
}

// Pre-compiled regex cache to avoid repeated compilation.
var (
	CompiledFullPatterns  []*regexp.Regexp
	CompiledBasicPatterns []*regexp.Regexp
	PatternsOnce          sync.Once
)

// InitPatterns initializes the pre-compiled regex patterns.
// This is called once on first use to avoid startup overhead.
func InitPatterns() {
	PatternsOnce.Do(func() {
		CompiledFullPatterns = make([]*regexp.Regexp, 0, len(AllPatterns))
		CompiledBasicPatterns = make([]*regexp.Regexp, 0, len(AllPatterns))

		for _, pd := range AllPatterns {
			// Skip ReDoS check for built-in patterns (already validated)
			re, err := regexp.Compile(pd.Pattern)
			if err != nil {
				// Output warning in debug/test mode for built-in pattern compilation failures
				if os.Getenv("DD_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "dd: warning: failed to compile pattern %q: %v\n", pd.Pattern, err)
				}
				continue
			}
			CompiledFullPatterns = append(CompiledFullPatterns, re)
			if pd.Basic {
				CompiledBasicPatterns = append(CompiledBasicPatterns, re)
			}
		}
	})
}

// HasNestedQuantifiers checks for regex patterns with nested quantifiers
// that can cause exponential backtracking (ReDoS vulnerability).
// Returns true if dangerous patterns like (a+)+, a++, or a{1,10000} are found.
func HasNestedQuantifiers(pattern string, maxQuantifierRange int) bool {
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
				if err := ValidateQuantifierRange(rangeContent, maxQuantifierRange); err != nil {
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

// ValidateQuantifierRange checks if a quantifier range is within safe limits.
func ValidateQuantifierRange(rangeStr string, maxQuantifierRange int) error {
	parts := strings.Split(rangeStr, ",")

	// Parse the maximum value
	var maxVal int
	var err error

	if len(parts) == 1 {
		// Exact count: {n}
		maxVal, err = ParseInt(parts[0])
	} else if len(parts) == 2 {
		// Range: {n,m} or {n,}
		if parts[1] == "" {
			// Open-ended range {n,} - dangerous, but handled elsewhere
			return nil
		}
		maxVal, err = ParseInt(parts[1])
	} else {
		return fmt.Errorf("invalid quantifier range")
	}

	if err != nil {
		return err
	}

	if maxVal > maxQuantifierRange {
		return fmt.Errorf("quantifier range %d exceeds maximum %d", maxVal, maxQuantifierRange)
	}

	return nil
}

// ParseInt safely parses an integer from a string.
func ParseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}
	return strconv.Atoi(s)
}

// SensitiveKeywords contains field names that indicate sensitive data.
// These keywords support both exact match and substring matching.
// For short keywords that may cause false positives (e.g., "db", "url"),
// use ExactMatchOnlyKeywords instead.
//
// Categories:
//   - Credentials: password, passwd, pwd, secret, token, bearer, auth, authorization
//   - API Keys: api_key, apikey, api-key, access_key, accesskey, access-key
//   - Secrets: secret_key, secretkey, secret-key, private_key, privatekey, private-key
//   - PII: credit_card, creditcard, ssn, social_security
//   - Contact: phone, telephone, mobile, cell, cellular, tel, fax, contact
var SensitiveKeywords = map[string]struct{}{
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

// ExactMatchOnlyKeywords contains keywords that should only match exactly.
// These are typically short words that could cause false positives with substring matching.
// For example, "db" should not match "mongodb", and "url" should not match "curl".
var ExactMatchOnlyKeywords = map[string]struct{}{
	// Short words that need exact matching to avoid false positives
	"conn": {},
	"dsn":  {},
	"db":   {},
	"host": {},
	"uri":  {},
	"url":  {},
}

// IsSensitiveKey checks if a key indicates sensitive data.
// It uses both exact match and substring matching for comprehensive detection.
func IsSensitiveKey(key string) bool {
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
		if _, exists := SensitiveKeywords[lowerKey]; exists {
			return true
		}
		if _, exists := ExactMatchOnlyKeywords[lowerKey]; exists {
			return true
		}

		// Substring match for compound keys like "user_password", "api_key_secret", etc.
		for keyword := range SensitiveKeywords {
			if strings.Contains(lowerKey, keyword) {
				return true
			}
		}
		return false
	}

	// Slow path: for long keys, use strings.ToLower
	lowerKey := strings.ToLower(key)

	// Check exact match for all keywords
	if _, exists := SensitiveKeywords[lowerKey]; exists {
		return true
	}
	if _, exists := ExactMatchOnlyKeywords[lowerKey]; exists {
		return true
	}

	// Substring match for compound keys like "user_password", "api_key_secret", etc.
	// Only use SensitiveKeywords (not ExactMatchOnlyKeywords) for substring matching
	for keyword := range SensitiveKeywords {
		if strings.Contains(lowerKey, keyword) {
			return true
		}
	}
	return false
}
