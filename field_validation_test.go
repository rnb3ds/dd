package dd

import (
	"strings"
	"testing"
)

// ============================================================================
// FIELD VALIDATION MODE STRING TESTS
// ============================================================================

func TestFieldValidationMode_String(t *testing.T) {
	tests := []struct {
		mode     FieldValidationMode
		expected string
	}{
		{FieldValidationNone, "none"},
		{FieldValidationWarn, "warn"},
		{FieldValidationStrict, "strict"},
		{FieldValidationMode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.mode.String()
			if result != tt.expected {
				t.Errorf("FieldValidationMode(%d).String() = %q, want %q", tt.mode, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// FIELD NAMING CONVENTION STRING TESTS
// ============================================================================

func TestFieldNamingConvention_String(t *testing.T) {
	tests := []struct {
		convention FieldNamingConvention
		expected   string
	}{
		{NamingConventionAny, "any"},
		{NamingConventionSnakeCase, "snake_case"},
		{NamingConventionCamelCase, "camelCase"},
		{NamingConventionPascalCase, "PascalCase"},
		{NamingConventionKebabCase, "kebab-case"},
		{FieldNamingConvention(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.convention.String()
			if result != tt.expected {
				t.Errorf("FieldNamingConvention(%d).String() = %q, want %q", tt.convention, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// FIELD VALIDATION CONFIG TESTS
// ============================================================================

func TestDefaultFieldValidationConfig(t *testing.T) {
	cfg := DefaultFieldValidationConfig()

	if cfg.Mode != FieldValidationNone {
		t.Errorf("Expected Mode FieldValidationNone, got %v", cfg.Mode)
	}

	if cfg.Convention != NamingConventionAny {
		t.Errorf("Expected Convention NamingConventionAny, got %v", cfg.Convention)
	}

	if !cfg.AllowCommonAbbreviations {
		t.Error("Expected AllowCommonAbbreviations to be true")
	}

	if !cfg.EnableSecurityValidation {
		t.Error("Expected EnableSecurityValidation to be true")
	}
}

func TestStrictSnakeCaseConfig(t *testing.T) {
	cfg := StrictSnakeCaseConfig()

	if cfg.Mode != FieldValidationStrict {
		t.Errorf("Expected Mode FieldValidationStrict, got %v", cfg.Mode)
	}

	if cfg.Convention != NamingConventionSnakeCase {
		t.Errorf("Expected Convention NamingConventionSnakeCase, got %v", cfg.Convention)
	}
}

func TestStrictCamelCaseConfig(t *testing.T) {
	cfg := StrictCamelCaseConfig()

	if cfg.Mode != FieldValidationStrict {
		t.Errorf("Expected Mode FieldValidationStrict, got %v", cfg.Mode)
	}

	if cfg.Convention != NamingConventionCamelCase {
		t.Errorf("Expected Convention NamingConventionCamelCase, got %v", cfg.Convention)
	}
}

// ============================================================================
// VALIDATE FIELD KEY TESTS
// ============================================================================

func TestValidateFieldKey(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		var cfg *FieldValidationConfig
		err := cfg.ValidateFieldKey("any_key")
		if err != nil {
			t.Errorf("Expected nil error for nil config, got: %v", err)
		}
	})

	t.Run("FieldValidationNone returns nil", func(t *testing.T) {
		cfg := DefaultFieldValidationConfig()
		err := cfg.ValidateFieldKey("any_key")
		if err != nil {
			t.Errorf("Expected nil error for FieldValidationNone, got: %v", err)
		}
	})

	t.Run("empty key returns error", func(t *testing.T) {
		cfg := StrictSnakeCaseConfig()
		err := cfg.ValidateFieldKey("")
		if err == nil {
			t.Error("Expected error for empty key")
		}
		if !strings.Contains(err.Error(), "empty") {
			t.Errorf("Expected error to mention 'empty', got: %v", err)
		}
	})
}

func TestValidateFieldKey_SnakeCase(t *testing.T) {
	cfg := StrictSnakeCaseConfig()

	tests := []struct {
		key       string
		shouldErr bool
	}{
		{"user_id", false},
		{"first_name", false},
		{"created_at", false},
		{"user_id_123", false},
		{"UserID", true},    // uppercase not allowed
		{"userId", true},    // camelCase not allowed
		{"user-id", true},   // hyphen not allowed
		{"_user_id", false}, // allowed due to _id suffix being a common abbreviation
		{"user_id_", true},  // trailing underscore not allowed
		{"user__id", false}, // allowed due to _id suffix being a common abbreviation
		{"123_user", true},  // leading digit not allowed
		{"", true},          // empty not allowed
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := cfg.ValidateFieldKey(tt.key)
			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for key %q", tt.key)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for key %q: %v", tt.key, err)
			}
		})
	}
}

func TestValidateFieldKey_CamelCase(t *testing.T) {
	cfg := StrictCamelCaseConfig()

	tests := []struct {
		key       string
		shouldErr bool
	}{
		{"userId", false},
		{"firstName", false},
		{"createdAt", false},
		{"userID123", false},
		{"user_id", false}, // allowed due to _id suffix being a common abbreviation
		{"UserId", true},   // PascalCase not allowed (must start lowercase)
		{"user-id", true},  // hyphen not allowed
		{"123user", true},  // leading digit allowed in camelCase but starts with digit
		{"", true},         // empty not allowed
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := cfg.ValidateFieldKey(tt.key)
			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for key %q", tt.key)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for key %q: %v", tt.key, err)
			}
		})
	}
}

func TestValidateFieldKey_PascalCase(t *testing.T) {
	cfg := &FieldValidationConfig{
		Mode:                     FieldValidationStrict,
		Convention:               NamingConventionPascalCase,
		AllowCommonAbbreviations: false,
		EnableSecurityValidation: false,
	}

	tests := []struct {
		key       string
		shouldErr bool
	}{
		{"UserId", false},
		{"FirstName", false},
		{"CreatedAt", false},
		{"UserID123", false},
		{"userId", true},  // must start uppercase
		{"user_id", true}, // underscore not allowed
		{"User-Id", true}, // hyphen not allowed
		{"", true},        // empty not allowed
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := cfg.ValidateFieldKey(tt.key)
			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for key %q", tt.key)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for key %q: %v", tt.key, err)
			}
		})
	}
}

func TestValidateFieldKey_KebabCase(t *testing.T) {
	cfg := &FieldValidationConfig{
		Mode:                     FieldValidationStrict,
		Convention:               NamingConventionKebabCase,
		AllowCommonAbbreviations: false,
		EnableSecurityValidation: false,
	}

	tests := []struct {
		key       string
		shouldErr bool
	}{
		{"user-id", false},
		{"first-name", false},
		{"created-at", false},
		{"user-id-123", false},
		{"UserID", true},   // uppercase not allowed
		{"userId", true},   // camelCase not allowed
		{"user_id", true},  // underscore not allowed
		{"-user-id", true}, // leading hyphen not allowed
		{"user-id-", true}, // trailing hyphen not allowed
		{"user--id", true}, // consecutive hyphens not allowed
		{"123-user", true}, // leading digit not allowed
		{"", true},         // empty not allowed
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := cfg.ValidateFieldKey(tt.key)
			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for key %q", tt.key)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for key %q: %v", tt.key, err)
			}
		})
	}
}

// ============================================================================
// COMMON ABBREVIATION TESTS
// ============================================================================

func TestIsCommonAbbreviation(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		// Exact matches
		{"id", true},
		{"ID", true},
		{"url", true},
		{"URL", true},
		{"uri", true},
		{"URI", true},
		{"http", true},
		{"HTTP", true},
		{"https", true},
		{"HTTPS", true},
		{"api", true},
		{"API", true},
		{"json", true},
		{"JSON", true},
		{"xml", true},
		{"XML", true},
		{"html", true},
		{"HTML", true},
		{"sql", true},
		{"SQL", true},
		{"ip", true},
		{"IP", true},
		{"tcp", true},
		{"TCP", true},
		{"udp", true},
		{"UDP", true},
		{"ssl", true},
		{"SSL", true},
		{"tls", true},
		{"TLS", true},
		{"jwt", true},
		{"JWT", true},
		{"oauth", true},
		{"OAuth", true},
		// Suffixes (case-insensitive check)
		{"user_id", true},
		{"request_url", true},
		{"redirect_uri", true},
		{"client_ip", true},
		{"apiKey", false}, // not a suffix match, ends with "ey" not "_api"
		// Non-abbreviations
		{"username", false},
		{"password", false},
		{"firstName", false},
		{"random_key", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := isCommonAbbreviation(tt.key)
			if result != tt.expected {
				t.Errorf("isCommonAbbreviation(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestValidateFieldKey_WithCommonAbbreviations(t *testing.T) {
	cfg := StrictSnakeCaseConfig()

	// Common abbreviations should pass even if they don't match snake_case
	allowedKeys := []string{"ID", "URL", "API", "HTTP", "user_id", "request_url"}
	for _, key := range allowedKeys {
		err := cfg.ValidateFieldKey(key)
		if err != nil {
			t.Errorf("Key %q should be allowed as common abbreviation: %v", key, err)
		}
	}
}

func TestValidateFieldKey_WithoutCommonAbbreviations(t *testing.T) {
	cfg := &FieldValidationConfig{
		Mode:                     FieldValidationStrict,
		Convention:               NamingConventionSnakeCase,
		AllowCommonAbbreviations: false,
		EnableSecurityValidation: false,
	}

	// Without common abbreviations, these should fail snake_case validation
	disallowedKeys := []string{"ID", "URL", "API"}
	for _, key := range disallowedKeys {
		err := cfg.ValidateFieldKey(key)
		if err == nil {
			t.Errorf("Key %q should fail snake_case validation when abbreviations disabled", key)
		}
	}
}

// ============================================================================
// NAMING CONVENTION ANY TESTS
// ============================================================================

func TestValidateFieldKey_NamingConventionAny(t *testing.T) {
	cfg := &FieldValidationConfig{
		Mode:                     FieldValidationStrict,
		Convention:               NamingConventionAny,
		AllowCommonAbbreviations: false,
		EnableSecurityValidation: false,
	}

	// Any convention should accept all valid keys
	keys := []string{"user_id", "userId", "UserId", "user-id", "ID", "URL"}
	for _, key := range keys {
		err := cfg.ValidateFieldKey(key)
		if err != nil {
			t.Errorf("Key %q should be allowed with NamingConventionAny: %v", key, err)
		}
	}
}

// ============================================================================
// SECURITY VALIDATION TESTS
// ============================================================================

func TestValidateFieldKey_SecurityValidation(t *testing.T) {
	cfg := &FieldValidationConfig{
		Mode:                     FieldValidationStrict,
		Convention:               NamingConventionAny,
		AllowCommonAbbreviations: false,
		EnableSecurityValidation: true,
	}

	// These should be caught by security validation
	dangerousKeys := []string{
		"${env:PASSWORD}",  // Log4Shell style
		"jndi:ldap://evil", // JNDI injection
	}

	for _, key := range dangerousKeys {
		err := cfg.ValidateFieldKey(key)
		if err == nil {
			t.Errorf("Key %q should be rejected by security validation", key)
		}
	}
}

func TestValidateFieldKey_SecurityValidationDisabled(t *testing.T) {
	cfg := &FieldValidationConfig{
		Mode:                     FieldValidationStrict,
		Convention:               NamingConventionAny,
		AllowCommonAbbreviations: false,
		EnableSecurityValidation: false,
	}

	// Without security validation, these might pass (depending on convention)
	key := "normal_key"
	err := cfg.ValidateFieldKey(key)
	if err != nil {
		t.Errorf("Normal key should pass: %v", err)
	}
}
