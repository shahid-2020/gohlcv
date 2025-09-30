package ratelimit

import (
	"context"
	"sync"
	"time"
)

type RateLimiter struct {
	mu                sync.Mutex
	secCount          int
	minCount          int
	hrCount           int
	secReset          time.Time
	minReset          time.Time
	hrReset           time.Time
	requestsPerSecond int
	requestsPerMinute int
	requestsPerHour   int
}

func NewRateLimiter(requestsPerSecond, requestsPerMinute, requestsPerHour int) *RateLimiter {
	now := time.Now().UTC()
	return &RateLimiter{
		secReset:          now.Add(time.Second),
		minReset:          now.Add(time.Minute),
		hrReset:           now.Add(time.Hour),
		requestsPerSecond: requestsPerSecond,
		requestsPerMinute: requestsPerMinute,
		requestsPerHour:   requestsPerHour,
	}
}

func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if r.canProceed() {
			r.increment()
			return nil
		}

		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *RateLimiter) canProceed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	r.resetIfNeeded(now)

	return r.secCount < r.requestsPerSecond &&
		r.minCount < r.requestsPerMinute &&
		r.hrCount < r.requestsPerHour
}

func (r *RateLimiter) resetIfNeeded(now time.Time) {
	if now.After(r.secReset) {
		r.secCount = 0
		r.secReset = now.Add(1 * time.Second)
	}

	if now.After(r.minReset) {
		r.minCount = 0
		r.minReset = now.Add(1 * time.Minute)
	}

	if now.After(r.hrReset) {
		r.hrCount = 0
		r.hrReset = now.Add(1 * time.Hour)
	}
}

func (r *RateLimiter) increment() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.secCount++
	r.minCount++
	r.hrCount++
}
