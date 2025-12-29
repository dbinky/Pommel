package subproject

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Detector Scan Tests
// =============================================================================

func TestDetector_Scan_SingleMarker(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create a subproject with go.mod
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))

	detector := NewDetector(tmpDir, nil, nil)
	detected, err := detector.Scan()
	require.NoError(t, err)

	assert.Len(t, detected, 1)
	assert.Equal(t, "backend", detected[0].ID)
	assert.Equal(t, "backend", detected[0].Path)
	assert.Equal(t, "go.mod", detected[0].MarkerFile)
	assert.Equal(t, "go", detected[0].LanguageHint)
}

func TestDetector_Scan_MultipleMarkers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple subprojects
	goDir := filepath.Join(tmpDir, "services", "api")
	require.NoError(t, os.MkdirAll(goDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module api"), 0644))

	jsDir := filepath.Join(tmpDir, "packages", "frontend")
	require.NoError(t, os.MkdirAll(jsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(jsDir, "package.json"), []byte("{}"), 0644))

	pyDir := filepath.Join(tmpDir, "tools", "scripts")
	require.NoError(t, os.MkdirAll(pyDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pyDir, "pyproject.toml"), []byte(""), 0644))

	detector := NewDetector(tmpDir, nil, nil)
	detected, err := detector.Scan()
	require.NoError(t, err)

	assert.Len(t, detected, 3)

	// Map by path for easier assertions
	byPath := make(map[string]*DetectedSubproject)
	for _, d := range detected {
		byPath[d.Path] = d
	}

	assert.Contains(t, byPath, filepath.Join("services", "api"))
	assert.Contains(t, byPath, filepath.Join("packages", "frontend"))
	assert.Contains(t, byPath, filepath.Join("tools", "scripts"))

	assert.Equal(t, "go", byPath[filepath.Join("services", "api")].LanguageHint)
	assert.Equal(t, "javascript", byPath[filepath.Join("packages", "frontend")].LanguageHint)
	assert.Equal(t, "python", byPath[filepath.Join("tools", "scripts")].LanguageHint)
}

func TestDetector_Scan_NestedMarkers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested subprojects - both parent and child have markers
	parentDir := filepath.Join(tmpDir, "packages", "frontend")
	require.NoError(t, os.MkdirAll(parentDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "package.json"), []byte("{}"), 0644))

	childDir := filepath.Join(parentDir, "apps", "admin")
	require.NoError(t, os.MkdirAll(childDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "package.json"), []byte("{}"), 0644))

	detector := NewDetector(tmpDir, nil, nil)
	detected, err := detector.Scan()
	require.NoError(t, err)

	// Both should be detected - nesting is resolved at query time
	assert.Len(t, detected, 2)

	byPath := make(map[string]*DetectedSubproject)
	for _, d := range detected {
		byPath[d.Path] = d
	}

	assert.Contains(t, byPath, filepath.Join("packages", "frontend"))
	assert.Contains(t, byPath, filepath.Join("packages", "frontend", "apps", "admin"))
}

func TestDetector_Scan_PriorityMarkers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory with multiple markers
	subDir := filepath.Join(tmpDir, "myproject")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	// Both .csproj (priority 2) and package.json (priority 3) in same dir
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "MyApp.csproj"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "package.json"), []byte("{}"), 0644))

	detector := NewDetector(tmpDir, nil, nil)
	detected, err := detector.Scan()
	require.NoError(t, err)

	assert.Len(t, detected, 1)
	// .csproj should win due to higher priority (lower number)
	assert.Equal(t, "csharp", detected[0].LanguageHint)
	assert.Equal(t, "MyApp.csproj", detected[0].MarkerFile)
}

func TestDetector_Scan_SlnPriority(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory with .sln and .csproj
	subDir := filepath.Join(tmpDir, "myproject")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	// .sln (priority 1) vs .csproj (priority 2)
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "MySolution.sln"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "MyApp.csproj"), []byte(""), 0644))

	detector := NewDetector(tmpDir, nil, nil)
	detected, err := detector.Scan()
	require.NoError(t, err)

	assert.Len(t, detected, 1)
	// .sln should win
	assert.Equal(t, "MySolution.sln", detected[0].MarkerFile)
}

func TestDetector_Scan_Excludes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subprojects
	includedDir := filepath.Join(tmpDir, "packages", "app")
	require.NoError(t, os.MkdirAll(includedDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(includedDir, "package.json"), []byte("{}"), 0644))

	excludedDir := filepath.Join(tmpDir, "vendor", "lib")
	require.NoError(t, os.MkdirAll(excludedDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(excludedDir, "package.json"), []byte("{}"), 0644))

	nodeModules := filepath.Join(tmpDir, "node_modules", "dep")
	require.NoError(t, os.MkdirAll(nodeModules, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nodeModules, "package.json"), []byte("{}"), 0644))

	detector := NewDetector(tmpDir, nil, []string{"vendor", "node_modules"})
	detected, err := detector.Scan()
	require.NoError(t, err)

	assert.Len(t, detected, 1)
	assert.Equal(t, filepath.Join("packages", "app"), detected[0].Path)
}

func TestDetector_Scan_RootMarkerSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	// Create marker at root level
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module root"), 0644))

	// Create subproject
	subDir := filepath.Join(tmpDir, "cmd", "app")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "package.json"), []byte("{}"), 0644))

	detector := NewDetector(tmpDir, nil, nil)
	detected, err := detector.Scan()
	require.NoError(t, err)

	// Root marker should be skipped, only subdir found
	assert.Len(t, detected, 1)
	assert.Equal(t, filepath.Join("cmd", "app"), detected[0].Path)
}

func TestDetector_Scan_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some files but no markers
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(""), 0644))

	detector := NewDetector(tmpDir, nil, nil)
	detected, err := detector.Scan()
	require.NoError(t, err)

	assert.Empty(t, detected)
}

// =============================================================================
// ID Generation Tests
// =============================================================================

func TestDetector_GenerateID(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			path:     "frontend",
			expected: "frontend",
		},
		{
			name:     "nested path uses last component",
			path:     "packages/frontend",
			expected: "frontend",
		},
		{
			name:     "deeply nested path",
			path:     "apps/web/client",
			expected: "client",
		},
		{
			name:     "path with special chars",
			path:     "my.app-v2",
			expected: "my-app-v2",
		},
		{
			name:     "uppercase normalized",
			path:     "MyApp",
			expected: "myapp",
		},
	}

	detector := NewDetector(".", nil, nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := detector.generateID(tt.path)
			assert.Equal(t, tt.expected, id)
		})
	}
}

// =============================================================================
// Marker Matching Tests
// =============================================================================

func TestDetector_MatchesMarker(t *testing.T) {
	detector := NewDetector(".", nil, nil)

	tests := []struct {
		filename string
		pattern  string
		expected bool
	}{
		{"go.mod", "go.mod", true},
		{"go.sum", "go.mod", false},
		{"package.json", "package.json", true},
		{"MyApp.csproj", "*.csproj", true},
		{"Other.csproj", "*.csproj", true},
		{"csproj", "*.csproj", false},
		{"MySolution.sln", "*.sln", true},
		{"file.txt", "*.sln", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename+"_"+tt.pattern, func(t *testing.T) {
			result := detector.matchesMarker(tt.filename, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Custom Markers Tests
// =============================================================================

func TestDetector_CustomMarkers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create custom marker
	subDir := filepath.Join(tmpDir, "mylib")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "BUILD.bazel"), []byte(""), 0644))

	customMarkers := []MarkerDef{
		{Pattern: "BUILD.bazel", Priority: 1, Language: "bazel"},
	}

	detector := NewDetector(tmpDir, customMarkers, nil)
	detected, err := detector.Scan()
	require.NoError(t, err)

	assert.Len(t, detected, 1)
	assert.Equal(t, "bazel", detected[0].LanguageHint)
	assert.Equal(t, "BUILD.bazel", detected[0].MarkerFile)
}
