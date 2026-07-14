//go:build !windows

package utils

import (
	"os"
	"syscall"
)

// IsProcessRunning reports whether a process with the given PID is alive.
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
