package yahoo

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

func createMockYahooResponse(timestamps []int64, opens, highs, lows, closes []float64, volumes []int64) *http.Response {
	response := yahooResponse{
		Chart: struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []int64   `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
			Error interface{} `json:"error"`
		}{
			Result: []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []int64   `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			}{
				{
					Timestamp: timestamps,
					Indicators: struct {
						Quote []struct {
							Open   []float64 `json:"open"`
							High   []float64 `json:"high"`
							Low    []float64 `json:"low"`
							Close  []float64 `json:"close"`
							Volume []int64   `json:"volume"`
						} `json:"quote"`
					}{
						Quote: []struct {
							Open   []float64 `json:"open"`
							High   []float64 `json:"high"`
							Low    []float64 `json:"low"`
							Close  []float64 `json:"close"`
							Volume []int64   `json:"volume"`
						}{
							{
								Open:   opens,
								High:   highs,
								Low:    lows,
								Close:  closes,
								Volume: volumes,
							},
						},
					},
				},
			},
			Error: nil,
		},
	}

	body, _ := json.Marshal(response)
	return &http.Response{
		StatusCode: 200,
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

func TestNewYahooProvider(t *testing.T) {

	provider := NewYahooProvider()

	if provider == nil {
		t.Fatal("Expected provider to be created")
	}
	if provider.Name() != "yahoo" {
		t.Errorf("Expected name 'yahoo', got '%s'", provider.Name())
	}
}

func TestYahooProvider_Name(t *testing.T) {
	provider := &YahooProvider{}
	if name := provider.Name(); name != "yahoo" {
		t.Errorf("Expected name 'yahoo', got '%s'", name)
	}
}

func TestYahooProvider_Provide_Success_NSE(t *testing.T) {
	timestamps := []int64{
		time.Date(2023, 10, 1, 9, 15, 0, 0, time.UTC).Unix(),
		time.Date(2023, 10, 1, 9, 16, 0, 0, time.UTC).Unix(),
	}
	opens := []float64{100.123, 102.456}
	highs := []float64{105.678, 107.891}
	lows := []float64{95.111, 101.222}
	closes := []float64{102.999, 106.777}
	volumes := []int64{1000, 2000}

	mockClient := NewMockHTTPClient([]*http.Response{
		createMockYahooResponse(timestamps, opens, highs, lows, closes, volumes),
	})

	provider := NewYahooProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Date(2023, 10, 1, 9, 15, 0, 0, time.UTC)
	to := time.Date(2023, 10, 1, 9, 16, 0, 0, time.UTC)

	ohlcvs, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval1m, from, to)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(ohlcvs) != 2 {
		t.Errorf("Expected 2 OHLCV records, got %d", len(ohlcvs))
	}

	expectedURL := "https://query2.finance.yahoo.com/v8/finance/chart/RELIANCE.NS?interval=1m&period1=1696151700&period2=1696151760"
	if mockClient.requests[0].URL.String() != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, mockClient.requests[0].URL.String())
	}

	if mockClient.requests[0].Header.Get("Accept") != "application/json" {
		t.Error("Expected Accept header to be application/json")
	}
	if mockClient.requests[0].Header.Get("User-Agent") == "" {
		t.Error("Expected User-Agent header to be set")
	}

	first := ohlcvs[0]
	if first.Symbol != "RELIANCE" {
		t.Errorf("Expected symbol RELIANCE, got %s", first.Symbol)
	}
	if first.Exchange != types.ExchangeNSE {
		t.Errorf("Expected exchange NSE, got %v", first.Exchange)
	}
	if first.Open != 100.12 {
		t.Errorf("Expected open 100.12, got %f", first.Open)
	}
	if first.High != 105.68 {
		t.Errorf("Expected high 105.68, got %f", first.High)
	}
	if first.Low != 95.11 {
		t.Errorf("Expected low 95.11, got %f", first.Low)
	}
	if first.Close != 103.00 {
		t.Errorf("Expected close 103.00, got %f", first.Close)
	}
	if first.Volume != 1000 {
		t.Errorf("Expected volume 1000, got %d", first.Volume)
	}
	if first.Source != "yahoo" {
		t.Errorf("Expected source yahoo, got %s", first.Source)
	}
	if first.Freshness != types.FreshnessDelayed {
		t.Errorf("Expected freshness delayed, got %v", first.Freshness)
	}
	if first.DateTime.Location().String() != "Asia/Kolkata" {
		t.Errorf("Expected time in IST, got %v", first.DateTime.Location())
	}
}

func TestYahooProvider_Provide_Success_BSE(t *testing.T) {

	timestamps := []int64{time.Date(2023, 10, 1, 9, 15, 0, 0, time.UTC).Unix()}
	opens := []float64{2500.0}
	highs := []float64{2550.0}
	lows := []float64{2480.0}
	closes := []float64{2520.0}
	volumes := []int64{50000}

	mockClient := NewMockHTTPClient([]*http.Response{
		createMockYahooResponse(timestamps, opens, highs, lows, closes, volumes),
	})

	provider := NewYahooProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Date(2023, 10, 1, 9, 15, 0, 0, time.UTC)
	to := time.Date(2023, 10, 1, 9, 16, 0, 0, time.UTC)

	ohlcvs, err := provider.Provide(ctx, "RELIANCE", types.ExchangeBSE, types.Interval1m, from, to)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(ohlcvs) != 1 {
		t.Errorf("Expected 1 OHLCV record, got %d", len(ohlcvs))
	}

	expectedURL := "https://query2.finance.yahoo.com/v8/finance/chart/RELIANCE.BO?interval=1m&period1=1696151700&period2=1696151760"
	if mockClient.requests[0].URL.String() != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, mockClient.requests[0].URL.String())
	}
}

func TestYahooProvider_Provide_WithoutToDate(t *testing.T) {

	timestamps := []int64{time.Date(2023, 10, 1, 9, 15, 0, 0, time.UTC).Unix()}
	opens := []float64{100.0}
	highs := []float64{105.0}
	lows := []float64{95.0}
	closes := []float64{102.0}
	volumes := []int64{1000}

	mockClient := NewMockHTTPClient([]*http.Response{
		createMockYahooResponse(timestamps, opens, highs, lows, closes, volumes),
	})

	provider := NewYahooProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Date(2023, 10, 1, 9, 15, 0, 0, time.UTC)

	ohlcvs, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval1m, from, time.Time{})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(ohlcvs) != 1 {
		t.Errorf("Expected 1 OHLCV record, got %d", len(ohlcvs))
	}

	expectedURL := "https://query2.finance.yahoo.com/v8/finance/chart/RELIANCE.NS?interval=1m&period1=1696151700&period2=1696151700"
	if mockClient.requests[0].URL.String() != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, mockClient.requests[0].URL.String())
	}
}

func TestYahooProvider_Provide_DefaultExchange(t *testing.T) {
	timestamps := []int64{time.Date(2023, 10, 1, 9, 15, 0, 0, time.UTC).Unix()}
	opens := []float64{100.0}
	highs := []float64{105.0}
	lows := []float64{95.0}
	closes := []float64{102.0}
	volumes := []int64{1000}

	mockClient := NewMockHTTPClient([]*http.Response{
		createMockYahooResponse(timestamps, opens, highs, lows, closes, volumes),
	})

	provider := NewYahooProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Date(2023, 10, 1, 9, 15, 0, 0, time.UTC)
	to := time.Date(2023, 10, 1, 9, 16, 0, 0, time.UTC)

	ohlcvs, err := provider.Provide(ctx, "AAPL", types.Exchange("UNKNOWN"), types.Interval1m, from, to)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(ohlcvs) != 1 {
		t.Errorf("Expected 1 OHLCV record, got %d", len(ohlcvs))
	}

	expectedURL := "https://query2.finance.yahoo.com/v8/finance/chart/AAPL?interval=1m&period1=1696151700&period2=1696151760"
	if mockClient.requests[0].URL.String() != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, mockClient.requests[0].URL.String())
	}
}

func TestYahooProvider_Provide_RequestCreationError(t *testing.T) {
	provider := NewYahooProvider()
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

func TestYahooProvider_Provide_HTTPClientError(t *testing.T) {

	mockClient := NewMockHTTPClient([]*http.Response{
		{
			StatusCode: 200,
			Body:       io.NopCloser(&errorReader{}),
			Header:     make(http.Header),
		},
	})

	provider := NewYahooProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	_, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval1m, from, to)

	if err == nil {
		t.Error("Expected error from HTTP client")
	}
}

func TestYahooProvider_Provide_NonOKResponse(t *testing.T) {
	mockClient := NewMockHTTPClient([]*http.Response{
		createErrorResponse(429, `{"error": "rate limited"}`),
	})

	provider := NewYahooProvider()
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

func TestYahooProvider_Provide_ResponseReadError(t *testing.T) {
	mockClient := NewMockHTTPClient([]*http.Response{
		{
			StatusCode: 200,
			Body:       io.NopCloser(&errorReader{}),
			Header:     make(http.Header),
		},
	})

	provider := NewYahooProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	_, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval1m, from, to)

	if err == nil {
		t.Error("Expected error reading response body")
	}
}

func TestYahooProvider_Provide_InvalidJSONResponse(t *testing.T) {
	mockClient := NewMockHTTPClient([]*http.Response{
		{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte("invalid json"))),
			Header:     make(http.Header),
		},
	})

	provider := NewYahooProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	_, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval1m, from, to)

	if err == nil {
		t.Error("Expected error unmarshaling JSON")
	}
}

func TestYahooProvider_Provide_EmptyResult(t *testing.T) {
	response := yahooResponse{
		Chart: struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []int64   `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
			Error interface{} `json:"error"`
		}{
			Result: []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []int64   `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			}{},
			Error: nil,
		},
	}

	body, _ := json.Marshal(response)
	mockClient := NewMockHTTPClient([]*http.Response{
		{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(string(body))),
			Header:     make(http.Header),
		},
	})

	provider := NewYahooProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	_, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval1m, from, to)

	if err == nil {
		t.Error("Expected error for empty result")
	}
}

func TestYahooProvider_Provide_ErrorInResponse(t *testing.T) {
	response := yahooResponse{
		Chart: struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []int64   `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
			Error interface{} `json:"error"`
		}{
			Result: []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []int64   `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			}{},
			Error: map[string]interface{}{
				"code":        "Not Found",
				"description": "No data found",
			},
		},
	}

	body, _ := json.Marshal(response)
	mockClient := NewMockHTTPClient([]*http.Response{
		{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(string(body))),
			Header:     make(http.Header),
		},
	})

	provider := NewYahooProvider()
	provider.client = mockClient

	ctx := context.Background()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	_, err := provider.Provide(ctx, "RELIANCE", types.ExchangeNSE, types.Interval1m, from, to)

	if err == nil {
		t.Error("Expected error for response with error field")
	}
}

func TestYahooProvider_FormatSymbol(t *testing.T) {
	provider := &YahooProvider{}

	testCases := []struct {
		symbol   string
		exchange types.Exchange
		expected string
	}{
		{"RELIANCE", types.ExchangeNSE, "RELIANCE.NS"},
		{"INFY", types.ExchangeNSE, "INFY.NS"},
		{"RELIANCE", types.ExchangeBSE, "RELIANCE.BO"},
		{"TCS", types.ExchangeBSE, "TCS.BO"},
		{"AAPL", types.Exchange("NASDAQ"), "AAPL"},
		{"GOOGL", types.Exchange("UNKNOWN"), "GOOGL"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%s", tc.symbol, tc.exchange), func(t *testing.T) {
			result := provider.formatSymbol(tc.symbol, tc.exchange)
			if result != tc.expected {
				t.Errorf("formatSymbol(%s, %s) = %s, expected %s", tc.symbol, tc.exchange, result, tc.expected)
			}
		})
	}
}

func TestYahooProvider_NormalizeOHLCVs(t *testing.T) {
	provider := &YahooProvider{}

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

func TestYahooProvider_Round2(t *testing.T) {
	provider := &YahooProvider{}

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

func TestYahooProvider_AllIntervals(t *testing.T) {
	provider := NewYahooProvider()

	intervals := []types.Interval{
		types.Interval1m, types.Interval5m, types.Interval15m, types.Interval30m,
		types.Interval1h, types.Interval1d, types.Interval1wk, types.Interval1mo,
	}

	for _, interval := range intervals {
		t.Run(string(interval), func(t *testing.T) {

			timestamps := []int64{time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC).Unix()}
			opens := []float64{100.0}
			highs := []float64{105.0}
			lows := []float64{95.0}
			closes := []float64{102.0}
			volumes := []int64{1000}

			mockClient := NewMockHTTPClient([]*http.Response{
				createMockYahooResponse(timestamps, opens, highs, lows, closes, volumes),
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
