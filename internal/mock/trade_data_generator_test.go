package mock

import (
	"CandleService/internal/model"
	"context"
	"sort"
	"testing"
	"time"
)

func TestDefaultGeneratorConfig(t *testing.T) {
	config := DefaultGeneratorConfig()

	// Test symbols
	expectedSymbols := []string{"BTC-PERP", "ETH-PERP", "SOL-PERP"}
	if len(config.Symbols) != len(expectedSymbols) {
		t.Errorf("Expected %d symbols, got %d", len(expectedSymbols), len(config.Symbols))
	}
	for i, symbol := range expectedSymbols {
		if config.Symbols[i] != symbol {
			t.Errorf("Expected symbol %s at index %d, got %s", symbol, i, config.Symbols[i])
		}
	}

	// Test base prices
	expectedPrices := map[string]float64{
		"BTC-PERP": 50000.0,
		"ETH-PERP": 3000.0,
		"SOL-PERP": 100.0,
	}
	for symbol, expectedPrice := range expectedPrices {
		if price, exists := config.BasePrices[symbol]; !exists {
			t.Errorf("Expected base price for %s to exist", symbol)
		} else if price != expectedPrice {
			t.Errorf("Expected base price for %s to be %f, got %f", symbol, expectedPrice, price)
		}
	}

	// Test other config values
	if config.Interval != 2*time.Second {
		t.Errorf("Expected interval to be 2s, got %v", config.Interval)
	}
	if config.Volatility != 0.01 {
		t.Errorf("Expected volatility to be 0.01, got %f", config.Volatility)
	}
	if config.HistoryHours != 1 {
		t.Errorf("Expected history hours to be 1, got %d", config.HistoryHours)
	}
}

func TestNewTradeDataGenerator(t *testing.T) {
	tradeChan := make(chan model.Trade, 100)
	generator := NewTradeDataGenerator(tradeChan)

	// Test that generator is created with default config
	if generator == nil {
		t.Fatal("Expected generator to be non-nil")
	}
	if generator.tradeChan == nil {
		t.Error("Expected tradeChan to be set")
	}
	if generator.tradeID != 1 {
		t.Errorf("Expected initial tradeID to be 1, got %d", generator.tradeID)
	}
	if generator.rng == nil {
		t.Error("Expected rng to be initialized")
	}

	// Test base prices are copied correctly
	defaultConfig := DefaultGeneratorConfig()
	for symbol, expectedPrice := range defaultConfig.BasePrices {
		if price, exists := generator.basePrice[symbol]; !exists {
			t.Errorf("Expected base price for %s to exist", symbol)
		} else if price != expectedPrice {
			t.Errorf("Expected base price for %s to be %f, got %f", symbol, expectedPrice, price)
		}
	}
}

func TestNewTradeDataGeneratorWithConfig(t *testing.T) {
	tradeChan := make(chan model.Trade, 100)
	customConfig := GeneratorConfig{
		Symbols:      []string{"TEST-PERP"},
		BasePrices:   map[string]float64{"TEST-PERP": 1000.0},
		Interval:     5 * time.Second,
		Volatility:   0.02,
		HistoryHours: 2,
	}

	generator := NewTradeDataGeneratorWithConfig(tradeChan, customConfig)

	if generator == nil {
		t.Fatal("Expected generator to be non-nil")
	}
	if len(generator.config.Symbols) != 1 || generator.config.Symbols[0] != "TEST-PERP" {
		t.Errorf("Expected symbols to be [TEST-PERP], got %v", generator.config.Symbols)
	}
	if generator.config.Interval != 5*time.Second {
		t.Errorf("Expected interval to be 5s, got %v", generator.config.Interval)
	}
	if generator.config.Volatility != 0.02 {
		t.Errorf("Expected volatility to be 0.02, got %f", generator.config.Volatility)
	}
	if generator.config.HistoryHours != 2 {
		t.Errorf("Expected history hours to be 2, got %d", generator.config.HistoryHours)
	}

	// Test that base prices are copied (not referenced)
	customConfig.BasePrices["TEST-PERP"] = 2000.0 // Modify original
	if generator.basePrice["TEST-PERP"] != 1000.0 {
		t.Error("Expected base prices to be copied, not referenced")
	}
}

func TestGenerateRandomTrade(t *testing.T) {
	tradeChan := make(chan model.Trade, 100)
	generator := NewTradeDataGenerator(tradeChan)
	symbol := "BTC-PERP"
	price := 50000.0
	timestamp := time.Now().UnixMilli()

	trade := generator.generateRandomTrade(symbol, price, timestamp)

	// Test trade fields
	if trade.Symbol != symbol {
		t.Errorf("Expected symbol to be %s, got %s", symbol, trade.Symbol)
	}
	if trade.Timestamp != timestamp {
		t.Errorf("Expected timestamp to be %d, got %d", timestamp, trade.Timestamp)
	}
	if trade.Price <= 0 {
		t.Error("Expected price to be positive")
	}
	if trade.Size < 0.1 || trade.Size > 0.6 {
		t.Errorf("Expected size to be between 0.1 and 0.6, got %f", trade.Size)
	}
	if trade.TradeID == "" {
		t.Error("Expected trade ID to be non-empty")
	}

	// Test price variation is reasonable (within 10% of base price for normal volatility)
	if trade.Price < price*0.9 || trade.Price > price*1.1 {
		t.Logf("Warning: Price variation seems large. Base: %f, Generated: %f", price, trade.Price)
	}
}

func TestGenerateRandomTradeNegativePriceProtection(t *testing.T) {
	tradeChan := make(chan model.Trade, 100)
	config := GeneratorConfig{
		Symbols:      []string{"TEST-PERP"},
		BasePrices:   map[string]float64{"TEST-PERP": 1.0},
		Volatility:   10.0, // Very high volatility to potentially cause negative prices
		Interval:     time.Second,
		HistoryHours: 1,
	}
	generator := NewTradeDataGeneratorWithConfig(tradeChan, config)

	// Generate many trades to test negative price protection
	for i := 0; i < 100; i++ {
		trade := generator.generateRandomTrade("TEST-PERP", 1.0, time.Now().UnixMilli())
		if trade.Price <= 0 {
			t.Errorf("Generated negative or zero price: %f", trade.Price)
		}
	}
}

func TestGenerateHistoricalData(t *testing.T) {
	tradeChan := make(chan model.Trade, 1000)
	config := GeneratorConfig{
		Symbols:      []string{"BTC-PERP"},
		BasePrices:   map[string]float64{"BTC-PERP": 50000.0},
		Interval:     time.Second,
		Volatility:   0.01,
		HistoryHours: 1, // 1 hour = 60 minutes
	}
	generator := NewTradeDataGeneratorWithConfig(tradeChan, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Generate historical data
	generator.generateHistoricalData(ctx)

	// Close channel to count trades
	close(tradeChan)

	// Count trades
	tradeCount := 0
	var trades []model.Trade
	for trade := range tradeChan {
		trades = append(trades, trade)
		tradeCount++
	}

	// Should generate trades for 60 minutes, with 1-10 trades per minute
	if tradeCount < 60 || tradeCount > 600 {
		t.Errorf("Expected between 60 and 600 trades for 1 hour, got %d", tradeCount)
	}

	// Test that trades are in chronological order
	if !sort.SliceIsSorted(trades, func(i, j int) bool {
		return trades[i].Timestamp < trades[j].Timestamp
	}) {
		t.Error("Expected trades to be in chronological order")
	}

	// Test that all trades have the correct symbol
	for _, trade := range trades {
		if trade.Symbol != "BTC-PERP" {
			t.Errorf("Expected all trades to have symbol BTC-PERP, got %s", trade.Symbol)
		}
	}

	// Test that timestamps are distributed across the hour
	now := time.Now().UnixMilli()
	hourAgo := now - 60*60*1000
	for _, trade := range trades {
		if trade.Timestamp < hourAgo || trade.Timestamp > now {
			t.Errorf("Trade timestamp %d is outside expected range [%d, %d]",
				trade.Timestamp, hourAgo, now)
		}
	}
}

func TestGenerateHistoricalDataWithCancellation(t *testing.T) {
	tradeChan := make(chan model.Trade, 1000)
	config := GeneratorConfig{
		Symbols:      []string{"BTC-PERP"},
		BasePrices:   map[string]float64{"BTC-PERP": 50000.0},
		Interval:     time.Second,
		Volatility:   0.01,
		HistoryHours: 24, // Large amount to ensure cancellation works
	}
	generator := NewTradeDataGeneratorWithConfig(tradeChan, config)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Generate historical data with quick cancellation
	generator.generateHistoricalData(ctx)

	// Should exit quickly due to context cancellation
	close(tradeChan)
	tradeCount := 0
	for range tradeChan {
		tradeCount++
	}

	// Should have generated fewer trades due to cancellation
	if tradeCount > 1000 {
		t.Errorf("Expected cancellation to limit trades, but got %d", tradeCount)
	}
}

func TestGenerateRealTimeData(t *testing.T) {
	tradeChan := make(chan model.Trade, 100)
	config := GeneratorConfig{
		Symbols:      []string{"BTC-PERP", "ETH-PERP"},
		BasePrices:   map[string]float64{"BTC-PERP": 50000.0, "ETH-PERP": 3000.0},
		Interval:     50 * time.Millisecond, // Fast interval for testing
		Volatility:   0.01,
		HistoryHours: 0, // No historical data
	}
	generator := NewTradeDataGeneratorWithConfig(tradeChan, config)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start real-time generation
	go generator.generateRealTimeData(ctx)

	// Wait for some trades to be generated
	time.Sleep(150 * time.Millisecond)
	cancel()

	// Close channel and count trades
	close(tradeChan)
	tradeCount := 0
	symbolCounts := make(map[string]int)
	for trade := range tradeChan {
		tradeCount++
		symbolCounts[trade.Symbol]++
	}

	// Should have generated trades for both symbols
	if tradeCount == 0 {
		t.Error("Expected some trades to be generated")
	}
	if symbolCounts["BTC-PERP"] == 0 {
		t.Error("Expected BTC-PERP trades to be generated")
	}
	if symbolCounts["ETH-PERP"] == 0 {
		t.Error("Expected ETH-PERP trades to be generated")
	}
}

func TestStart(t *testing.T) {
	tradeChan := make(chan model.Trade, 100)
	config := GeneratorConfig{
		Symbols:      []string{"BTC-PERP"},
		BasePrices:   map[string]float64{"BTC-PERP": 50000.0},
		Interval:     100 * time.Millisecond,
		Volatility:   0.01,
		HistoryHours: 0, // No historical data for simpler testing
	}
	generator := NewTradeDataGeneratorWithConfig(tradeChan, config)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	// Start the generator
	generator.Start(ctx)

	// Wait for some trades
	time.Sleep(250 * time.Millisecond)
	cancel()

	// Close channel and count trades
	close(tradeChan)
	tradeCount := 0
	for range tradeChan {
		tradeCount++
	}

	if tradeCount == 0 {
		t.Error("Expected Start() to generate trades")
	}
}

func TestTradeIDIncrement(t *testing.T) {
	tradeChan := make(chan model.Trade, 100)
	generator := NewTradeDataGenerator(tradeChan)

	// Generate several trades and check ID increment
	trade1 := generator.generateRandomTrade("BTC-PERP", 50000.0, time.Now().UnixMilli())
	generator.tradeID++
	trade2 := generator.generateRandomTrade("BTC-PERP", 50000.0, time.Now().UnixMilli())
	generator.tradeID++
	trade3 := generator.generateRandomTrade("BTC-PERP", 50000.0, time.Now().UnixMilli())

	if trade1.TradeID == trade2.TradeID || trade2.TradeID == trade3.TradeID {
		t.Error("Expected trade IDs to be unique")
	}

	// Check that IDs are sequential
	if trade1.TradeID != "t1" {
		t.Errorf("Expected first trade ID to be t1, got %s", trade1.TradeID)
	}
	if trade2.TradeID != "t2" {
		t.Errorf("Expected second trade ID to be t2, got %s", trade2.TradeID)
	}
	if trade3.TradeID != "t3" {
		t.Errorf("Expected third trade ID to be t3, got %s", trade3.TradeID)
	}
}

func TestTimestampDistributionWithinMinute(t *testing.T) {
	tradeChan := make(chan model.Trade, 1000)
	config := GeneratorConfig{
		Symbols:      []string{"BTC-PERP"},
		BasePrices:   map[string]float64{"BTC-PERP": 50000.0},
		Interval:     time.Second,
		Volatility:   0.01,
		HistoryHours: 1,
	}
	generator := NewTradeDataGeneratorWithConfig(tradeChan, config)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Generate historical data
	generator.generateHistoricalData(ctx)
	close(tradeChan)

	// Group trades by minute and check timestamp distribution
	minuteGroups := make(map[int64][]int64)
	for trade := range tradeChan {
		minute := trade.Timestamp / (60 * 1000)          // Convert to minute bucket
		second := (trade.Timestamp % (60 * 1000)) / 1000 // Get second within minute
		minuteGroups[minute] = append(minuteGroups[minute], second)
	}

	// Check that trades within each minute are sorted
	for minute, seconds := range minuteGroups {
		if !sort.SliceIsSorted(seconds, func(i, j int) bool {
			return seconds[i] < seconds[j]
		}) {
			t.Errorf("Trades within minute %d are not sorted by timestamp", minute)
		}

		// Check that seconds are within valid range (0-59)
		for _, second := range seconds {
			if second < 0 || second >= 60 {
				t.Errorf("Invalid second value %d in minute %d", second, minute)
			}
		}
	}
}

func TestBasePriceUpdate(t *testing.T) {
	tradeChan := make(chan model.Trade, 100)
	generator := NewTradeDataGenerator(tradeChan)
	symbol := "BTC-PERP"
	originalPrice := generator.basePrice[symbol]

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Generate some historical data to update base price
	generator.generateHistoricalData(ctx)

	// Base price should potentially be updated
	newPrice := generator.basePrice[symbol]
	if newPrice == originalPrice {
		t.Log("Base price didn't change (this is possible but worth noting)")
	}

	// Price should still be reasonable
	if newPrice <= 0 {
		t.Errorf("Base price became invalid: %f", newPrice)
	}
}

func TestEmptySymbolsList(t *testing.T) {
	tradeChan := make(chan model.Trade, 100)
	config := GeneratorConfig{
		Symbols:      []string{}, // Empty symbols list
		BasePrices:   map[string]float64{},
		Interval:     time.Second,
		Volatility:   0.01,
		HistoryHours: 1,
	}
	generator := NewTradeDataGeneratorWithConfig(tradeChan, config)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should not panic or generate any trades
	generator.generateHistoricalData(ctx)
	close(tradeChan)

	tradeCount := 0
	for range tradeChan {
		tradeCount++
	}

	if tradeCount != 0 {
		t.Errorf("Expected no trades with empty symbols list, got %d", tradeCount)
	}
}
