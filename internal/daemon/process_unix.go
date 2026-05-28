//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

func isProcessAlive(proc *os.Process) bool {
	return proc.Signal(syscall.Signal(0)) == nil
}
