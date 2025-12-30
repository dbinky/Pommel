//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

// sendSIGTERM sends SIGTERM to a process (Unix only).
func sendSIGTERM(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}
