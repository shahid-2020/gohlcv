package provider

import (
	"context"
	"time"

	"github.com/shahid-2020/gohlcv/types"
)

type OHLCVProvider interface {
	Name() string
	Provide(ctx context.Context, symbol string, exchange types.Exchange, interval types.Interval, start, end time.Time) ([]types.OHLCV, error)
}
