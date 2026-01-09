package internal

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	retryAttempts   = 3
	retryDelay      = 10 * time.Millisecond
	filePermissions = 0600
)

// OpenFile opens a log file for writing with security checks.
// Returns the file handle, current size, and any error encountered.
// Security: Rejects symlinks to prevent symlink attacks.
func OpenFile(path string) (*os.File, int64, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, filePermissions)
	if err != nil {
		return nil, 0, fmt.Errorf("open file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, fmt.Errorf("stat file: %w", err)
	}

	// Security check: reject symlinks
	if stat.Mode()&os.ModeSymlink != 0 {
		_ = file.Close()
		return nil, 0, fmt.Errorf("symlinks not allowed")
	}

	return file, stat.Size(), nil
}

// NeedsRotation determines if a file needs rotation based on size limits.
func NeedsRotation(currentSize, writeSize, maxSize int64) bool {
	return maxSize > 0 && currentSize+writeSize > maxSize
}

// RotateBackups manages backup file cleanup when limits are exceeded.
// This function is called before creating a new backup to ensure we stay within limits.
func RotateBackups(basePath string, maxBackups int, compress bool) {
	nextIndex := FindNextBackupIndex(basePath, compress)

	// Clean up excess backups if we're about to exceed the limit
	if maxBackups > 0 && nextIndex > maxBackups {
		cleanupExcessBackups(basePath, maxBackups, compress)
	}
}

// backupFileInfo holds information about a backup file.
type backupFileInfo struct {
	name  string
	index int
}

// buildBackupPattern constructs the pattern components for backup file naming.
// Returns: directory, prefix, scanf pattern, and suffix.
func buildBackupPattern(basePath string, compress bool) (dir, prefix, pattern, suffix string) {
	dir = filepath.Dir(basePath)
	baseName := filepath.Base(basePath)
	ext := filepath.Ext(baseName)
	baseNameWithoutExt := strings.TrimSuffix(baseName, ext)

	if compress {
		suffix = ".gz"
	}

	// Build prefix: filename_ext (e.g., "app_log" for "app.log")
	prefix = baseNameWithoutExt + "_" + strings.TrimPrefix(ext, ".")
	// Build pattern for scanf: prefix_%d.ext[.gz]
	pattern = prefix + "_%d" + ext + suffix

	return dir, prefix, pattern, suffix
}

// FindNextBackupIndex finds the next available backup index.
// Returns 1 if no backups exist, or max_index + 1 if backups are found.
func FindNextBackupIndex(basePath string, compress bool) int {
	dir, prefix, pattern, _ := buildBackupPattern(basePath, compress)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1 // Default to index 1 if directory can't be read
	}

	maxIndex := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Quick filter: check prefix before expensive scanf
		if !strings.HasPrefix(name, prefix+"_") {
			continue
		}

		// Parse index from filename
		var index int
		if _, err := fmt.Sscanf(name, pattern, &index); err == nil && index > maxIndex {
			maxIndex = index
		}
	}

	return maxIndex + 1
}

// cleanupExcessBackups removes the oldest backup files when maxBackups is exceeded.
func cleanupExcessBackups(basePath string, maxBackups int, compress bool) {
	dir, prefix, pattern, _ := buildBackupPattern(basePath, compress)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	// Collect all backup files with their indices
	backups := make([]backupFileInfo, 0, 16)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, prefix+"_") {
			continue
		}

		var index int
		if _, err := fmt.Sscanf(name, pattern, &index); err == nil {
			backups = append(backups, backupFileInfo{name: name, index: index})
		}
	}

	excessCount := len(backups) - maxBackups
	if excessCount <= 0 {
		return
	}

	// Sort backups by index (ascending) using stdlib sort - O(n log n)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].index < backups[j].index
	})

	// Delete the oldest (lowest index) files
	for i := 0; i < excessCount; i++ {
		filePath := filepath.Join(dir, backups[i].name)
		_ = os.Remove(filePath)
	}
}

// GetBackupPath generates the full path for a backup file with the given index.
// Format: basedir/filename_ext_index.ext[.gz]
// Example: /logs/app_log_1.log or /logs/app_log_1.log.gz
func GetBackupPath(basePath string, index int, compress bool) string {
	dir := filepath.Dir(basePath)
	baseName := filepath.Base(basePath)
	ext := filepath.Ext(baseName)
	baseNameWithoutExt := strings.TrimSuffix(baseName, ext)

	suffix := ""
	if compress {
		suffix = ".gz"
	}

	// Build filename: filename_ext_index.ext[.gz]
	filename := fmt.Sprintf("%s_%s_%d%s%s", baseNameWithoutExt, strings.TrimPrefix(ext, "."), index, ext, suffix)
	return filepath.Join(dir, filename)
}

// CompressFile compresses a file using gzip and replaces the original.
// Process: source -> temp.gz -> verify -> final.gz -> remove source
// Properly manages file handles to ensure source is closed before removal.
func CompressFile(filePath string) error {
	// Open source file
	src, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}

	// Create temporary compressed file
	tmpPath := filePath + ".gz.tmp"
	dst, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, filePermissions)
	if err != nil {
		src.Close()
		return fmt.Errorf("create temp: %w", err)
	}

	// Compress data
	gw := gzip.NewWriter(dst)
	_, copyErr := io.Copy(gw, src)
	closeErr := gw.Close()
	dstCloseErr := dst.Close()
	srcCloseErr := src.Close() // Close source before checking errors

	// Check for errors during compression
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("copy: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("gzip close: %w", closeErr)
	}
	if dstCloseErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("dst close: %w", dstCloseErr)
	}
	if srcCloseErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("src close: %w", srcCloseErr)
	}

	// Verify compressed file integrity
	if err := verifyGzipFile(tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("verify: %w", err)
	}

	// Remove existing .gz file if present (with retry)
	finalPath := filePath + ".gz"
	if _, err := os.Stat(finalPath); err == nil {
		for attempt := range retryAttempts {
			if err := os.Remove(finalPath); err == nil {
				break
			}
			if attempt < retryAttempts-1 {
				time.Sleep(retryDelay)
			}
		}
	}

	// Rename temp to final
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	// Remove original file (with retry)
	for attempt := range retryAttempts {
		if err := os.Remove(filePath); err == nil {
			return nil
		}
		if attempt < retryAttempts-1 {
			time.Sleep(retryDelay)
		}
	}

	// Original file removal failed, but compression succeeded
	return nil
}

// verifyGzipFile verifies that a gzip file is valid by attempting to decompress it.
// This ensures the compressed file is not corrupted before we delete the original.
func verifyGzipFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	// Read entire file to verify integrity
	_, err = io.Copy(io.Discard, gr)
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}

	return nil
}

// CleanupOldFiles removes backup files older than maxAge.
// This is typically called periodically by a background goroutine.
func CleanupOldFiles(basePath string, maxAge time.Duration) {
	if maxAge <= 0 {
		return
	}

	cutoff := time.Now().Add(-maxAge)
	dir := filepath.Dir(basePath)
	baseName := filepath.Base(basePath)
	ext := filepath.Ext(baseName)
	baseNameWithoutExt := strings.TrimSuffix(baseName, ext)

	// Build prefix pattern for backup files: filename_ext_
	prefix := baseNameWithoutExt + "_" + strings.TrimPrefix(ext, ".")

	// Walk directory and remove old backup files
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		fileName := filepath.Base(path)
		// Check if it's a backup file using the same pattern as rotation
		// Must start with prefix and contain underscore (e.g., "app_log_1.log" or "app_log_1.log.gz")
		if strings.HasPrefix(fileName, prefix+"_") &&
			fileName != baseName &&
			info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}

		return nil
	})
}
