package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// APIError Type Tests
// =============================================================================

// TestAPIErrorImplementsErrorInterface verifies that APIError implements the error interface
func TestAPIErrorImplementsErrorInterface(t *testing.T) {
	var err error = APIError{
		Code:    "TEST_ERROR",
		Message: "Test error message",
	}
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "TEST_ERROR")
}

// TestAPIErrorErrorMethod verifies the Error() method output format
func TestAPIErrorErrorMethod(t *testing.T) {
	testCases := []struct {
		name     string
		err      APIError
		expected string
	}{
		{
			name: "without suggestion",
			err: APIError{
				Code:    "TEST_CODE",
				Message: "Test message",
			},
			expected: "TEST_CODE: Test message",
		},
		{
			name: "with suggestion",
			err: APIError{
				Code:       "TEST_CODE",
				Message:    "Test message",
				Suggestion: "Try this fix",
			},
			expected: "TEST_CODE: Test message. Try this fix",
		},
		{
			name: "with details but no suggestion",
			err: APIError{
				Code:    "TEST_CODE",
				Message: "Test message",
				Details: "Additional details",
			},
			expected: "TEST_CODE: Test message",
		},
		{
			name: "empty error",
			err: APIError{
				Code:    "",
				Message: "",
			},
			expected: ": ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.err.Error())
		})
	}
}

// TestAPIErrorWithDetails verifies the WithDetails() method
func TestAPIErrorWithDetails(t *testing.T) {
	original := APIError{
		Code:       "ORIGINAL_CODE",
		Message:    "Original message",
		Suggestion: "Original suggestion",
	}

	withDetails := original.WithDetails("Additional details here")

	// Original should be unchanged
	assert.Equal(t, "", original.Details)

	// New error should have details
	assert.Equal(t, "Additional details here", withDetails.Details)
	assert.Equal(t, original.Code, withDetails.Code)
	assert.Equal(t, original.Message, withDetails.Message)
	assert.Equal(t, original.Suggestion, withDetails.Suggestion)
}

// TestAPIErrorWithDetailsChaining verifies that WithDetails can be called multiple times
func TestAPIErrorWithDetailsChaining(t *testing.T) {
	err := ErrQueryEmpty.WithDetails("first detail").WithDetails("second detail")
	assert.Equal(t, "second detail", err.Details)
}

// TestNewError verifies the NewError constructor function
func TestNewError(t *testing.T) {
	err := NewError("CUSTOM_CODE", "Custom message", "Custom suggestion")

	assert.Equal(t, "CUSTOM_CODE", err.Code)
	assert.Equal(t, "Custom message", err.Message)
	assert.Equal(t, "Custom suggestion", err.Suggestion)
	assert.Equal(t, "", err.Details)
}

// TestNewErrorWithEmptyValues verifies NewError with empty values
func TestNewErrorWithEmptyValues(t *testing.T) {
	err := NewError("", "", "")

	assert.Equal(t, "", err.Code)
	assert.Equal(t, "", err.Message)
	assert.Equal(t, "", err.Suggestion)
}

// =============================================================================
// Predefined Error Tests
// =============================================================================

// TestPredefinedQueryErrors verifies the predefined query errors
func TestPredefinedQueryErrors(t *testing.T) {
	testCases := []struct {
		name string
		err  APIError
		code string
	}{
		{"ErrQueryEmpty", ErrQueryEmpty, "QUERY_EMPTY"},
		{"ErrQueryTooLong", ErrQueryTooLong, "QUERY_TOO_LONG"},
		{"ErrInvalidJSON", ErrInvalidJSON, "INVALID_JSON"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.code, tc.err.Code)
			assert.NotEmpty(t, tc.err.Message)
			assert.NotEmpty(t, tc.err.Suggestion)
		})
	}
}

// TestPredefinedDaemonErrors verifies the predefined daemon errors
func TestPredefinedDaemonErrors(t *testing.T) {
	testCases := []struct {
		name string
		err  APIError
		code string
	}{
		{"ErrDaemonNotRunning", ErrDaemonNotRunning, "DAEMON_NOT_RUNNING"},
		{"ErrDaemonAlreadyRunning", ErrDaemonAlreadyRunning, "DAEMON_ALREADY_RUNNING"},
		{"ErrDaemonStartFailed", ErrDaemonStartFailed, "DAEMON_START_FAILED"},
		{"ErrDaemonHealthCheckTimeout", ErrDaemonHealthCheckTimeout, "DAEMON_HEALTH_TIMEOUT"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.code, tc.err.Code)
			assert.NotEmpty(t, tc.err.Message)
			assert.NotEmpty(t, tc.err.Suggestion)
		})
	}
}

// TestPredefinedOllamaErrors verifies the predefined Ollama/embedding errors
func TestPredefinedOllamaErrors(t *testing.T) {
	testCases := []struct {
		name string
		err  APIError
		code string
	}{
		{"ErrOllamaUnavailable", ErrOllamaUnavailable, "OLLAMA_UNAVAILABLE"},
		{"ErrOllamaModelNotFound", ErrOllamaModelNotFound, "OLLAMA_MODEL_NOT_FOUND"},
		{"ErrOllamaTimeout", ErrOllamaTimeout, "OLLAMA_TIMEOUT"},
		{"ErrEmbeddingFailed", ErrEmbeddingFailed, "EMBEDDING_FAILED"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.code, tc.err.Code)
			assert.NotEmpty(t, tc.err.Message)
			assert.NotEmpty(t, tc.err.Suggestion)
		})
	}
}

// TestPredefinedDatabaseErrors verifies the predefined database errors
func TestPredefinedDatabaseErrors(t *testing.T) {
	testCases := []struct {
		name string
		err  APIError
		code string
	}{
		{"ErrDatabaseUnavailable", ErrDatabaseUnavailable, "DATABASE_UNAVAILABLE"},
		{"ErrDatabaseCorrupted", ErrDatabaseCorrupted, "DATABASE_CORRUPTED"},
		{"ErrSearchFailed", ErrSearchFailed, "SEARCH_FAILED"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.code, tc.err.Code)
			assert.NotEmpty(t, tc.err.Message)
			assert.NotEmpty(t, tc.err.Suggestion)
		})
	}
}

// TestPredefinedProjectErrors verifies the predefined project errors
func TestPredefinedProjectErrors(t *testing.T) {
	testCases := []struct {
		name string
		err  APIError
		code string
	}{
		{"ErrProjectNotInitialized", ErrProjectNotInitialized, "PROJECT_NOT_INITIALIZED"},
		{"ErrProjectAlreadyInitialized", ErrProjectAlreadyInitialized, "PROJECT_ALREADY_INITIALIZED"},
		{"ErrInvalidProjectRoot", ErrInvalidProjectRoot, "INVALID_PROJECT_ROOT"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.code, tc.err.Code)
			assert.NotEmpty(t, tc.err.Message)
			assert.NotEmpty(t, tc.err.Suggestion)
		})
	}
}

// TestPredefinedIndexingErrors verifies the predefined indexing errors
func TestPredefinedIndexingErrors(t *testing.T) {
	testCases := []struct {
		name string
		err  APIError
		code string
	}{
		{"ErrIndexingInProgress", ErrIndexingInProgress, "INDEXING_IN_PROGRESS"},
		{"ErrNoFilesToIndex", ErrNoFilesToIndex, "NO_FILES_TO_INDEX"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.code, tc.err.Code)
			assert.NotEmpty(t, tc.err.Message)
			assert.NotEmpty(t, tc.err.Suggestion)
		})
	}
}

// =============================================================================
// WriteError Helper Tests
// =============================================================================

// TestWriteError verifies the WriteError function
func TestWriteError(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		err        APIError
	}{
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			err:        ErrQueryEmpty,
		},
		{
			name:       "internal server error",
			statusCode: http.StatusInternalServerError,
			err:        ErrSearchFailed,
		},
		{
			name:       "service unavailable",
			statusCode: http.StatusServiceUnavailable,
			err:        ErrDaemonNotRunning,
		},
		{
			name:       "custom error with details",
			statusCode: http.StatusBadRequest,
			err:        ErrInvalidJSON.WithDetails("unexpected EOF"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			WriteError(rr, tc.statusCode, tc.err)

			// Check status code
			assert.Equal(t, tc.statusCode, rr.Code)

			// Check content type
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

			// Check response body is valid JSON
			var response APIError
			err := json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tc.err.Code, response.Code)
			assert.Equal(t, tc.err.Message, response.Message)
			assert.Equal(t, tc.err.Suggestion, response.Suggestion)
			assert.Equal(t, tc.err.Details, response.Details)
		})
	}
}

// TestWriteBadRequest verifies the WriteBadRequest helper
func TestWriteBadRequest(t *testing.T) {
	rr := httptest.NewRecorder()
	testErr := ErrQueryEmpty

	WriteBadRequest(rr, testErr)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var response APIError
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, testErr.Code, response.Code)
}

// TestWriteInternalError verifies the WriteInternalError helper
func TestWriteInternalError(t *testing.T) {
	rr := httptest.NewRecorder()
	testErr := ErrSearchFailed.WithDetails("database connection lost")

	WriteInternalError(rr, testErr)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var response APIError
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, testErr.Code, response.Code)
	assert.Equal(t, testErr.Details, response.Details)
}

// TestWriteServiceUnavailable verifies the WriteServiceUnavailable helper
func TestWriteServiceUnavailable(t *testing.T) {
	rr := httptest.NewRecorder()
	testErr := ErrOllamaUnavailable

	WriteServiceUnavailable(rr, testErr)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var response APIError
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, testErr.Code, response.Code)
}

// =============================================================================
// JSON Serialization Tests
// =============================================================================

// TestAPIErrorJSONSerialization verifies JSON marshaling/unmarshaling
func TestAPIErrorJSONSerialization(t *testing.T) {
	original := APIError{
		Code:       "TEST_CODE",
		Message:    "Test message",
		Suggestion: "Test suggestion",
		Details:    "Test details",
	}

	// Marshal
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Verify JSON structure
	var raw map[string]string
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Equal(t, "TEST_CODE", raw["code"])
	assert.Equal(t, "Test message", raw["message"])
	assert.Equal(t, "Test suggestion", raw["suggestion"])
	assert.Equal(t, "Test details", raw["details"])

	// Unmarshal back
	var unmarshaled APIError
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, original, unmarshaled)
}

// TestAPIErrorJSONOmitsEmpty verifies that empty fields are omitted in JSON
func TestAPIErrorJSONOmitsEmpty(t *testing.T) {
	err := APIError{
		Code:    "TEST_CODE",
		Message: "Test message",
	}

	data, jsonErr := json.Marshal(err)
	require.NoError(t, jsonErr)

	var raw map[string]interface{}
	jsonErr = json.Unmarshal(data, &raw)
	require.NoError(t, jsonErr)

	// Suggestion and Details should be omitted (not present in JSON)
	_, hasSuggestion := raw["suggestion"]
	_, hasDetails := raw["details"]

	assert.False(t, hasSuggestion, "empty suggestion should be omitted")
	assert.False(t, hasDetails, "empty details should be omitted")
}

// TestWriteErrorWithEmptyFields verifies WriteError handles empty optional fields
func TestWriteErrorWithEmptyFields(t *testing.T) {
	rr := httptest.NewRecorder()
	err := APIError{
		Code:    "MINIMAL_ERROR",
		Message: "Minimal message",
	}

	WriteError(rr, http.StatusBadRequest, err)

	var raw map[string]interface{}
	jsonErr := json.Unmarshal(rr.Body.Bytes(), &raw)
	require.NoError(t, jsonErr)

	assert.Equal(t, "MINIMAL_ERROR", raw["code"])
	assert.Equal(t, "Minimal message", raw["message"])
}
