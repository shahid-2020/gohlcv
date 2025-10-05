package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewRetryer(t *testing.T) {
	maxRetries := uint(3)
	baseDelay := 100 * time.Millisecond
	maxDelay := 1 * time.Second

	retryer := NewRetryer(maxRetries, baseDelay, maxDelay)

	if retryer.maxRetries != maxRetries {
		t.Errorf("Expected maxRetries %d, got %d", maxRetries, retryer.maxRetries)
	}
	if retryer.baseDelay != baseDelay {
		t.Errorf("Expected baseDelay %v, got %v", baseDelay, retryer.baseDelay)
	}
	if retryer.maxDelay != maxDelay {
		t.Errorf("Expected maxDelay %v, got %v", maxDelay, retryer.maxDelay)
	}
}

func TestRetryer_Do_SuccessOnFirstAttempt(t *testing.T) {
	retryer := NewRetryer(3, 10*time.Millisecond, 1*time.Second)

	called := 0
	err := retryer.Do(context.Background(), func() (bool, error) {
		called++
		return false, nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if called != 1 {
		t.Errorf("Expected function to be called 1 time, got %d", called)
	}
}

func TestRetryer_Do_SuccessAfterRetries(t *testing.T) {
	retryer := NewRetryer(3, 10*time.Millisecond, 1*time.Second)

	attempts := 0
	err := retryer.Do(context.Background(), func() (bool, error) {
		attempts++
		if attempts < 3 {
			return true, errors.New("temporary error")
		}
		return false, nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected function to be called 3 times, got %d", attempts)
	}
}

func TestRetryer_Do_MaxRetriesExceeded(t *testing.T) {
	retryer := NewRetryer(2, 10*time.Millisecond, 1*time.Second)

	expectedErr := errors.New("persistent error")
	attempts := 0
	err := retryer.Do(context.Background(), func() (bool, error) {
		attempts++
		return true, expectedErr
	})

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
	if attempts != 3 {
		t.Errorf("Expected function to be called 3 times, got %d", attempts)
	}
}

func TestRetryer_Do_ContextCancelledBeforeFirstAttempt(t *testing.T) {
	retryer := NewRetryer(3, 100*time.Millisecond, 1*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := 0
	err := retryer.Do(ctx, func() (bool, error) {
		called++
		return true, errors.New("should not be called")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
	if called != 0 {
		t.Errorf("Expected function not to be called, but was called %d times", called)
	}
}

func TestRetryer_Do_ContextCancelledDuringRetry(t *testing.T) {
	retryer := NewRetryer(3, 100*time.Millisecond, 1*time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := retryer.Do(ctx, func() (bool, error) {
		attempts++
		return true, errors.New("temporary error")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
	if attempts < 1 {
		t.Errorf("Expected function to be called at least once, got %d", attempts)
	}
}

func TestRetryer_Do_ContextTimeout(t *testing.T) {
	retryer := NewRetryer(5, 50*time.Millisecond, 1*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	attempts := 0
	err := retryer.Do(ctx, func() (bool, error) {
		attempts++
		return true, errors.New("temporary error")
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded error, got %v", err)
	}
	if attempts < 1 {
		t.Errorf("Expected function to be called at least once, got %d", attempts)
	}
}

func TestRetryer_Do_NoRetries(t *testing.T) {
	retryer := NewRetryer(0, 10*time.Millisecond, 1*time.Second)

	attempts := 0
	expectedErr := errors.New("first attempt error")
	err := retryer.Do(context.Background(), func() (bool, error) {
		attempts++
		return true, expectedErr
	})

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
	if attempts != 1 {
		t.Errorf("Expected function to be called 1 time, got %d", attempts)
	}
}

func TestRetryer_Do_SuccessWithNilError(t *testing.T) {
	retryer := NewRetryer(3, 10*time.Millisecond, 1*time.Second)

	called := 0
	err := retryer.Do(context.Background(), func() (bool, error) {
		called++
		return false, nil // Success with nil error
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if called != 1 {
		t.Errorf("Expected function to be called 1 time, got %d", called)
	}
}

func TestRetryer_CalculateBackoff(t *testing.T) {
	retryer := NewRetryer(5, 100*time.Millisecond, 1*time.Second)

	testCases := []struct {
		name     string
		attempt  uint
		expected time.Duration
	}{
		{"Attempt 0", 0, 100 * time.Millisecond}, // 100ms * 2^0 = 100ms
		{"Attempt 1", 1, 200 * time.Millisecond}, // 100ms * 2^1 = 200ms
		{"Attempt 2", 2, 400 * time.Millisecond}, // 100ms * 2^2 = 400ms
		{"Attempt 3", 3, 800 * time.Millisecond}, // 100ms * 2^3 = 800ms
		{"Attempt 4", 4, 1 * time.Second},        // Capped at maxDelay
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			delay := retryer.calculateBackoff(tc.attempt)
			if delay != tc.expected {
				t.Errorf("For attempt %d, expected delay %v, got %v", tc.attempt, tc.expected, delay)
			}
		})
	}
}

func TestRetryer_CalculateBackoff_ZeroBaseDelay(t *testing.T) {
	retryer := NewRetryer(3, 0, 1*time.Second)

	delay := retryer.calculateBackoff(2)
	if delay != 0 {
		t.Errorf("Expected 0 delay with zero base delay, got %v", delay)
	}
}

func TestRetryer_Do_BackoffTiming(t *testing.T) {
	retryer := NewRetryer(3, 50*time.Millisecond, 200*time.Millisecond)

	start := time.Now()
	attempts := 0

	err := retryer.Do(context.Background(), func() (bool, error) {
		attempts++
		if attempts < 3 {
			return true, errors.New("temporary error")
		}
		return false, nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	elapsed := time.Since(start)
	minExpected := 140 * time.Millisecond
	if elapsed < minExpected {
		t.Errorf("Expected elapsed time to be at least %v, got %v", minExpected, elapsed)
	}
}
