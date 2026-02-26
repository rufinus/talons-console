//go:build !windows

package config

import (
	"os"
	"syscall"
)

// isProcessRunning returns true if a process with the given PID exists and is running.
// On Unix, sending signal 0 is a portable existence check: it succeeds iff the process
// exists and the caller has permission to signal it.
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// syscall.Signal(0) = "null signal": tests existence without delivering a signal.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
