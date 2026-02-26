//go:build windows

package config

import (
	"os"
	"syscall"
	"unsafe"
)

const (
	stillActive                 = 259 // STILL_ACTIVE
	processQueryLimitedInfo     = 0x1000
)

var (
	modkernel32              = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess          = modkernel32.NewProc("OpenProcess")
	procGetExitCodeProcess   = modkernel32.NewProc("GetExitCodeProcess")
	procCloseHandle          = modkernel32.NewProc("CloseHandle")
)

// isProcessRunning returns true if a process with the given PID exists and is still running.
// On Windows, os.FindProcess always succeeds, so we use OpenProcess + GetExitCodeProcess.
func isProcessRunning(pid int) bool {
	// First try os.FindProcess (lightweight)
	_, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Open a handle to query exit code.
	handle, _, err := procOpenProcess.Call(
		uintptr(processQueryLimitedInfo),
		0, // bInheritHandle = false
		uintptr(pid),
	)
	if handle == 0 {
		// OpenProcess failed — process doesn't exist or access denied.
		// Treat access-denied as "running" (conservative).
		errno, ok := err.(syscall.Errno)
		if ok && errno == syscall.ERROR_ACCESS_DENIED {
			return true
		}
		return false
	}
	defer procCloseHandle.Call(handle)

	var exitCode uint32
	ret, _, _ := procGetExitCodeProcess.Call(handle, uintptr(unsafe.Pointer(&exitCode)))
	if ret == 0 {
		// GetExitCodeProcess failed; assume running.
		return true
	}
	return exitCode == stillActive
}
