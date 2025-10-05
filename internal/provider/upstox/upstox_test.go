package upstox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/shahid-2020/gohlcv/types"
)

type mockHTTPClient struct {
	calledCount int
	requests    []*http.Request
	responses   []*http.Response
}

func NewMockHTTPClient(responses []*http.Response) *mockHTTPClient {
	return &mockHTTPClient{
		calledCount: 0,
		requests:    []*http.Request{},
		responses:   responses,
	}
}

func (m *mockHTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	m.calledCount++
	m.requests = append(m.requests, req)

	if m.calledCount-1 >= len(m.responses) {
		return nil, errors.New("no more mock responses")
	}
	return m.responses[m.calledCount-1], nil
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func createMockResponse(candles [][]any, statusCode int) *http.Response {
	response := upstoxResponse{
		Status: "success",
		Data: struct {
			Candles [][]any `json:"candles"`
		}{
			Candles: candles,
		},
	}
	body, _ := json.Marshal(response)
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(string(body))),
		Header:     make(http.Header),
	}
}

func createErrorResponse(statusCode int, errorMsg string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(errorMsg)),
		Header:     make(http.Header),
	}
}

func TestNewUpstoxProvider(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		provider := NewUpstoxProvider()

		if provider == nil {
			t.Fatal("Expected provider to be created")
		}
		if provider.Name() != "upstox" {
			t.Errorf("Expected name 'upstox', got '%s'", provider.Name())
		}
		if len(provider.instrumentMap) == 0 {
			t.Error("Expected instrument map to be populated")
		}

		if provider.instrumentMap["RELIANCE:NSE"].TradingSymbol != "RELIANCE" {
			t.Error("Expected RELIANCE:NSE to be in instrument map")
		}
	})

	t.Run("PanicOnInvalidInstruments", func(t *testing.T) {
		originalInstruments := instrumentsJSON
		defer func() {
			instrumentsJSON = originalInstruments
			if r := recover(); r == nil {
				t.Error("Expected panic when instruments JSON is invalid")
			}
		}()

		instrumentsJSON = []byte("invalid json")
		NewUpstoxProvider()
	})
}

func TestUpstoxProvider_Name(t *testing.T) {
	provider := &UpstoxProvider{}
	if name := provider.Name(); name != "upstox" {
		t.Errorf("Expected name 'upstox', got '%s'", name)
	}
}

func TestUpstoxProvider_Provide_Success(t *testing.T) {
	candles := [][]any{
		{"2025-09-25T15:25:00+05:30", 1374.5, 1375, 1373.5, 1374.8, 283572},
		{"2025-09-25T15:20:00+05:30", 1374.3, 1374.9, 1372.9, 1374.4, 461782},
	}

	mockClient := NewMockHTTPClient([]*http.Response{
		createMockResponse(candles, 200),
	})
	provider := NewUpstoxProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Date(2025, 9, 25, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 9, 25, 0, 0, 0, 0, time.UTC)

	ohlcvs, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval5m, from, to)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(ohlcvs) == 0 {
		t.Errorf("Expected OHLCV records to be greater than 0")
	}

	expectedURL := "https://api.upstox.com/v3/historical-candle/NSE_EQ%7CINE002A01018/minutes/5/2025-09-25/2025-09-25"

	if mockClient.requests[0].URL.String() != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, mockClient.requests[0].URL.String())
	}

	if mockClient.calledCount != 1 {
		t.Errorf("Expected HTTP client to be called once, got %d", mockClient.calledCount)
	}

	for _, ohlcv := range ohlcvs {
		if ohlcv.Symbol != "RELIANCE" {
			t.Errorf("Expected symbol RELIANCE, got %s", ohlcv.Symbol)
		}
		if ohlcv.Exchange != types.ExchangeNSE {
			t.Errorf("Expected exchange NSE, got %v", ohlcv.Exchange)
		}
		if ohlcv.Source != "upstox" {
			t.Errorf("Expected source upstox, got %s", ohlcv.Source)
		}
		if ohlcv.Freshness != types.FreshnessHistorical {
			t.Errorf("Expected freshness historical, got %v", ohlcv.Freshness)
		}
		if ohlcv.Open < 0 || ohlcv.High < 0 || ohlcv.Low < 0 || ohlcv.Close < 0 || ohlcv.Volume < 0 {
			t.Error("OHLCV values should be non-negative")
		}
		if ohlcv.DateTime.Location().String() != "Asia/Kolkata" {
			t.Errorf("Expected time in IST, got %v", ohlcv.DateTime.Location())
		}
	}
}

func TestUpstoxProvider_Provide_WithoutFromDate(t *testing.T) {
	candles := [][]any{
		{"2023-10-02T00:00:00Z", 1500.0, 1520.0, 1490.0, 1510.0, 50000.0},
	}

	mockClient := NewMockHTTPClient([]*http.Response{
		createMockResponse(candles, 200),
	})

	provider := NewUpstoxProvider()
	provider.client = mockClient
	provider.instrumentMap = map[string]instrument{
		"INFY:NSE": {
			InstrumentKey: "NSE_EQ|INE009A01021",
			TradingSymbol: "INFY",
			Exchange:      "NSE",
		},
	}

	ctx := context.Background()
	to := time.Date(2023, 10, 2, 0, 0, 0, 0, time.UTC)

	ohlcvs, err := provider.Provide(ctx, "INFY", types.ExchangeNSE, types.Interval1d, time.Time{}, to)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(ohlcvs) != 1 {
		t.Errorf("Expected 1 OHLCV record, got %d", len(ohlcvs))
	}

	expectedURL := "https://api.upstox.com/v3/historical-candle/NSE_EQ%7CINE009A01021/days/1/2023-10-02"
	if mockClient.requests[0].URL.String() != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, mockClient.requests[0].URL.String())
	}
}

func TestUpstoxProvider_Provide_BSE_Exchange(t *testing.T) {
	candles := [][]any{
		{"2023-10-01T09:15:00Z", 2500.0, 2550.0, 2480.0, 2520.0, 75000.0},
	}

	mockClient := NewMockHTTPClient([]*http.Response{
		createMockResponse(candles, 200),
	})

	provider := NewUpstoxProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 10, 2, 0, 0, 0, 0, time.UTC)

	ohlcvs, err := provider.Provide(ctx, "RELIANCE", types.ExchangeBSE, types.Interval1h, from, to)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(ohlcvs) != 1 {
		t.Errorf("Expected 1 OHLCV record, got %d", len(ohlcvs))
	}
}

func TestUpstoxProvider_Provide_SymbolNotFound(t *testing.T) {
	provider := NewUpstoxProvider()
	provider.instrumentMap = map[string]instrument{}

	ctx := context.Background()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	_, err := provider.Provide(ctx, "UNKNOWN", types.ExchangeNSE, types.Interval1m, from, to)

	if err == nil {
		t.Error("Expected error for unknown symbol")
	}
	expectedError := "symbol not found: UNKNOWN on exchange NSE"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%v'", expectedError, err)
	}
}

func TestUpstoxProvider_Provide_InvalidInterval(t *testing.T) {
	provider := NewUpstoxProvider()
	provider.client = NewMockHTTPClient([]*http.Response{})

	ctx := context.Background()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	_, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, "invalid_interval", from, to)

	if err == nil {
		t.Error("Expected error for invalid interval")
	}
	if err.Error() != "invalid interval: unknown interval: invalid_interval" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestUpstoxProvider_Provide_RequestCreationError(t *testing.T) {
	provider := NewUpstoxProvider()
	provider.client = NewMockHTTPClient([]*http.Response{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	_, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval1m, from, to)

	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestUpstoxProvider_Provide_HTTPClientError(t *testing.T) {
	mockClient := NewMockHTTPClient([]*http.Response{
		{
			StatusCode: 200,
			Body:       io.NopCloser(&errorReader{}),
			Header:     make(http.Header),
		},
	})

	provider := NewUpstoxProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	_, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval1m, from, to)

	if err == nil {
		t.Error("Expected error from HTTP client")
	}
}

func TestUpstoxProvider_Provide_NonOKResponse(t *testing.T) {
	mockClient := NewMockHTTPClient([]*http.Response{
		createErrorResponse(429, `{"error": "rate limited"}`),
	})

	provider := NewUpstoxProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	_, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval1m, from, to)

	if err == nil {
		t.Error("Expected error for non-200 response")
	}
	expectedError := "non-OK response: 429 {\"error\": \"rate limited\"}"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%v'", expectedError, err)
	}
}

func TestUpstoxProvider_Provide_ResponseReadError(t *testing.T) {
	mockClient := NewMockHTTPClient([]*http.Response{
		{
			StatusCode: 200,
			Body:       io.NopCloser(&errorReader{}),
			Header:     make(http.Header),
		},
	})

	provider := NewUpstoxProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	_, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval1m, from, to)

	if err == nil {
		t.Error("Expected error reading response body")
	}
}

func TestUpstoxProvider_Provide_InvalidJSONResponse(t *testing.T) {
	mockClient := NewMockHTTPClient([]*http.Response{
		{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte("invalid json"))),
			Header:     make(http.Header),
		},
	})

	provider := NewUpstoxProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	_, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval1m, from, to)

	if err == nil {
		t.Error("Expected error unmarshaling JSON")
	}
}

func TestUpstoxProvider_IntervalToUnitInterval(t *testing.T) {
	provider := &UpstoxProvider{}

	testCases := []struct {
		interval     types.Interval
		expectedUnit string
		expectedInt  string
		shouldError  bool
	}{
		{types.Interval1m, "minutes", "1", false},
		{types.Interval5m, "minutes", "5", false},
		{types.Interval15m, "minutes", "15", false},
		{types.Interval30m, "minutes", "30", false},
		{types.Interval1h, "hours", "1", false},
		{types.Interval1d, "days", "1", false},
		{types.Interval1wk, "weeks", "1", false},
		{types.Interval1mo, "months", "1", false},
		{"invalid", "", "", true},
	}

	for _, tc := range testCases {
		t.Run(string(tc.interval), func(t *testing.T) {
			unit, interval, err := provider.intervalToUnitInterval(tc.interval)

			if tc.shouldError {
				if err == nil {
					t.Error("Expected error for invalid interval")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if unit != tc.expectedUnit {
					t.Errorf("Expected unit %s, got %s", tc.expectedUnit, unit)
				}
				if interval != tc.expectedInt {
					t.Errorf("Expected interval %s, got %s", tc.expectedInt, interval)
				}
			}
		})
	}
}

func TestUpstoxProvider_NormalizeOHLCVs(t *testing.T) {
	provider := &UpstoxProvider{}

	ohlcvs := []types.OHLCV{
		{
			Open:  100.123456,
			High:  105.678901,
			Low:   95.111111,
			Close: 102.999999,
		},
		{
			Open:  200.555555,
			High:  205.444444,
			Low:   195.666666,
			Close: 203.333333,
		},
	}

	normalized := provider.normalizeOHLCVs(ohlcvs)

	if normalized[0].Open != 100.12 {
		t.Errorf("Expected open 100.12, got %f", normalized[0].Open)
	}
	if normalized[0].High != 105.68 {
		t.Errorf("Expected high 105.68, got %f", normalized[0].High)
	}
	if normalized[0].Low != 95.11 {
		t.Errorf("Expected low 95.11, got %f", normalized[0].Low)
	}
	if normalized[0].Close != 103.00 {
		t.Errorf("Expected close 103.00, got %f", normalized[0].Close)
	}

	if normalized[1].Open != 200.56 {
		t.Errorf("Expected open 200.56, got %f", normalized[1].Open)
	}
	if normalized[1].High != 205.44 {
		t.Errorf("Expected high 205.44, got %f", normalized[1].High)
	}
	if normalized[1].Low != 195.67 {
		t.Errorf("Expected low 195.67, got %f", normalized[1].Low)
	}
	if normalized[1].Close != 203.33 {
		t.Errorf("Expected close 203.33, got %f", normalized[1].Close)
	}
}

func TestUpstoxProvider_Round2(t *testing.T) {
	provider := &UpstoxProvider{}

	testCases := []struct {
		input    float64
		expected float64
	}{
		{100.123, 100.12},
		{100.125, 100.13},
		{100.129, 100.13},
		{100.0, 100.0},
		{99.999, 100.0},
		{0.001, 0.00},
		{0.005, 0.01},
		{123.456, 123.46},
		{123.454, 123.45},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("round2(%f)", tc.input), func(t *testing.T) {
			result := provider.round2(tc.input)
			if result != tc.expected {
				t.Errorf("round2(%f) = %f, expected %f", tc.input, result, tc.expected)
			}
		})
	}
}

func TestUpstoxProvider_AllIntervals(t *testing.T) {
	provider := NewUpstoxProvider()

	intervals := []types.Interval{
		types.Interval1m, types.Interval5m, types.Interval15m, types.Interval30m,
		types.Interval1h, types.Interval1d, types.Interval1wk, types.Interval1mo,
	}

	for _, interval := range intervals {
		t.Run(string(interval), func(t *testing.T) {
			candles := [][]any{
				{"2023-10-01T00:00:00Z", 100.0, 105.0, 95.0, 102.0, 1000.0},
			}

			mockClient := NewMockHTTPClient([]*http.Response{
				createMockResponse(candles, 200),
			})
			provider.client = mockClient

			ctx := context.Background()
			from := time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2023, 10, 2, 0, 0, 0, 0, time.UTC)

			_, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, interval, from, to)

			if err != nil {
				t.Errorf("Interval %s: Expected no error, got %v", interval, err)
			}
		})
	}
}
