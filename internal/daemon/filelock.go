package daemon

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"time"
)

// FileLockError indicates a file is temporarily locked by another process.
type FileLockError struct {
	Path string
	Err  error
}

func (e *FileLockError) Error() string {
	return "file is locked: " + e.Path
}

func (e *FileLockError) Unwrap() error {
	return e.Err
}

// ReadFileWithRetry attempts to read a file, retrying if it's temporarily locked.
// This is particularly important on Windows where files may be locked by IDEs,
// virus scanners, or other processes.
func ReadFileWithRetry(path string, maxRetries int) ([]byte, error) {
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		content, err := os.ReadFile(path)
		if err == nil {
			return content, nil
		}
		lastErr = err

		// Check if it's a lock error worth retrying
		if !isFileLocked(err) {
			// Not a lock error, don't retry
			return nil, err
		}

		// Wait before retry with exponential backoff
		backoff := time.Duration(100*(attempt+1)) * time.Millisecond
		time.Sleep(backoff)
	}

	return nil, &FileLockError{Path: path, Err: lastErr}
}

// isFileLocked checks if an error indicates the file is temporarily locked.
// This handles Windows-specific sharing violation errors.
func isFileLocked(err error) bool {
	if err == nil {
		return false
	}

	// Check for permission denied (may indicate lock on some systems)
	if os.IsPermission(err) {
		return true
	}

	// On Windows, check for specific sharing violation errors
	if runtime.GOOS == "windows" {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			if errno, ok := pathErr.Err.(syscall.Errno); ok {
				// Windows error codes:
				// ERROR_SHARING_VIOLATION = 32
				// ERROR_LOCK_VIOLATION = 33
				return errno == 32 || errno == 33
			}
		}
	}

	return false
}
