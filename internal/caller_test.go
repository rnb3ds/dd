package internal

import (
	"strings"
	"testing"
)

func TestGetCallerComprehensive(t *testing.T) {
	tests := []struct {
		name        string
		depth       int
		fullPath    bool
		wantContain string
		dontWant    string
	}{
		{
			name:        "depth 1 with full path",
			depth:       1,
			fullPath:    true,
			wantContain: "caller_test.go",
		},
		{
			name:        "depth 1 without full path",
			depth:       1,
			fullPath:    false,
			wantContain: "caller_test.go",
			dontWant:    "/", // Should not contain path separators
		},
		{
			name:     "invalid high depth",
			depth:    1000,
			fullPath: false,
		},
		{
			name:     "negative depth",
			depth:    -1,
			fullPath: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCaller(tt.depth, tt.fullPath)

			if tt.depth >= 1000 {
				// High depth should return empty
				if result != "" {
					t.Errorf("GetCaller(%d, %v) should return empty, got %q", tt.depth, tt.fullPath, result)
				}
				return
			}

			// For valid depths, should contain line number
			if !strings.Contains(result, ":") {
				t.Errorf("GetCaller() should contain ':', got %q", result)
			}

			if tt.wantContain != "" && !strings.Contains(result, tt.wantContain) {
				t.Errorf("GetCaller() should contain %q, got %q", tt.wantContain, result)
			}

			if tt.dontWant != "" && strings.Contains(result, tt.dontWant) {
				// On Windows, check for both separators
				if tt.dontWant == "/" && strings.Contains(result, "\\") {
					t.Errorf("GetCaller(fullPath=false) should not contain path separators, got %q", result)
				} else if strings.Contains(result, tt.dontWant) {
					t.Errorf("GetCaller() should not contain %q, got %q", tt.dontWant, result)
				}
			}
		})
	}
}

func TestGetCallerFullPath(t *testing.T) {
	// Test with full path enabled
	callerInfo := GetCaller(1, true)

	if !strings.Contains(callerInfo, "caller_test.go") {
		t.Errorf("Should contain file name, got: %s", callerInfo)
	}

	if !strings.Contains(callerInfo, ":") {
		t.Error("Should contain line number separator")
	}

	// Full path should contain path separator on most systems
	// (except when running from current directory)
}

func TestGetCallerBaseName(t *testing.T) {
	// Test with full path disabled
	callerInfo := GetCaller(1, false)

	if !strings.Contains(callerInfo, "caller_test.go") {
		t.Errorf("Should contain file name, got: %s", callerInfo)
	}

	// Should NOT contain directory path
	if strings.Contains(callerInfo, "/") || strings.Contains(callerInfo, "\\") {
		t.Errorf("Should not contain path separators, got: %s", callerInfo)
	}
}

func TestGetCallerConsistentResults(t *testing.T) {
	// Multiple calls should return consistent results
	results := make([]string, 10)
	for i := 0; i < 10; i++ {
		results[i] = GetCaller(1, false)
	}

	for i := 1; i < 10; i++ {
		if results[i] != results[0] {
			t.Errorf("Inconsistent results: [%d]=%q vs [0]=%q", i, results[i], results[0])
		}
	}
}

func TestGetCallerLineNumbers(t *testing.T) {
	// Get caller info
	callerInfo := GetCaller(1, false)

	// Should have format "filename.go:linenum"
	parts := strings.Split(callerInfo, ":")
	if len(parts) != 2 {
		t.Errorf("Expected format 'file:line', got: %s", callerInfo)
		return
	}

	// Line number should be numeric and reasonable
	lineNum := parts[1]
	for _, c := range lineNum {
		if c < '0' || c > '9' {
			t.Errorf("Line number should be numeric, got: %s", lineNum)
			return
		}
	}
}

func TestCallerBuilderPool(t *testing.T) {
	// Test buffer pool reuse
	for i := 0; i < 100; i++ {
		_ = GetCaller(1, false)
	}
}

func TestGetCallerFromHelper(t *testing.T) {
	// Test that caller depth correctly tracks through helper functions
	caller := getCallerHelper(t)
	if !strings.Contains(caller, "caller_test.go") {
		t.Errorf("Caller should be from test file, got: %s", caller)
	}
}

func getCallerHelper(t *testing.T) string {
	// depth 2 should get the caller of this helper function
	return GetCaller(2, false)
}

func TestGetCallerDepth0(t *testing.T) {
	// depth 0 gets runtime.Caller itself which is in runtime package
	// This test verifies it doesn't crash
	result := GetCaller(0, false)
	// Result might be empty or contain runtime info
	t.Logf("GetCaller(0) = %q", result)
}

func TestGetCallerWithDeepStack(t *testing.T) {
	// Test with deeper call stack
	result := deepCallStack(5, t)
	if result == "" {
		t.Error("Expected non-empty caller info")
	}
}

func deepCallStack(depth int, t *testing.T) string {
	if depth == 0 {
		return GetCaller(1, false)
	}
	return deepCallStack(depth-1, t)
}

func TestCallerBuilderPoolGrowth(t *testing.T) {
	// Test that the pool handles growth correctly
	for i := 0; i < 1000; i++ {
		// Use full path to potentially create longer strings
		_ = GetCaller(1, true)
	}
}
