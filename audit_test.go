package dd

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestAuditEventType_String(t *testing.T) {
	tests := []struct {
		eventType AuditEventType
		expected  string
	}{
		{AuditEventSensitiveDataRedacted, "SENSITIVE_DATA_REDACTED"},
		{AuditEventRateLimitExceeded, "RATE_LIMIT_EXCEEDED"},
		{AuditEventReDoSAttempt, "REDOS_ATTEMPT"},
		{AuditEventSecurityViolation, "SECURITY_VIOLATION"},
		{AuditEventIntegrityViolation, "INTEGRITY_VIOLATION"},
		{AuditEventInputSanitized, "INPUT_SANITIZED"},
		{AuditEventPathTraversalAttempt, "PATH_TRAVERSAL_ATTEMPT"},
		{AuditEventLog4ShellAttempt, "LOG4SHELL_ATTEMPT"},
		{AuditEventNullByteInjection, "NULL_BYTE_INJECTION"},
		{AuditEventOverlongEncoding, "OVERLONG_ENCODING"},
		{AuditEventHomographAttack, "HOMOGRAPH_ATTACK"},
		{AuditEventType(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.eventType.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAuditSeverity_String(t *testing.T) {
	tests := []struct {
		severity AuditSeverity
		expected string
	}{
		{AuditSeverityInfo, "INFO"},
		{AuditSeverityWarning, "WARNING"},
		{AuditSeverityError, "ERROR"},
		{AuditSeverityCritical, "CRITICAL"},
		{AuditSeverity(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAuditSeverity_MarshalJSON(t *testing.T) {
	severity := AuditSeverityError
	data, err := json.Marshal(severity)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}
	if string(data) != `"ERROR"` {
		t.Errorf("MarshalJSON() = %s, want %q", string(data), `"ERROR"`)
	}
}

func TestDefaultAuditConfig(t *testing.T) {
	config := DefaultAuditConfig()

	// Security enhancement: audit logging is now enabled by default
	if !config.Enabled {
		t.Error("Default config should have Enabled=true (security enhancement)")
	}
	if config.BufferSize != 1000 {
		t.Errorf("Default BufferSize = %d, want 1000", config.BufferSize)
	}
	if !config.IncludeTimestamp {
		t.Error("Default config should have IncludeTimestamp=true")
	}
	if !config.JSONFormat {
		t.Error("Default config should have JSONFormat=true")
	}
}

func TestAuditLogger_Log(t *testing.T) {
	// Test with nil output to just capture events in stats

	config := &AuditConfig{
		Enabled:          true,
		Output:           nil, // No output, just capture events
		BufferSize:       100,
		IncludeTimestamp: true,
		JSONFormat:       true,
		MinimumSeverity:  AuditSeverityInfo,
	}

	al := NewAuditLogger(config)
	defer al.Close()

	// Log some events
	al.Log(AuditEvent{
		Type:     AuditEventSensitiveDataRedacted,
		Message:  "Test redaction",
		Pattern:  `\b\d{16}\b`,
		Severity: AuditSeverityInfo,
	})

	// Wait for async processing
	time.Sleep(50 * time.Millisecond)

	stats := al.Stats()
	if stats.TotalEvents != 1 {
		t.Errorf("TotalEvents = %d, want 1", stats.TotalEvents)
	}
}

func TestAuditLogger_SeverityFilter(t *testing.T) {
	config := &AuditConfig{
		Enabled:         true,
		Output:          nil,
		BufferSize:      100,
		MinimumSeverity: AuditSeverityWarning,
	}

	al := NewAuditLogger(config)
	defer al.Close()

	// Log events below threshold - should be filtered
	al.Log(AuditEvent{
		Type:     AuditEventSensitiveDataRedacted,
		Message:  "Info event",
		Severity: AuditSeverityInfo,
	})

	// Log events at threshold
	al.Log(AuditEvent{
		Type:     AuditEventRateLimitExceeded,
		Message:  "Warning event",
		Severity: AuditSeverityWarning,
	})

	// Log events above threshold
	al.Log(AuditEvent{
		Type:     AuditEventSecurityViolation,
		Message:  "Error event",
		Severity: AuditSeverityError,
	})

	time.Sleep(50 * time.Millisecond)

	stats := al.Stats()
	if stats.TotalEvents != 2 {
		t.Errorf("TotalEvents = %d, want 2 (filtered info events)", stats.TotalEvents)
	}
}

func TestAuditLogger_HelperMethods(t *testing.T) {
	config := &AuditConfig{
		Enabled:         true,
		Output:          nil,
		BufferSize:      100,
		MinimumSeverity: AuditSeverityInfo,
	}

	al := NewAuditLogger(config)
	defer al.Close()

	// Test all helper methods
	al.LogSensitiveDataRedaction("pattern", "field", "message")
	al.LogRateLimitExceeded("rate limit", map[string]any{"count": 100})
	al.LogSecurityViolation("type", "message", map[string]any{"key": "value"})
	al.LogReDoSAttempt("pattern", "message")
	al.LogIntegrityViolation("message", map[string]any{"hash": "abc123"})
	al.LogPathTraversalAttempt("/etc/passwd", "path traversal detected")

	time.Sleep(50 * time.Millisecond)

	stats := al.Stats()
	if stats.TotalEvents != 6 {
		t.Errorf("TotalEvents = %d, want 6", stats.TotalEvents)
	}
}

func TestAuditLogger_BufferOverflow(t *testing.T) {
	config := &AuditConfig{
		Enabled:         true,
		Output:          nil,
		BufferSize:      10, // Small buffer
		MinimumSeverity: AuditSeverityInfo,
	}

	al := NewAuditLogger(config)
	defer al.Close()

	// Send more events than buffer can hold
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			al.Log(AuditEvent{
				Type:     AuditEventSensitiveDataRedacted,
				Message:  "Test event",
				Severity: AuditSeverityInfo,
			})
		}()
	}
	wg.Wait()

	stats := al.Stats()
	// Some events may have been dropped
	if stats.Dropped == 0 && stats.TotalEvents < 100 {
		t.Logf("Some events processed: TotalEvents=%d, Dropped=%d", stats.TotalEvents, stats.Dropped)
	}
}

func TestAuditLogger_Disabled(t *testing.T) {
	config := &AuditConfig{
		Enabled: false,
	}

	al := NewAuditLogger(config)
	defer al.Close()

	al.Log(AuditEvent{
		Type:     AuditEventSensitiveDataRedacted,
		Message:  "Test event",
		Severity: AuditSeverityInfo,
	})

	stats := al.Stats()
	if stats.TotalEvents != 0 {
		t.Errorf("Disabled logger should not log events, got %d", stats.TotalEvents)
	}
}

func TestAuditLogger_NilSafety(t *testing.T) {
	var al *AuditLogger

	// Should not panic
	al.Log(AuditEvent{Type: AuditEventSensitiveDataRedacted})
	al.LogSensitiveDataRedaction("pattern", "field", "message")
	al.Close()

	stats := al.Stats()
	if stats.TotalEvents != 0 {
		t.Error("Nil logger should return zero stats")
	}
}

func TestAuditLogger_Close(t *testing.T) {
	config := &AuditConfig{
		Enabled:         true,
		Output:          nil,
		BufferSize:      100,
		MinimumSeverity: AuditSeverityInfo,
	}

	al := NewAuditLogger(config)

	// Log some events
	for i := 0; i < 10; i++ {
		al.Log(AuditEvent{
			Type:     AuditEventSensitiveDataRedacted,
			Message:  "Test event",
			Severity: AuditSeverityInfo,
		})
	}

	// Close should flush remaining events
	err := al.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Double close should be safe
	err = al.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestAuditLogger_StatsByType(t *testing.T) {
	config := &AuditConfig{
		Enabled:         true,
		Output:          nil,
		BufferSize:      100,
		MinimumSeverity: AuditSeverityInfo,
	}

	al := NewAuditLogger(config)
	defer al.Close()

	// Log different types of events
	al.Log(AuditEvent{Type: AuditEventSensitiveDataRedacted, Severity: AuditSeverityInfo})
	al.Log(AuditEvent{Type: AuditEventSensitiveDataRedacted, Severity: AuditSeverityInfo})
	al.Log(AuditEvent{Type: AuditEventRateLimitExceeded, Severity: AuditSeverityInfo})
	al.Log(AuditEvent{Type: AuditEventSecurityViolation, Severity: AuditSeverityInfo})

	time.Sleep(50 * time.Millisecond)

	stats := al.Stats()
	if stats.ByType[AuditEventSensitiveDataRedacted] != 2 {
		t.Errorf("SensitiveDataRedacted count = %d, want 2", stats.ByType[AuditEventSensitiveDataRedacted])
	}
	if stats.ByType[AuditEventRateLimitExceeded] != 1 {
		t.Errorf("RateLimitExceeded count = %d, want 1", stats.ByType[AuditEventRateLimitExceeded])
	}
}

func TestAuditConfig_Clone(t *testing.T) {
	original := &AuditConfig{
		Enabled:          true,
		BufferSize:       500,
		IncludeTimestamp: false,
		JSONFormat:       false,
		MinimumSeverity:  AuditSeverityWarning,
	}

	cloned := original.Clone()

	if cloned == original {
		t.Error("Clone should return a new instance")
	}

	if cloned.BufferSize != original.BufferSize {
		t.Error("BufferSize should be copied")
	}

	// Modify original
	original.BufferSize = 999
	if cloned.BufferSize == 999 {
		t.Error("Clone should not be affected by original modifications")
	}
}

func TestAuditConfig_CloneNil(t *testing.T) {
	var config *AuditConfig
	cloned := config.Clone()
	if cloned != nil {
		t.Error("Cloning nil should return nil")
	}
}

func TestNewAuditLogger_NilConfig(t *testing.T) {
	al := NewAuditLogger(nil)

	if al == nil {
		t.Fatal("NewAuditLogger should not return nil")
	}

	if al.config.BufferSize != 1000 {
		t.Error("Nil config should use defaults")
	}

	al.Close()
}
