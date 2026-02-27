package internal

import (
	"errors"
	"testing"
)

var (
	errEmptyPath     = errors.New("empty path")
	errNullByte      = errors.New("null byte")
	errPathTooLong   = errors.New("path too long")
	errPathTraversal = errors.New("path traversal")
	errInvalidPath   = errors.New("invalid path")
)

func TestValidateAndSecurePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{"empty path", "", errEmptyPath},
		{"null byte", "test\x00.log", errNullByte},
		{"simple traversal", "../secret", errPathTraversal},
		{"nested traversal", "logs/../../../etc/passwd", errPathTraversal},
		{"url encoded traversal", "%2e%2e%2fsecret", errPathTraversal},
		{"double encoded", "%252e%252e%252fsecret", errPathTraversal},
		{"backslash encoded", "%2e%2e%5csecret", errPathTraversal},
		{"mixed encoding", "..%2fsecret", errPathTraversal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateAndSecurePath(tt.path, 4096, errEmptyPath, errNullByte, errPathTooLong, errPathTraversal, errInvalidPath)
			if err == nil {
				if tt.wantErr != nil {
					t.Errorf("ValidateAndSecurePath(%q) expected error %v, got nil", tt.path, tt.wantErr)
				}
			} else if tt.wantErr != nil && !errors.Is(err, tt.wantErr) && !errors.Is(err, errInvalidPath) {
				t.Errorf("ValidateAndSecurePath(%q) expected error %v, got %v", tt.path, tt.wantErr, err)
			}
		})
	}
}
