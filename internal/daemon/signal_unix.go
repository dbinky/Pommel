//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

// ShutdownSignals returns the signals that should trigger graceful shutdown.
func ShutdownSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}
