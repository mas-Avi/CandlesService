package core

import (
	"CandleService/internal/model"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"
)

// Test constants
const floatDelta = 1e-9 // Delta for floating point comparisons

// Helper function for floating point comparison
func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < floatDelta
}

// MockTradeStorage implements TradeStorage interface for testing
type MockTradeStorage struct {
	trades          []model.Trade
	candles         map[string]model.Candle // symbol -> latest candle
	addTradeErr     error
	addCandleErr    error
	updateCandleErr error
	getLatestErr    error
	mu              sync.RWMutex

	// Track method calls for verification
	addTradeCalls        int
	addCandleCalls       int
	updateCandleCalls    int
	getLatestCandleCalls int
}

func NewMockTradeStorage() *MockTradeStorage {
	return &MockTradeStorage{
		trades:  make([]model.Trade, 0),
		candles: make(map[string]model.Candle),
	}
}

func (m *MockTradeStorage) AddTrade(trade model.Trade) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.addTradeCalls++
	if m.addTradeErr != nil {
		return m.addTradeErr
	}

	m.trades = append(m.trades, trade)
	return nil
}

func (m *MockTradeStorage) AddOneMinuteCandle(symbol string, candle model.Candle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.addCandleCalls++
	if m.addCandleErr != nil {
		return m.addCandleErr
	}

	m.candles[symbol] = candle
	return nil
}

func (m *MockTradeStorage) UpdateOneMinuteCandle(symbol string, candle model.Candle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updateCandleCalls++
	if m.updateCandleErr != nil {
		return m.updateCandleErr
	}

	m.candles[symbol] = candle
	return nil
}

func (m *MockTradeStorage) GetLatestCandle(ctx context.Context, symbol string) (model.Candle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.getLatestCandleCalls++
	if m.getLatestErr != nil {
		return model.Candle{}, m.getLatestErr
	}

	candle, exists := m.candles[symbol]
	if !exists {
		return model.Candle{}, errors.New("candle not found")
	}

	return candle, nil
}

// Helper methods for testing
func (m *MockTradeStorage) GetTrades() []model.Trade {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]model.Trade{}, m.trades...)
}

func (m *MockTradeStorage) GetCandle(symbol string) (model.Candle, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	candle, exists := m.candles[symbol]
	return candle, exists
}

func (m *MockTradeStorage) SetErrors(addTradeErr, addCandleErr, updateCandleErr, getLatestErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addTradeErr = addTradeErr
	m.addCandleErr = addCandleErr
	m.updateCandleErr = updateCandleErr
	m.getLatestErr = getLatestErr
}

func (m *MockTradeStorage) GetCallCounts() (int, int, int, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.addTradeCalls, m.addCandleCalls, m.updateCandleCalls, m.getLatestCandleCalls
}

func (m *MockTradeStorage) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trades = make([]model.Trade, 0)
	m.candles = make(map[string]model.Candle)
	m.addTradeCalls = 0
	m.addCandleCalls = 0
	m.updateCandleCalls = 0
	m.getLatestCandleCalls = 0
	m.addTradeErr = nil
	m.addCandleErr = nil
	m.updateCandleErr = nil
	m.getLatestErr = nil
}

// Helper functions for creating test data
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

// Test NewTradeIngestionService
func TestNewTradeIngestionService(t *testing.T) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	if service == nil {
		t.Fatal("NewTradeIngestionService returned nil")
	}

	if service.storage != storage {
		t.Error("Storage not set correctly")
	}

	if service.tradeChan == nil {
		t.Error("Trade channel not initialized")
	}

	if cap(service.tradeChan) != 1000 {
		t.Errorf("Expected channel capacity 1000, got %d", cap(service.tradeChan))
	}

	if service.stopped {
		t.Error("Service should not be stopped initially")
	}

	if service.logger == nil {
		t.Error("Logger not initialized")
	}
}

// Test GetTradeChannel
func TestGetTradeChannel(t *testing.T) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	channel := service.GetTradeChannel()
	if channel == nil {
		t.Fatal("GetTradeChannel returned nil")
	}

	// Test that we can send to the channel
	testTrade := createTestTrade("test1", "BTC-USD", 50000.0, 0.1, time.Now().UnixMilli())

	select {
	case channel <- testTrade:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Could not send trade to channel")
	}
}

// Test Stop method
func TestStop(t *testing.T) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	if service.stopped {
		t.Error("Service should not be stopped initially")
	}

	service.Stop()

	if !service.stopped {
		t.Error("Service should be stopped after calling Stop()")
	}
}

// Test Start method with immediate shutdown
func TestStart_ImmediateShutdown(t *testing.T) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	ctx, cancel := context.WithCancel(context.Background())

	// Start the service
	service.Start(ctx)

	// Immediately cancel the context
	cancel()

	// Give it a moment to process the cancellation
	time.Sleep(50 * time.Millisecond)

	// Check that the service marked itself as stopped
	service.mu.RLock()
	stopped := service.stopped
	service.mu.RUnlock()

	if !stopped {
		t.Error("Service should be stopped after context cancellation")
	}
}

// Test processing a single trade
func TestProcessSingleTrade(t *testing.T) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start the service
	service.Start(ctx)

	// Send a trade
	testTrade := createTestTrade("trade1", "BTC-USD", 50000.0, 0.1, time.Now().UnixMilli())
	tradeChan := service.GetTradeChannel()

	select {
	case tradeChan <- testTrade:
		// Trade sent successfully
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Failed to send trade")
	}

	// Give the service time to process
	time.Sleep(100 * time.Millisecond)

	// Verify trade was stored
	trades := storage.GetTrades()
	if len(trades) != 1 {
		t.Fatalf("Expected 1 trade, got %d", len(trades))
	}

	if trades[0].TradeID != "trade1" {
		t.Errorf("Expected trade ID 'trade1', got '%s'", trades[0].TradeID)
	}

	// Verify candle was created
	candle, exists := storage.GetCandle("BTC-USD")
	if !exists {
		t.Fatal("Candle was not created")
	}

	if !floatEquals(candle.Open, 50000.0) {
		t.Errorf("Expected candle open 50000.0, got %f", candle.Open)
	}

	if !floatEquals(candle.Volume, 0.1) {
		t.Errorf("Expected candle volume 0.1, got %f", candle.Volume)
	}
}

// Test updating existing candle
func TestUpdateExistingCandle(t *testing.T) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	baseTime := time.Now().UnixMilli()
	candleTimestamp := (baseTime / (60 * 1000)) * (60 * 1000) // Round to minute boundary

	// Pre-populate with an existing candle
	existingCandle := createTestCandle(candleTimestamp, 49000.0, 49500.0, 48500.0, 49200.0, 0.5)
	storage.AddOneMinuteCandle("BTC-USD", existingCandle)

	// Start the service
	service.Start(ctx)

	// Send a trade with the same timestamp (same minute)
	testTrade := createTestTrade("trade1", "BTC-USD", 50000.0, 0.1, candleTimestamp+30000) // 30 seconds into the minute
	tradeChan := service.GetTradeChannel()

	tradeChan <- testTrade

	// Give the service time to process
	time.Sleep(100 * time.Millisecond)

	// Verify candle was updated
	candle, exists := storage.GetCandle("BTC-USD")
	if !exists {
		t.Fatal("Candle not found")
	}

	// Check that high was updated
	if candle.High != 50000.0 {
		t.Errorf("Expected high to be updated to 50000.0, got %f", candle.High)
	}

	// Check that close was updated
	if candle.Close != 50000.0 {
		t.Errorf("Expected close to be updated to 50000.0, got %f", candle.Close)
	}

	// Check that volume was added
	if candle.Volume != 0.6 { // 0.5 + 0.1
		t.Errorf("Expected volume to be 0.6, got %f", candle.Volume)
	}

	// Check that open and low remained unchanged
	if candle.Open != 49000.0 {
		t.Errorf("Expected open to remain 49000.0, got %f", candle.Open)
	}

	if candle.Low != 48500.0 {
		t.Errorf("Expected low to remain 48500.0, got %f", candle.Low)
	}
}

// Test candle update with new low
func TestUpdateCandleWithNewLow(t *testing.T) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	baseTime := time.Now().UnixMilli()
	candleTimestamp := (baseTime / (60 * 1000)) * (60 * 1000)

	// Pre-populate with an existing candle
	existingCandle := createTestCandle(candleTimestamp, 50000.0, 50500.0, 49500.0, 50200.0, 0.5)
	storage.AddOneMinuteCandle("BTC-USD", existingCandle)

	service.Start(ctx)

	// Send a trade with a new low price
	testTrade := createTestTrade("trade1", "BTC-USD", 49000.0, 0.1, candleTimestamp+30000)
	tradeChan := service.GetTradeChannel()

	tradeChan <- testTrade
	time.Sleep(100 * time.Millisecond)

	candle, _ := storage.GetCandle("BTC-USD")

	// Check that low was updated
	if !floatEquals(candle.Low, 49000.0) {
		t.Errorf("Expected low to be updated to 49000.0, got %f", candle.Low)
	}

	// Check that close was updated
	if !floatEquals(candle.Close, 49000.0) {
		t.Errorf("Expected close to be updated to 49000.0, got %f", candle.Close)
	}
}

// Test multiple trades in sequence
func TestMultipleTrades(t *testing.T) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	service.Start(ctx)

	baseTime := time.Now().UnixMilli()
	tradeChan := service.GetTradeChannel()

	// Send multiple trades
	trades := []model.Trade{
		createTestTrade("trade1", "BTC-USD", 50000.0, 0.1, baseTime),
		createTestTrade("trade2", "BTC-USD", 50100.0, 0.2, baseTime+1000),
		createTestTrade("trade3", "ETH-USD", 3000.0, 1.0, baseTime+2000),
		createTestTrade("trade4", "BTC-USD", 49900.0, 0.15, baseTime+3000),
	}

	for _, trade := range trades {
		tradeChan <- trade
	}

	// Give time to process all trades
	time.Sleep(200 * time.Millisecond)

	// Verify all trades were stored
	storedTrades := storage.GetTrades()
	if len(storedTrades) != 4 {
		t.Fatalf("Expected 4 trades, got %d", len(storedTrades))
	}

	// Verify BTC candle
	btcCandle, exists := storage.GetCandle("BTC-USD")
	if !exists {
		t.Fatal("BTC candle not found")
	}

	// Should have aggregated the BTC trades
	expectedVolume := 0.1 + 0.2 + 0.15 // Sum of BTC trade volumes
	if !floatEquals(btcCandle.Volume, expectedVolume) {
		t.Errorf("Expected BTC volume %f, got %f", expectedVolume, btcCandle.Volume)
	}

	// Verify ETH candle
	ethCandle, exists := storage.GetCandle("ETH-USD")
	if !exists {
		t.Fatal("ETH candle not found")
	}

	if !floatEquals(ethCandle.Volume, 1.0) {
		t.Errorf("Expected ETH volume 1.0, got %f", ethCandle.Volume)
	}
}

// Test error handling in storage operations
func TestStorageErrors(t *testing.T) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Set errors for storage operations
	storage.SetErrors(
		errors.New("add trade error"),
		errors.New("add candle error"),
		errors.New("update candle error"),
		nil,
	)

	service.Start(ctx)

	testTrade := createTestTrade("trade1", "BTC-USD", 50000.0, 0.1, time.Now().UnixMilli())
	tradeChan := service.GetTradeChannel()

	tradeChan <- testTrade
	time.Sleep(100 * time.Millisecond)

	// Service should continue running despite errors
	// The trade processing should have been attempted
	addTradeCalls, addCandleCalls, _, _ := storage.GetCallCounts()

	if addTradeCalls != 1 {
		t.Errorf("Expected 1 AddTrade call, got %d", addTradeCalls)
	}

	if addCandleCalls != 1 {
		t.Errorf("Expected 1 AddCandle call, got %d", addCandleCalls)
	}
}

// Test concurrent trade processing
func TestConcurrentTradeProcessing(t *testing.T) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	service.Start(ctx)

	const numGoroutines = 10
	const tradesPerGoroutine = 10

	var wg sync.WaitGroup
	tradeChan := service.GetTradeChannel()
	baseTime := time.Now().UnixMilli()

	// Launch multiple goroutines sending trades
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < tradesPerGoroutine; i++ {
				trade := createTestTrade(
					fmt.Sprintf("trade_%d_%d", goroutineID, i),
					"BTC-USD",
					50000.0+float64(i),
					0.1,
					baseTime+int64(goroutineID*tradesPerGoroutine+i)*1000,
				)

				select {
				case tradeChan <- trade:
					// Successfully sent
				case <-time.After(100 * time.Millisecond):
					t.Errorf("Failed to send trade from goroutine %d", goroutineID)
				}
			}
		}(g)
	}

	wg.Wait()

	// Give time for processing
	time.Sleep(300 * time.Millisecond)

	// Verify all trades were processed
	trades := storage.GetTrades()
	expectedTrades := numGoroutines * tradesPerGoroutine

	if len(trades) != expectedTrades {
		t.Errorf("Expected %d trades, got %d", expectedTrades, len(trades))
	}
}

// Test channel capacity and blocking behavior
func TestChannelCapacity(t *testing.T) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	tradeChan := service.GetTradeChannel()

	// Fill the channel to capacity (1000)
	baseTime := time.Now().UnixMilli()
	for i := 0; i < 1000; i++ {
		trade := createTestTrade(
			fmt.Sprintf("trade%d", i),
			"BTC-USD",
			50000.0,
			0.1,
			baseTime+int64(i)*1000,
		)

		select {
		case tradeChan <- trade:
			// Successfully sent
		case <-time.After(10 * time.Millisecond):
			t.Fatalf("Channel should not block until capacity is reached, failed at trade %d", i)
		}
	}

	// Next trade should block since channel is full
	extraTrade := createTestTrade("extra", "BTC-USD", 50000.0, 0.1, baseTime+1001000)

	select {
	case tradeChan <- extraTrade:
		t.Error("Channel should block when full")
	case <-time.After(10 * time.Millisecond):
		// Expected behavior - channel is full
	}
}

// Test timestamp rounding for candle creation
func TestTimestampRounding(t *testing.T) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	service.Start(ctx)

	// Test various timestamps within the same minute
	baseTime := time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC).UnixMilli() // 10:05:00
	expectedCandleTimestamp := baseTime                                  // Should round to minute boundary

	testCases := []struct {
		name      string
		timestamp int64
	}{
		{"start of minute", baseTime},
		{"15 seconds in", baseTime + 15*1000},
		{"30 seconds in", baseTime + 30*1000},
		{"45 seconds in", baseTime + 45*1000},
		{"59 seconds in", baseTime + 59*1000},
	}

	tradeChan := service.GetTradeChannel()

	for i, tc := range testCases {
		trade := createTestTrade(
			fmt.Sprintf("trade%d", i),
			"BTC-USD",
			50000.0+float64(i*100),
			0.1,
			tc.timestamp,
		)
		tradeChan <- trade
	}

	time.Sleep(200 * time.Millisecond)

	// All trades should have created/updated the same candle
	candle, exists := storage.GetCandle("BTC-USD")
	if !exists {
		t.Fatal("Candle not found")
	}

	if candle.Timestamp != expectedCandleTimestamp {
		t.Errorf("Expected candle timestamp %d, got %d", expectedCandleTimestamp, candle.Timestamp)
	}

	// Volume should be sum of all trades
	expectedVolume := float64(len(testCases)) * 0.1
	if !floatEquals(candle.Volume, expectedVolume) {
		t.Errorf("Expected volume %f, got %f", expectedVolume, candle.Volume)
	}
}

// Benchmark tests
func BenchmarkTradeProcessing(b *testing.B) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	service.Start(ctx)

	tradeChan := service.GetTradeChannel()
	baseTime := time.Now().UnixMilli()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trade := createTestTrade(
			fmt.Sprintf("trade%d", i),
			"BTC-USD",
			50000.0+float64(i%1000),
			0.1,
			baseTime+int64(i)*1000,
		)

		tradeChan <- trade
	}

	// Wait for processing to complete
	time.Sleep(100 * time.Millisecond)
}

func BenchmarkCandleUpdate(b *testing.B) {
	storage := NewMockTradeStorage()
	service := NewTradeIngestionService(storage)

	baseTime := time.Now().UnixMilli()
	candleTimestamp := (baseTime / (60 * 1000)) * (60 * 1000)

	// Pre-populate with a candle
	existingCandle := createTestCandle(candleTimestamp, 50000.0, 50500.0, 49500.0, 50200.0, 10.0)
	storage.AddOneMinuteCandle("BTC-USD", existingCandle)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trade := createTestTrade(
			fmt.Sprintf("trade%d", i),
			"BTC-USD",
			50000.0+float64(i%100),
			0.1,
			candleTimestamp+int64(i%60)*1000, // Keep within same minute
		)

		service.updateOneMinuteCandle(trade)
	}
}
