//go:build !windows

package internal

import (
	"fmt"
	"os"
	"syscall"
)

// isHardlink checks if a file has multiple hard links.
// On Unix systems, a file with nlink > 1 has multiple hard links,
// which could indicate a security risk where an attacker has created
// a hard link to redirect log output.
func isHardlink(file *os.File) (bool, error) {
	var stat syscall.Stat_t
	if err := syscall.Fstat(int(file.Fd()), &stat); err != nil {
		return false, fmt.Errorf("fstat: %w", err)
	}
	// nlink > 1 means the file has multiple hard links
	return stat.Nlink > 1, nil
}
