//go:build windows

package utils

import "syscall"

const (
	// STILL_ACTIVE exit code returned by GetExitCodeProcess for live processes.
	stillActive = 259
	// PROCESS_QUERY_LIMITED_INFORMATION lets us query processes we don't own.
	processQueryLimitedInformation = 0x1000
)

// IsProcessRunning reports whether a process with the given PID is alive.
// os.Process.Wait cannot be used here: on Windows it blocks until the target
// process exits when the target is not a child of this process.
func IsProcessRunning(pid int) bool {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		// Access denied means the process exists but we lack permissions.
		return err == syscall.ERROR_ACCESS_DENIED
	}
	defer syscall.CloseHandle(h)

	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
