package types

import "time"

type Market string

const (
	MarketStocks Market = "stocks"
)

type Exchange string

const (
	ExchangeNSE Exchange = "NSE"
	ExchangeBSE Exchange = "BSE"
)

type DataFreshness string

const (
	FreshnessRealtime   DataFreshness = "realtime"
	FreshnessDelayed    DataFreshness = "delayed"
	FreshnessEndOfDay   DataFreshness = "endOfDay"
	FreshnessHistorical DataFreshness = "historical"
)

type OHLCV struct {
	Symbol    string        `json:"symbol"`
	Exchange  Exchange      `json:"exchange"`
	Open      float64       `json:"open"`
	High      float64       `json:"high"`
	Low       float64       `json:"low"`
	Close     float64       `json:"close"`
	Volume    int64         `json:"volume"`
	DateTime  time.Time     `json:"datetime"`
	Source    string        `json:"source"`
	Freshness DataFreshness `json:"freshness"`
}

type Interval string

const (
	Interval1m  Interval = "1m"
	Interval5m  Interval = "5m"
	Interval15m Interval = "15m"
	Interval30m Interval = "30m"
	Interval1h  Interval = "1h"
	Interval1d  Interval = "1d"
	Interval5d  Interval = "5d"
	Interval1wk Interval = "1wk"
	Interval1mo Interval = "1mo"
	Interval3mo Interval = "3mo"
)
