package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// APIError represents a structured error response from the Pommel API.
// It provides clear information about what went wrong, why it might have happened,
// and how to fix it.
type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
	Details    string `json:"details,omitempty"`
}

// Error implements the error interface for APIError.
func (e APIError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("%s: %s. %s", e.Code, e.Message, e.Suggestion)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// WithDetails returns a copy of the error with additional details.
func (e APIError) WithDetails(details string) APIError {
	e.Details = details
	return e
}

// =============================================================================
// Query Errors
// =============================================================================

var (
	// ErrQueryEmpty is returned when a search query is empty or whitespace-only.
	ErrQueryEmpty = APIError{
		Code:       "QUERY_EMPTY",
		Message:    "Search query cannot be empty",
		Suggestion: "Provide a search query describing what you're looking for, e.g., 'authentication middleware' or 'database connection handling'",
	}

	// ErrQueryTooLong is returned when a search query exceeds the maximum length.
	ErrQueryTooLong = APIError{
		Code:       "QUERY_TOO_LONG",
		Message:    "Search query exceeds maximum length",
		Suggestion: "Try a shorter, more focused query. Semantic search works best with concise descriptions",
	}

	// ErrInvalidJSON is returned when the request body contains invalid JSON.
	ErrInvalidJSON = APIError{
		Code:       "INVALID_JSON",
		Message:    "Request body contains invalid JSON",
		Suggestion: "Check your JSON syntax and ensure all strings are properly quoted",
	}
)

// =============================================================================
// Daemon Errors
// =============================================================================

var (
	// ErrDaemonNotRunning is returned when the daemon is not running.
	ErrDaemonNotRunning = APIError{
		Code:       "DAEMON_NOT_RUNNING",
		Message:    "Pommel daemon is not running",
		Suggestion: "Start the daemon with 'pm start' or check if it crashed. You can view logs in .pommel/logs/",
	}

	// ErrDaemonAlreadyRunning is returned when trying to start a daemon that's already running.
	ErrDaemonAlreadyRunning = APIError{
		Code:       "DAEMON_ALREADY_RUNNING",
		Message:    "Pommel daemon is already running",
		Suggestion: "Use 'pm status' to check the daemon status, or 'pm stop' to stop it first",
	}

	// ErrDaemonStartFailed is returned when the daemon fails to start.
	ErrDaemonStartFailed = APIError{
		Code:       "DAEMON_START_FAILED",
		Message:    "Failed to start Pommel daemon",
		Suggestion: "Check if the port is already in use, or if you have permission to create files in the .pommel directory",
	}

	// ErrDaemonHealthCheckTimeout is returned when the daemon doesn't respond to health checks.
	ErrDaemonHealthCheckTimeout = APIError{
		Code:       "DAEMON_HEALTH_TIMEOUT",
		Message:    "Daemon failed to respond to health check within timeout",
		Suggestion: "The daemon may have crashed during startup. Check .pommel/logs/ for error messages",
	}
)

// =============================================================================
// Embedding/Ollama Errors
// =============================================================================

var (
	// ErrOllamaUnavailable is returned when Ollama is not running or accessible.
	ErrOllamaUnavailable = APIError{
		Code:       "OLLAMA_UNAVAILABLE",
		Message:    "Cannot connect to Ollama embedding service",
		Suggestion: "Is Ollama running? Start it with 'ollama serve' or check if it's listening on the configured port (default: 11434)",
	}

	// ErrOllamaModelNotFound is returned when the embedding model is not available.
	ErrOllamaModelNotFound = APIError{
		Code:       "OLLAMA_MODEL_NOT_FOUND",
		Message:    "Embedding model not found in Ollama",
		Suggestion: "Pull the model with 'ollama pull unclemusclez/jina-embeddings-v2-base-code' or check your embedding.model config",
	}

	// ErrOllamaTimeout is returned when Ollama takes too long to respond.
	ErrOllamaTimeout = APIError{
		Code:       "OLLAMA_TIMEOUT",
		Message:    "Ollama request timed out",
		Suggestion: "Ollama may be overloaded or the model may be loading. Try again in a few seconds",
	}

	// ErrEmbeddingFailed is returned when embedding generation fails.
	ErrEmbeddingFailed = APIError{
		Code:       "EMBEDDING_FAILED",
		Message:    "Failed to generate embeddings",
		Suggestion: "This may be a temporary issue. Check Ollama logs and try again",
	}
)

// =============================================================================
// Database Errors
// =============================================================================

var (
	// ErrDatabaseUnavailable is returned when the database cannot be accessed.
	ErrDatabaseUnavailable = APIError{
		Code:       "DATABASE_UNAVAILABLE",
		Message:    "Cannot access the Pommel database",
		Suggestion: "Check if the .pommel directory exists and has proper permissions. Try 'pm init' to reinitialize",
	}

	// ErrDatabaseCorrupted is returned when the database appears corrupted.
	ErrDatabaseCorrupted = APIError{
		Code:       "DATABASE_CORRUPTED",
		Message:    "Database appears to be corrupted",
		Suggestion: "Try running 'pm reindex --force' to rebuild the index, or delete .pommel/pommel.db and run 'pm init'",
	}

	// ErrSearchFailed is returned when a search operation fails.
	ErrSearchFailed = APIError{
		Code:       "SEARCH_FAILED",
		Message:    "Search operation failed",
		Suggestion: "Check if the daemon is running and the database is accessible",
	}
)

// =============================================================================
// Project Errors
// =============================================================================

var (
	// ErrProjectNotInitialized is returned when Pommel hasn't been initialized.
	ErrProjectNotInitialized = APIError{
		Code:       "PROJECT_NOT_INITIALIZED",
		Message:    "Pommel has not been initialized in this project",
		Suggestion: "Run 'pm init' in your project root to set up Pommel",
	}

	// ErrProjectAlreadyInitialized is returned when trying to init an already initialized project.
	ErrProjectAlreadyInitialized = APIError{
		Code:       "PROJECT_ALREADY_INITIALIZED",
		Message:    "Pommel is already initialized in this project",
		Suggestion: "The .pommel directory already exists. Use 'pm status' to check the current state",
	}

	// ErrInvalidProjectRoot is returned when the project root is invalid.
	ErrInvalidProjectRoot = APIError{
		Code:       "INVALID_PROJECT_ROOT",
		Message:    "Invalid project root directory",
		Suggestion: "Ensure the path exists and is a directory. Use --project to specify a different path",
	}
)

// =============================================================================
// Indexing Errors
// =============================================================================

var (
	// ErrIndexingInProgress is returned when an indexing operation is already running.
	ErrIndexingInProgress = APIError{
		Code:       "INDEXING_IN_PROGRESS",
		Message:    "An indexing operation is already in progress",
		Suggestion: "Wait for the current indexing to complete. Use 'pm status' to check progress",
	}

	// ErrNoFilesToIndex is returned when there are no files to index.
	ErrNoFilesToIndex = APIError{
		Code:       "NO_FILES_TO_INDEX",
		Message:    "No indexable files found in the project",
		Suggestion: "Check your .pommelignore patterns. Pommel indexes: .go, .py, .js, .ts, .java, .rs, .cs files by default",
	}
)

// =============================================================================
// HTTP Response Helpers
// =============================================================================

// WriteError writes an APIError as a JSON response with the appropriate status code.
func WriteError(w http.ResponseWriter, statusCode int, err APIError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(err)
}

// WriteBadRequest writes a 400 Bad Request response with the given error.
func WriteBadRequest(w http.ResponseWriter, err APIError) {
	WriteError(w, http.StatusBadRequest, err)
}

// WriteInternalError writes a 500 Internal Server Error response with the given error.
func WriteInternalError(w http.ResponseWriter, err APIError) {
	WriteError(w, http.StatusInternalServerError, err)
}

// WriteServiceUnavailable writes a 503 Service Unavailable response with the given error.
func WriteServiceUnavailable(w http.ResponseWriter, err APIError) {
	WriteError(w, http.StatusServiceUnavailable, err)
}

// NewError creates a custom APIError with the given code, message, and suggestion.
func NewError(code, message, suggestion string) APIError {
	return APIError{
		Code:       code,
		Message:    message,
		Suggestion: suggestion,
	}
}
