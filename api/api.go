package api

import (
	"CandleService/internal/model"
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// This file serves as the main entry point for the API package. It defines the APIHandler struct and its dependencies.
// The actual implementation of the HTTP handlers, routing, server management, middleware, and validation are organized into separate files for better maintainability.
// The package structure is as follows:
// - api.go: Main API handler and dependencies (this file)
// - handler.go: HTTP request handlers
// - middleware.go: Middleware functions
// - validator.go: Request validation

// Constants
const (
	DefaultTimeout      = 30 * time.Second
	DefaultInterval     = "1m"
	ServiceVersion      = "1.0.0"
	ServiceName         = "trade-candle-service"
	RequestIDContextKey = "request_id"
	RequestIDHeaderKey  = "X-Request-ID"
)

// TradeService is an interface defining methods to get trades and candles
type TradeService interface {
	GetTrades(ctx context.Context, symbol string) ([]model.Trade, error)
	GetCandles(ctx context.Context, symbol, interval string, limit int64) ([]model.Candle, error)
}

// APIHandler handles HTTP requests using Gin framework
type APIHandler struct {
	tradeService TradeService
	validator    *Validator
	logger       *slog.Logger
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(tradeService TradeService, logger *slog.Logger) *APIHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &APIHandler{
		tradeService: tradeService,
		validator:    GetValidator(),
		logger:       logger,
	}
}

// StartServer starts the HTTP server
func (h *APIHandler) StartServer(port int) error {
	router := h.SetupRoutes()
	return router.Run(":" + strconv.Itoa(port))
}

// SetupRoutes configures all API routes
func (h *APIHandler) SetupRoutes() *gin.Engine {
	// Set Gin to release mode for production
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Add middleware
	router.Use(requestIDMiddleware())
	router.Use(ginLoggerMiddleware())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	// API routes
	router.GET("/trades", h.GetTrades)
	router.GET("/candles", h.GetCandles)
	router.GET("/health", h.HealthCheck)

	return router
}
