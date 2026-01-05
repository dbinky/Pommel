package embedder

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// === Happy Path Tests ===

func TestRetry_SucceedsFirstTry(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return nil
	}

	err := WithRetry(context.Background(), fn, RetryConfig{MaxRetries: 3})

	assert.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetry_SucceedsAfterRetries(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return &EmbeddingError{Retryable: true}
		}
		return nil
	}

	err := WithRetry(context.Background(), fn, RetryConfig{
		MaxRetries:  5,
		BaseBackoff: 10 * time.Millisecond,
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetry_RespectsRetryAfter(t *testing.T) {
	start := time.Now()
	attempts := 0

	fn := func() error {
		attempts++
		if attempts == 1 {
			return &EmbeddingError{
				Retryable:  true,
				RetryAfter: 100 * time.Millisecond,
			}
		}
		return nil
	}

	WithRetry(context.Background(), fn, RetryConfig{MaxRetries: 3})

	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond)
}

// === Failure Scenario Tests ===

func TestRetry_NonRetryableError(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return &EmbeddingError{
			Code:      "AUTH_FAILED",
			Retryable: false,
		}
	}

	err := WithRetry(context.Background(), fn, RetryConfig{MaxRetries: 5})

	assert.Error(t, err)
	assert.Equal(t, 1, attempts) // No retries for non-retryable
}

func TestRetry_MaxRetriesExceeded(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return &EmbeddingError{Retryable: true}
	}

	err := WithRetry(context.Background(), fn, RetryConfig{
		MaxRetries:  3,
		BaseBackoff: 10 * time.Millisecond,
	})

	assert.Error(t, err)
	assert.Equal(t, 4, attempts) // Initial + 3 retries
	assert.Contains(t, err.Error(), "max retries exceeded")
}

func TestRetry_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	fn := func() error {
		attempts++
		if attempts == 2 {
			cancel()
		}
		return &EmbeddingError{Retryable: true}
	}

	err := WithRetry(ctx, fn, RetryConfig{
		MaxRetries:  10,
		BaseBackoff: 10 * time.Millisecond,
	})

	assert.ErrorIs(t, err, context.Canceled)
	assert.LessOrEqual(t, attempts, 3)
}

func TestRetry_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	fn := func() error {
		return &EmbeddingError{Retryable: true}
	}

	err := WithRetry(ctx, fn, RetryConfig{
		MaxRetries:  100,
		BaseBackoff: 100 * time.Millisecond, // Longer than timeout
	})

	assert.Error(t, err)
}

// === Edge Case Tests ===

func TestRetry_ExponentialBackoff(t *testing.T) {
	var backoffs []time.Duration
	lastCall := time.Now()

	attempts := 0
	fn := func() error {
		attempts++
		now := time.Now()
		if attempts > 1 {
			backoffs = append(backoffs, now.Sub(lastCall))
		}
		lastCall = now
		if attempts < 4 {
			return &EmbeddingError{Retryable: true}
		}
		return nil
	}

	WithRetry(context.Background(), fn, RetryConfig{
		MaxRetries:  5,
		BaseBackoff: 20 * time.Millisecond,
	})

	// Verify backoffs increase (exponential)
	for i := 1; i < len(backoffs); i++ {
		assert.Greater(t, backoffs[i], backoffs[i-1]*time.Duration(8)/10) // Allow some variance
	}
}

func TestRetry_RegularErrorNotRetried(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("regular error")
	}

	err := WithRetry(context.Background(), fn, RetryConfig{MaxRetries: 3})

	// Regular errors are not retried
	assert.Error(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetry_EmptyConfig(t *testing.T) {
	fn := func() error { return nil }

	// Should use defaults
	assert.NotPanics(t, func() {
		WithRetry(context.Background(), fn, RetryConfig{})
	})
}

func TestRetry_MaxBackoffCap(t *testing.T) {
	var backoffs []time.Duration
	lastCall := time.Now()
	attempts := 0

	fn := func() error {
		attempts++
		now := time.Now()
		if attempts > 1 {
			backoffs = append(backoffs, now.Sub(lastCall))
		}
		lastCall = now
		if attempts < 5 {
			return &EmbeddingError{Retryable: true}
		}
		return nil
	}

	WithRetry(context.Background(), fn, RetryConfig{
		MaxRetries:  10,
		BaseBackoff: 50 * time.Millisecond,
		MaxBackoff:  100 * time.Millisecond,
	})

	// Later backoffs should be capped at max
	for _, b := range backoffs {
		assert.LessOrEqual(t, b, 150*time.Millisecond) // Some tolerance
	}
}
