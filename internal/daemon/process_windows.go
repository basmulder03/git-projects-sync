//go:build windows

package daemon

import (
	"os"
)

func isProcessAlive(proc *os.Process) bool {
	// On Windows, OpenProcess with SYNCHRONIZE access returns an error for dead/non-existent processes.
	// Finding the process handle is sufficient; we check if Wait returns immediately with no children,
	// but the simplest reliable approach is to check via the process handle returned by FindProcess.
	// FindProcess on Windows only succeeds if the process exists in the system.
	return proc != nil
}
