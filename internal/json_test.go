package internal

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFormatMessage(t *testing.T) {
	tests := []struct {
		name            string
		level           LogLevel
		includeTime     bool
		timeFormat      string
		includeLevel    bool
		includeCaller   bool
		callerDepth     int
		fullPath        bool
		message         string
		fields          map[string]any
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:          "basic message",
			level:         LevelInfo,
			includeTime:   false,
			includeLevel:  true,
			includeCaller: false,
			message:       "test message",
			fields:        nil,
			wantContains:  []string{`"level":"INFO"`, `"message":"test message"`},
		},
		{
			name:          "with time",
			level:         LevelError,
			includeTime:   true,
			timeFormat:    time.RFC3339,
			includeLevel:  true,
			includeCaller: false,
			message:       "error occurred",
			fields:        nil,
			wantContains:  []string{`"level":"ERROR"`, `"message":"error occurred"`, `"timestamp"`},
		},
		{
			name:          "with fields",
			level:         LevelWarn,
			includeTime:   false,
			includeLevel:  true,
			includeCaller: false,
			message:       "warning",
			fields:        map[string]any{"key": "value", "count": 42},
			wantContains:  []string{`"level":"WARN"`, `"message":"warning"`, `"fields"`, `"key":"value"`, `"count":42`},
		},
		{
			name:          "with caller",
			level:         LevelDebug,
			includeTime:   false,
			includeLevel:  true,
			includeCaller: true,
			callerDepth:   1,
			fullPath:      false,
			message:       "debug info",
			fields:        nil,
			wantContains:  []string{`"level":"DEBUG"`, `"message":"debug info"`, `"caller"`},
		},
		{
			name:            "minimal config",
			level:           LevelInfo,
			includeTime:     false,
			includeLevel:    false,
			includeCaller:   false,
			message:         "minimal",
			fields:          nil,
			wantContains:    []string{`"message":"minimal"`},
			wantNotContains: []string{`"level"`, `"timestamp"`, `"caller"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMessageWithOptions(
				tt.level,
				tt.includeTime,
				tt.timeFormat,
				tt.includeLevel,
				tt.includeCaller,
				tt.callerDepth,
				tt.fullPath,
				tt.message,
				tt.fields,
				nil,
			)

			// Verify it's valid Json
			var jsonData map[string]any
			if err := json.Unmarshal([]byte(result), &jsonData); err != nil {
				t.Fatalf("Result is not valid Json: %v", err)
			}

			// Check required content
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("Result should contain %q, got: %s", want, result)
				}
			}

			// Check excluded content
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(result, notWant) {
					t.Errorf("Result should not contain %q, got: %s", notWant, result)
				}
			}
		})
	}
}

func TestFormatMessageWithOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    *JSONOptions
		wantErr bool
	}{
		{
			name: "pretty print",
			opts: &JSONOptions{
				PrettyPrint: true,
				Indent:      "  ",
				FieldNames:  DefaultJSONFieldNames(),
			},
			wantErr: false,
		},
		{
			name: "custom field names",
			opts: &JSONOptions{
				PrettyPrint: false,
				FieldNames: &JSONFieldNames{
					Timestamp: "ts",
					Level:     "severity",
					Caller:    "source",
					Message:   "msg",
					Fields:    "data",
				},
			},
			wantErr: false,
		},
		{
			name:    "nil options",
			opts:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMessageWithOptions(
				LevelInfo,
				true,
				time.RFC3339,
				true,
				true,
				1,
				false,
				"test message",
				map[string]any{"key": "value"},
				tt.opts,
			)

			// Verify it's valid Json
			var jsonData map[string]any
			if err := json.Unmarshal([]byte(result), &jsonData); err != nil {
				t.Fatalf("Result is not valid Json: %v", err)
			}

			// Test pretty print
			if tt.opts != nil && tt.opts.PrettyPrint {
				if !strings.Contains(result, "\n") {
					t.Error("Pretty print should contain newlines")
				}
			}

			// Test custom field names
			if tt.opts != nil && tt.opts.FieldNames != nil {
				if tt.opts.FieldNames.Level == "severity" {
					if !strings.Contains(result, `"severity"`) {
						t.Error("Should use custom field name 'severity'")
					}
					if strings.Contains(result, `"level"`) {
						t.Error("Should not use default field name 'level'")
					}
				}
			}
		})
	}
}

func TestLevelToString(t *testing.T) {
	tests := []struct {
		level LogLevel
		want  string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.want {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCaller(t *testing.T) {
	// Test with full path (depth 1 to get this test function)
	callerInfo := GetCaller(1, true)
	if !strings.Contains(callerInfo, "json_test.go") {
		t.Errorf("GetCaller(true) should contain file name, got: %s", callerInfo)
	}
	if !strings.Contains(callerInfo, ":") {
		t.Error("GetCaller() should contain line number")
	}

	// Test without full path
	callerInfo = GetCaller(1, false)
	if !strings.Contains(callerInfo, "json_test.go") {
		t.Errorf("GetCaller(false) should contain file name, got: %s", callerInfo)
	}
	// Should not contain full path
	if strings.Contains(callerInfo, "/") || strings.Contains(callerInfo, "\\") {
		t.Errorf("GetCaller(false) should not contain path separators, got: %s", callerInfo)
	}

	// Test with invalid depth
	callerInfo = GetCaller(100, false)
	if callerInfo != "" {
		t.Errorf("GetCaller(100) should return empty string, got: %s", callerInfo)
	}
}

func TestDefaultJSONFieldNames(t *testing.T) {
	names := DefaultJSONFieldNames()

	if names.Timestamp != "timestamp" {
		t.Errorf("Timestamp = %q, want 'timestamp'", names.Timestamp)
	}
	if names.Level != "level" {
		t.Errorf("Level = %q, want 'level'", names.Level)
	}
	if names.Caller != "caller" {
		t.Errorf("Caller = %q, want 'caller'", names.Caller)
	}
	if names.Message != "message" {
		t.Errorf("Message = %q, want 'message'", names.Message)
	}
	if names.Fields != "fields" {
		t.Errorf("Fields = %q, want 'fields'", names.Fields)
	}
}

func TestJSONFieldNamesMergeWithDefaults(t *testing.T) {
	tests := []struct {
		name   string
		input  *JSONFieldNames
		verify func(*testing.T, *JSONFieldNames)
	}{
		{
			name:  "nil input",
			input: nil,
			verify: func(t *testing.T, result *JSONFieldNames) {
				defaults := DefaultJSONFieldNames()
				if result.Timestamp != defaults.Timestamp {
					t.Error("Should use default timestamp")
				}
			},
		},
		{
			name: "partial custom",
			input: &JSONFieldNames{
				Level:   "severity",
				Message: "msg",
				// Others empty - should use defaults
			},
			verify: func(t *testing.T, result *JSONFieldNames) {
				if result.Level != "severity" {
					t.Error("Should use custom level")
				}
				if result.Message != "msg" {
					t.Error("Should use custom message")
				}
				if result.Timestamp != "timestamp" {
					t.Error("Should use default timestamp")
				}
				if result.Caller != "caller" {
					t.Error("Should use default caller")
				}
				if result.Fields != "fields" {
					t.Error("Should use default fields")
				}
			},
		},
		{
			name: "all custom",
			input: &JSONFieldNames{
				Timestamp: "ts",
				Level:     "lvl",
				Caller:    "src",
				Message:   "msg",
				Fields:    "data",
			},
			verify: func(t *testing.T, result *JSONFieldNames) {
				if result.Timestamp != "ts" {
					t.Error("Should use custom timestamp")
				}
				if result.Level != "lvl" {
					t.Error("Should use custom level")
				}
				if result.Caller != "src" {
					t.Error("Should use custom caller")
				}
				if result.Message != "msg" {
					t.Error("Should use custom message")
				}
				if result.Fields != "data" {
					t.Error("Should use custom fields")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeWithDefaults(tt.input)
			tt.verify(t, result)
		})
	}
}

func TestFormatMessageComplexFields(t *testing.T) {
	complexFields := map[string]any{
		"string": "value",
		"int":    42,
		"float":  3.14,
		"bool":   true,
		"null":   nil,
		"array":  []int{1, 2, 3},
		"object": map[string]string{"nested": "value"},
	}

	result := FormatMessageWithOptions(
		LevelInfo,
		false,
		"",
		true,
		false,
		0,
		false,
		"complex message",
		complexFields,
		nil,
	)

	// Verify it's valid Json
	var jsonData map[string]any
	if err := json.Unmarshal([]byte(result), &jsonData); err != nil {
		t.Fatalf("Result is not valid Json: %v", err)
	}

	// Verify fields are present
	fields, ok := jsonData["fields"].(map[string]any)
	if !ok {
		t.Fatal("Fields should be a map")
	}

	if fields["string"] != "value" {
		t.Error("String field not preserved")
	}
	if fields["int"].(float64) != 42 {
		t.Error("Int field not preserved")
	}
	if fields["bool"] != true {
		t.Error("Bool field not preserved")
	}
}

func TestFormatMessageEmptyFields(t *testing.T) {
	result := FormatMessageWithOptions(
		LevelInfo,
		false,
		"",
		true,
		false,
		0,
		false,
		"message without fields",
		nil,
		nil,
	)

	// Should not contain fields key when no fields provided
	if strings.Contains(result, `"fields"`) {
		t.Error("Should not include fields key when no fields provided")
	}
}

func TestFormatMessageSpecialCharacters(t *testing.T) {
	message := `message with "quotes" and \backslashes and
newlines`

	result := FormatMessageWithOptions(
		LevelInfo,
		false,
		"",
		true,
		false,
		0,
		false,
		message,
		nil,
		nil,
	)

	// Verify it's valid JSON (should properly escape special characters)
	var jsonData map[string]any
	if err := json.Unmarshal([]byte(result), &jsonData); err != nil {
		t.Fatalf("Result is not valid Json: %v", err)
	}

	// Verify message is preserved correctly
	if jsonData["message"] != message {
		t.Error("Message with special characters not preserved correctly")
	}
}
