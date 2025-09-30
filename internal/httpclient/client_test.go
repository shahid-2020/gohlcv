package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

type mockResponse struct {
	statusCode int
	body       string
	err        error
}

type mockTransport struct {
	attempts  *int
	responses []*mockResponse
	index     int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.attempts != nil {
		*m.attempts++
	}

	if m.index >= len(m.responses) {
		return nil, errors.New("no more mock responses")
	}

	response := m.responses[m.index]
	m.index++

	if response.err != nil {
		return nil, response.err
	}

	return &http.Response{
		StatusCode: response.statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(response.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func TestNewClient(t *testing.T) {
	t.Run("WithCustomConfig", func(t *testing.T) {
		customHTTPClient := &http.Client{Timeout: 10 * time.Second}
		config := ClientConfig{
			HttpClient: customHTTPClient,
			RateLimitConfig: RateLimitConfig{
				RequestsPerSecond: 10,
				RequestsPerMinute: 100,
				RequestsPerHour:   1000,
			},
			RetryConfig: RetryConfig{
				MaxRetries:    3,
				BaseDelay:     100 * time.Millisecond,
				MaxDelay:      1 * time.Second,
				RetryOnStatus: []uint{500, 502},
			},
		}

		client := NewClient(config)

		if client.httpClient != customHTTPClient {
			t.Error("Expected custom HTTP client to be used")
		}
		if client.retryOnStatus[0] != 500 || client.retryOnStatus[1] != 502 {
			t.Error("Expected retryOnStatus to be set correctly")
		}
	})

	t.Run("WithNilHttpClient", func(t *testing.T) {
		config := ClientConfig{
			HttpClient: nil,
			RateLimitConfig: RateLimitConfig{
				RequestsPerSecond: 10,
				RequestsPerMinute: 100,
				RequestsPerHour:   1000,
			},
			RetryConfig: RetryConfig{
				MaxRetries: 3,
				BaseDelay:  100 * time.Millisecond,
				MaxDelay:   1 * time.Second,
			},
		}

		client := NewClient(config)

		if client.httpClient == nil {
			t.Error("Expected default HTTP client to be created")
		}
		if client.httpClient.Timeout != 30*time.Second {
			t.Errorf("Expected default timeout of 30s, got %v", client.httpClient.Timeout)
		}
	})

	t.Run("WithEmptyRetryOnStatus", func(t *testing.T) {
		config := ClientConfig{
			HttpClient: &http.Client{},
			RateLimitConfig: RateLimitConfig{
				RequestsPerSecond: 10,
				RequestsPerMinute: 100,
				RequestsPerHour:   1000,
			},
			RetryConfig: RetryConfig{
				MaxRetries:    3,
				BaseDelay:     100 * time.Millisecond,
				MaxDelay:      1 * time.Second,
				RetryOnStatus: []uint{},
			},
		}

		client := NewClient(config)

		if len(client.retryOnStatus) != 0 {
			t.Error("Expected empty retryOnStatus slice")
		}
	})
}

func TestClient_Do_Success(t *testing.T) {
	attempts := 0
	config := ClientConfig{
		HttpClient: &http.Client{
			Transport: &mockTransport{
				attempts: &attempts,
				responses: []*mockResponse{
					{statusCode: 200, body: "OK"},
				},
			},
		},
		RateLimitConfig: RateLimitConfig{
			RequestsPerSecond: 100,
			RequestsPerMinute: 1000,
			RequestsPerHour:   10000,
		},
		RetryConfig: RetryConfig{
			MaxRetries: 3,
			BaseDelay:  10 * time.Millisecond,
			MaxDelay:   100 * time.Millisecond,
		},
	}

	client := NewClient(config)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := client.Do(context.Background(), req)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", string(body))
	}
}

func TestClient_Do_NetworkErrorWithRetry(t *testing.T) {
	attempts := 0
	config := ClientConfig{
		HttpClient: &http.Client{
			Transport: &mockTransport{
				attempts: &attempts,
				responses: []*mockResponse{
					{err: errors.New("network error")},
					{err: errors.New("network error")},
					{statusCode: 200, body: "Success"},
				},
			},
		},
		RateLimitConfig: RateLimitConfig{
			RequestsPerSecond: 100,
			RequestsPerMinute: 1000,
			RequestsPerHour:   10000,
		},
		RetryConfig: RetryConfig{
			MaxRetries: 2,
			BaseDelay:  10 * time.Millisecond,
			MaxDelay:   100 * time.Millisecond,
		},
	}

	client := NewClient(config)
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	resp, err := client.Do(context.Background(), req)

	if err != nil {
		t.Errorf("Expected no error after retries, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts (initial + 2 retries), got %d", attempts)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	resp.Body.Close()
}

func TestClient_Do_StatusBasedRetry(t *testing.T) {
	attempts := 0
	config := ClientConfig{
		HttpClient: &http.Client{
			Transport: &mockTransport{
				attempts: &attempts,
				responses: []*mockResponse{
					{statusCode: 500, body: "Error"},
					{statusCode: 500, body: "Error"},
					{statusCode: 200, body: "Success"},
				},
			},
		},
		RateLimitConfig: RateLimitConfig{
			RequestsPerSecond: 100,
			RequestsPerMinute: 1000,
			RequestsPerHour:   10000,
		},
		RetryConfig: RetryConfig{
			MaxRetries:    3,
			BaseDelay:     10 * time.Millisecond,
			MaxDelay:      100 * time.Millisecond,
			RetryOnStatus: []uint{500},
		},
	}

	client := NewClient(config)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := client.Do(context.Background(), req)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	resp.Body.Close()
}

func TestClient_Do_RateLimitError(t *testing.T) {
	config := ClientConfig{
		HttpClient: &http.Client{
			Transport: &mockTransport{
				responses: []*mockResponse{
					{statusCode: 200, body: "OK"},
				},
			},
		},
		RateLimitConfig: RateLimitConfig{
			RequestsPerSecond: 0, // Zero limits - will always block
			RequestsPerMinute: 0,
			RequestsPerHour:   0,
		},
		RetryConfig: RetryConfig{
			MaxRetries: 3,
			BaseDelay:  10 * time.Millisecond,
			MaxDelay:   100 * time.Millisecond,
		},
	}

	client := NewClient(config)
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	resp, err := client.Do(ctx, req)

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
	if resp != nil {
		t.Error("Expected no response when rate limiting fails")
	}
}

func TestClient_Do_ContextCancelled(t *testing.T) {
	config := ClientConfig{
		HttpClient: &http.Client{
			Transport: &mockTransport{
				responses: []*mockResponse{
					{statusCode: 200, body: "OK"},
				},
			},
		},
		RateLimitConfig: RateLimitConfig{
			RequestsPerSecond: 100,
			RequestsPerMinute: 1000,
			RequestsPerHour:   10000,
		},
		RetryConfig: RetryConfig{
			MaxRetries: 3,
			BaseDelay:  10 * time.Millisecond,
			MaxDelay:   100 * time.Millisecond,
		},
	}

	client := NewClient(config)
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := client.Do(ctx, req)

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
	if resp != nil {
		t.Error("Expected no response when context is cancelled")
	}
}

func TestClient_Do_NoRetryOnSuccessStatus(t *testing.T) {
	attempts := 0
	config := ClientConfig{
		HttpClient: &http.Client{
			Transport: &mockTransport{
				attempts: &attempts,
				responses: []*mockResponse{
					{statusCode: 200, body: "Success"},
				},
			},
		},
		RateLimitConfig: RateLimitConfig{
			RequestsPerSecond: 100,
			RequestsPerMinute: 1000,
			RequestsPerHour:   10000,
		},
		RetryConfig: RetryConfig{
			MaxRetries:    3,
			BaseDelay:     10 * time.Millisecond,
			MaxDelay:      100 * time.Millisecond,
			RetryOnStatus: []uint{500},
		},
	}

	client := NewClient(config)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := client.Do(context.Background(), req)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("Expected only 1 attempt for success status, got %d", attempts)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	resp.Body.Close()
}

func TestClient_Do_NoRetryOnNonMatchingStatus(t *testing.T) {
	attempts := 0
	config := ClientConfig{
		HttpClient: &http.Client{
			Transport: &mockTransport{
				attempts: &attempts,
				responses: []*mockResponse{
					{statusCode: 404, body: "Not Found"},
				},
			},
		},
		RateLimitConfig: RateLimitConfig{
			RequestsPerSecond: 100,
			RequestsPerMinute: 1000,
			RequestsPerHour:   10000,
		},
		RetryConfig: RetryConfig{
			MaxRetries:    3,
			BaseDelay:     10 * time.Millisecond,
			MaxDelay:      100 * time.Millisecond,
			RetryOnStatus: []uint{500, 502},
		},
	}

	client := NewClient(config)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := client.Do(context.Background(), req)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("Expected only 1 attempt for non-matching status, got %d", attempts)
	}
	if resp.StatusCode != 404 {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}

	resp.Body.Close()
}

func TestClient_Do_NilRetryOnStatus(t *testing.T) {
	attempts := 0
	config := ClientConfig{
		HttpClient: &http.Client{
			Transport: &mockTransport{
				attempts: &attempts,
				responses: []*mockResponse{
					{statusCode: 500, body: "Error"},
				},
			},
		},
		RateLimitConfig: RateLimitConfig{
			RequestsPerSecond: 100,
			RequestsPerMinute: 1000,
			RequestsPerHour:   10000,
		},
		RetryConfig: RetryConfig{
			MaxRetries:    3,
			BaseDelay:     10 * time.Millisecond,
			MaxDelay:      100 * time.Millisecond,
			RetryOnStatus: nil,
		},
	}

	client := NewClient(config)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := client.Do(context.Background(), req)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("Expected only 1 attempt when retryOnStatus is nil, got %d", attempts)
	}
	if resp.StatusCode != 500 {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}

	resp.Body.Close()
}

func TestClient_Do_MaxRetriesExceeded(t *testing.T) {
	attempts := 0
	config := ClientConfig{
		HttpClient: &http.Client{
			Transport: &mockTransport{
				attempts: &attempts,
				responses: []*mockResponse{
					{statusCode: 500, body: "Error"},
					{statusCode: 500, body: "Error"},
					{statusCode: 500, body: "Error"},
				},
			},
		},
		RateLimitConfig: RateLimitConfig{
			RequestsPerSecond: 100,
			RequestsPerMinute: 1000,
			RequestsPerHour:   10000,
		},
		RetryConfig: RetryConfig{
			MaxRetries:    2,
			BaseDelay:     10 * time.Millisecond,
			MaxDelay:      100 * time.Millisecond,
			RetryOnStatus: []uint{500},
		},
	}

	client := NewClient(config)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := client.Do(context.Background(), req)

	if err != nil {
		t.Errorf("Expected no error (final error is returned via response), got %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts (initial + 2 retries), got %d", attempts)
	}
	if resp.StatusCode != 500 {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}

	resp.Body.Close()
}
