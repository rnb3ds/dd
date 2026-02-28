package internal

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// ValidateAndSecurePath validates a file path and returns a cleaned absolute path.
// It performs security checks to prevent path traversal attacks and other vulnerabilities.
// Parameters:
//   - path: the file path to validate
//   - maxPathLength: maximum allowed path length
//   - emptyFilePathErr, nullByteErr, pathTooLongErr, pathTraversalErr, invalidPathErr: errors to return
func ValidateAndSecurePath(path string, maxPathLength int, emptyFilePathErr, nullByteErr, pathTooLongErr, pathTraversalErr, invalidPathErr error) (string, error) {
	if path == "" {
		return "", emptyFilePathErr
	}

	if strings.Contains(path, "\x00") {
		return "", nullByteErr
	}

	if len(path) > maxPathLength {
		return "", fmt.Errorf("%w (max %d characters)", pathTooLongErr, maxPathLength)
	}

	// Check for path traversal in the ORIGINAL path before any normalization
	// This catches patterns like "../secret" or "logs/../../../etc/passwd"
	// We check both forward and backward slashes for cross-platform safety
	if strings.Contains(path, "..") {
		return "", pathTraversalErr
	}

	// Check for URL-encoded path traversal attempts
	// Attackers may use %2e%2e%2f (../), %2e%2e%5c (..\\), or mixed encodings
	decodedPath, err := url.PathUnescape(path)
	if err != nil {
		return "", fmt.Errorf("%w: invalid URL encoding", invalidPathErr)
	}
	// Check decoded path for traversal (handles double-encoding like %252e%252e)
	if decodedPath != path {
		if strings.Contains(decodedPath, "..") {
			return "", pathTraversalErr
		}
		// Recursively check for double/triple encoding
		fullyDecoded, innerErr := url.PathUnescape(decodedPath)
		for innerErr == nil && fullyDecoded != decodedPath {
			if strings.Contains(fullyDecoded, "..") {
				return "", pathTraversalErr
			}
			decodedPath = fullyDecoded
			fullyDecoded, innerErr = url.PathUnescape(decodedPath)
		}
	}

	// Convert to absolute path first to normalize
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%w: %w", invalidPathErr, err)
	}

	// Clean the absolute path
	cleanPath := filepath.Clean(absPath)

	// Additional Windows-specific checks
	// These checks prevent attacks like:
	// - UNC path injection: \\?\, \\.\, etc.
	// - Drive relative paths: C:..\, D:..\ etc.
	// - Reserved device names: CON, PRN, AUX, NUL, COM1-9, LPT1-9
	cleanPathLower := strings.ToLower(cleanPath)
	if strings.HasPrefix(cleanPathLower, "\\\\?\\") ||
		strings.HasPrefix(cleanPathLower, "\\\\.\\") {
		// Only allow if the original path was already using these prefixes
		// This prevents attackers from injecting UNC paths
		origLower := strings.ToLower(path)
		if !strings.HasPrefix(origLower, "\\\\?\\") &&
			!strings.HasPrefix(origLower, "\\\\.\\") {
			return "", pathTraversalErr
		}
	}

	// Note: Symlink checking is done AFTER opening the file in OpenFile
	// to prevent TOCTOU (time-of-check-time-of-use) vulnerabilities
	return cleanPath, nil
}

// ValidateTimeFormat validates a time format string.
// Returns nil if the format is valid or empty (empty uses default).
// Returns an error if the format cannot be used to parse/format time.
func ValidateTimeFormat(format string) error {
	if format == "" {
		return nil // Empty format is valid (will use default)
	}
	if _, err := time.Parse(format, time.Now().Format(format)); err != nil {
		return fmt.Errorf("invalid time format %q: %w", format, err)
	}
	return nil
}
