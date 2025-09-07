package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GetTrades handles GET /trades requests
func (h *APIHandler) GetTrades(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), DefaultTimeout)
	defer cancel()

	symbol := c.Query("symbol")

	cleanSymbol, err := h.validator.ValidateTradesRequest(symbol)
	if err != nil {
		h.handleValidationError(c, err)
		return
	}

	trades, err := h.tradeService.GetTrades(ctx, cleanSymbol)
	if err != nil {
		h.handleError(c, err, http.StatusInternalServerError, "Internal server error")
		return
	}

	c.JSON(http.StatusOK, trades)
}

// GetCandles handles GET /candles requests
func (h *APIHandler) GetCandles(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), DefaultTimeout)
	defer cancel()

	symbol := c.Query("symbol")
	interval := c.DefaultQuery("interval", DefaultInterval)
	limit := c.Query("limit")

	cleanSymbol, cleanInterval, validLimit, err := h.validator.ValidateCandlesRequest(symbol, interval, limit)
	if err != nil {
		h.handleValidationError(c, err)
		return
	}

	candles, err := h.tradeService.GetCandles(ctx, cleanSymbol, cleanInterval, validLimit)
	if err != nil {
		h.handleError(c, err, http.StatusInternalServerError, "Internal server error")
		return
	}

	c.JSON(http.StatusOK, candles)
}

// HealthCheck handles GET /health requests
func (h *APIHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "OK",
		"service":   ServiceName,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   ServiceVersion,
	})
}

// handleError logs the error and sends appropriate HTTP response
func (h *APIHandler) handleError(c *gin.Context, err error, statusCode int, userMessage string) {
	requestID, exists := c.Get(RequestIDContextKey)
	requestIDStr := "unknown"
	if exists {
		if id, ok := requestID.(string); ok {
			requestIDStr = id
		}
	}

	h.logger.Error("API error",
		slog.String("request_id", requestIDStr),
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.String("error", err.Error()),
		slog.Int("status_code", statusCode),
	)

	c.JSON(statusCode, gin.H{
		"error":      userMessage,
		"request_id": requestIDStr,
	})
}

// handleValidationError handles validation errors specifically
func (h *APIHandler) handleValidationError(c *gin.Context, err error) {
	h.handleError(c, err, http.StatusBadRequest, err.Error())
}
