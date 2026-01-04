package embedder

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// RetryConfig configures retry behavior for embedding operations
type RetryConfig struct {
	MaxRetries  int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
}

// DefaultRetryConfig returns sensible defaults for retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  3,
		BaseBackoff: 1 * time.Second,
		MaxBackoff:  30 * time.Second,
	}
}

// WithRetry executes a function with automatic retry for retryable errors
func WithRetry(ctx context.Context, fn func() error, cfg RetryConfig) error {
	if cfg.MaxRetries == 0 {
		cfg = DefaultRetryConfig()
	}

	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		var embErr *EmbeddingError
		if !errors.As(err, &embErr) || !embErr.Retryable {
			return err // Non-retryable, return immediately
		}

		if attempt == cfg.MaxRetries {
			break // No more retries
		}

		// Calculate backoff
		backoff := embErr.RetryAfter
		if backoff == 0 {
			// Exponential backoff: base * 2^attempt
			backoff = cfg.BaseBackoff * time.Duration(1<<attempt)
			if cfg.MaxBackoff > 0 && backoff > cfg.MaxBackoff {
				backoff = cfg.MaxBackoff
			}
		}

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}
