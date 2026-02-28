// Package dd provides field validation functionality for structured logging.
package dd

import (
	"fmt"
	"strings"
	"unicode"
)

// FieldValidationMode determines how field key validation is performed.
type FieldValidationMode int

const (
	// FieldValidationNone disables field key validation (default).
	// All field keys are accepted without any checks.
	FieldValidationNone FieldValidationMode = iota

	// FieldValidationWarn logs a warning for field keys that don't match
	// the configured naming convention, but still accepts them.
	FieldValidationWarn

	// FieldValidationStrict rejects field keys that don't match the configured
	// naming convention by returning an error from the logging method.
	// Note: For performance reasons, validation errors are logged rather than
	// returned from logging methods, as they don't return errors.
	FieldValidationStrict
)

// FieldNamingConvention specifies the expected naming convention for field keys.
type FieldNamingConvention int

const (
	// NamingConventionAny accepts any valid field key (default).
	NamingConventionAny FieldNamingConvention = iota

	// NamingConventionSnakeCase expects field keys in snake_case format.
	// Example: user_id, first_name, created_at
	NamingConventionSnakeCase

	// NamingConventionCamelCase expects field keys in camelCase format.
	// Example: userId, firstName, createdAt
	NamingConventionCamelCase

	// NamingConventionPascalCase expects field keys in PascalCase format.
	// Example: UserId, FirstName, CreatedAt
	NamingConventionPascalCase

	// NamingConventionKebabCase expects field keys in kebab-case format.
	// Example: user-id, first-name, created-at
	NamingConventionKebabCase
)

// String returns the string representation of the validation mode.
func (m FieldValidationMode) String() string {
	switch m {
	case FieldValidationNone:
		return "none"
	case FieldValidationWarn:
		return "warn"
	case FieldValidationStrict:
		return "strict"
	default:
		return "unknown"
	}
}

// String returns the string representation of the naming convention.
func (c FieldNamingConvention) String() string {
	switch c {
	case NamingConventionAny:
		return "any"
	case NamingConventionSnakeCase:
		return "snake_case"
	case NamingConventionCamelCase:
		return "camelCase"
	case NamingConventionPascalCase:
		return "PascalCase"
	case NamingConventionKebabCase:
		return "kebab-case"
	default:
		return "unknown"
	}
}

// FieldValidationConfig configures field key validation.
type FieldValidationConfig struct {
	// Mode determines how validation failures are handled.
	Mode FieldValidationMode

	// Convention specifies the expected naming convention for field keys.
	Convention FieldNamingConvention

	// AllowCommonAbbreviations allows common abbreviations like ID, URL, HTTP
	// even when they don't strictly match the naming convention.
	AllowCommonAbbreviations bool
}

// DefaultFieldValidationConfig returns the default field validation configuration
// which disables validation.
func DefaultFieldValidationConfig() *FieldValidationConfig {
	return &FieldValidationConfig{
		Mode:                     FieldValidationNone,
		Convention:               NamingConventionAny,
		AllowCommonAbbreviations: true,
	}
}

// StrictSnakeCaseConfig returns a config for strict snake_case validation.
func StrictSnakeCaseConfig() *FieldValidationConfig {
	return &FieldValidationConfig{
		Mode:                     FieldValidationStrict,
		Convention:               NamingConventionSnakeCase,
		AllowCommonAbbreviations: true,
	}
}

// StrictCamelCaseConfig returns a config for strict camelCase validation.
func StrictCamelCaseConfig() *FieldValidationConfig {
	return &FieldValidationConfig{
		Mode:                     FieldValidationStrict,
		Convention:               NamingConventionCamelCase,
		AllowCommonAbbreviations: true,
	}
}

// ValidateFieldKey validates a field key against the configured naming convention.
// Returns an error describing the validation failure, or nil if valid.
func (c *FieldValidationConfig) ValidateFieldKey(key string) error {
	if c == nil || c.Mode == FieldValidationNone || c.Convention == NamingConventionAny {
		return nil
	}

	if key == "" {
		return fmt.Errorf("field key cannot be empty")
	}

	// Check if it's a common abbreviation
	if c.AllowCommonAbbreviations && isCommonAbbreviation(key) {
		return nil
	}

	switch c.Convention {
	case NamingConventionSnakeCase:
		if !isValidSnakeCase(key) {
			return fmt.Errorf("field key %q does not match snake_case convention", key)
		}
	case NamingConventionCamelCase:
		if !isValidCamelCase(key) {
			return fmt.Errorf("field key %q does not match camelCase convention", key)
		}
	case NamingConventionPascalCase:
		if !isValidPascalCase(key) {
			return fmt.Errorf("field key %q does not match PascalCase convention", key)
		}
	case NamingConventionKebabCase:
		if !isValidKebabCase(key) {
			return fmt.Errorf("field key %q does not match kebab-case convention", key)
		}
	}

	return nil
}

// Common abbreviations that are allowed regardless of naming convention
var commonAbbreviations = map[string]bool{
	"id":    true,
	"ID":    true,
	"url":   true,
	"URL":   true,
	"uri":   true,
	"URI":   true,
	"http":  true,
	"HTTP":  true,
	"https": true,
	"HTTPS": true,
	"api":   true,
	"API":   true,
	"json":  true,
	"JSON":  true,
	"xml":   true,
	"XML":   true,
	"html":  true,
	"HTML":  true,
	"sql":   true,
	"SQL":   true,
	"ip":    true,
	"IP":    true,
	"tcp":   true,
	"TCP":   true,
	"udp":   true,
	"UDP":   true,
	"ssl":   true,
	"SSL":   true,
	"tls":   true,
	"TLS":   true,
	"jwt":   true,
	"JWT":   true,
	"oauth": true,
	"OAuth": true,
}

func isCommonAbbreviation(key string) bool {
	// Check exact match
	if commonAbbreviations[key] {
		return true
	}

	// Check if key ends with a common abbreviation suffix
	lowerKey := strings.ToLower(key)
	suffixes := []string{"_id", "_url", "_uri", "_ip", "_api"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(lowerKey, suffix) {
			prefix := key[:len(key)-len(suffix)]
			if len(prefix) > 0 {
				return true
			}
		}
	}

	return false
}

func isValidSnakeCase(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Must not start or end with underscore
	if s[0] == '_' || s[len(s)-1] == '_' {
		return false
	}

	// Must not have consecutive underscores
	hasUnderscore := false
	for i, r := range s {
		if r == '_' {
			if hasUnderscore {
				return false // Consecutive underscores
			}
			hasUnderscore = true
		} else {
			hasUnderscore = false
			// Must be lowercase letter or digit
			if !unicode.IsLower(r) && !unicode.IsDigit(r) {
				return false
			}
		}
		// First character must be a letter
		if i == 0 && unicode.IsDigit(r) {
			return false
		}
	}

	return true
}

func isValidCamelCase(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be lowercase letter
	firstRune := rune(s[0])
	if !unicode.IsLower(firstRune) {
		return false
	}

	// Must contain only letters and digits
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}

	return true
}

func isValidPascalCase(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be uppercase letter
	firstRune := rune(s[0])
	if !unicode.IsUpper(firstRune) {
		return false
	}

	// Must contain only letters and digits
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}

	return true
}

func isValidKebabCase(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Must not start or end with hyphen
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}

	// Must not have consecutive hyphens
	hasHyphen := false
	for i, r := range s {
		if r == '-' {
			if hasHyphen {
				return false // Consecutive hyphens
			}
			hasHyphen = true
		} else {
			hasHyphen = false
			// Must be lowercase letter or digit
			if !unicode.IsLower(r) && !unicode.IsDigit(r) {
				return false
			}
		}
		// First character must be a letter
		if i == 0 && unicode.IsDigit(r) {
			return false
		}
	}

	return true
}
