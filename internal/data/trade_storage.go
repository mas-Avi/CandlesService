package data

import (
	"CandleService/internal/model"
	"context"
	"fmt"
	"sync"
)

// StorageConfig holds configuration for the trade storage
type StorageConfig struct {
	MaxTradesPerSymbol  int
	MaxCandlesPerSymbol int
}

// DefaultStorageConfig returns sensible default configuration
func DefaultStorageConfig() StorageConfig {
	return StorageConfig{
		MaxTradesPerSymbol:  1000 * 10, // Keep last 10k trades per symbol
		MaxCandlesPerSymbol: 1000,
	}
}

// InMemoryTradeStorage implements TradeStorage using an in-memory map
type InMemoryTradeStorage struct {
	trades        map[string][]model.Trade  // symbol -> trades
	oneMinCandles map[string][]model.Candle // symbol -> 1m candles
	config        StorageConfig
	mu            sync.RWMutex
}

// NewInMemoryTradeStorage creates a new in-memory trade storage with default config
func NewInMemoryTradeStorage() *InMemoryTradeStorage {
	return NewInMemoryTradeStorageWithConfig(DefaultStorageConfig())
}

// NewInMemoryTradeStorageWithConfig creates a new in-memory trade storage with custom config
func NewInMemoryTradeStorageWithConfig(config StorageConfig) *InMemoryTradeStorage {
	return &InMemoryTradeStorage{
		trades:        make(map[string][]model.Trade),
		oneMinCandles: make(map[string][]model.Candle),
		config:        config,
	}
}

// AddTrade adds a trade to the storage with optimized insertion
func (s *InMemoryTradeStorage) AddTrade(trade model.Trade) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	trades := s.trades[trade.Symbol]

	// Assumption is we can append as trades should be received in order (within an exchange)
	s.trades[trade.Symbol] = append(trades, trade)

	// Keep only last config.MaxTradesPerSymbol trades per symbol to prevent memory issues
	trades = s.trades[trade.Symbol]
	if len(trades) > s.config.MaxTradesPerSymbol {
		oldestTimestamp := trades[0].Timestamp
		s.trades[trade.Symbol] = trades[len(trades)-s.config.MaxTradesPerSymbol:]

		// Log context about trimming (useful for monitoring/debugging)
		_ = fmt.Sprintf("trimmed trades for symbol %s: removed %d trades (oldest timestamp: %d, limit: %d)",
			trade.Symbol, len(trades)-s.config.MaxTradesPerSymbol, oldestTimestamp, s.config.MaxTradesPerSymbol)
	}

	return nil
}

// GetLatestCandle returns the last candle for a symbol
func (s *InMemoryTradeStorage) GetLatestCandle(ctx context.Context, symbol string) (model.Candle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candles, exists := s.oneMinCandles[symbol]
	if !exists || len(candles) == 0 {
		return model.Candle{}, nil
	}

	return candles[len(candles)-1], nil
}

// GetLatestTrades returns the last 'limit' trades for a symbol
func (s *InMemoryTradeStorage) GetLatestTrades(ctx context.Context, symbol string, limit int64) ([]model.Trade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trades, exists := s.trades[symbol]
	if !exists {
		return []model.Trade{}, nil
	}

	originalCount := len(trades)

	// Limit the number of trades returned
	if limit > 0 && int64(len(trades)) > limit {
		trades = trades[len(trades)-int(limit):]
	}

	// Return a copy to prevent external modification
	result := make([]model.Trade, len(trades))
	copy(result, trades)

	// Add context for debugging (could be logged)
	_ = fmt.Sprintf("retrieved %d trades for symbol %s (total available: %d, requested limit: %d)",
		len(result), symbol, originalCount, limit)

	return result, nil
}

// AddOneMinuteCandle adds a 1m candle in storage
func (s *InMemoryTradeStorage) AddOneMinuteCandle(symbol string, candle model.Candle) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	candles := s.oneMinCandles[symbol]

	// Assumption: Will only receive candles for completed minutes in order
	candles = append(candles, candle)

	// Keep only last config.MaxCandlesPerSymbol candles per symbol to prevent memory issues
	if len(candles) > s.config.MaxCandlesPerSymbol {
		oldestTimestamp := candles[0].Timestamp
		candles = candles[len(candles)-s.config.MaxCandlesPerSymbol:]

		_ = fmt.Sprintf("trimmed candles for symbol %s: removed %d candles (oldest timestamp: %d, limit: %d)",
			symbol, len(candles)-s.config.MaxCandlesPerSymbol, oldestTimestamp, s.config.MaxCandlesPerSymbol)
	}

	s.oneMinCandles[symbol] = candles
	return nil
}

// UpdateOneMinuteCandle updates a 1m candle in storage
func (s *InMemoryTradeStorage) UpdateOneMinuteCandle(symbol string, candle model.Candle) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	candles := s.oneMinCandles[symbol]

	// Assumption: Will only receive update for latest candle
	// update last candle
	candles[len(candles)-1] = candle

	return nil
}

// GetOneMinuteCandles returns all 1m candles for a symbol
func (s *InMemoryTradeStorage) GetOneMinuteCandles(ctx context.Context, symbol string, limit int64) ([]model.Candle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candles, exists := s.oneMinCandles[symbol]
	if !exists {
		return []model.Candle{}, nil
	}

	// Limit the number of candles returned
	if limit > 0 && int64(len(candles)) > limit {
		candles = candles[len(candles)-int(limit):]
	}

	// Return a copy to prevent external modification
	result := make([]model.Candle, len(candles))
	copy(result, candles)

	// Add context for debugging
	_ = fmt.Sprintf("retrieved all %d candles for symbol %s", len(result), symbol)

	return result, nil
}
