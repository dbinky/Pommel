package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// LanguagesDir tests
// ============================================================================

func TestLanguagesDir_Unix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix test on Windows")
	}

	// Clear any override env var
	originalEnv := os.Getenv(LanguagesDirEnvVar)
	originalXDG := os.Getenv("XDG_DATA_HOME")
	defer func() {
		os.Setenv(LanguagesDirEnvVar, originalEnv)
		os.Setenv("XDG_DATA_HOME", originalXDG)
	}()
	os.Unsetenv(LanguagesDirEnvVar)
	os.Unsetenv("XDG_DATA_HOME")

	dir, err := LanguagesDir()
	require.NoError(t, err)

	// Should end with expected path
	assert.True(t, filepath.IsAbs(dir), "should return absolute path")
	assert.Contains(t, dir, ".local/share/pommel/languages")
}

func TestLanguagesDir_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows test on non-Windows")
	}

	// Clear any override env var
	originalEnv := os.Getenv(LanguagesDirEnvVar)
	defer os.Setenv(LanguagesDirEnvVar, originalEnv)
	os.Unsetenv(LanguagesDirEnvVar)

	dir, err := LanguagesDir()
	require.NoError(t, err)

	// Should end with expected path
	assert.True(t, filepath.IsAbs(dir), "should return absolute path")
	assert.Contains(t, dir, "Pommel")
	assert.Contains(t, dir, "languages")
}

func TestLanguagesDir_WithEnvOverride(t *testing.T) {
	originalEnv := os.Getenv(LanguagesDirEnvVar)
	defer os.Setenv(LanguagesDirEnvVar, originalEnv)

	// Set custom path via env var
	customPath := filepath.Join(t.TempDir(), "custom-languages")
	os.Setenv(LanguagesDirEnvVar, customPath)

	dir, err := LanguagesDir()
	require.NoError(t, err)
	assert.Equal(t, customPath, dir)
}

func TestLanguagesDir_XDGDataHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping XDG test on Windows")
	}

	// Save and clear relevant env vars
	originalEnv := os.Getenv(LanguagesDirEnvVar)
	originalXDG := os.Getenv("XDG_DATA_HOME")
	defer func() {
		os.Setenv(LanguagesDirEnvVar, originalEnv)
		os.Setenv("XDG_DATA_HOME", originalXDG)
	}()
	os.Unsetenv(LanguagesDirEnvVar)

	// Set custom XDG_DATA_HOME
	xdgPath := filepath.Join(t.TempDir(), "xdg-data")
	os.Setenv("XDG_DATA_HOME", xdgPath)

	dir, err := LanguagesDir()
	require.NoError(t, err)

	expected := filepath.Join(xdgPath, "pommel", "languages")
	assert.Equal(t, expected, dir)
}

func TestLanguagesDir_EnvOverrideTakesPrecedenceOverXDG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping XDG precedence test on Windows")
	}

	originalEnv := os.Getenv(LanguagesDirEnvVar)
	originalXDG := os.Getenv("XDG_DATA_HOME")
	defer func() {
		os.Setenv(LanguagesDirEnvVar, originalEnv)
		os.Setenv("XDG_DATA_HOME", originalXDG)
	}()

	// Set both env vars - POMMEL_LANGUAGES_DIR should take precedence
	customPath := filepath.Join(t.TempDir(), "custom-languages")
	xdgPath := filepath.Join(t.TempDir(), "xdg-data")
	os.Setenv(LanguagesDirEnvVar, customPath)
	os.Setenv("XDG_DATA_HOME", xdgPath)

	dir, err := LanguagesDir()
	require.NoError(t, err)
	assert.Equal(t, customPath, dir)
}

// ============================================================================
// EnsureLanguagesDir tests
// ============================================================================

func TestEnsureLanguagesDir_CreatesIfNotExists(t *testing.T) {
	originalEnv := os.Getenv(LanguagesDirEnvVar)
	defer os.Setenv(LanguagesDirEnvVar, originalEnv)

	// Set to a path that doesn't exist
	tempBase := t.TempDir()
	newDir := filepath.Join(tempBase, "new", "nested", "languages")
	os.Setenv(LanguagesDirEnvVar, newDir)

	// Verify it doesn't exist yet
	_, err := os.Stat(newDir)
	assert.True(t, os.IsNotExist(err), "directory should not exist initially")

	// Call EnsureLanguagesDir
	dir, err := EnsureLanguagesDir()
	require.NoError(t, err)
	assert.Equal(t, newDir, dir)

	// Verify directory was created
	info, err := os.Stat(newDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "should be a directory")
}

func TestEnsureLanguagesDir_ExistingDirUnchanged(t *testing.T) {
	originalEnv := os.Getenv(LanguagesDirEnvVar)
	defer os.Setenv(LanguagesDirEnvVar, originalEnv)

	// Create an existing directory with a file inside
	existingDir := filepath.Join(t.TempDir(), "existing-languages")
	require.NoError(t, os.MkdirAll(existingDir, 0755))

	testFile := filepath.Join(existingDir, "test.yaml")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	os.Setenv(LanguagesDirEnvVar, existingDir)

	// Call EnsureLanguagesDir
	dir, err := EnsureLanguagesDir()
	require.NoError(t, err)
	assert.Equal(t, existingDir, dir)

	// Verify the directory still exists and file is intact
	info, err := os.Stat(existingDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content))
}

func TestEnsureLanguagesDir_ReturnsCorrectPath(t *testing.T) {
	originalEnv := os.Getenv(LanguagesDirEnvVar)
	defer os.Setenv(LanguagesDirEnvVar, originalEnv)

	customPath := filepath.Join(t.TempDir(), "languages")
	os.Setenv(LanguagesDirEnvVar, customPath)

	dir, err := EnsureLanguagesDir()
	require.NoError(t, err)
	assert.Equal(t, customPath, dir)
}

// ============================================================================
// Edge cases and error conditions
// ============================================================================

func TestLanguagesDir_EmptyEnvVarFallsBackToDefault(t *testing.T) {
	originalEnv := os.Getenv(LanguagesDirEnvVar)
	originalXDG := os.Getenv("XDG_DATA_HOME")
	defer func() {
		os.Setenv(LanguagesDirEnvVar, originalEnv)
		os.Setenv("XDG_DATA_HOME", originalXDG)
	}()

	// Set empty string (different from unset)
	os.Setenv(LanguagesDirEnvVar, "")
	os.Unsetenv("XDG_DATA_HOME")

	dir, err := LanguagesDir()
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(dir), "should return absolute path")
	assert.Contains(t, dir, "languages")
}

func TestLanguagesDir_ReturnsConsistentResults(t *testing.T) {
	originalEnv := os.Getenv(LanguagesDirEnvVar)
	defer os.Setenv(LanguagesDirEnvVar, originalEnv)

	customPath := filepath.Join(t.TempDir(), "languages")
	os.Setenv(LanguagesDirEnvVar, customPath)

	// Call multiple times and verify consistency
	dir1, err1 := LanguagesDir()
	dir2, err2 := LanguagesDir()
	dir3, err3 := LanguagesDir()

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)

	assert.Equal(t, dir1, dir2)
	assert.Equal(t, dir2, dir3)
}

func TestEnsureLanguagesDir_DirectoryPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	originalEnv := os.Getenv(LanguagesDirEnvVar)
	defer os.Setenv(LanguagesDirEnvVar, originalEnv)

	newDir := filepath.Join(t.TempDir(), "new-languages")
	os.Setenv(LanguagesDirEnvVar, newDir)

	_, err := EnsureLanguagesDir()
	require.NoError(t, err)

	info, err := os.Stat(newDir)
	require.NoError(t, err)

	// Check permissions (should be at least readable and writable by owner)
	perm := info.Mode().Perm()
	assert.True(t, perm&0700 == 0700, "owner should have rwx permissions, got %o", perm)
}

// ============================================================================
// Constants tests
// ============================================================================

func TestLanguagesDirEnvVar_IsCorrectName(t *testing.T) {
	assert.Equal(t, "POMMEL_LANGUAGES_DIR", LanguagesDirEnvVar)
}
