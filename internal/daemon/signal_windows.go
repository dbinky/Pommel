//go:build windows

package daemon

import (
	"os"
)

// ShutdownSignals returns the signals that should trigger graceful shutdown.
// On Windows, only os.Interrupt (Ctrl+C) is reliably supported.
func ShutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
