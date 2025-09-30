package marketdata

import (
	"context"
	"time"

	"github.com/shahid-2020/gohlcv/internal/provider"
	"github.com/shahid-2020/gohlcv/internal/provider/upstox"
	"github.com/shahid-2020/gohlcv/internal/provider/yahoo"
	"github.com/shahid-2020/gohlcv/types"
)

type MarketData struct {
	exchange types.Exchange
	upstox   provider.OHLCVProvider
	yahoo    provider.OHLCVProvider
}

func NewMarketData(exchange types.Exchange) *MarketData {
	return &MarketData{
		exchange: exchange,
		upstox:   upstox.NewUpstoxProvider(),
		yahoo:    yahoo.NewYahooProvider(),
	}
}

func (m *MarketData) Fetch(
	ctx context.Context,
	symbol string,
	interval types.Interval,
	start, end time.Time,
) ([]types.OHLCV, error) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	now := time.Now().In(loc)

	if start.IsZero() {
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	} else {
		start = start.In(loc)
	}

	if !end.IsZero() {
		end = end.In(loc)
	}

	if start.Year() == now.Year() &&
		start.Month() == now.Month() &&
		start.Day() == now.Day() {
		return m.yahoo.Provide(ctx, symbol, m.exchange, interval, start, end)
	}

	data, err := m.upstox.Provide(ctx, symbol, m.exchange, interval, start, end)
	if err != nil || len(data) == 0 {
		return m.yahoo.Provide(ctx, symbol, m.exchange, interval, start, end)
	}

	return data, nil
}
