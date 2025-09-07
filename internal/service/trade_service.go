package service

import (
	"CandleService/internal/model"
	"context"
	"fmt"
)

// Configuration constants
const (
	DefaultTradesLimit = 50
	OneMinuteMs        = 60 * 1000
	FiveMinuteMs       = 5 * 60 * 1000
	FifteenMinuteMs    = 15 * 60 * 1000
	OneHourMs          = 60 * 60 * 1000
)

type TradeStorage interface {
	GetLatestTrades(ctx context.Context, symbol string, limit int64) ([]model.Trade, error)
	GetOneMinuteCandles(ctx context.Context, symbol string, limit int64) ([]model.Candle, error)
}

// TradeService provides trade and candle data for the API
type TradeService struct {
	storage TradeStorage
}

// NewTradeService creates a new trade service
func NewTradeService(storage TradeStorage) *TradeService {
	return &TradeService{
		storage: storage,
	}
}

// GetTrades returns the last 50 trades for a symbol
func (ts *TradeService) GetTrades(ctx context.Context, symbol string) ([]model.Trade, error) {

	trades, err := ts.storage.GetLatestTrades(ctx, symbol, DefaultTradesLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest trades for symbol %s: %w", symbol, err)
	}

	return trades, nil
}

// GetCandles generates candles for a symbol and interval
func (ts *TradeService) GetCandles(ctx context.Context, symbol string, interval string, limit int64) ([]model.Candle, error) {

	// Calculate the no of 1m candles needed to generate the requested candles
	oneMinCandlesNeeded := ts.calculateOneMinCandlesNeeded(interval, limit)

	// Get pre-generated 1m candles from storage
	oneMinCandles, err := ts.storage.GetOneMinuteCandles(ctx, symbol, oneMinCandlesNeeded)
	if err != nil {
		return nil, fmt.Errorf("failed to get 1m candles for symbol %s: %w", symbol, err)
	}

	if len(oneMinCandles) == 0 {
		return []model.Candle{}, nil
	}

	// If requesting 1m, return the pre-generated candles with limit applied
	if interval == "1m" {
		return oneMinCandles, nil
	}

	// For other intervals, aggregate from pre-generated 1m candles
	aggregatedCandles := ts.aggregateCandles(oneMinCandles, interval)

	return aggregatedCandles, nil
}

// aggregateCandles creates higher timeframe candles from 1m candles
func (ts *TradeService) aggregateCandles(oneMinCandles []model.Candle, interval string) []model.Candle {
	intervalMs := getIntervalMs(interval)
	if intervalMs == 0 {
		return []model.Candle{}
	}

	var result []model.Candle
	var currentCandle *model.Candle

	for _, oneMinCandle := range oneMinCandles {
		// Round timestamp down to target interval boundary
		candleTime := (oneMinCandle.Timestamp / intervalMs) * intervalMs

		if currentCandle == nil || currentCandle.Timestamp != candleTime {
			// Start a new interval candle
			if currentCandle != nil {
				result = append(result, *currentCandle)
			}
			currentCandle = &model.Candle{
				Timestamp: candleTime,
				Open:      oneMinCandle.Open,
				High:      oneMinCandle.High,
				Low:       oneMinCandle.Low,
				Close:     oneMinCandle.Close,
				Volume:    oneMinCandle.Volume,
			}
		} else {
			// Aggregate into the current interval candle
			// Keep the original open (first candle's open)
			// Update high to max of current high and this candle's high
			if oneMinCandle.High > currentCandle.High {
				currentCandle.High = oneMinCandle.High
			}
			// Update low to min of current low and this candle's low
			if oneMinCandle.Low < currentCandle.Low {
				currentCandle.Low = oneMinCandle.Low
			}
			// Update close to the latest candle's close (since they're sorted)
			currentCandle.Close = oneMinCandle.Close
			// Add volume
			currentCandle.Volume += oneMinCandle.Volume
		}
	}

	// Add the final candle if it exists
	if currentCandle != nil {
		result = append(result, *currentCandle)
	}

	return result
}

// getIntervalMs converts interval string to milliseconds
func getIntervalMs(interval string) int64 {
	switch interval {
	case "1m":
		return OneMinuteMs
	case "5m":
		return FiveMinuteMs
	case "15m":
		return FifteenMinuteMs
	case "1h":
		return OneHourMs
	default:
		return OneMinuteMs // default to 1m
	}
}

// calculateOneMinCandlesNeeded calculates the number of 1-minute candles needed
// to generate the requested number of candles for a given interval
func (ts *TradeService) calculateOneMinCandlesNeeded(interval string, limit int64) int64 {
	if limit <= 0 {
		return 0 // No limit specified, get all available candles
	}

	// For 1m interval, we need exactly 'limit' candles
	if interval == "1m" {
		return limit
	}

	// For other intervals, calculate based on how many 1m candles fit into each interval
	switch interval {
	case "5m":
		return limit * 5 // 5 one-minute candles per 5-minute candle
	case "15m":
		return limit * 15 // 15 one-minute candles per 15-minute candle
	case "1h":
		return limit * 60 // 60 one-minute candles per 1-hour candle
	default:
		return limit // fallback to 1m behavior
	}
}
