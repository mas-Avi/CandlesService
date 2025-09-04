package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// Trade represents a single trade
type Trade struct {
	TradeID   string  `json:"trade_id"`
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Size      float64 `json:"size"`
	Timestamp int64   `json:"timestamp"`
}

// Candle represents OHLCV data for a time interval
type Candle struct {
	Timestamp int64   `json:"timestamp"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

// TradeService manages trade data and candle generation
type TradeService struct {
	trades map[string][]Trade // symbol -> trades
	mu     sync.RWMutex
}

// NewTradeService creates a new trade service
func NewTradeService() *TradeService {
	return &TradeService{
		trades: make(map[string][]Trade),
	}
}

// AddTrade adds a new trade to the service
func (ts *TradeService) AddTrade(trade Trade) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.trades[trade.Symbol] = append(ts.trades[trade.Symbol], trade)

	// Keep trades sorted by timestamp and limit to reasonable size
	trades := ts.trades[trade.Symbol]
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].Timestamp < trades[j].Timestamp
	})

	// Keep only last 1000 trades per symbol to prevent memory issues
	if len(trades) > 1000 {
		ts.trades[trade.Symbol] = trades[len(trades)-1000:]
	} else {
		ts.trades[trade.Symbol] = trades
	}
}

// GetTrades returns the last 50 trades for a symbol
func (ts *TradeService) GetTrades(symbol string) []Trade {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	trades, exists := ts.trades[symbol]
	if !exists {
		return []Trade{}
	}

	// Return last 50 trades
	start := 0
	if len(trades) > 50 {
		start = len(trades) - 50
	}

	result := make([]Trade, len(trades)-start)
	copy(result, trades[start:])
	return result
}

// GetCandles generates candles for a symbol and interval
func (ts *TradeService) GetCandles(symbol string, interval string) []Candle {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	trades, exists := ts.trades[symbol]
	if !exists {
		return []Candle{}
	}

	intervalMs := getIntervalMs(interval)
	if intervalMs == 0 {
		return []Candle{}
	}

	return ts.generateCandles(trades, intervalMs)
}

// getIntervalMs converts interval string to milliseconds
func getIntervalMs(interval string) int64 {
	switch interval {
	case "1m":
		return 60 * 1000
	case "5m":
		return 5 * 60 * 1000
	case "15m":
		return 15 * 60 * 1000
	case "1h":
		return 60 * 60 * 1000
	default:
		return 60 * 1000 // default to 1m
	}
}

// generateCandles creates OHLCV candles from trades
func (ts *TradeService) generateCandles(trades []Trade, intervalMs int64) []Candle {
	if len(trades) == 0 {
		return []Candle{}
	}

	candleMap := make(map[int64]*Candle)

	for _, trade := range trades {
		// Round timestamp down to interval boundary
		candleTime := (trade.Timestamp / intervalMs) * intervalMs

		candle, exists := candleMap[candleTime]
		if !exists {
			candle = &Candle{
				Timestamp: candleTime,
				Open:      trade.Price,
				High:      trade.Price,
				Low:       trade.Price,
				Close:     trade.Price,
				Volume:    trade.Size,
			}
			candleMap[candleTime] = candle
		} else {
			// Update OHLCV
			if trade.Price > candle.High {
				candle.High = trade.Price
			}
			if trade.Price < candle.Low {
				candle.Low = trade.Price
			}
			candle.Close = trade.Price // Last trade becomes close
			candle.Volume += trade.Size
		}
	}

	// Convert map to sorted slice
	var candles []Candle
	for _, candle := range candleMap {
		candles = append(candles, *candle)
	}

	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Timestamp < candles[j].Timestamp
	})

	return candles
}

// HTTP Handlers
func (ts *TradeService) handleGetTrades(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "symbol parameter is required", http.StatusBadRequest)
		return
	}

	trades := ts.GetTrades(symbol)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trades)
}

func (ts *TradeService) handleGetCandles(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "symbol parameter is required", http.StatusBadRequest)
		return
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1m"
	}

	candles := ts.GetCandles(symbol, interval)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(candles)
}

// simulateTradeData generates mock trade data for testing
func (ts *TradeService) simulateTradeData() {
	symbols := []string{"BTC-PERP", "ETH-PERP", "SOL-PERP"}
	basePrice := map[string]float64{
		"BTC-PERP": 50000.0,
		"ETH-PERP": 3000.0,
		"SOL-PERP": 100.0,
	}

	tradeID := 1

	// Generate initial historical data
	now := time.Now().UnixMilli()
	for _, symbol := range symbols {
		price := basePrice[symbol]

		// Generate trades for the last hour
		for i := 0; i < 60; i++ {
			timestamp := now - int64(60-i)*60*1000 // Every minute

			// Generate 1-3 trades per minute
			numTrades := 1 + tradeID%3
			for j := 0; j < numTrades; j++ {
				// Add some price variation
				priceVariation := (float64(tradeID%100) - 50) * price * 0.001
				tradePrice := price + priceVariation

				trade := Trade{
					TradeID:   fmt.Sprintf("t%d", tradeID),
					Symbol:    symbol,
					Price:     tradePrice,
					Size:      0.1 + float64(tradeID%10)*0.01,
					Timestamp: timestamp + int64(j*1000), // Spread trades within the minute
				}

				ts.AddTrade(trade)
				tradeID++
			}
		}
	}

	// Continue generating real-time data
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			for _, symbol := range symbols {
				price := basePrice[symbol]

				// Add some price variation
				priceVariation := (float64(tradeID%100) - 50) * price * 0.001
				tradePrice := price + priceVariation

				trade := Trade{
					TradeID:   fmt.Sprintf("t%d", tradeID),
					Symbol:    symbol,
					Price:     tradePrice,
					Size:      0.1 + float64(tradeID%10)*0.01,
					Timestamp: time.Now().UnixMilli(),
				}

				ts.AddTrade(trade)
				tradeID++

				// Update base price slightly for trending
				basePrice[symbol] = tradePrice
			}
		}
	}()
}

func main() {
	// Create trade service
	ts := NewTradeService()

	// Start simulating trade data
	ts.simulateTradeData()

	// Setup routes
	r := mux.NewRouter()
	r.HandleFunc("/trades", ts.handleGetTrades).Methods("GET")
	r.HandleFunc("/candles", ts.handleGetCandles).Methods("GET")

	// Health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Start server
	port := 8080
	fmt.Printf("Trade service starting on port %d\n", port)
	fmt.Printf("Endpoints:\n")
	fmt.Printf("  GET /trades?symbol=BTC-PERP\n")
	fmt.Printf("  GET /candles?symbol=BTC-PERP&interval=1m\n")
	fmt.Printf("  GET /health\n")

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), r))
}
