package api

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Validator handles validation logic separate from HTTP concerns
type Validator struct {
	supportedIntervals map[string]bool
	symbolRegex        *regexp.Regexp
}

var (
	validatorInstance *Validator
	validatorOnce     sync.Once
)

// GetValidator returns the singleton validator instance
func GetValidator() *Validator {
	validatorOnce.Do(func() {
		validatorInstance = &Validator{
			supportedIntervals: map[string]bool{
				"1m":  true,
				"5m":  true,
				"15m": true,
				"1h":  true,
			},
			// Symbol regex:
			symbolRegex: regexp.MustCompile(`^[a-zA-Z]{3,5}-[a-zA-Z]{3,5}$`),
		}
	})
	return validatorInstance
}

// ValidateTradesRequest validates and sanitizes the symbol for trades
func (v *Validator) ValidateTradesRequest(symbol string) (string, error) {
	cleanSymbol := v.sanitizeInput(symbol)
	if err := v.validateSymbol(cleanSymbol); err != nil {
		return "", err
	}
	return cleanSymbol, nil
}

// ValidateCandlesRequest validates and sanitizes the symbol, interval, and limit for candles
func (v *Validator) ValidateCandlesRequest(symbol, interval, limitStr string) (string, string, int64, error) {
	cleanSymbol := v.sanitizeInput(symbol)
	if err := v.validateSymbol(cleanSymbol); err != nil {
		return "", "", 0, err
	}
	cleanInterval := v.sanitizeInput(interval)
	if cleanInterval == "" {
		cleanInterval = "1m"
	}
	if err := v.validateInterval(cleanInterval); err != nil {
		return "", "", 0, err
	}

	limit, err := v.validateLimit(limitStr)
	if err != nil {
		return "", "", 0, err
	}

	return cleanSymbol, cleanInterval, limit, nil
}

// sanitizeInput removes potentially dangerous characters and trims whitespace
func (v *Validator) sanitizeInput(input string) string {
	// Trim whitespace
	input = strings.TrimSpace(input)

	// Remove null bytes and control characters
	input = strings.ReplaceAll(input, "\x00", "")
	input = strings.Map(func(r rune) rune {
		// Keep printable ASCII and common symbols, remove control chars
		if r < 32 && r != 9 && r != 10 && r != 13 { // Keep tab, LF, CR
			return -1 // Remove character
		}
		return r
	}, input)

	// Limit length to prevent DoS
	if len(input) > 100 {
		input = input[:100]
	}

	return input
}

// validateSymbol validates a trading symbol with input sanitization
func (v *Validator) validateSymbol(symbol string) error {
	// Sanitize input first
	symbol = v.sanitizeInput(symbol)

	if symbol == "" {
		return errors.New("symbol parameter is required")
	}

	// Validate format using regex
	if !v.symbolRegex.MatchString(symbol) {
		return errors.New("symbol must be 3-20 characters and contain only letters, numbers, hyphens, or underscores")
	}

	return nil
}

// validateInterval validates a time interval with input sanitization
func (v *Validator) validateInterval(interval string) error {
	// Sanitize input first
	interval = v.sanitizeInput(interval)

	if interval == "" {
		return errors.New("interval cannot be empty")
	}

	if !v.supportedIntervals[interval] {
		return fmt.Errorf("invalid interval '%s'. Supported values: 1m, 5m, 15m, 1h", interval)
	}

	return nil
}

// validateLimit validates the limit parameter for candle requests
func (v *Validator) validateLimit(limitStr string) (int64, error) {
	// If limit is not provided, return 0 (no limit)
	if limitStr == "" {
		return 0, nil
	}

	// Sanitize input first
	limitStr = v.sanitizeInput(limitStr)

	// Convert to int64
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		return 0, errors.New("limit must be a valid number")
	}

	// Validate range - allow 0 for no limit
	if limit < 0 || limit > 1000 {
		return 0, errors.New("limit must be between 0 and 1000 (0 means no limit)")
	}

	return limit, nil
}
