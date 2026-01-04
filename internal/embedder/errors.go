package embedder

import (
	"errors"
	"time"
)

// EmbeddingError represents an error from embedding operations with helpful context.
type EmbeddingError struct {
	Code       string
	Message    string
	Suggestion string
	Retryable  bool
	RetryAfter time.Duration
	Cause      error
}

// Error implements the error interface.
func (e *EmbeddingError) Error() string {
	if e.Suggestion == "" {
		return e.Message
	}
	return e.Message + ". " + e.Suggestion
}

// Unwrap returns the underlying cause for errors.Is/As compatibility.
func (e *EmbeddingError) Unwrap() error {
	return e.Cause
}

// WithCause returns a copy of the error with the given cause.
func (e *EmbeddingError) WithCause(cause error) *EmbeddingError {
	return &EmbeddingError{
		Code:       e.Code,
		Message:    e.Message,
		Suggestion: e.Suggestion,
		Retryable:  e.Retryable,
		RetryAfter: e.RetryAfter,
		Cause:      cause,
	}
}

// WithRetryAfter returns a copy of the error with the given retry duration.
func (e *EmbeddingError) WithRetryAfter(d time.Duration) *EmbeddingError {
	return &EmbeddingError{
		Code:       e.Code,
		Message:    e.Message,
		Suggestion: e.Suggestion,
		Retryable:  e.Retryable,
		RetryAfter: d,
		Cause:      e.Cause,
	}
}

// Predefined embedding errors
var (
	// ErrRateLimited indicates the API rate limit was exceeded
	ErrRateLimited = &EmbeddingError{
		Code:       "RATE_LIMITED",
		Message:    "API rate limit exceeded",
		Suggestion: "Waiting to retry automatically...",
		Retryable:  true,
	}

	// ErrAuthFailed indicates invalid API credentials
	ErrAuthFailed = &EmbeddingError{
		Code:       "AUTH_FAILED",
		Message:    "Invalid API key",
		Suggestion: "Run 'pm config provider' to update your API key",
		Retryable:  false,
	}

	// ErrQuotaExceeded indicates the API quota has been exhausted
	ErrQuotaExceeded = &EmbeddingError{
		Code:       "QUOTA_EXCEEDED",
		Message:    "API quota exhausted",
		Suggestion: "Check your billing at the provider's dashboard",
		Retryable:  false,
	}

	// ErrProviderUnavailable indicates the embedding provider is not responding
	ErrProviderUnavailable = &EmbeddingError{
		Code:       "PROVIDER_UNAVAILABLE",
		Message:    "Embedding provider is not responding",
		Suggestion: "Check your network connection and provider status",
		Retryable:  true,
	}

	// ErrInvalidRequest indicates the request was malformed
	ErrInvalidRequest = &EmbeddingError{
		Code:      "INVALID_REQUEST",
		Message:   "Invalid embedding request",
		Retryable: false,
	}

	// ErrProviderNotConfigured indicates no embedding provider is configured
	ErrProviderNotConfigured = &EmbeddingError{
		Code:       "PROVIDER_NOT_CONFIGURED",
		Message:    "No embedding provider configured",
		Suggestion: "Run 'pm config provider' to configure an embedding provider",
		Retryable:  false,
	}
)

// IsRetryableError returns true if the error is retryable.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var embErr *EmbeddingError
	if errors.As(err, &embErr) {
		return embErr.Retryable
	}
	return false
}

// GetRetryAfter returns the retry-after duration from an error, or 0 if not available.
func GetRetryAfter(err error) time.Duration {
	if err == nil {
		return 0
	}
	var embErr *EmbeddingError
	if errors.As(err, &embErr) {
		return embErr.RetryAfter
	}
	return 0
}
