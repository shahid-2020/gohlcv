package httpclient

import (
	"context"
	"net/http"
	"time"

	"github.com/shahid-2020/gohlcv/internal/ratelimit"
	"github.com/shahid-2020/gohlcv/internal/retry"
)

type Client struct {
	httpClient    *http.Client
	limiter       *ratelimit.RateLimiter
	retryer       *retry.Retryer
	retryOnStatus []uint
}

type RateLimitConfig struct {
	RequestsPerSecond int
	RequestsPerMinute int
	RequestsPerHour   int
}

type RetryConfig struct {
	MaxRetries    uint
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	RetryOnStatus []uint
}

type ClientConfig struct {
	HttpClient      *http.Client
	RateLimitConfig RateLimitConfig
	RetryConfig     RetryConfig
}

func NewClient(config ClientConfig) *Client {
	if config.HttpClient == nil {
		config.HttpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &Client{
		httpClient:    config.HttpClient,
		limiter:       ratelimit.NewRateLimiter(config.RateLimitConfig.RequestsPerSecond, config.RateLimitConfig.RequestsPerMinute, config.RateLimitConfig.RequestsPerHour),
		retryer:       retry.NewRetryer(config.RetryConfig.MaxRetries, config.RetryConfig.BaseDelay, config.RetryConfig.MaxDelay),
		retryOnStatus: config.RetryConfig.RetryOnStatus,
	}
}

func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response

	err := c.retryer.Do(ctx, func() (bool, error) {
		if err := c.limiter.Wait(ctx); err != nil {
			return false, err
		}

		var err error
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return true, err
		}

		if c.retryOnStatus != nil {
			for _, status := range c.retryOnStatus {
				if resp.StatusCode == int(status) {
					resp.Body.Close()
					return true, nil
				}
			}
		}

		return false, nil
	})

	return resp, err
}
