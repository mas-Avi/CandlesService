package model

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
