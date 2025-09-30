package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rps := 10
	rpm := 100
	rph := 1000

	rl := NewRateLimiter(rps, rpm, rph)

	if rl.requestsPerSecond != rps {
		t.Errorf("Expected requestsPerSecond %d, got %d", rps, rl.requestsPerSecond)
	}
	if rl.requestsPerMinute != rpm {
		t.Errorf("Expected requestsPerMinute %d, got %d", rpm, rl.requestsPerMinute)
	}
	if rl.requestsPerHour != rph {
		t.Errorf("Expected requestsPerHour %d, got %d", rph, rl.requestsPerHour)
	}

	now := time.Now().UTC()
	if rl.secReset.Before(now) {
		t.Error("secReset should be in the future")
	}
	if rl.minReset.Before(now) {
		t.Error("minReset should be in the future")
	}
	if rl.hrReset.Before(now) {
		t.Error("hrReset should be in the future")
	}
}

func TestRateLimiter_Wait_Success(t *testing.T) {
	rl := NewRateLimiter(10, 100, 1000)

	ctx := context.Background()
	err := rl.Wait(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if rl.secCount != 1 {
		t.Errorf("Expected secCount to be 1, got %d", rl.secCount)
	}
	if rl.minCount != 1 {
		t.Errorf("Expected minCount to be 1, got %d", rl.minCount)
	}
	if rl.hrCount != 1 {
		t.Errorf("Expected hrCount to be 1, got %d", rl.hrCount)
	}
}

func TestRateLimiter_Wait_ContextCancelled(t *testing.T) {
	rl := NewRateLimiter(0, 0, 0) // Zero limits - will always block

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rl.Wait(ctx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestRateLimiter_Wait_ContextTimeout(t *testing.T) {
	rl := NewRateLimiter(0, 0, 0) // Zero limits - will always block

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := rl.Wait(ctx)
	elapsed := time.Since(start)

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded error, got %v", err)
	}

	if elapsed < 10*time.Millisecond {
		t.Errorf("Expected to wait at least 10ms, waited %v", elapsed)
	}
}

func TestRateLimiter_Wait_RespectsAllLimits(t *testing.T) {
	rl := NewRateLimiter(2, 100, 1000)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		err := rl.Wait(ctx)
		if err != nil {
			t.Errorf("Request %d: Expected no error, got %v", i, err)
		}
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- rl.Wait(ctxWithTimeout)
	}()

	select {
	case err := <-done:
		if err != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Test timed out - goroutine is stuck")
	}
}
func TestRateLimiter_CanProceed_AllLimitsZero(t *testing.T) {
	rl := NewRateLimiter(0, 0, 0)

	if rl.canProceed() {
		t.Error("Expected canProceed to return false when all limits are zero")
	}
}

func TestRateLimiter_CanProceed_SecondLimitReached(t *testing.T) {
	rl := NewRateLimiter(1, 100, 1000)

	if !rl.canProceed() {
		t.Error("First call: Expected canProceed to return true")
	}

	rl.increment()

	if rl.canProceed() {
		t.Error("Second call: Expected canProceed to return false (second limit reached)")
	}
}

func TestRateLimiter_CanProceed_MinuteLimitReached(t *testing.T) {
	rl := NewRateLimiter(100, 1, 1000)

	if !rl.canProceed() {
		t.Error("First call: Expected canProceed to return true")
	}
	rl.increment()

	if rl.canProceed() {
		t.Error("Second call: Expected canProceed to return false (minute limit reached)")
	}
}

func TestRateLimiter_CanProceed_HourLimitReached(t *testing.T) {
	rl := NewRateLimiter(100, 1000, 1)

	if !rl.canProceed() {
		t.Error("First call: Expected canProceed to return true")
	}

	rl.increment()

	if rl.canProceed() {
		t.Error("Second call: Expected canProceed to return false (hour limit reached)")
	}
}

func TestRateLimiter_ResetIfNeeded(t *testing.T) {
	rl := NewRateLimiter(10, 100, 1000)

	rl.increment()
	rl.increment()

	if rl.secCount != 2 {
		t.Errorf("Expected secCount to be 2, got %d", rl.secCount)
	}

	past := time.Now().UTC().Add(-time.Second)
	rl.secReset = past
	rl.minReset = past
	rl.hrReset = past

	rl.resetIfNeeded(time.Now().UTC())

	if rl.secCount != 0 {
		t.Errorf("Expected secCount to be reset to 0, got %d", rl.secCount)
	}
	if rl.minCount != 0 {
		t.Errorf("Expected minCount to be reset to 0, got %d", rl.minCount)
	}
	if rl.hrCount != 0 {
		t.Errorf("Expected hrCount to be reset to 0, got %d", rl.hrCount)
	}

	now := time.Now().UTC()
	if rl.secReset.Before(now) {
		t.Error("secReset should be updated to future")
	}
	if rl.minReset.Before(now) {
		t.Error("minReset should be updated to future")
	}
	if rl.hrReset.Before(now) {
		t.Error("hrReset should be updated to future")
	}
}

func TestRateLimiter_ResetIfNeeded_PartialReset(t *testing.T) {
	rl := NewRateLimiter(10, 100, 1000)

	rl.increment()
	rl.increment()

	rl.secReset = time.Now().UTC().Add(-time.Second)

	rl.resetIfNeeded(time.Now().UTC())

	if rl.secCount != 0 {
		t.Errorf("Expected secCount to be reset to 0, got %d", rl.secCount)
	}
	if rl.minCount != 2 {
		t.Errorf("Expected minCount to remain 2, got %d", rl.minCount)
	}
	if rl.hrCount != 2 {
		t.Errorf("Expected hrCount to remain 2, got %d", rl.hrCount)
	}
}

func TestRateLimiter_Increment(t *testing.T) {
	rl := NewRateLimiter(10, 100, 1000)

	initialSec := rl.secCount
	initialMin := rl.minCount
	initialHr := rl.hrCount

	rl.increment()

	if rl.secCount != initialSec+1 {
		t.Errorf("Expected secCount to be %d, got %d", initialSec+1, rl.secCount)
	}
	if rl.minCount != initialMin+1 {
		t.Errorf("Expected minCount to be %d, got %d", initialMin+1, rl.minCount)
	}
	if rl.hrCount != initialHr+1 {
		t.Errorf("Expected hrCount to be %d, got %d", initialHr+1, rl.hrCount)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(100, 1000, 10000)

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := rl.Wait(context.Background())
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if successCount != 10 {
		t.Errorf("Expected 10 successful requests, got %d", successCount)
	}

	if rl.secCount != 10 {
		t.Errorf("Expected secCount to be 10, got %d", rl.secCount)
	}
}

func TestRateLimiter_Wait_AfterReset(t *testing.T) {
	rl := NewRateLimiter(1, 100, 1000)

	err := rl.Wait(context.Background())
	if err != nil {
		t.Errorf("First request: Expected no error, got %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- rl.Wait(context.Background())
	}()

	select {
	case err := <-done:
		t.Errorf("Expected request to block, but got: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	rl.mu.Lock()
	rl.secCount = 0
	rl.secReset = time.Now().UTC().Add(time.Second)
	rl.mu.Unlock()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("After reset: Expected no error, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Request should have succeeded after reset")
	}
}

func TestRateLimiter_HighLimits(t *testing.T) {
	rl := NewRateLimiter(1000000, 10000000, 100000000)

	ctx := context.Background()
	for i := 0; i < 100; i++ {
		err := rl.Wait(ctx)
		if err != nil {
			t.Errorf("Request %d: Expected no error, got %v", i, err)
		}
	}
}

func TestRateLimiter_MinimalLimits(t *testing.T) {
	rl := NewRateLimiter(1, 1, 1)
	ctx := context.Background()

	err := rl.Wait(ctx)
	if err != nil {
		t.Errorf("First request: Expected no error, got %v", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	err = rl.Wait(ctxWithTimeout)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}
