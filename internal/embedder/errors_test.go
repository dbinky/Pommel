package embedder

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// === EmbeddingError Tests ===

func TestEmbeddingError_Error(t *testing.T) {
	t.Run("with suggestion", func(t *testing.T) {
		err := &EmbeddingError{
			Code:       "TEST_ERROR",
			Message:    "Test error message",
			Suggestion: "Try again",
		}
		assert.Contains(t, err.Error(), "Test error message")
		assert.Contains(t, err.Error(), "Try again")
	})

	t.Run("without suggestion", func(t *testing.T) {
		err := &EmbeddingError{
			Code:    "TEST_ERROR",
			Message: "Test error message",
		}
		assert.Equal(t, "Test error message", err.Error())
	})
}

func TestEmbeddingError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &EmbeddingError{
		Code:    "WRAPPED",
		Message: "Wrapped error",
		Cause:   cause,
	}
	assert.Equal(t, cause, errors.Unwrap(err))
}

func TestEmbeddingError_Is(t *testing.T) {
	t.Run("same error", func(t *testing.T) {
		err := &EmbeddingError{Code: "RATE_LIMITED", Message: "Rate limited"}
		assert.True(t, errors.Is(err, err))
	})

	t.Run("different errors with same code", func(t *testing.T) {
		err1 := &EmbeddingError{Code: "RATE_LIMITED", Message: "Rate limited 1"}
		err2 := &EmbeddingError{Code: "RATE_LIMITED", Message: "Rate limited 2"}
		// errors.Is checks identity by default, not code equality
		assert.False(t, errors.Is(err1, err2))
	})
}

func TestEmbeddingError_Retryable(t *testing.T) {
	t.Run("rate limited is retryable", func(t *testing.T) {
		assert.True(t, ErrRateLimited.Retryable)
	})

	t.Run("provider unavailable is retryable", func(t *testing.T) {
		assert.True(t, ErrProviderUnavailable.Retryable)
	})

	t.Run("auth failed is not retryable", func(t *testing.T) {
		assert.False(t, ErrAuthFailed.Retryable)
	})

	t.Run("quota exceeded is not retryable", func(t *testing.T) {
		assert.False(t, ErrQuotaExceeded.Retryable)
	})

	t.Run("invalid request is not retryable", func(t *testing.T) {
		assert.False(t, ErrInvalidRequest.Retryable)
	})
}

func TestEmbeddingError_RetryAfter(t *testing.T) {
	err := &EmbeddingError{
		Code:       "RATE_LIMITED",
		Message:    "Rate limited",
		Retryable:  true,
		RetryAfter: 5 * time.Second,
	}
	assert.Equal(t, 5*time.Second, err.RetryAfter)
}

func TestEmbeddingError_WithCause(t *testing.T) {
	cause := errors.New("network failure")
	err := ErrProviderUnavailable.WithCause(cause)

	assert.Equal(t, "PROVIDER_UNAVAILABLE", err.Code)
	assert.Equal(t, cause, err.Cause)
	assert.True(t, errors.Is(err, cause))
}

func TestEmbeddingError_WithRetryAfter(t *testing.T) {
	err := ErrRateLimited.WithRetryAfter(10 * time.Second)

	assert.Equal(t, "RATE_LIMITED", err.Code)
	assert.Equal(t, 10*time.Second, err.RetryAfter)
	assert.True(t, err.Retryable)
}

// === Predefined Errors Tests ===

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        *EmbeddingError
		code       string
		retryable  bool
		hasSuggest bool
	}{
		{"ErrRateLimited", ErrRateLimited, "RATE_LIMITED", true, true},
		{"ErrAuthFailed", ErrAuthFailed, "AUTH_FAILED", false, true},
		{"ErrQuotaExceeded", ErrQuotaExceeded, "QUOTA_EXCEEDED", false, true},
		{"ErrProviderUnavailable", ErrProviderUnavailable, "PROVIDER_UNAVAILABLE", true, true},
		{"ErrInvalidRequest", ErrInvalidRequest, "INVALID_REQUEST", false, false},
		{"ErrProviderNotConfigured", ErrProviderNotConfigured, "PROVIDER_NOT_CONFIGURED", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, tt.retryable, tt.err.Retryable)
			if tt.hasSuggest {
				assert.NotEmpty(t, tt.err.Suggestion)
			}
			assert.NotEmpty(t, tt.err.Message)
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	t.Run("retryable embedding error", func(t *testing.T) {
		assert.True(t, IsRetryableError(ErrRateLimited))
	})

	t.Run("non-retryable embedding error", func(t *testing.T) {
		assert.False(t, IsRetryableError(ErrAuthFailed))
	})

	t.Run("wrapped retryable error", func(t *testing.T) {
		wrapped := &EmbeddingError{
			Code:      "WRAPPED",
			Message:   "Wrapped",
			Retryable: true,
			Cause:     errors.New("underlying"),
		}
		assert.True(t, IsRetryableError(wrapped))
	})

	t.Run("non-embedding error", func(t *testing.T) {
		assert.False(t, IsRetryableError(errors.New("random error")))
	})

	t.Run("nil error", func(t *testing.T) {
		assert.False(t, IsRetryableError(nil))
	})
}

func TestGetRetryAfter(t *testing.T) {
	t.Run("error with retry after", func(t *testing.T) {
		err := ErrRateLimited.WithRetryAfter(5 * time.Second)
		duration := GetRetryAfter(err)
		assert.Equal(t, 5*time.Second, duration)
	})

	t.Run("error without retry after", func(t *testing.T) {
		duration := GetRetryAfter(ErrRateLimited)
		assert.Zero(t, duration)
	})

	t.Run("non-embedding error", func(t *testing.T) {
		duration := GetRetryAfter(errors.New("random"))
		assert.Zero(t, duration)
	})

	t.Run("nil error", func(t *testing.T) {
		duration := GetRetryAfter(nil)
		assert.Zero(t, duration)
	})
}
