# MarketData - GoHLCV

A clean, high-level Go library for fetching OHLCV (Open, High, Low, Close, Volume) market data from multiple providers with intelligent fallback and timezone-aware handling.

## Features

- **Multi-Provider Support**: Seamlessly integrates with Upstox and Yahoo Finance
- **Intelligent Fallback**: Automatically uses the best available data source
- **Timezone-Aware**: All timestamps are converted to IST (Asia/Kolkata)
- **Simple API**: Clean, easy-to-use interface for market data
- **Production Ready**: Built with reliability and error handling in mind

## Installation

```bash
go get github.com/shahid-2020/gohlcv
```

## Quick Start

```go
package main

import (
    "context"
    "time"
    
    "github.com/shahid-2020/gohlcv/internal/marketdata"
    "github.com/shahid-2020/gohlcv/types"
)

func main() {
    // Create a market data client for NSE
    md := marketdata.NewMarketData(types.ExchangeNSE)
    
    ctx := context.Background()
    
    // Fetch today's data for RELIANCE
    ohlcvs, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, time.Time{}, time.Time{})
    if err != nil {
        panic(err)
    }
    
    // Use the data
    for _, ohlcv := range ohlcvs {
        fmt.Printf("Time: %s, Open: %.2f, High: %.2f, Low: %.2f, Close: %.2f, Volume: %d\n",
            ohlcv.DateTime.Format("2006-01-02 15:04"),
            ohlcv.Open, ohlcv.High, ohlcv.Low, ohlcv.Close, ohlcv.Volume)
    }
}
```

## Usage Examples

### Fetch Current Day Data
```go
md := marketdata.NewMarketData(types.ExchangeNSE)
ohlcvs, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, time.Time{}, time.Time{})
// Automatically uses Yahoo Finance for current day data
```

### Fetch Historical Data
```go
start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
end := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
ohlcvs, err := md.Fetch(ctx, "INFY", types.Interval1d, start, end)
// Tries Upstox first, falls back to Yahoo if needed
```

### Different Intervals
```go
// 1-minute data
ohlcvs, err := md.Fetch(ctx, "TCS", types.Interval1m, start, end)

// 1-hour data  
ohlcvs, err := md.Fetch(ctx, "HDFC", types.Interval1h, start, end)

// 1-week data
ohlcvs, err := md.Fetch(ctx, "HINDUNILVR", types.Interval1wk, start, end)
```

### BSE Support
```go
md := marketdata.NewMarketData(types.ExchangeBSE)
ohlcvs, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, start, end)
```

## API Reference

### NewMarketData
```go
func NewMarketData(exchange types.Exchange) *MarketData
```
Creates a new MarketData instance for the specified exchange.

**Parameters:**
- `exchange`: The stock exchange (`types.ExchangeNSE` or `types.ExchangeBSE`)

### Fetch
```go
func (m *MarketData) Fetch(
    ctx context.Context,
    symbol string,
    interval types.Interval,
    start, end time.Time,
) ([]types.OHLCV, error)
```

Fetches OHLCV data for the given symbol and time range.

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `symbol`: Stock symbol (e.g., "RELIANCE", "INFY")
- `interval`: Time interval (`Interval1m`, `Interval5m`, `Interval15m`, `Interval30m`, `Interval1h`, `Interval1d`, `Interval1wk`, `Interval1mo`)
- `start`: Start time (uses today's start if zero)
- `end`: End time (uses current time if zero)

**Returns:**
- `[]types.OHLCV`: Array of OHLCV records
- `error`: Error if any occurred

## Provider Strategy

The library intelligently selects data providers:

- **Current Day Data**: Uses Yahoo Finance (faster, more reliable for recent data)
- **Historical Data**: Tries Upstox first, falls back to Yahoo Finance if:
  - Upstox API fails
  - Upstox returns no data
  - Upstox rate limit exceeded

## Data Structure

```go
type OHLCV struct {
    Symbol    string
    Exchange  types.Exchange
    Open      float64
    High      float64  
    Low       float64
    Close     float64
    Volume    int64
    DateTime  time.Time  // Always in IST (Asia/Kolkata)
    Source    string     // Data source: "upstox" or "yahoo"
    Freshness types.Freshness
}
```

## Error Handling

```go
ohlcvs, err := md.Fetch(ctx, "INVALID", types.Interval1d, start, end)
if err != nil {
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        fmt.Println("Request timed out")
    case errors.Is(err, context.Canceled):
        fmt.Println("Request cancelled") 
    default:
        fmt.Printf("Failed to fetch data: %v\n", err)
    }
    return
}
```

## Best Practices

### 1. Always Use Context
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

ohlcvs, err := md.Fetch(ctx, "RELIANCE", types.Interval1d, start, end)
```

### 2. Handle Time Zones Properly
```go
// The library automatically converts to IST
start := time.Date(2024, 1, 1, 9, 15, 0, 0, time.UTC) // UTC input
// Internally converted to IST for API calls
```

### 3. Check for Empty Results
```go
ohlcvs, err := md.Fetch(ctx, symbol, interval, start, end)
if err != nil {
    // Handle error
}
if len(ohlcvs) == 0 {
    fmt.Println("No data available for the given range")
}
```

## Supported Symbols

### NSE (National Stock Exchange)
- RELIANCE, INFY, TCS, HDFC, HINDUNILVR, etc.

### BSE (Bombay Stock Exchange)  
- RELIANCE, SBIN, etc.

## Rate Limiting

The library includes built-in rate limiting to respect API provider limits:
- Upstox: 50 requests/second, 500 requests/minute, 2000 requests/hour
- Yahoo: 50 requests/second, 500 requests/minute, 2000 requests/hour

## Examples

### Complete Working Example
```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/shahid-2020/gohlcv/internal/marketdata"
    "github.com/shahid-2020/gohlcv/types"
)

func main() {
    ctx := context.Background()
    md := marketdata.NewMarketData(types.ExchangeNSE)
    
    // Fetch last 7 days of data
    start := time.Now().Add(-7 * 24 * time.Hour)
    end := time.Now()
    
    symbols := []string{"RELIANCE", "INFY", "TCS", "HDFC"}
    
    for _, symbol := range symbols {
        ohlcvs, err := md.Fetch(ctx, symbol, types.Interval1d, start, end)
        if err != nil {
            fmt.Printf("Error fetching %s: %v\n", symbol, err)
            continue
        }
        
        fmt.Printf("\n%s (%d records):\n", symbol, len(ohlcvs))
        for _, ohlcv := range ohlcvs {
            fmt.Printf("  %s: O:%.2f H:%.2f L:%.2f C:%.2f V:%d (%s)\n",
                ohlcv.DateTime.Format("2006-01-02"),
                ohlcv.Open, ohlcv.High, ohlcv.Low, ohlcv.Close, ohlcv.Volume,
                ohlcv.Source)
        }
    }
}
```

## License

MIT License - see LICENSE file for details.
