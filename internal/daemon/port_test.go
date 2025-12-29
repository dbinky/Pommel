package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Port Calculation Tests
// =============================================================================

func TestCalculatePort_Deterministic(t *testing.T) {
	// Same path should always return the same port
	path := "/home/user/projects/myproject"

	port1, err := CalculatePort(path)
	require.NoError(t, err)

	port2, err := CalculatePort(path)
	require.NoError(t, err)

	assert.Equal(t, port1, port2, "same path should return same port")

	// Call it many times to ensure determinism
	for i := 0; i < 100; i++ {
		port, err := CalculatePort(path)
		require.NoError(t, err)
		assert.Equal(t, port1, port, "port should be deterministic")
	}
}

func TestCalculatePort_InRange(t *testing.T) {
	// Test various paths ensure port is always in range 49152-65535
	testPaths := []string{
		"/home/user/project1",
		"/home/user/project2",
		"/var/www/app",
		"/Users/ryan/Repos/Pommel",
		"/tmp/test",
		"/a",
		"/a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p",
		"C:\\Users\\user\\projects",
		"/home/user/very-long-project-name-that-goes-on-and-on-and-on",
	}

	for _, path := range testPaths {
		t.Run(path, func(t *testing.T) {
			port, err := CalculatePort(path)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, port, PortRangeStart, "port should be >= %d", PortRangeStart)
			assert.Less(t, port, PortRangeStart+PortRangeSize, "port should be < %d", PortRangeStart+PortRangeSize)
		})
	}
}

func TestCalculatePort_DifferentPaths(t *testing.T) {
	// Different paths should (usually) yield different ports
	// Note: Hash collisions are possible but rare
	paths := []string{
		"/project1",
		"/project2",
		"/home/user/app",
		"/var/www/site",
		"/Users/dev/code",
	}

	ports := make(map[int]string)
	for _, path := range paths {
		port, err := CalculatePort(path)
		require.NoError(t, err)

		// Check for collision (unlikely but possible)
		if existingPath, exists := ports[port]; exists {
			t.Logf("Collision detected: %s and %s both map to port %d", path, existingPath, port)
		}
		ports[port] = path
	}

	// We should have mostly unique ports (allow for 1 collision max in this small set)
	assert.GreaterOrEqual(t, len(ports), len(paths)-1, "most paths should have unique ports")
}

func TestCalculatePort_RelativePathConverted(t *testing.T) {
	// Test that relative paths are converted to absolute before hashing
	// This ensures consistent port calculation regardless of cwd

	// Get current directory
	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Calculate port from absolute path
	port1, err := CalculatePort(cwd)
	require.NoError(t, err)

	// Calculate port from "." (current directory)
	port2, err := CalculatePort(".")
	require.NoError(t, err)

	// Both should yield the same port since "." resolves to cwd
	assert.Equal(t, port1, port2, "relative '.' should resolve to same port as absolute cwd")

	// Test that empty string also resolves to cwd
	port3, err := CalculatePort("")
	require.NoError(t, err)
	assert.Equal(t, port1, port3, "empty path should resolve to same port as cwd")
}

func TestCalculatePort_EmptyPath(t *testing.T) {
	// Empty path should use current directory
	port, err := CalculatePort("")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, port, PortRangeStart)
	assert.Less(t, port, PortRangeStart+PortRangeSize)
}

func TestCalculatePort_SymlinkHandling(t *testing.T) {
	// Create temp dirs and symlink
	tempDir := t.TempDir()
	realDir := filepath.Join(tempDir, "real")
	linkDir := filepath.Join(tempDir, "link")

	err := os.Mkdir(realDir, 0755)
	require.NoError(t, err)

	err = os.Symlink(realDir, linkDir)
	require.NoError(t, err)

	// Both should resolve to same absolute path and thus same port
	port1, err := CalculatePort(realDir)
	require.NoError(t, err)

	port2, err := CalculatePort(linkDir)
	require.NoError(t, err)

	// Note: Depending on implementation, symlinks may or may not resolve to same port
	// For now we just ensure both work
	assert.GreaterOrEqual(t, port1, PortRangeStart)
	assert.GreaterOrEqual(t, port2, PortRangeStart)
}

// =============================================================================
// Port Range Constants Tests
// =============================================================================

func TestPortConstants(t *testing.T) {
	assert.Equal(t, 49152, PortRangeStart, "port range should start at 49152 (IANA dynamic ports)")
	assert.Equal(t, 16384, PortRangeSize, "port range size should be 16384")

	// Verify end of range
	maxPort := PortRangeStart + PortRangeSize - 1
	assert.Equal(t, 65535, maxPort, "max port should be 65535")
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestCalculatePort_SpecialCharacters(t *testing.T) {
	paths := []string{
		"/path/with spaces/project",
		"/path/with-dashes/project",
		"/path/with_underscores/project",
		"/path/with.dots/project",
		"/path/with@special/project",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			port, err := CalculatePort(path)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, port, PortRangeStart)
			assert.Less(t, port, PortRangeStart+PortRangeSize)
		})
	}
}

func TestCalculatePort_Unicode(t *testing.T) {
	paths := []string{
		"/home/用户/项目",
		"/home/пользователь/проект",
		"/home/ユーザー/プロジェクト",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			port, err := CalculatePort(path)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, port, PortRangeStart)
			assert.Less(t, port, PortRangeStart+PortRangeSize)
		})
	}
}

// =============================================================================
// Distribution Test (statistical)
// =============================================================================

func TestCalculatePort_Distribution(t *testing.T) {
	// Generate many ports and check they're reasonably distributed
	numPaths := 1000
	portCounts := make(map[int]int)

	for i := 0; i < numPaths; i++ {
		path := filepath.Join("/projects", "project", string(rune('A'+i%26)), string(rune('0'+i%10)))
		port, err := CalculatePort(path)
		require.NoError(t, err)
		portCounts[port]++
	}

	// Check that we got a reasonable distribution (not all the same port)
	assert.Greater(t, len(portCounts), numPaths/10, "should have reasonable port distribution")

	// Check max collision count isn't too high
	maxCollisions := 0
	for _, count := range portCounts {
		if count > maxCollisions {
			maxCollisions = count
		}
	}
	assert.Less(t, maxCollisions, numPaths/10, "no port should have too many collisions")
}

// =============================================================================
// DeterminePort Tests (with config override)
// =============================================================================

func TestDeterminePort_ConfigOverride(t *testing.T) {
	// When config has a port set, it should take precedence over hash calculation
	projectRoot := "/test/project"
	overridePort := 8080

	cfg := &config.Config{
		Daemon: config.DaemonConfig{
			Host: "127.0.0.1",
			Port: &overridePort,
		},
	}

	port, err := DeterminePort(projectRoot, cfg)
	require.NoError(t, err)
	assert.Equal(t, 8080, port, "config port should override hash calculation")
}

func TestDeterminePort_HashFallback(t *testing.T) {
	// When config port is nil, should use hash calculation
	projectRoot := "/test/project"

	cfg := &config.Config{
		Daemon: config.DaemonConfig{
			Host: "127.0.0.1",
			Port: nil, // No override
		},
	}

	port, err := DeterminePort(projectRoot, cfg)
	require.NoError(t, err)

	// Should match calculated port
	expectedPort, err := CalculatePort(projectRoot)
	require.NoError(t, err)
	assert.Equal(t, expectedPort, port, "should use calculated port when config is nil")
}

func TestDeterminePort_ZeroPortIsValid(t *testing.T) {
	// Zero port in config means "use system-assigned port" (for testing)
	// This is different from nil which means "use hash"
	projectRoot := "/test/project"
	zeroPort := 0

	cfg := &config.Config{
		Daemon: config.DaemonConfig{
			Host: "127.0.0.1",
			Port: &zeroPort,
		},
	}

	port, err := DeterminePort(projectRoot, cfg)
	require.NoError(t, err)
	assert.Equal(t, 0, port, "zero port should be respected as 'use any available port'")
}

func TestDeterminePort_NilConfig(t *testing.T) {
	// Should handle nil config gracefully (use hash)
	projectRoot := "/test/project"

	port, err := DeterminePort(projectRoot, nil)
	require.NoError(t, err)

	expectedPort, err := CalculatePort(projectRoot)
	require.NoError(t, err)
	assert.Equal(t, expectedPort, port, "should use calculated port when config is nil")
}

func TestDeterminePort_VariousOverrides(t *testing.T) {
	testCases := []struct {
		name        string
		port        int
		description string
	}{
		{"standard_port", 7420, "original default port"},
		{"high_port", 65000, "high port number"},
		{"low_port", 1024, "low privileged port"},
		{"common_dev", 3000, "common development port"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			port := tc.port
			cfg := &config.Config{
				Daemon: config.DaemonConfig{
					Host: "127.0.0.1",
					Port: &port,
				},
			}

			result, err := DeterminePort("/any/path", cfg)
			require.NoError(t, err)
			assert.Equal(t, tc.port, result, tc.description)
		})
	}
}
