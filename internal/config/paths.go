// Package config provides configuration loading, validation, and path resolution.
package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// LanguagesDirEnvVar is the environment variable name for overriding
// the languages configuration directory.
const LanguagesDirEnvVar = "POMMEL_LANGUAGES_DIR"

// LanguagesDir returns the path to the languages configuration directory.
// The path is determined in the following order of precedence:
//
//  1. POMMEL_LANGUAGES_DIR environment variable (if set and non-empty)
//  2. Platform-specific default:
//     - macOS/Linux: $XDG_DATA_HOME/pommel/languages or ~/.local/share/pommel/languages
//     - Windows: %LOCALAPPDATA%\Pommel\languages
//
// The directory may not exist; use EnsureLanguagesDir to create it if needed.
func LanguagesDir() (string, error) {
	// Check for environment variable override
	if envDir := os.Getenv(LanguagesDirEnvVar); envDir != "" {
		return envDir, nil
	}

	// Platform-specific default paths
	if runtime.GOOS == "windows" {
		return windowsLanguagesDir()
	}
	return unixLanguagesDir()
}

// EnsureLanguagesDir returns the languages directory path, creating it if it doesn't exist.
// Uses the same path resolution logic as LanguagesDir.
func EnsureLanguagesDir() (string, error) {
	dir, err := LanguagesDir()
	if err != nil {
		return "", err
	}

	// Create directory with parents if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	return dir, nil
}

// unixLanguagesDir returns the languages directory for Unix-like systems (macOS, Linux).
// Respects XDG_DATA_HOME if set, otherwise falls back to ~/.local/share.
func unixLanguagesDir() (string, error) {
	// Check XDG_DATA_HOME first
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		return filepath.Join(xdgDataHome, "pommel", "languages"), nil
	}

	// Fall back to ~/.local/share
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".local", "share", "pommel", "languages"), nil
}

// windowsLanguagesDir returns the languages directory for Windows.
// Uses %LOCALAPPDATA%\Pommel\languages.
func windowsLanguagesDir() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		// Fall back to UserHomeDir if LOCALAPPDATA is not set
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		localAppData = filepath.Join(homeDir, "AppData", "Local")
	}

	return filepath.Join(localAppData, "Pommel", "languages"), nil
}
