package core

import (
	"CandleService/internal/model"
	"context"
	"log/slog"
	"sync"
	"time"
)

type TradeStorage interface {
	AddTrade(trade model.Trade) error
	AddOneMinuteCandle(symbol string, candle model.Candle) error
	UpdateOneMinuteCandle(symbol string, candle model.Candle) error
	GetLatestCandle(ctx context.Context, symbol string) (model.Candle, error)
}

// TradeIngestionService handles receiving trades from a channel and storing them
type TradeIngestionService struct {
	storage   TradeStorage
	tradeChan chan model.Trade
	stopped   bool
	mu        sync.RWMutex
	logger    *slog.Logger
}

// NewTradeIngestionService creates a new trade ingestion service
func NewTradeIngestionService(storage TradeStorage) *TradeIngestionService {
	return &TradeIngestionService{
		storage:   storage,
		tradeChan: make(chan model.Trade, 1000), // Buffered channel for better performance
		stopped:   false,
		logger:    slog.Default(),
	}
}

// Start begins processing trades from the channel using the provided context
func (tis *TradeIngestionService) Start(ctx context.Context) {
	tis.logger.Info("starting trade ingestion service")

	go func() {
		defer close(tis.tradeChan) // Close trade channel when goroutine exits
		defer tis.logger.Info("trade ingestion service stopped")

		for {
			select {
			case trade := <-tis.tradeChan:
				// Store the trade with error handling
				if err := tis.storage.AddTrade(trade); err != nil {
					tis.logger.Error("failed to store trade",
						"trade_id", trade.TradeID,
						"symbol", trade.Symbol,
						"error", err)
					// Continue processing despite storage error
				}

				// Generate/update the 1m candle for this trade
				tis.updateOneMinuteCandle(trade)

			case <-ctx.Done():
				tis.logger.Info("received shutdown signal, stopping")

				// Context cancelled, drain any remaining trades in the channel before stopping
				tis.mu.Lock()
				tis.stopped = true
				tis.mu.Unlock()

				return
			}
		}
	}()
}

// updateOneMinuteCandle generates or updates a 1m candle based on the incoming trade
func (tis *TradeIngestionService) updateOneMinuteCandle(trade model.Trade) {
	// Create context with timeout for storage operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Calculate the 1m candle timestamp (round down to minute boundary)
	candleTimestamp := (trade.Timestamp / (60 * 1000)) * (60 * 1000)

	// Get latest 1m candles for this symbol with timeout
	latestCandle, err := tis.storage.GetLatestCandle(ctx, trade.Symbol)
	isUpdate := err == nil && latestCandle.Timestamp == candleTimestamp

	if isUpdate {
		tis.updateCandle(trade, latestCandle)
	} else {
		tis.addCandle(trade, candleTimestamp)
	}
}

func (tis *TradeIngestionService) addCandle(trade model.Trade, candleTimestamp int64) {

	// Create new candle
	candle := model.Candle{
		Timestamp: candleTimestamp,
		Open:      trade.Price,
		High:      trade.Price,
		Low:       trade.Price,
		Close:     trade.Price,
		Volume:    trade.Size,
	}

	tis.logger.Debug("created new 1m candle",
		"symbol", trade.Symbol,
		"timestamp", candleTimestamp,
		"price", trade.Price,
		"volume", trade.Size)

	// Store the updated candle with error handling
	if err := tis.storage.AddOneMinuteCandle(trade.Symbol, candle); err != nil {
		tis.logger.Error("failed to store 1m candle",
			"symbol", trade.Symbol,
			"trade_id", trade.TradeID,
			"timestamp", candleTimestamp,
			"error", err)
	}
}

func (tis *TradeIngestionService) updateCandle(trade model.Trade, updateCandle model.Candle) {

	// Update high
	if trade.Price > updateCandle.High {
		updateCandle.High = trade.Price
	}

	// Update low
	if trade.Price < updateCandle.Low {
		updateCandle.Low = trade.Price
	}

	// Update close (always the latest trade price)
	updateCandle.Close = trade.Price

	// Add volume
	updateCandle.Volume += trade.Size

	tis.logger.Debug("updated existing 1m candle",
		"symbol", trade.Symbol,
		"timestamp", updateCandle,
		"new_price", trade.Price,
		"total_volume", updateCandle.Volume)

	// Store the updated candle with error handling
	if err := tis.storage.UpdateOneMinuteCandle(trade.Symbol, updateCandle); err != nil {
		tis.logger.Error("failed to store 1m candle",
			"symbol", trade.Symbol,
			"trade_id", trade.TradeID,
			"timestamp", updateCandle.Timestamp,
			"error", err)
	}
}

// Stop stops the trade ingestion service (kept for backward compatibility)
func (tis *TradeIngestionService) Stop() {
	tis.mu.Lock()
	tis.stopped = true
	tis.mu.Unlock()
}

// GetTradeChannel returns the channel for sending trades to the service
func (tis *TradeIngestionService) GetTradeChannel() chan<- model.Trade {
	return tis.tradeChan
}
