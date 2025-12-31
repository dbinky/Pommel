package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFileWithRetry_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create a regular file
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Should read successfully
	content, err := ReadFileWithRetry(testFile, 3)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content))
}

func TestReadFileWithRetry_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "nonexistent.txt")

	// Should fail immediately (not retry) for non-existent file
	_, err := ReadFileWithRetry(testFile, 3)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestReadFileWithRetry_DefaultRetries(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	err := os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Should work with 0 retries (uses default of 3)
	content, err := ReadFileWithRetry(testFile, 0)
	require.NoError(t, err)
	assert.Equal(t, "test", string(content))
}

func TestFileLockError_Error(t *testing.T) {
	err := &FileLockError{
		Path: "/path/to/file",
		Err:  os.ErrPermission,
	}

	assert.Equal(t, "file is locked: /path/to/file", err.Error())
}

func TestFileLockError_Unwrap(t *testing.T) {
	innerErr := os.ErrPermission
	err := &FileLockError{
		Path: "/path/to/file",
		Err:  innerErr,
	}

	assert.Equal(t, innerErr, err.Unwrap())
}

func TestIsFileLocked_NilError(t *testing.T) {
	assert.False(t, isFileLocked(nil))
}

func TestIsFileLocked_PermissionError(t *testing.T) {
	// Permission errors should be considered as potentially locked
	assert.True(t, isFileLocked(os.ErrPermission))
}

func TestIsFileLocked_NotExistError(t *testing.T) {
	// File not existing is not a lock error
	assert.False(t, isFileLocked(os.ErrNotExist))
}

func TestReadFileWithRetry_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	// Create an empty file
	err := os.WriteFile(testFile, []byte{}, 0644)
	require.NoError(t, err)

	// Should read empty content successfully
	content, err := ReadFileWithRetry(testFile, 3)
	require.NoError(t, err)
	assert.Equal(t, 0, len(content))
}

func TestReadFileWithRetry_BinaryContent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "binary.bin")

	// Create a file with binary content
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	err := os.WriteFile(testFile, binaryData, 0644)
	require.NoError(t, err)

	// Should read binary content correctly
	content, err := ReadFileWithRetry(testFile, 3)
	require.NoError(t, err)
	assert.Equal(t, binaryData, content)
}

// TestReadFileWithRetry_LockedFile tests reading a file that's locked by another handle.
// This test is more relevant on Windows where file locking is stricter.
func TestReadFileWithRetry_LockedFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("File locking behavior differs on non-Windows systems")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "locked.txt")

	// Create the file
	err := os.WriteFile(testFile, []byte("locked content"), 0644)
	require.NoError(t, err)

	// Open the file with exclusive access (Windows-specific behavior)
	// Note: On Windows, opening for write may prevent reads
	f, err := os.OpenFile(testFile, os.O_RDWR|os.O_CREATE, 0644)
	require.NoError(t, err)
	defer f.Close()

	// Try to read while file is open
	// On Windows with certain flags, this may fail or succeed depending on sharing mode
	// The important thing is that our retry logic handles any transient failures
	content, err := ReadFileWithRetry(testFile, 3)
	if err != nil {
		// If we get an error, it should be a FileLockError after retries
		var lockErr *FileLockError
		if assert.ErrorAs(t, err, &lockErr) {
			assert.Equal(t, testFile, lockErr.Path)
		}
	} else {
		// If we successfully read, content should match
		assert.Equal(t, "locked content", string(content))
	}
}
