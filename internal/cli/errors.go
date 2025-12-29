package cli

import (
	"fmt"
	"strings"
)

// CLIError represents a user-friendly error with context and suggestions.
type CLIError struct {
	Message    string
	Suggestion string
	Cause      error
}

// Error implements the error interface.
func (e *CLIError) Error() string {
	var sb strings.Builder
	sb.WriteString(e.Message)
	if e.Suggestion != "" {
		sb.WriteString("\n\nSuggestion: ")
		sb.WriteString(e.Suggestion)
	}
	return sb.String()
}

// Unwrap returns the underlying cause for errors.Is/As compatibility.
func (e *CLIError) Unwrap() error {
	return e.Cause
}

// NewCLIError creates a new CLIError with a message and suggestion.
func NewCLIError(message, suggestion string) *CLIError {
	return &CLIError{
		Message:    message,
		Suggestion: suggestion,
	}
}

// WrapError wraps an existing error with additional context.
func WrapError(cause error, message, suggestion string) *CLIError {
	return &CLIError{
		Message:    message,
		Suggestion: suggestion,
		Cause:      cause,
	}
}

// =============================================================================
// Common CLI Errors
// =============================================================================

// ErrNotInitialized returns an error for uninitialized projects.
func ErrNotInitialized() *CLIError {
	return &CLIError{
		Message:    "Pommel has not been initialized in this project",
		Suggestion: "Run 'pm init' in your project root to set up Pommel",
	}
}

// ErrDaemonNotRunning returns an error when the daemon is not running.
func ErrDaemonNotRunning() *CLIError {
	return &CLIError{
		Message:    "Pommel daemon is not running",
		Suggestion: "Start the daemon with 'pm start'",
	}
}

// ErrDaemonAlreadyRunning returns an error when daemon is already running.
func ErrDaemonAlreadyRunning(pid int) *CLIError {
	return &CLIError{
		Message:    fmt.Sprintf("Pommel daemon is already running (PID %d)", pid),
		Suggestion: "Use 'pm status' to check the daemon status, or 'pm stop' to stop it first",
	}
}

// ErrDaemonStartFailed returns an error when daemon fails to start.
func ErrDaemonStartFailed(cause error) *CLIError {
	return &CLIError{
		Message:    "Failed to start Pommel daemon",
		Suggestion: "Check if pommeld is installed and in your PATH. You may need to run 'go install ./cmd/pommeld'",
		Cause:      cause,
	}
}

// ErrDaemonHealthTimeout returns an error when daemon doesn't respond.
func ErrDaemonHealthTimeout() *CLIError {
	return &CLIError{
		Message:    "Daemon failed to respond to health check within timeout",
		Suggestion: "The daemon may have crashed during startup. Check .pommel/logs/ for error messages, or try running 'pommeld' directly to see errors",
	}
}

// ErrDaemonConnectionFailed returns an error when connection to daemon fails.
func ErrDaemonConnectionFailed(cause error) *CLIError {
	return &CLIError{
		Message:    "Cannot connect to Pommel daemon",
		Suggestion: "Is the daemon running? Check with 'pm status' or start it with 'pm start'",
		Cause:      cause,
	}
}

// ErrConfigNotFound returns an error when config file is missing.
func ErrConfigNotFound() *CLIError {
	return &CLIError{
		Message:    "Configuration file not found",
		Suggestion: "Run 'pm init' to create a default configuration",
	}
}

// ErrConfigInvalid returns an error for invalid configuration.
func ErrConfigInvalid(cause error) *CLIError {
	return &CLIError{
		Message:    "Configuration file is invalid",
		Suggestion: "Check .pommel/config.yaml for syntax errors, or delete it and run 'pm init' to recreate",
		Cause:      cause,
	}
}

// ErrInvalidProjectRoot returns an error for invalid project directory.
func ErrInvalidProjectRoot(path string) *CLIError {
	return &CLIError{
		Message:    fmt.Sprintf("Invalid project root: %s", path),
		Suggestion: "Ensure the path exists and is a directory. Use --project to specify a different path",
	}
}

// ErrEmptyQuery returns an error for empty search queries.
func ErrEmptyQuery() *CLIError {
	return &CLIError{
		Message:    "Search query cannot be empty",
		Suggestion: "Provide a search query, e.g., 'pm search \"authentication middleware\"'",
	}
}

// ErrInvalidLevel returns an error for invalid chunk level filter.
func ErrInvalidLevel(level string, validLevels []string) *CLIError {
	return &CLIError{
		Message:    fmt.Sprintf("Invalid level filter: %s", level),
		Suggestion: fmt.Sprintf("Valid levels are: %s", strings.Join(validLevels, ", ")),
	}
}

// ErrReindexFailed returns an error when reindexing fails.
func ErrReindexFailed(cause error) *CLIError {
	return &CLIError{
		Message:    "Failed to trigger reindex",
		Suggestion: "Check if the daemon is running with 'pm status'",
		Cause:      cause,
	}
}

// ErrNoSearchResults returns a message for empty search results.
func ErrNoSearchResults(query string) *CLIError {
	return &CLIError{
		Message:    fmt.Sprintf("No results found for query: %s", query),
		Suggestion: "Try a different search query, or check that files have been indexed with 'pm status'",
	}
}
