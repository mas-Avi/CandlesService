package mock

import (
	"CandleService/internal/model"
	"context"
	"fmt"
	"math/rand"
	"sort"
	"time"
)

// GeneratorConfig holds configuration for the trade data generator
type GeneratorConfig struct {
	Symbols      []string
	BasePrices   map[string]float64
	Interval     time.Duration
	Volatility   float64
	HistoryHours int
}

// DefaultGeneratorConfig returns a sensible default configuration
func DefaultGeneratorConfig() GeneratorConfig {
	return GeneratorConfig{
		Symbols: []string{"BTC-PERP", "ETH-PERP", "SOL-PERP"},
		BasePrices: map[string]float64{
			"BTC-PERP": 50000.0,
			"ETH-PERP": 3000.0,
			"SOL-PERP": 100.0,
		},
		Interval:     2 * time.Second,
		Volatility:   0.01, // 1% volatility
		HistoryHours: 1,
	}
}

// TradeDataGenerator generates mock trade data and sends it to a channel
type TradeDataGenerator struct {
	tradeChan chan<- model.Trade
	config    GeneratorConfig
	basePrice map[string]float64
	tradeID   int
	rng       *rand.Rand
}

// NewTradeDataGenerator creates a new trade data generator with default config
func NewTradeDataGenerator(tradeChan chan<- model.Trade) *TradeDataGenerator {
	return NewTradeDataGeneratorWithConfig(tradeChan, DefaultGeneratorConfig())
}

// NewTradeDataGeneratorWithConfig creates a new trade data generator with custom config
func NewTradeDataGeneratorWithConfig(tradeChan chan<- model.Trade, config GeneratorConfig) *TradeDataGenerator {
	// Create a copy of base prices for modification
	basePrice := make(map[string]float64)
	for k, v := range config.BasePrices {
		basePrice[k] = v
	}

	return &TradeDataGenerator{
		tradeChan: tradeChan,
		config:    config,
		basePrice: basePrice,
		tradeID:   1,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Start begins generating trade data using the provided context
func (tdg *TradeDataGenerator) Start(ctx context.Context) {
	// Generate initial historical data
	tdg.generateHistoricalData(ctx)

	// Start real-time data generation
	go tdg.generateRealTimeData(ctx)
}

// generateHistoricalData creates historical trade data
func (tdg *TradeDataGenerator) generateHistoricalData(ctx context.Context) {
	now := time.Now().UnixMilli()
	minutesToGenerate := tdg.config.HistoryHours * 60

	for _, symbol := range tdg.config.Symbols {
		price := tdg.basePrice[symbol]

		// Generate trades for the configured history period
		for i := 0; i < minutesToGenerate; i++ {
			minuteTimestamp := now - int64(minutesToGenerate-i)*60*1000

			// Generate 1-10 trades per minute
			numTrades := 1 + tdg.rng.Intn(10)

			// Generate random seconds within the minute for these trades
			tradeSeconds := make([]int, numTrades)
			for k := 0; k < numTrades; k++ {
				tradeSeconds[k] = tdg.rng.Intn(60) // 0-59 seconds
			}
			// Sort the seconds to maintain chronological order within the minute
			sort.Ints(tradeSeconds)

			for j := 0; j < numTrades; j++ {
				// Check if context is cancelled
				select {
				case <-ctx.Done():
					return
				default:
					// Continue processing
				}

				// Set the timestamp to the specific second within the minute
				timestamp := minuteTimestamp + int64(tradeSeconds[j]*1000)

				trade := tdg.generateRandomTrade(symbol, price, timestamp)

				select {
				case tdg.tradeChan <- trade:
					// Trade sent successfully
				case <-ctx.Done():
					return
				}

				tdg.tradeID++
				// Update price for next iteration (creates trending)
				price = trade.Price
			}
		}

		// Update base price after generating history
		tdg.basePrice[symbol] = price
	}
}

// generateRealTimeData continuously generates new trade data
func (tdg *TradeDataGenerator) generateRealTimeData(ctx context.Context) {
	ticker := time.NewTicker(tdg.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, symbol := range tdg.config.Symbols {
				price := tdg.basePrice[symbol]

				trade := tdg.generateRandomTrade(symbol, price, time.Now().UnixMilli())

				select {
				case tdg.tradeChan <- trade:
					// Trade sent successfully
					// blocked until sent, so tradeChan should be large enough to handle bursts
				case <-ctx.Done():
					return
				}

				tdg.tradeID++

				// Update base price for trending behavior
				tdg.basePrice[symbol] = trade.Price
			}
		case <-ctx.Done():
			return
		}
	}
}

func (tdg *TradeDataGenerator) generateRandomTrade(symbol string, price float64, timestamp int64) model.Trade {
	// More realistic price variation using normal distribution
	priceVariation := tdg.rng.NormFloat64() * tdg.config.Volatility * price
	tradePrice := price + priceVariation

	// Ensure price doesn't go negative
	if tradePrice <= 0 {
		tradePrice = price * 0.99
	}

	trade := model.Trade{
		TradeID:   fmt.Sprintf("t%d", tdg.tradeID),
		Symbol:    symbol,
		Price:     tradePrice,
		Size:      0.1 + tdg.rng.Float64()*0.5, // Random size between 0.1 and 0.6
		Timestamp: timestamp,
	}
	return trade
}
