package data

import (
	"CandleService/internal/model"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Test helper functions

func createTestTrade(tradeID, symbol string, price, size float64, timestamp int64) model.Trade {
	return model.Trade{
		TradeID:   tradeID,
		Symbol:    symbol,
		Price:     price,
		Size:      size,
		Timestamp: timestamp,
	}
}

func createTestCandle(timestamp int64, open, high, low, close, volume float64) model.Candle {
	return model.Candle{
		Timestamp: timestamp,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
	}
}

func createSequentialTrades(symbol string, count int, startTime int64, intervalMs int64) []model.Trade {
	trades := make([]model.Trade, count)
	for i := 0; i < count; i++ {
		trades[i] = createTestTrade(
			fmt.Sprintf("trade_%d", i),
			symbol,
			100.0+float64(i),
			10.0,
			startTime+int64(i)*intervalMs,
		)
	}
	return trades
}

func createSequentialCandles(count int, startTime int64, intervalMs int64) []model.Candle {
	candles := make([]model.Candle, count)
	for i := 0; i < count; i++ {
		candles[i] = createTestCandle(
			startTime+int64(i)*intervalMs,
			100.0+float64(i),
			105.0+float64(i),
			95.0+float64(i),
			102.0+float64(i),
			1000.0+float64(i*10),
		)
	}
	return candles
}

// Unit Tests

func TestDefaultStorageConfig(t *testing.T) {
	config := DefaultStorageConfig()

	if config.MaxTradesPerSymbol != 10000 {
		t.Errorf("Expected MaxTradesPerSymbol to be 10000, got %d", config.MaxTradesPerSymbol)
	}

	if config.MaxCandlesPerSymbol != 1000 {
		t.Errorf("Expected MaxCandlesPerSymbol to be 1000, got %d", config.MaxCandlesPerSymbol)
	}
}

func TestNewInMemoryTradeStorage(t *testing.T) {
	storage := NewInMemoryTradeStorage()

	if storage == nil {
		t.Fatal("NewInMemoryTradeStorage returned nil")
	}

	if storage.trades == nil {
		t.Error("trades map not initialized")
	}

	if storage.oneMinCandles == nil {
		t.Error("oneMinCandles map not initialized")
	}

	if storage.config.MaxTradesPerSymbol != 10000 {
		t.Error("Default config not applied correctly")
	}
}

func TestNewInMemoryTradeStorageWithConfig(t *testing.T) {
	customConfig := StorageConfig{
		MaxTradesPerSymbol:  5000,
		MaxCandlesPerSymbol: 500,
	}

	storage := NewInMemoryTradeStorageWithConfig(customConfig)

	if storage == nil {
		t.Fatal("NewInMemoryTradeStorageWithConfig returned nil")
	}

	if storage.config.MaxTradesPerSymbol != 5000 {
		t.Errorf("Expected MaxTradesPerSymbol to be 5000, got %d", storage.config.MaxTradesPerSymbol)
	}

	if storage.config.MaxCandlesPerSymbol != 500 {
		t.Errorf("Expected MaxCandlesPerSymbol to be 500, got %d", storage.config.MaxCandlesPerSymbol)
	}
}

func TestAddTrade(t *testing.T) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	tests := []struct {
		name    string
		trade   model.Trade
		wantErr bool
	}{
		{
			name:    "valid trade",
			trade:   createTestTrade("trade1", "BTC-USD", 50000.0, 0.1, startTime),
			wantErr: false,
		},
		{
			name:    "another valid trade",
			trade:   createTestTrade("trade2", "BTC-USD", 50100.0, 0.2, startTime+1000),
			wantErr: false,
		},
		{
			name:    "different symbol",
			trade:   createTestTrade("trade3", "ETH-USD", 3000.0, 1.0, startTime+2000),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := storage.AddTrade(tt.trade)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddTrade() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Verify trades were stored correctly
	btcTrades := storage.trades["BTC-USD"]
	if len(btcTrades) != 2 {
		t.Errorf("Expected 2 BTC-USD trades, got %d", len(btcTrades))
	}

	ethTrades := storage.trades["ETH-USD"]
	if len(ethTrades) != 1 {
		t.Errorf("Expected 1 ETH-USD trade, got %d", len(ethTrades))
	}
}

func TestAddTrade_OrderMaintained(t *testing.T) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	// Add trades in sequence
	trades := createSequentialTrades("BTC-USD", 5, startTime, 1000)
	for _, trade := range trades {
		err := storage.AddTrade(trade)
		if err != nil {
			t.Fatalf("Failed to add trade: %v", err)
		}
	}

	storedTrades := storage.trades["BTC-USD"]
	if len(storedTrades) != 5 {
		t.Fatalf("Expected 5 trades, got %d", len(storedTrades))
	}

	// Verify order is maintained
	for i := 1; i < len(storedTrades); i++ {
		if storedTrades[i].Timestamp < storedTrades[i-1].Timestamp {
			t.Errorf("Trades not in chronological order at index %d", i)
		}
	}
}

func TestAddTrade_MaxTradesLimit(t *testing.T) {
	config := StorageConfig{
		MaxTradesPerSymbol:  3,
		MaxCandlesPerSymbol: 1000,
	}
	storage := NewInMemoryTradeStorageWithConfig(config)
	startTime := time.Now().UnixMilli()

	// Add more trades than the limit
	trades := createSequentialTrades("BTC-USD", 5, startTime, 1000)
	for _, trade := range trades {
		err := storage.AddTrade(trade)
		if err != nil {
			t.Fatalf("Failed to add trade: %v", err)
		}
	}

	storedTrades := storage.trades["BTC-USD"]
	if len(storedTrades) != 3 {
		t.Errorf("Expected 3 trades (limit), got %d", len(storedTrades))
	}

	// Verify we kept the latest trades (trade_2, trade_3, trade_4)
	if storedTrades[0].TradeID != "trade_2" {
		t.Errorf("Expected first trade to be trade_2, got %s", storedTrades[0].TradeID)
	}

	if storedTrades[2].TradeID != "trade_4" {
		t.Errorf("Expected last trade to be trade_4, got %s", storedTrades[2].TradeID)
	}
}

func TestGetLatestTrades(t *testing.T) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	// Add test trades
	trades := createSequentialTrades("BTC-USD", 10, startTime, 1000)
	for _, trade := range trades {
		storage.AddTrade(trade)
	}

	tests := []struct {
		name          string
		symbol        string
		limit         int64
		expectedCount int
		expectError   bool
	}{
		{
			name:          "get latest 5 trades",
			symbol:        "BTC-USD",
			limit:         5,
			expectedCount: 5,
			expectError:   false,
		},
		{
			name:          "get all trades (limit 0)",
			symbol:        "BTC-USD",
			limit:         0,
			expectedCount: 10,
			expectError:   false,
		},
		{
			name:          "get more than available",
			symbol:        "BTC-USD",
			limit:         20,
			expectedCount: 10,
			expectError:   false,
		},
		{
			name:          "non-existent symbol",
			symbol:        "XRP-USD",
			limit:         5,
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := storage.GetLatestTrades(context.Background(), tt.symbol, tt.limit)

			if (err != nil) != tt.expectError {
				t.Errorf("GetLatestTrades() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if len(result) != tt.expectedCount {
				t.Errorf("GetLatestTrades() returned %d trades, expected %d", len(result), tt.expectedCount)
			}

			// Verify we got the latest trades
			if tt.expectedCount > 0 && tt.symbol == "BTC-USD" {
				expectedLastTradeID := fmt.Sprintf("trade_%d", 10-1) // trade_9 for 10 trades
				actualLastTradeID := result[len(result)-1].TradeID
				if tt.limit > 0 && tt.limit < 10 {
					// When limited, the last trade should be the most recent
					if actualLastTradeID != expectedLastTradeID {
						t.Errorf("Expected last trade to be %s, got %s", expectedLastTradeID, actualLastTradeID)
					}
				}
			}
		})
	}
}

func TestGetLatestTrades_ReturnsLatestData(t *testing.T) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	// Add 10 trades
	trades := createSequentialTrades("BTC-USD", 10, startTime, 1000)
	for _, trade := range trades {
		storage.AddTrade(trade)
	}

	// Get latest 3 trades
	result, err := storage.GetLatestTrades(context.Background(), "BTC-USD", 3)
	if err != nil {
		t.Fatalf("GetLatestTrades failed: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected 3 trades, got %d", len(result))
	}

	// Should be trades 7, 8, 9 (indices 7, 8, 9 from the original 10)
	expectedTradeIDs := []string{"trade_7", "trade_8", "trade_9"}
	for i, trade := range result {
		if trade.TradeID != expectedTradeIDs[i] {
			t.Errorf("Trade %d: expected %s, got %s", i, expectedTradeIDs[i], trade.TradeID)
		}
	}
}

func TestAddOneMinuteCandle(t *testing.T) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	candle1 := createTestCandle(startTime, 100, 105, 95, 102, 1000)
	candle2 := createTestCandle(startTime+60000, 102, 108, 100, 106, 1500)

	tests := []struct {
		name    string
		symbol  string
		candle  model.Candle
		wantErr bool
	}{
		{
			name:    "add first candle",
			symbol:  "BTC-USD",
			candle:  candle1,
			wantErr: false,
		},
		{
			name:    "add second candle",
			symbol:  "BTC-USD",
			candle:  candle2,
			wantErr: false,
		},
		{
			name:    "add candle for different symbol",
			symbol:  "ETH-USD",
			candle:  candle1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := storage.AddOneMinuteCandle(tt.symbol, tt.candle)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddOneMinuteCandle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Verify candles were stored
	btcCandles := storage.oneMinCandles["BTC-USD"]
	if len(btcCandles) != 2 {
		t.Errorf("Expected 2 BTC-USD candles, got %d", len(btcCandles))
	}

	ethCandles := storage.oneMinCandles["ETH-USD"]
	if len(ethCandles) != 1 {
		t.Errorf("Expected 1 ETH-USD candle, got %d", len(ethCandles))
	}
}

func TestAddOneMinuteCandle_OrderMaintained(t *testing.T) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	// Add candles in sequence
	candles := createSequentialCandles(5, startTime, 60000) // 1 minute apart
	for _, candle := range candles {
		err := storage.AddOneMinuteCandle("BTC-USD", candle)
		if err != nil {
			t.Fatalf("Failed to add candle: %v", err)
		}
	}

	storedCandles := storage.oneMinCandles["BTC-USD"]
	if len(storedCandles) != 5 {
		t.Fatalf("Expected 5 candles, got %d", len(storedCandles))
	}

	// Verify chronological order
	for i := 1; i < len(storedCandles); i++ {
		if storedCandles[i].Timestamp <= storedCandles[i-1].Timestamp {
			t.Errorf("Candles not in chronological order at index %d", i)
		}
	}
}

func TestAddOneMinuteCandle_MaxCandlesLimit(t *testing.T) {
	config := StorageConfig{
		MaxTradesPerSymbol:  1000,
		MaxCandlesPerSymbol: 3,
	}
	storage := NewInMemoryTradeStorageWithConfig(config)
	startTime := time.Now().UnixMilli()

	// Add more candles than the limit
	candles := createSequentialCandles(5, startTime, 60000)
	for _, candle := range candles {
		err := storage.AddOneMinuteCandle("BTC-USD", candle)
		if err != nil {
			t.Fatalf("Failed to add candle: %v", err)
		}
	}

	storedCandles := storage.oneMinCandles["BTC-USD"]
	if len(storedCandles) != 3 {
		t.Errorf("Expected 3 candles (limit), got %d", len(storedCandles))
	}

	// Verify we kept the latest candles
	expectedStartTime := startTime + 2*60000 // Should start from the 3rd candle
	if storedCandles[0].Timestamp != expectedStartTime {
		t.Errorf("Expected first candle timestamp %d, got %d", expectedStartTime, storedCandles[0].Timestamp)
	}
}

func TestGetOneMinuteCandles(t *testing.T) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	// Add test candles
	candles := createSequentialCandles(5, startTime, 60000)
	for _, candle := range candles {
		storage.AddOneMinuteCandle("BTC-USD", candle)
	}

	tests := []struct {
		name          string
		symbol        string
		expectedCount int
		expectError   bool
	}{
		{
			name:          "get existing candles",
			symbol:        "BTC-USD",
			expectedCount: 5,
			expectError:   false,
		},
		{
			name:          "get non-existent symbol",
			symbol:        "XRP-USD",
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:          "empty symbol",
			symbol:        "",
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := storage.GetOneMinuteCandles(context.Background(), tt.symbol, 1000)

			if (err != nil) != tt.expectError {
				t.Errorf("GetOneMinuteCandles() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if len(result) != tt.expectedCount {
				t.Errorf("GetOneMinuteCandles() returned %d candles, expected %d", len(result), tt.expectedCount)
			}
		})
	}
}

func TestGetOneMinuteCandles_ReturnsCopy(t *testing.T) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	originalCandle := createTestCandle(startTime, 100, 105, 95, 102, 1000)
	storage.AddOneMinuteCandle("BTC-USD", originalCandle)

	// Get candles
	result, err := storage.GetOneMinuteCandles(context.Background(), "BTC-USD", 1000)
	if err != nil {
		t.Fatalf("GetOneMinuteCandles failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 candle, got %d", len(result))
	}

	// Modify the returned slice
	result[0].Close = 999.0

	// Verify original data is unchanged
	storedCandles := storage.oneMinCandles["BTC-USD"]
	if storedCandles[0].Close == 999.0 {
		t.Error("Original data was modified - GetOneMinuteCandles should return a copy")
	}

	if storedCandles[0].Close != 102.0 {
		t.Errorf("Expected original close to be 102.0, got %f", storedCandles[0].Close)
	}
}

// Concurrency Tests

func TestConcurrentAddTrade(t *testing.T) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	const goroutines = 10
	const tradesPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Launch concurrent goroutines adding trades
	for g := 0; g < goroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < tradesPerGoroutine; i++ {
				trade := createTestTrade(
					fmt.Sprintf("trade_%d_%d", goroutineID, i),
					"BTC-USD",
					50000.0+float64(i),
					0.1,
					startTime+int64(goroutineID*tradesPerGoroutine+i)*1000,
				)
				err := storage.AddTrade(trade)
				if err != nil {
					t.Errorf("Failed to add trade: %v", err)
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify all trades were added
	trades := storage.trades["BTC-USD"]
	expectedTotal := goroutines * tradesPerGoroutine
	if len(trades) != expectedTotal {
		t.Errorf("Expected %d trades, got %d", expectedTotal, len(trades))
	}
}

func TestConcurrentAddCandle(t *testing.T) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	const goroutines = 5
	const candlesPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Launch concurrent goroutines adding candles
	for g := 0; g < goroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < candlesPerGoroutine; i++ {
				candle := createTestCandle(
					startTime+int64(goroutineID*candlesPerGoroutine+i)*60000,
					100.0+float64(i),
					105.0+float64(i),
					95.0+float64(i),
					102.0+float64(i),
					1000.0,
				)
				err := storage.AddOneMinuteCandle("BTC-USD", candle)
				if err != nil {
					t.Errorf("Failed to add candle: %v", err)
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify all candles were added
	candles := storage.oneMinCandles["BTC-USD"]
	expectedTotal := goroutines * candlesPerGoroutine
	if len(candles) != expectedTotal {
		t.Errorf("Expected %d candles, got %d", expectedTotal, len(candles))
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	const duration = 100 * time.Millisecond
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		counter := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
				trade := createTestTrade(
					fmt.Sprintf("trade_%d", counter),
					"BTC-USD",
					50000.0+float64(counter),
					0.1,
					startTime+int64(counter)*1000,
				)
				storage.AddTrade(trade)
				counter++
				time.Sleep(time.Millisecond)
			}
		}
	}()

	// Reader goroutines
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_, err := storage.GetLatestTrades(context.Background(), "BTC-USD", 10)
					if err != nil {
						t.Errorf("GetLatestTrades failed: %v", err)
					}
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()

	// Test should complete without race conditions or deadlocks
}

// Benchmark Tests

func BenchmarkAddTrade(b *testing.B) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trade := createTestTrade(
			fmt.Sprintf("trade_%d", i),
			"BTC-USD",
			50000.0+float64(i),
			0.1,
			startTime+int64(i)*1000,
		)
		storage.AddTrade(trade)
	}
}

func BenchmarkAddOneMinuteCandle(b *testing.B) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		candle := createTestCandle(
			startTime+int64(i)*60000,
			100.0+float64(i),
			105.0+float64(i),
			95.0+float64(i),
			102.0+float64(i),
			1000.0,
		)
		storage.AddOneMinuteCandle("BTC-USD", candle)
	}
}

func BenchmarkGetLatestTrades(b *testing.B) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	// Pre-populate with trades
	trades := createSequentialTrades("BTC-USD", 1000, startTime, 1000)
	for _, trade := range trades {
		storage.AddTrade(trade)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := storage.GetLatestTrades(context.Background(), "BTC-USD", 100)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetOneMinuteCandles(b *testing.B) {
	storage := NewInMemoryTradeStorage()
	startTime := time.Now().UnixMilli()

	// Pre-populate with candles
	candles := createSequentialCandles(1000, startTime, 60000)
	for _, candle := range candles {
		storage.AddOneMinuteCandle("BTC-USD", candle)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := storage.GetOneMinuteCandles(context.Background(), "BTC-USD", 100)
		if err != nil {
			b.Fatal(err)
		}
	}
}
