package marketdata

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shahid-2020/gohlcv/types"
)

type mockProvider struct {
	name        string
	provideFunc func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error)
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Provide(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
	if m.provideFunc != nil {
		return m.provideFunc(ctx, symbol, exchange, interval, start, end)
	}
	return []types.OHLCV{}, nil
}

func TestNewMarketData(t *testing.T) {
	tests := []struct {
		name     string
		exchange types.Exchange
	}{
		{"NSE Exchange", types.ExchangeNSE},
		{"BSE Exchange", types.ExchangeBSE},
		{"Unknown Exchange", types.Exchange("UNKNOWN")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := NewMarketData(tt.exchange)

			if md == nil {
				t.Fatal("Expected MarketData instance, got nil")
			}
			if md.exchange != tt.exchange {
				t.Errorf("Expected exchange %v, got %v", tt.exchange, md.exchange)
			}

			if md.upstox == nil {
				t.Error("Expected upstox provider to be initialized")
			}
			if md.yahoo == nil {
				t.Error("Expected yahoo provider to be initialized")
			}

			if md.upstox.Name() == "" {
				t.Error("Expected upstox provider to have a name")
			}
			if md.yahoo.Name() == "" {
				t.Error("Expected yahoo provider to have a name")
			}
		})
	}
}

func TestMarketData_Fetch_CurrentDay_UsesYahoo(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	today := time.Now().In(loc)

	mockYahoo := &mockProvider{
		name: "yahoo",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			return []types.OHLCV{
				{
					Symbol:   symbol,
					Exchange: exchange,
					Open:     100.0,
					High:     105.0,
					Low:      95.0,
					Close:    102.0,
					Volume:   1000,
					DateTime: start,
					Source:   "yahoo",
				},
			}, nil
		},
	}

	mockUpstox := &mockProvider{
		name: "upstox",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			t.Error("Upstox should not be called for current day")
			return nil, nil
		},
	}

	md := &MarketData{
		exchange: types.ExchangeNSE,
		yahoo:    mockYahoo,
		upstox:   mockUpstox,
	}

	ctx := context.Background()
	ohlcvs, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, today, time.Time{})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(ohlcvs) != 1 {
		t.Errorf("Expected 1 OHLCV record, got %d", len(ohlcvs))
	}
	if ohlcvs[0].Source != "yahoo" {
		t.Errorf("Expected source 'yahoo', got %s", ohlcvs[0].Source)
	}
}

func TestMarketData_Fetch_HistoricalDay_UsesUpstoxFirst(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	yesterday := time.Now().In(loc).Add(-24 * time.Hour)

	mockUpstox := &mockProvider{
		name: "upstox",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			return []types.OHLCV{
				{
					Symbol:   symbol,
					Exchange: exchange,
					Open:     100.0,
					High:     105.0,
					Low:      95.0,
					Close:    102.0,
					Volume:   1000,
					DateTime: start,
					Source:   "upstox",
				},
			}, nil
		},
	}

	mockYahoo := &mockProvider{
		name: "yahoo",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			t.Error("Yahoo should not be called when Upstox succeeds")
			return nil, nil
		},
	}

	md := &MarketData{
		exchange: types.ExchangeNSE,
		yahoo:    mockYahoo,
		upstox:   mockUpstox,
	}

	ctx := context.Background()
	ohlcvs, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, yesterday, time.Time{})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(ohlcvs) != 1 {
		t.Errorf("Expected 1 OHLCV record, got %d", len(ohlcvs))
	}
	if ohlcvs[0].Source != "upstox" {
		t.Errorf("Expected source 'upstox', got %s", ohlcvs[0].Source)
	}
}

func TestMarketData_Fetch_UpstoxFails_FallsBackToYahoo(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	yesterday := time.Now().In(loc).Add(-24 * time.Hour)

	mockUpstox := &mockProvider{
		name: "upstox",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			return nil, errors.New("upstox api error")
		},
	}

	mockYahoo := &mockProvider{
		name: "yahoo",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			return []types.OHLCV{
				{
					Symbol:   symbol,
					Exchange: exchange,
					Open:     100.0,
					High:     105.0,
					Low:      95.0,
					Close:    102.0,
					Volume:   1000,
					DateTime: start,
					Source:   "yahoo",
				},
			}, nil
		},
	}

	md := &MarketData{
		exchange: types.ExchangeNSE,
		yahoo:    mockYahoo,
		upstox:   mockUpstox,
	}

	ctx := context.Background()
	ohlcvs, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, yesterday, time.Time{})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(ohlcvs) != 1 {
		t.Errorf("Expected 1 OHLCV record, got %d", len(ohlcvs))
	}
	if ohlcvs[0].Source != "yahoo" {
		t.Errorf("Expected source 'yahoo', got %s", ohlcvs[0].Source)
	}
}

func TestMarketData_Fetch_UpstoxEmpty_FallsBackToYahoo(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	yesterday := time.Now().In(loc).Add(-24 * time.Hour)

	mockUpstox := &mockProvider{
		name: "upstox",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			return []types.OHLCV{}, nil
		},
	}

	mockYahoo := &mockProvider{
		name: "yahoo",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			return []types.OHLCV{
				{
					Symbol:   symbol,
					Exchange: exchange,
					Open:     100.0,
					High:     105.0,
					Low:      95.0,
					Close:    102.0,
					Volume:   1000,
					DateTime: start,
					Source:   "yahoo",
				},
			}, nil
		},
	}

	md := &MarketData{
		exchange: types.ExchangeNSE,
		yahoo:    mockYahoo,
		upstox:   mockUpstox,
	}

	ctx := context.Background()
	ohlcvs, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, yesterday, time.Time{})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(ohlcvs) != 1 {
		t.Errorf("Expected 1 OHLCV record, got %d", len(ohlcvs))
	}
	if ohlcvs[0].Source != "yahoo" {
		t.Errorf("Expected source 'yahoo', got %s", ohlcvs[0].Source)
	}
}

func TestMarketData_Fetch_TimeZoneHandling(t *testing.T) {
	tests := []struct {
		name     string
		start    time.Time
		end      time.Time
		location string
	}{
		{"UTC times", time.Now().UTC(), time.Time{}, "UTC"},
		{"IST times", time.Now(), time.Time{}, "Asia/Kolkata"},
		{"EST times", time.Now().In(time.FixedZone("EST", -5*60*60)), time.Time{}, "EST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			mockProvider := &mockProvider{
				name: "test-provider",
				provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
					called = true
					if start.Location().String() != "Asia/Kolkata" {
						t.Errorf("Expected time in Asia/Kolkata, got %v", start.Location())
					}
					if !end.IsZero() && end.Location().String() != "Asia/Kolkata" {
						t.Errorf("Expected end time in Asia/Kolkata, got %v", end.Location())
					}
					return []types.OHLCV{{Source: "test"}}, nil
				},
			}

			md := &MarketData{
				exchange: types.ExchangeNSE,
				yahoo:    mockProvider,
				upstox:   mockProvider,
			}

			ctx := context.Background()
			_, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, tt.start, tt.end)

			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if !called {
				t.Error("Provider was not called")
			}
		})
	}
}

func TestMarketData_Fetch_DefaultStartTime(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	now := time.Now().In(loc)
	expectedStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	mockProvider := &mockProvider{
		name: "test-provider",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			if !start.Equal(expectedStart) {
				t.Errorf("Expected start time %v, got %v", expectedStart, start)
			}
			return []types.OHLCV{{Source: "yahoo"}}, nil
		},
	}

	md := &MarketData{
		exchange: types.ExchangeNSE,
		yahoo:    mockProvider,
		upstox:   mockProvider,
	}

	ctx := context.Background()
	_, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, time.Time{}, time.Time{})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestMarketData_Fetch_AllProvidersFail(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	yesterday := time.Now().In(loc).Add(-24 * time.Hour)

	mockUpstox := &mockProvider{
		name: "upstox",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			return nil, errors.New("upstox failed")
		},
	}

	mockYahoo := &mockProvider{
		name: "yahoo",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			return nil, errors.New("yahoo failed")
		},
	}

	md := &MarketData{
		exchange: types.ExchangeNSE,
		yahoo:    mockYahoo,
		upstox:   mockUpstox,
	}

	ctx := context.Background()
	_, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, yesterday, time.Time{})

	if err == nil {
		t.Error("Expected error when all providers fail")
	}
}

func TestMarketData_Fetch_ProviderNames(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	today := time.Now().In(loc)

	mockYahoo := &mockProvider{
		name: "mock-yahoo",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			return []types.OHLCV{
				{
					Symbol: symbol,
					Source: "mock-yahoo",
				},
			}, nil
		},
	}

	mockUpstox := &mockProvider{
		name: "mock-upstox",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			return []types.OHLCV{
				{
					Symbol: symbol,
					Source: "mock-upstox",
				},
			}, nil
		},
	}

	md := &MarketData{
		exchange: types.ExchangeNSE,
		yahoo:    mockYahoo,
		upstox:   mockUpstox,
	}

	if md.yahoo.Name() != "mock-yahoo" {
		t.Errorf("Expected yahoo name 'mock-yahoo', got %s", md.yahoo.Name())
	}
	if md.upstox.Name() != "mock-upstox" {
		t.Errorf("Expected upstox name 'mock-upstox', got %s", md.upstox.Name())
	}

	ctx := context.Background()
	ohlcvs, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, today, time.Time{})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(ohlcvs) != 1 {
		t.Errorf("Expected 1 OHLCV record, got %d", len(ohlcvs))
	}
	if ohlcvs[0].Source != "mock-yahoo" {
		t.Errorf("Expected source 'mock-yahoo', got %s", ohlcvs[0].Source)
	}
}

func TestMarketData_Fetch_ContextCancellation(t *testing.T) {
	mockProvider := &mockProvider{
		name: "test-provider",
		provideFunc: func(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return []types.OHLCV{{Source: "test"}}, nil
			}
		},
	}

	md := &MarketData{
		exchange: types.ExchangeNSE,
		yahoo:    mockProvider,
		upstox:   mockProvider,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, time.Now(), time.Time{})

	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}
