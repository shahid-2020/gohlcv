package upstox

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shahid-2020/gohlcv/internal/httpclient"
	"github.com/shahid-2020/gohlcv/types"
)

//go:embed data/complete.json
var instrumentsJSON []byte

type instrument struct {
	Segment          string  `json:"segment"`
	Name             string  `json:"name"`
	Exchange         string  `json:"exchange"`
	ISIN             string  `json:"isin"`
	InstrumentType   string  `json:"instrument_type"`
	InstrumentKey    string  `json:"instrument_key"`
	LotSize          int     `json:"lot_size"`
	FreezeQuantity   float64 `json:"freeze_quantity"`
	ExchangeToken    string  `json:"exchange_token"`
	TickSize         float64 `json:"tick_size"`
	TradingSymbol    string  `json:"trading_symbol"`
	ShortName        string  `json:"short_name"`
	QtyMultiplier    float64 `json:"qty_multiplier"`
	IntradayMargin   float64 `json:"intraday_margin"`
	IntradayLeverage float64 `json:"intraday_leverage"`
}

type upstoxResponse struct {
	Status string `json:"status"`
	Data   struct {
		Candles [][]any `json:"candles"`
	} `json:"data"`
}

type UpstoxProvider struct {
	client        httpclient.Doer
	instrumentMap map[string]instrument
}

func NewUpstoxProvider() *UpstoxProvider {
	config := httpclient.ClientConfig{
		HttpClient: &http.Client{Timeout: 30 * time.Second},
		RateLimitConfig: httpclient.RateLimitConfig{
			RequestsPerSecond: 50,
			RequestsPerMinute: 500,
			RequestsPerHour:   4000,
		},
		RetryConfig: httpclient.RetryConfig{
			MaxRetries:    6,
			BaseDelay:     100 * time.Millisecond,
			MaxDelay:      5 * time.Second,
			RetryOnStatus: []uint{429, 500, 502, 503},
		},
	}

	var instruments []instrument
	if err := json.Unmarshal(instrumentsJSON, &instruments); err != nil {
		panic(fmt.Sprintf("failed to load instruments: %v", err))
	}
	instrumentMap := make(map[string]instrument)
	for _, inst := range instruments {
		instrumentMap[fmt.Sprint(inst.TradingSymbol, ":", inst.Exchange)] = inst
	}

	return &UpstoxProvider{
		client:        httpclient.NewClient(config),
		instrumentMap: instrumentMap,
	}
}

func (u *UpstoxProvider) Name() string {
	return "upstox"
}

func (u *UpstoxProvider) Provide(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, from, to time.Time) ([]types.OHLCV, error) {
	inst, ok := u.instrumentMap[fmt.Sprint(symbol, ":", exchange)]
	if !ok {
		return nil, fmt.Errorf("symbol not found: %s on exchange %s", symbol, exchange)
	}

	unit, unitInterval, err := u.intervalToUnitInterval(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval: %w", err)
	}

	toDate := to.Format("2006-01-02")
	var url string
	if from.IsZero() {
		url = fmt.Sprintf(
			"https://api.upstox.com/v3/historical-candle/%s/%s/%s/%s",
			inst.InstrumentKey, unit, unitInterval, toDate,
		)
	} else {
		fromDate := from.Format("2006-01-02")
		url = fmt.Sprintf(
			"https://api.upstox.com/v3/historical-candle/%s/%s/%s/%s/%s",
			inst.InstrumentKey, unit, unitInterval, toDate, fromDate,
		)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	res, err := u.client.Do(ctx, req)
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

	var resp upstoxResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	loc, _ := time.LoadLocation("Asia/Kolkata")
	var ohlcvs []types.OHLCV

	for _, c := range resp.Data.Candles {
		t, _ := time.Parse(time.RFC3339, c[0].(string))
		t = t.In(loc)

		open, _ := c[1].(float64)
		high, _ := c[2].(float64)
		low, _ := c[3].(float64)
		closePrice, _ := c[4].(float64)
		volume, _ := c[5].(float64)

		ohlcvs = append(ohlcvs, types.OHLCV{
			Symbol:    symbol,
			Exchange:  exchange,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closePrice,
			Volume:    int64(volume),
			DateTime:  t,
			Source:    u.Name(),
			Freshness: types.FreshnessHistorical,
		})
	}

	return u.normalizeOHLCVs(ohlcvs), nil
}

func (u *UpstoxProvider) intervalToUnitInterval(i types.Interval) (unit string, interval string, err error) {
	switch i {
	case types.Interval1m:
		return "minutes", "1", nil
	case types.Interval5m:
		return "minutes", "5", nil
	case types.Interval15m:
		return "minutes", "15", nil
	case types.Interval30m:
		return "minutes", "30", nil
	case types.Interval1h:
		return "hours", "1", nil
	case types.Interval1d:
		return "days", "1", nil
	case types.Interval1wk:
		return "weeks", "1", nil
	case types.Interval1mo:
		return "months", "1", nil
	default:
		return "", "", fmt.Errorf("unknown interval: %s", i)
	}
}

func (u *UpstoxProvider) normalizeOHLCVs(ohlcvs []types.OHLCV) []types.OHLCV {
	for i := range ohlcvs {
		c := &ohlcvs[i]
		c.Open = u.round2(c.Open)
		c.High = u.round2(c.High)
		c.Low = u.round2(c.Low)
		c.Close = u.round2(c.Close)
	}

	return ohlcvs
}

func (u *UpstoxProvider) round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
