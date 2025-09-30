package retry

import (
	"context"
	"time"
)

type Retryer struct {
	maxRetries uint
	baseDelay  time.Duration
	maxDelay   time.Duration
}

func NewRetryer(maxRetries uint, baseDelay time.Duration, maxDelay time.Duration) *Retryer {
	return &Retryer{
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		maxDelay:   maxDelay,
	}
}

func (r *Retryer) Do(ctx context.Context, fn func() (shouldRetry bool, err error)) error {
	var lastErr error

	for attempt := range r.maxRetries + 1 {
		if err := ctx.Err(); err != nil {
			return err
		}

		shouldRetry, err := fn()
		if !shouldRetry {
			return err
		}
		lastErr = err

		if attempt < r.maxRetries {
			delay := r.calculateBackoff(attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

	}

	return lastErr
}

func (r *Retryer) calculateBackoff(attempt uint) time.Duration {
	delay := r.baseDelay * (1 << attempt)
	return min(delay, r.maxDelay)
}
