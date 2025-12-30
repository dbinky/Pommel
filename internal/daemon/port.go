package daemon

import (
	"hash/fnv"
	"path/filepath"

	"github.com/pommel-dev/pommel/internal/config"
)

const (
	// PortRangeStart is the beginning of the IANA dynamic/private port range
	PortRangeStart = 49152

	// PortRangeSize is the number of ports in our allocation range (49152-65535)
	PortRangeSize = 16384
)

// CalculatePort returns a deterministic port for the given project root.
// The port is derived from the absolute path hash, falling in range 49152-65535.
// This allows multiple Pommel daemons to run simultaneously without port conflicts,
// as each project will consistently get the same port.
func CalculatePort(projectRoot string) (int, error) {
	// Convert to absolute path for consistency
	absPath, err := filepath.Abs(projectRoot)
	if err != nil {
		return 0, err
	}

	// Use FNV-1a hash for good distribution
	h := fnv.New32a()
	h.Write([]byte(absPath))

	// Map hash to port range
	port := PortRangeStart + int(h.Sum32()%uint32(PortRangeSize))
	return port, nil
}

// DeterminePort returns the port to use for the daemon.
// If the config has an explicit port set (including 0), that port is used.
// If the config port is nil or config is nil, the hash-based port is calculated.
func DeterminePort(projectRoot string, cfg *config.Config) (int, error) {
	// If config exists and has explicit port override, use it
	if cfg != nil && cfg.Daemon.Port != nil {
		return *cfg.Daemon.Port, nil
	}

	// Otherwise, calculate from project path
	return CalculatePort(projectRoot)
}
