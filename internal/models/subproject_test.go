package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Subproject Model Tests
// =============================================================================

func TestSubproject_NewSubproject(t *testing.T) {
	sp := &Subproject{
		ID:           "frontend",
		Path:         "packages/frontend",
		Name:         "Frontend App",
		MarkerFile:   "package.json",
		LanguageHint: "typescript",
		AutoDetected: true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	assert.Equal(t, "frontend", sp.ID)
	assert.Equal(t, "packages/frontend", sp.Path)
	assert.Equal(t, "Frontend App", sp.Name)
	assert.Equal(t, "package.json", sp.MarkerFile)
	assert.Equal(t, "typescript", sp.LanguageHint)
	assert.True(t, sp.AutoDetected)
}

func TestSubproject_GenerateID(t *testing.T) {
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
			name:     "nested path becomes slug",
			path:     "packages/frontend",
			expected: "packages-frontend",
		},
		{
			name:     "deeply nested path",
			path:     "apps/web/client",
			expected: "apps-web-client",
		},
		{
			name:     "path with dots",
			path:     "src/my.app",
			expected: "src-my-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := &Subproject{Path: tt.path}
			id := sp.GenerateID()
			assert.Equal(t, tt.expected, id)
		})
	}
}

func TestSubproject_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		sp        *Subproject
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid subproject",
			sp: &Subproject{
				ID:   "frontend",
				Path: "packages/frontend",
			},
			wantError: false,
		},
		{
			name: "missing ID",
			sp: &Subproject{
				Path: "packages/frontend",
			},
			wantError: true,
			errorMsg:  "ID is required",
		},
		{
			name: "missing path",
			sp: &Subproject{
				ID: "frontend",
			},
			wantError: true,
			errorMsg:  "path is required",
		},
		{
			name: "empty ID",
			sp: &Subproject{
				ID:   "",
				Path: "packages/frontend",
			},
			wantError: true,
			errorMsg:  "ID is required",
		},
		{
			name: "empty path",
			sp: &Subproject{
				ID:   "frontend",
				Path: "",
			},
			wantError: true,
			errorMsg:  "path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sp.IsValid()
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSubproject_ContainsPath(t *testing.T) {
	sp := &Subproject{
		ID:   "frontend",
		Path: "packages/frontend",
	}

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{
			name:     "file directly in subproject",
			filePath: "packages/frontend/index.ts",
			expected: true,
		},
		{
			name:     "file in nested directory",
			filePath: "packages/frontend/src/components/Button.tsx",
			expected: true,
		},
		{
			name:     "exact path match",
			filePath: "packages/frontend",
			expected: true,
		},
		{
			name:     "different subproject",
			filePath: "packages/backend/server.go",
			expected: false,
		},
		{
			name:     "similar prefix but different path",
			filePath: "packages/frontend-admin/index.ts",
			expected: false,
		},
		{
			name:     "root level file",
			filePath: "README.md",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sp.ContainsPath(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubproject_ContainsPath_RootSubproject(t *testing.T) {
	// Root subproject (path ".") should contain all files
	sp := &Subproject{
		ID:   "root",
		Path: ".",
	}

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{
			name:     "root level file",
			filePath: "README.md",
			expected: true,
		},
		{
			name:     "nested file",
			filePath: "packages/frontend/index.ts",
			expected: true,
		},
		{
			name:     "deep nested file",
			filePath: "src/components/Button/Button.test.tsx",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sp.ContainsPath(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubproject_SetTimestamps(t *testing.T) {
	sp := &Subproject{
		ID:   "test",
		Path: "test",
	}

	// Initially timestamps should be zero
	assert.True(t, sp.CreatedAt.IsZero())
	assert.True(t, sp.UpdatedAt.IsZero())

	// SetTimestamps should set both
	sp.SetTimestamps()

	assert.False(t, sp.CreatedAt.IsZero())
	assert.False(t, sp.UpdatedAt.IsZero())
	assert.Equal(t, sp.CreatedAt, sp.UpdatedAt)
}

func TestSubproject_Touch(t *testing.T) {
	sp := &Subproject{
		ID:        "test",
		Path:      "test",
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now().Add(-time.Hour),
	}

	originalCreatedAt := sp.CreatedAt
	originalUpdatedAt := sp.UpdatedAt

	// Wait a tiny bit to ensure time difference
	time.Sleep(time.Millisecond)

	sp.Touch()

	// CreatedAt should not change
	assert.Equal(t, originalCreatedAt, sp.CreatedAt)
	// UpdatedAt should be newer
	assert.True(t, sp.UpdatedAt.After(originalUpdatedAt))
}

// =============================================================================
// Subproject Language Detection Tests
// =============================================================================

func TestSubproject_DetectLanguageFromMarker(t *testing.T) {
	tests := []struct {
		name       string
		markerFile string
		expected   string
	}{
		{"go.mod", "go.mod", "go"},
		{"go.sum", "go.sum", "go"},
		{"package.json", "package.json", "javascript"},
		{"tsconfig.json", "tsconfig.json", "typescript"},
		{"Cargo.toml", "Cargo.toml", "rust"},
		{"pyproject.toml", "pyproject.toml", "python"},
		{"requirements.txt", "requirements.txt", "python"},
		{"setup.py", "setup.py", "python"},
		{"pom.xml", "pom.xml", "java"},
		{"build.gradle", "build.gradle", "java"},
		{".csproj file", "MyApp.csproj", "csharp"},
		{".sln file", "MyApp.sln", "csharp"},
		{"unknown marker", "unknown.file", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := &Subproject{MarkerFile: tt.markerFile}
			result := sp.DetectLanguageFromMarker()
			assert.Equal(t, tt.expected, result)
		})
	}
}
