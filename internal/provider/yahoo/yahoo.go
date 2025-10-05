package yahoo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shahid-2020/gohlcv/internal/httpclient"
	"github.com/shahid-2020/gohlcv/types"
)

type yahooResponse struct {
	Chart struct {
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
	} `json:"chart"`
}

type YahooProvider struct {
	client httpclient.Doer
}

func NewYahooProvider() *YahooProvider {
	config := httpclient.ClientConfig{
		HttpClient: &http.Client{Timeout: 30 * time.Second},
		RateLimitConfig: httpclient.RateLimitConfig{
			RequestsPerSecond: 50,
			RequestsPerMinute: 500,
			RequestsPerHour:   2000,
		},
		RetryConfig: httpclient.RetryConfig{
			MaxRetries:    6,
			BaseDelay:     100 * time.Millisecond,
			MaxDelay:      5 * time.Second,
			RetryOnStatus: []uint{429, 500, 502, 503},
		},
	}

	return &YahooProvider{
		client: httpclient.NewClient(config),
	}
}

func (y *YahooProvider) Name() string {
	return "yahoo"
}

func (y *YahooProvider) Provide(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, from, to time.Time) ([]types.OHLCV, error) {
	period1 := from.Unix()
	var url string
	if to.IsZero() {
		url = fmt.Sprintf("https://query2.finance.yahoo.com/v8/finance/chart/%s?interval=%s&period1=%d&period2=%d",
			y.formatSymbol(symbol, exchange), interval, period1, period1)
	} else {
		period2 := to.Unix()
		url = fmt.Sprintf("https://query2.finance.yahoo.com/v8/finance/chart/%s?interval=%s&period1=%d&period2=%d",
			y.formatSymbol(symbol, exchange), interval, period1, period2)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", uuid.NewString())
	req.Header.Set("Accept", "application/json")

	res, err := y.client.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-OK response: %d %s", res.StatusCode, string(body))
	}

	var data yahooResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(data.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data found for symbol %s on exchange %s", symbol, exchange)
	}

	result := data.Chart.Result[0]
	quotes := result.Indicators.Quote[0]

	ohlcvs := make([]types.OHLCV, 0, len(result.Timestamp))
	loc, _ := time.LoadLocation("Asia/Kolkata")
	for i, ts := range result.Timestamp {
		t := time.Unix(ts, 0).In(loc)

		ohlcvs = append(ohlcvs, types.OHLCV{
			Symbol:    symbol,
			Exchange:  exchange,
			Open:      quotes.Open[i],
			High:      quotes.High[i],
			Low:       quotes.Low[i],
			Close:     quotes.Close[i],
			Volume:    quotes.Volume[i],
			DateTime:  t,
			Source:    y.Name(),
			Freshness: types.FreshnessDelayed,
		})
	}

	return y.normalizeOHLCVs(ohlcvs), nil
}

func (y *YahooProvider) formatSymbol(symbol string, exchange types.Exchange) string {
	switch exchange {
	case types.ExchangeNSE:
		return symbol + ".NS"
	case types.ExchangeBSE:
		return symbol + ".BO"
	default:
		return symbol
	}
}

func (y *YahooProvider) normalizeOHLCVs(ohlcvs []types.OHLCV) []types.OHLCV {
	for i := range ohlcvs {
		c := &ohlcvs[i]
		c.Open = y.round2(c.Open)
		c.High = y.round2(c.High)
		c.Low = y.round2(c.Low)
		c.Close = y.round2(c.Close)
	}

	return ohlcvs
}

func (y *YahooProvider) round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
