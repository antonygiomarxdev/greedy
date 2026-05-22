package backtest

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"
)

func LoadCSV(path string, symbol string) ([]Candle, error) {
	/* #nosec G304 — path is user-provided CLI argument for backtesting */
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}

	candles := make([]Candle, 0, len(records)-1)
	for i, rec := range records {
		if i == 0 {
			continue
		}
		if len(rec) < 5 {
			continue
		}

		ts, err := parseTimestamp(rec[0])
		if err != nil {
			continue
		}
		open, _ := strconv.ParseFloat(rec[1], 64)
		high, _ := strconv.ParseFloat(rec[2], 64)
		low, _ := strconv.ParseFloat(rec[3], 64)
		close, _ := strconv.ParseFloat(rec[4], 64)
		volume := 0.0
		if len(rec) >= 6 {
			volume, _ = strconv.ParseFloat(rec[5], 64)
		}

		candles = append(candles, Candle{
			Timestamp: ts,
			Symbol:    symbol,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
		})
	}

	if len(candles) == 0 {
		return nil, fmt.Errorf("no valid candles found in csv")
	}

	return candles, nil
}

func parseTimestamp(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"20060102",
	}

	// Try unix timestamp (seconds)
	if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
		if ts > 1e12 {
			return time.UnixMilli(ts), nil
		}
		return time.Unix(ts, 0), nil
	}

	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse timestamp: %s", s)
}
