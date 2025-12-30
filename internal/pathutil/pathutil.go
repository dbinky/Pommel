// Package pathutil provides cross-platform path handling utilities.
// It wraps the standard filepath package to ensure consistent behavior
// across Windows, macOS, and Linux.
package pathutil

import (
	"path/filepath"
	"runtime"
	"strings"
)

// Normalize converts a path to use OS-appropriate separators
// and cleans redundant separators, removes trailing slashes, etc.
func Normalize(path string) string {
	return filepath.Clean(path)
}

// IsAbsolute checks if path is absolute.
// Handles Windows drive letters (C:\) and Unix absolute paths (/).
func IsAbsolute(path string) bool {
	return filepath.IsAbs(path)
}

// IsUNC checks if path is a Windows UNC path (\\server\share).
// Always returns false on non-Windows platforms.
func IsUNC(path string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return strings.HasPrefix(path, `\\`)
}

// ToSlash converts path to use forward slashes.
// Useful for storing paths in a platform-independent format.
func ToSlash(path string) string {
	return filepath.ToSlash(path)
}

// FromSlash converts a path with forward slashes to use OS separators.
// Useful when reading platform-independent paths.
func FromSlash(path string) string {
	return filepath.FromSlash(path)
}

// Join joins path elements using the OS-specific separator.
func Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Rel returns a relative path from basepath to targpath.
func Rel(basepath, targpath string) (string, error) {
	return filepath.Rel(basepath, targpath)
}

// Dir returns all but the last element of path, typically the directory.
func Dir(path string) string {
	return filepath.Dir(path)
}

// Base returns the last element of path.
func Base(path string) string {
	return filepath.Base(path)
}

// Ext returns the file name extension used by path.
func Ext(path string) string {
	return filepath.Ext(path)
}

// Abs returns an absolute representation of path.
func Abs(path string) (string, error) {
	return filepath.Abs(path)
}

// Match reports whether name matches the shell pattern.
func Match(pattern, name string) (bool, error) {
	return filepath.Match(pattern, name)
}

// Glob returns the names of all files matching pattern.
func Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

// Split splits path immediately following the final separator.
func Split(path string) (dir, file string) {
	return filepath.Split(path)
}

// VolumeName returns the volume name on Windows (e.g., "C:").
// Returns empty string on other platforms.
func VolumeName(path string) string {
	return filepath.VolumeName(path)
}

// HasPrefix reports whether the path begins with prefix.
// This is prefix-aware of path separators.
func HasPrefix(path, prefix string) bool {
	// Normalize both paths first
	path = Normalize(path)
	prefix = Normalize(prefix)

	// Use ToSlash for consistent comparison
	pathSlash := ToSlash(path)
	prefixSlash := ToSlash(prefix)

	// Check if path starts with prefix
	if !strings.HasPrefix(pathSlash, prefixSlash) {
		return false
	}

	// Ensure it's a complete path component match
	// (prefix "/home/user" shouldn't match path "/home/username")
	if len(pathSlash) > len(prefixSlash) {
		nextChar := pathSlash[len(prefixSlash)]
		if nextChar != '/' {
			return false
		}
	}

	return true
}
