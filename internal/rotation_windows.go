//go:build windows

package internal

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getFileInformation = kernel32.NewProc("GetFileInformationByHandle")
)

// ByHandleFileInformation contains file information from GetFileInformationByHandle.
type byHandleFileInformation struct {
	FileAttributes     uint32
	CreationTime       syscall.Filetime
	LastAccessTime     syscall.Filetime
	LastWriteTime      syscall.Filetime
	VolumeSerialNumber uint32
	FileSizeHigh       uint32
	FileSizeLow        uint32
	NumberOfLinks      uint32
	FileIndexHigh      uint32
	FileIndexLow       uint32
}

// isHardlink checks if a file has multiple hard links on Windows.
// Uses GetFileInformationByHandle to retrieve the NumberOfLinks field.
func isHardlink(file *os.File) (bool, error) {
	var info byHandleFileInformation

	ret, _, err := getFileInformation.Call(
		uintptr(file.Fd()),
		uintptr(unsafe.Pointer(&info)),
	)

	if ret == 0 {
		return false, fmt.Errorf("GetFileInformationByHandle: %w", err)
	}

	// NumberOfLinks > 1 means the file has multiple hard links
	return info.NumberOfLinks > 1, nil
}
