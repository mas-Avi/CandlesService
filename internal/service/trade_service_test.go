package service

import (
	"CandleService/internal/model"
	"context"
	"errors"
	"testing"
	"time"
)

// MockTradeStorage is a mock implementation of TradeStorage for testing
type MockTradeStorage struct {
	trades        []model.Trade
	oneMinCandles []model.Candle
	shouldError   bool
	errorMessage  string
}

func (m *MockTradeStorage) GetLatestTrades(ctx context.Context, symbol string, limit int64) ([]model.Trade, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMessage)
	}

	// Filter by symbol
	var symbolTrades []model.Trade
	for _, trade := range m.trades {
		if trade.Symbol == symbol {
			symbolTrades = append(symbolTrades, trade)
		}
	}

	// Apply limit
	if limit > 0 && int64(len(symbolTrades)) > limit {
		symbolTrades = symbolTrades[len(symbolTrades)-int(limit):]
	}

	return symbolTrades, nil
}

func (m *MockTradeStorage) GetOneMinuteCandles(ctx context.Context, symbol string, limit int64) ([]model.Candle, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMessage)
	}

	candles := m.oneMinCandles

	// Apply limit
	if limit > 0 && int64(len(candles)) > limit {
		candles = candles[len(candles)-int(limit):]
	}

	return candles, nil
}

// Helper function to create test candles
func createTestCandles(count int, intervalMs int64, startTime int64) []model.Candle {
	var candles []model.Candle
	for i := 0; i < count; i++ {
		timestamp := startTime + int64(i)*intervalMs
		candles = append(candles, model.Candle{
			Timestamp: timestamp,
			Open:      100.0 + float64(i),
			High:      105.0 + float64(i),
			Low:       95.0 + float64(i),
			Close:     102.0 + float64(i),
			Volume:    1000.0 + float64(i*10),
		})
	}
	return candles
}

// Helper function to create test trades
func createTestTrades(count int, symbol string, startTime int64) []model.Trade {
	var trades []model.Trade
	for i := 0; i < count; i++ {
		trades = append(trades, model.Trade{
			TradeID:   "trade_" + string(rune(i+48)), // Convert to ASCII
			Symbol:    symbol,
			Price:     100.0 + float64(i),
			Size:      10.0,
			Timestamp: startTime + int64(i*60000), // 1 minute apart
		})
	}
	return trades
}

func TestNewTradeService(t *testing.T) {
	mockStorage := &MockTradeStorage{}
	service := NewTradeService(mockStorage)

	if service == nil {
		t.Fatal("NewTradeService returned nil")
	}

	if service.storage != mockStorage {
		t.Error("TradeService storage not set correctly")
	}
}

func TestGetTrades(t *testing.T) {
	tests := []struct {
		name          string
		symbol        string
		trades        []model.Trade
		shouldError   bool
		errorMessage  string
		expectedCount int
		expectError   bool
	}{
		{
			name:          "successful retrieval",
			symbol:        "BTC-USD",
			trades:        createTestTrades(100, "BTC-USD", time.Now().UnixMilli()),
			shouldError:   false,
			expectedCount: DefaultTradesLimit,
			expectError:   false,
		},
		{
			name:          "empty symbol",
			symbol:        "",
			trades:        []model.Trade{},
			shouldError:   false,
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:         "storage error",
			symbol:       "BTC-USD",
			trades:       []model.Trade{},
			shouldError:  true,
			errorMessage: "storage error",
			expectError:  true,
		},
		{
			name:          "fewer trades than limit",
			symbol:        "BTC-USD",
			trades:        createTestTrades(10, "BTC-USD", time.Now().UnixMilli()),
			shouldError:   false,
			expectedCount: 10,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockTradeStorage{
				trades:       tt.trades,
				shouldError:  tt.shouldError,
				errorMessage: tt.errorMessage,
			}
			service := NewTradeService(mockStorage)

			result, err := service.GetTrades(context.Background(), tt.symbol)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d trades, got %d", tt.expectedCount, len(result))
			}
		})
	}
}

func TestGetCandles(t *testing.T) {
	// Create test data: 120 one-minute candles (2 hours worth)
	startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC).UnixMilli()
	oneMinCandles := createTestCandles(120, OneMinuteMs, startTime)

	tests := []struct {
		name          string
		symbol        string
		interval      string
		limit         int64
		oneMinCandles []model.Candle
		shouldError   bool
		errorMessage  string
		expectedCount int
		expectError   bool
	}{
		{
			name:          "1m candles with limit",
			symbol:        "BTC-USD",
			interval:      "1m",
			limit:         10,
			oneMinCandles: oneMinCandles,
			expectedCount: 10,
		},
		{
			name:          "5m candles aggregation",
			symbol:        "BTC-USD",
			interval:      "5m",
			limit:         5,
			oneMinCandles: oneMinCandles[:25], // 25 candles = 5 five-minute candles
			expectedCount: 5,
		},
		{
			name:          "15m candles aggregation",
			symbol:        "BTC-USD",
			interval:      "15m",
			limit:         2,
			oneMinCandles: oneMinCandles[:30], // 30 candles = 2 fifteen-minute candles
			expectedCount: 2,
		},
		{
			name:          "1h candles aggregation",
			symbol:        "BTC-USD",
			interval:      "1h",
			limit:         1,
			oneMinCandles: oneMinCandles[:60], // 60 candles = 1 hour candle
			expectedCount: 1,
		},
		{
			name:         "storage error",
			symbol:       "BTC-USD",
			interval:     "1m",
			limit:        10,
			shouldError:  true,
			errorMessage: "storage error",
			expectError:  true,
		},
		{
			name:          "empty candles",
			symbol:        "BTC-USD",
			interval:      "1m",
			limit:         10,
			oneMinCandles: []model.Candle{},
			expectedCount: 0,
		},
		{
			name:          "zero limit",
			symbol:        "BTC-USD",
			interval:      "1m",
			limit:         0,
			oneMinCandles: oneMinCandles,
			expectedCount: 120,
		},
		{
			name:          "negative limit",
			symbol:        "BTC-USD",
			interval:      "1m",
			limit:         -5,
			oneMinCandles: oneMinCandles,
			expectedCount: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockTradeStorage{
				oneMinCandles: tt.oneMinCandles,
				shouldError:   tt.shouldError,
				errorMessage:  tt.errorMessage,
			}
			service := NewTradeService(mockStorage)

			result, err := service.GetCandles(context.Background(), tt.symbol, tt.interval, tt.limit)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d candles, got %d", tt.expectedCount, len(result))
			}
		})
	}
}

func TestCalculateOneMinCandlesNeeded(t *testing.T) {
	service := NewTradeService(&MockTradeStorage{})

	tests := []struct {
		name     string
		interval string
		limit    int64
		expected int64
	}{
		{"1m interval", "1m", 10, 10},
		{"5m interval", "5m", 5, 25},
		{"15m interval", "15m", 4, 60},
		{"1h interval", "1h", 2, 120},
		{"zero limit", "1m", 0, 0},
		{"negative limit", "5m", -5, 0},
		{"unknown interval", "30m", 10, 10}, // fallback behavior
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.calculateOneMinCandlesNeeded(tt.interval, tt.limit)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestAggregateCandles(t *testing.T) {
	service := NewTradeService(&MockTradeStorage{})

	// Create test data: 10 one-minute candles
	startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC).UnixMilli()
	oneMinCandles := []model.Candle{
		{Timestamp: startTime, Open: 100, High: 105, Low: 95, Close: 102, Volume: 1000},
		{Timestamp: startTime + OneMinuteMs, Open: 102, High: 108, Low: 98, Close: 104, Volume: 1500},
		{Timestamp: startTime + 2*OneMinuteMs, Open: 104, High: 110, Low: 100, Close: 106, Volume: 2000},
		{Timestamp: startTime + 3*OneMinuteMs, Open: 106, High: 112, Low: 102, Close: 108, Volume: 1200},
		{Timestamp: startTime + 4*OneMinuteMs, Open: 108, High: 115, Low: 105, Close: 110, Volume: 1800},
		// Second 5-minute candle
		{Timestamp: startTime + 5*OneMinuteMs, Open: 110, High: 118, Low: 108, Close: 112, Volume: 2200},
		{Timestamp: startTime + 6*OneMinuteMs, Open: 112, High: 120, Low: 110, Close: 114, Volume: 1600},
		{Timestamp: startTime + 7*OneMinuteMs, Open: 114, High: 122, Low: 112, Close: 116, Volume: 1900},
		{Timestamp: startTime + 8*OneMinuteMs, Open: 116, High: 125, Low: 114, Close: 118, Volume: 2100},
		{Timestamp: startTime + 9*OneMinuteMs, Open: 118, High: 128, Low: 100, Close: 120, Volume: 2300},
	}

	tests := []struct {
		name           string
		interval       string
		candles        []model.Candle
		expectedCount  int
		validateFirst  bool
		expectedOpen   float64
		expectedHigh   float64
		expectedLow    float64
		expectedClose  float64
		expectedVolume float64
	}{
		{
			name:           "5m aggregation",
			interval:       "5m",
			candles:        oneMinCandles,
			expectedCount:  2,
			validateFirst:  true,
			expectedOpen:   100,  // First candle's open
			expectedHigh:   115,  // Max of first 5 candles
			expectedLow:    95,   // Min of first 5 candles
			expectedClose:  110,  // Last candle's close in the interval
			expectedVolume: 7500, // Sum of first 5 candles
		},
		{
			name:          "empty candles",
			interval:      "5m",
			candles:       []model.Candle{},
			expectedCount: 0,
		},
		{
			name:           "single candle",
			interval:       "5m",
			candles:        oneMinCandles[:1],
			expectedCount:  1,
			validateFirst:  true,
			expectedOpen:   100,
			expectedHigh:   105,
			expectedLow:    95,
			expectedClose:  102,
			expectedVolume: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.aggregateCandles(tt.candles, tt.interval)

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d candles, got %d", tt.expectedCount, len(result))
				return
			}

			if tt.validateFirst && len(result) > 0 {
				first := result[0]
				if first.Open != tt.expectedOpen {
					t.Errorf("Expected open %f, got %f", tt.expectedOpen, first.Open)
				}
				if first.High != tt.expectedHigh {
					t.Errorf("Expected high %f, got %f", tt.expectedHigh, first.High)
				}
				if first.Low != tt.expectedLow {
					t.Errorf("Expected low %f, got %f", tt.expectedLow, first.Low)
				}
				if first.Close != tt.expectedClose {
					t.Errorf("Expected close %f, got %f", tt.expectedClose, first.Close)
				}
				if first.Volume != tt.expectedVolume {
					t.Errorf("Expected volume %f, got %f", tt.expectedVolume, first.Volume)
				}
			}
		})
	}
}

func TestGetIntervalMs(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		expected int64
	}{
		{"1 minute", "1m", OneMinuteMs},
		{"5 minutes", "5m", FiveMinuteMs},
		{"15 minutes", "15m", FifteenMinuteMs},
		{"1 hour", "1h", OneHourMs},
		{"unknown interval", "30m", OneMinuteMs}, // default
		{"empty interval", "", OneMinuteMs},      // default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIntervalMs(tt.interval)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// Integration test for the complete workflow
func TestCompleteWorkflow(t *testing.T) {
	// Setup: Create 2 hours worth of 1-minute candles
	startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC).UnixMilli()
	oneMinCandles := createTestCandles(120, OneMinuteMs, startTime)

	mockStorage := &MockTradeStorage{
		oneMinCandles: oneMinCandles,
	}
	service := NewTradeService(mockStorage)

	// Test 1: Request 5-minute candles
	candles5m, err := service.GetCandles(context.Background(), "BTC-USD", "5m", 10)
	if err != nil {
		t.Fatalf("Error getting 5m candles: %v", err)
	}

	if len(candles5m) != 10 {
		t.Errorf("Expected 10 5m candles, got %d", len(candles5m))
	}

	// Test 2: Request 1-minute candles
	candles1m, err := service.GetCandles(context.Background(), "BTC-USD", "1m", 20)
	if err != nil {
		t.Fatalf("Error getting 1m candles: %v", err)
	}

	if len(candles1m) != 20 {
		t.Errorf("Expected 20 1m candles, got %d", len(candles1m))
	}

	// Test 3: Request hourly candles
	candles1h, err := service.GetCandles(context.Background(), "BTC-USD", "1h", 2)
	if err != nil {
		t.Fatalf("Error getting 1h candles: %v", err)
	}

	if len(candles1h) != 2 {
		t.Errorf("Expected 2 1h candles, got %d", len(candles1h))
	}
}

// Benchmark tests
func BenchmarkGetCandles1m(b *testing.B) {
	startTime := time.Now().UnixMilli()
	oneMinCandles := createTestCandles(1000, OneMinuteMs, startTime)

	mockStorage := &MockTradeStorage{
		oneMinCandles: oneMinCandles,
	}
	service := NewTradeService(mockStorage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GetCandles(context.Background(), "BTC-USD", "1m", 100)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetCandles5m(b *testing.B) {
	startTime := time.Now().UnixMilli()
	oneMinCandles := createTestCandles(1000, OneMinuteMs, startTime)

	mockStorage := &MockTradeStorage{
		oneMinCandles: oneMinCandles,
	}
	service := NewTradeService(mockStorage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GetCandles(context.Background(), "BTC-USD", "5m", 20)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAggregateCandles(b *testing.B) {
	startTime := time.Now().UnixMilli()
	oneMinCandles := createTestCandles(300, OneMinuteMs, startTime)

	service := NewTradeService(&MockTradeStorage{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.aggregateCandles(oneMinCandles, "5m")
	}
}
