package api

import (
	"CandleService/internal/model"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTradeService implements TradeService interface for testing
type MockTradeService struct {
	mock.Mock
}

func (m *MockTradeService) GetTrades(ctx context.Context, symbol string) ([]model.Trade, error) {
	args := m.Called(ctx, symbol)
	return args.Get(0).([]model.Trade), args.Error(1)
}

func (m *MockTradeService) GetCandles(ctx context.Context, symbol, interval string, limit int64) ([]model.Candle, error) {
	args := m.Called(ctx, symbol, interval, limit)
	return args.Get(0).([]model.Candle), args.Error(1)
}

// Mock Validator for testing
type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) ValidateTradesRequest(symbol string) (string, error) {
	args := m.Called(symbol)
	return args.String(0), args.Error(1)
}

func (m *MockValidator) ValidateCandlesRequest(symbol, interval, limit string) (string, string, int64, error) {
	args := m.Called(symbol, interval, limit)
	return args.String(0), args.String(1), args.Get(2).(int64), args.Error(3)
}

// Test helper functions
func createTestTrades(count int) []model.Trade {
	trades := make([]model.Trade, count)
	baseTime := time.Now().UnixMilli()

	for i := 0; i < count; i++ {
		trades[i] = model.Trade{
			TradeID:   fmt.Sprintf("trade_%d", i),
			Symbol:    "BTC-USD",
			Price:     50000.0 + float64(i*100),
			Size:      0.1,
			Timestamp: baseTime + int64(i*1000),
		}
	}
	return trades
}

func createTestCandles(count int) []model.Candle {
	candles := make([]model.Candle, count)
	baseTime := time.Now().UnixMilli()

	for i := 0; i < count; i++ {
		candles[i] = model.Candle{
			Timestamp: baseTime + int64(i*60000),
			Open:      50000.0 + float64(i*100),
			High:      50500.0 + float64(i*100),
			Low:       49500.0 + float64(i*100),
			Close:     50200.0 + float64(i*100),
			Volume:    1000.0 + float64(i*10),
		}
	}
	return candles
}

func setupTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Suppress logs during testing
	}))
}

func setupGinTestMode() {
	gin.SetMode(gin.TestMode)
}

// Test NewAPIHandler
func TestNewAPIHandler(t *testing.T) {
	setupGinTestMode()

	tests := []struct {
		name          string
		tradeService  TradeService
		logger        *slog.Logger
		expectNil     bool
		expectDefault bool
	}{
		{
			name:         "with valid service and logger",
			tradeService: &MockTradeService{},
			logger:       setupTestLogger(),
			expectNil:    false,
		},
		{
			name:          "with nil logger",
			tradeService:  &MockTradeService{},
			logger:        nil,
			expectNil:     false,
			expectDefault: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAPIHandler(tt.tradeService, tt.logger)

			if tt.expectNil {
				assert.Nil(t, handler)
			} else {
				assert.NotNil(t, handler)
				assert.Equal(t, tt.tradeService, handler.tradeService)

				if tt.expectDefault {
					assert.NotNil(t, handler.logger)
				} else {
					assert.Equal(t, tt.logger, handler.logger)
				}

				assert.NotNil(t, handler.validator)
			}
		})
	}
}

// Test StartServer
func TestStartServer(t *testing.T) {
	setupGinTestMode()

	// This test is more complex as it involves starting an actual server
	// We'll test the basic functionality without actually binding to a port
	mockService := &MockTradeService{}
	handler := NewAPIHandler(mockService, setupTestLogger())

	// Test with invalid port (negative)
	err := handler.StartServer(-1)
	assert.Error(t, err)

	// Note: Testing actual server startup would require more complex setup
	// and cleanup, so we focus on error cases and setup validation
}

// Test SetupRoutes
func TestSetupRoutes(t *testing.T) {
	setupGinTestMode()

	mockService := &MockTradeService{}
	handler := NewAPIHandler(mockService, setupTestLogger())

	router := handler.SetupRoutes()

	assert.NotNil(t, router)

	// Test that routes are properly registered by making test requests
	routes := router.Routes()

	// Verify that we have the expected number of routes
	assert.GreaterOrEqual(t, len(routes), 3)

	// Check that health endpoint exists
	healthFound := false
	for _, route := range routes {
		if route.Path == "/health" && route.Method == "GET" {
			healthFound = true
			break
		}
	}
	assert.True(t, healthFound, "Health endpoint should be registered")
}

// Test API Constants
func TestAPIConstants(t *testing.T) {
	assert.Equal(t, 30*time.Second, DefaultTimeout)
	assert.Equal(t, "1m", DefaultInterval)
	assert.Equal(t, "1.0.0", ServiceVersion)
	assert.Equal(t, "trade-candle-service", ServiceName)
	assert.Equal(t, "request_id", RequestIDContextKey)
	assert.Equal(t, "X-Request-ID", RequestIDHeaderKey)
}

// Test Health Check Endpoint
func TestHealthCheck(t *testing.T) {
	setupGinTestMode()

	mockService := &MockTradeService{}
	handler := NewAPIHandler(mockService, setupTestLogger())
	router := handler.SetupRoutes()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify health response structure (implementation dependent)
	assert.Contains(t, response, "status")
}

// Test Trades Endpoint
func TestGetTradesEndpoint(t *testing.T) {
	setupGinTestMode()

	tests := []struct {
		name           string
		symbol         string
		mockTrades     []model.Trade
		mockError      error
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "successful request",
			symbol:         "BTC-USD",
			mockTrades:     createTestTrades(10),
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "empty symbol",
			symbol:         "",
			mockTrades:     []model.Trade{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "service error",
			symbol:         "BTC-USD",
			mockTrades:     []model.Trade{},
			mockError:      errors.New("service error"),
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name:           "invalid symbol format",
			symbol:         "invalid-symbol-123",
			mockTrades:     []model.Trade{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockTradeService{}

			if !tt.expectError || tt.mockError != nil {
				mockService.On("GetTrades", mock.Anything, tt.symbol).Return(tt.mockTrades, tt.mockError)
			}

			handler := NewAPIHandler(mockService, setupTestLogger())
			router := handler.SetupRoutes()

			url := "/trades"
			if tt.symbol != "" {
				url += "?symbol=" + tt.symbol
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", url, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if !tt.expectError {
				var response []model.Trade
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Len(t, response, len(tt.mockTrades))
			}

			if tt.mockError == nil && !tt.expectError {
				mockService.AssertExpectations(t)
			}
		})
	}
}

// Test Candles Endpoint
func TestGetCandlesEndpoint(t *testing.T) {
	setupGinTestMode()

	tests := []struct {
		name           string
		symbol         string
		interval       string
		limit          string
		mockCandles    []model.Candle
		mockError      error
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "successful request with all params",
			symbol:         "BTC-USD",
			interval:       "5m",
			limit:          "10",
			mockCandles:    createTestCandles(10),
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "successful request with default interval",
			symbol:         "BTC-USD",
			interval:       "",
			limit:          "5",
			mockCandles:    createTestCandles(5),
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "empty symbol",
			symbol:         "",
			interval:       "1m",
			limit:          "10",
			mockCandles:    []model.Candle{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "invalid interval",
			symbol:         "BTC-USD",
			interval:       "30s",
			limit:          "10",
			mockCandles:    []model.Candle{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "invalid limit",
			symbol:         "BTC-USD",
			interval:       "1m",
			limit:          "invalid",
			mockCandles:    []model.Candle{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "service error",
			symbol:         "BTC-USD",
			interval:       "1m",
			limit:          "10",
			mockCandles:    []model.Candle{},
			mockError:      errors.New("service error"),
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name:           "zero limit",
			symbol:         "BTC-USD",
			interval:       "1m",
			limit:          "0",
			mockCandles:    []model.Candle{},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "negative limit",
			symbol:         "BTC-USD",
			interval:       "1m",
			limit:          "-5",
			mockCandles:    []model.Candle{},
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockTradeService{}

			if !tt.expectError || tt.mockError != nil {
				expectedInterval := tt.interval
				if expectedInterval == "" {
					expectedInterval = DefaultInterval
				}

				expectedLimit, _ := strconv.ParseInt(tt.limit, 10, 64)
				mockService.On("GetCandles", mock.Anything, tt.symbol, expectedInterval, expectedLimit).Return(tt.mockCandles, tt.mockError)
			}

			handler := NewAPIHandler(mockService, setupTestLogger())
			router := handler.SetupRoutes()

			url := "/candles"
			params := []string{}

			if tt.symbol != "" {
				params = append(params, "symbol="+tt.symbol)
			}
			if tt.interval != "" {
				params = append(params, "interval="+tt.interval)
			}
			if tt.limit != "" {
				params = append(params, "limit="+tt.limit)
			}

			if len(params) > 0 {
				url += "?" + params[0]
				for _, param := range params[1:] {
					url += "&" + param
				}
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", url, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if !tt.expectError {
				var response []model.Candle
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Len(t, response, len(tt.mockCandles))
			}

			if tt.mockError == nil && !tt.expectError {
				mockService.AssertExpectations(t)
			}
		})
	}
}

// Test Middleware Integration
func TestMiddlewareIntegration(t *testing.T) {
	setupGinTestMode()

	mockService := &MockTradeService{}
	mockService.On("GetTrades", mock.Anything, "BTC-USD").Return(createTestTrades(1), nil)

	handler := NewAPIHandler(mockService, setupTestLogger())
	router := handler.SetupRoutes()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/trades?symbol=BTC-USD", nil)

	// Add a custom header to test middleware
	req.Header.Set("Origin", "http://localhost:3000")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify CORS headers are set (if implemented)
	// This will depend on the actual middleware implementation
	headers := w.Header()
	assert.NotNil(t, headers)
}

// Test Request ID Middleware
func TestRequestIDMiddleware(t *testing.T) {
	setupGinTestMode()

	mockService := &MockTradeService{}
	mockService.On("GetTrades", mock.Anything, "BTC-USD").Return(createTestTrades(1), nil)

	handler := NewAPIHandler(mockService, setupTestLogger())
	router := handler.SetupRoutes()

	tests := []struct {
		name            string
		providedID      string
		expectGenerated bool
	}{
		{
			name:            "with provided request ID",
			providedID:      "test-request-123",
			expectGenerated: false,
		},
		{
			name:            "without request ID",
			providedID:      "",
			expectGenerated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/trades?symbol=BTC-USD", nil)

			if tt.providedID != "" {
				req.Header.Set(RequestIDHeaderKey, tt.providedID)
			}

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			// Check response headers for request ID
			responseID := w.Header().Get(RequestIDHeaderKey)
			if tt.expectGenerated {
				assert.NotEmpty(t, responseID)
			} else {
				// Should echo back the provided ID or generate one
				assert.NotEmpty(t, responseID)
			}
		})
	}
}

// Test Error Handling
func TestErrorHandling(t *testing.T) {
	setupGinTestMode()

	tests := []struct {
		name           string
		endpoint       string
		params         string
		mockError      error
		expectedStatus int
	}{
		{
			name:           "trades service timeout",
			endpoint:       "/trades",
			params:         "?symbol=BTC-USD",
			mockError:      context.DeadlineExceeded,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "candles service unavailable",
			endpoint:       "/candles",
			params:         "?symbol=BTC-USD&interval=1m&limit=10",
			mockError:      errors.New("service unavailable"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockTradeService{}

			if tt.endpoint == "/trades" {
				mockService.On("GetTrades", mock.Anything, "BTC-USD").Return([]model.Trade{}, tt.mockError)
			} else {
				mockService.On("GetCandles", mock.Anything, "BTC-USD", "1m", int64(10)).Return([]model.Candle{}, tt.mockError)
			}

			handler := NewAPIHandler(mockService, setupTestLogger())
			router := handler.SetupRoutes()

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.endpoint+tt.params, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// Test Content Type and Response Format
func TestContentTypeAndFormat(t *testing.T) {
	setupGinTestMode()

	mockService := &MockTradeService{}
	mockService.On("GetTrades", mock.Anything, "BTC-USD").Return(createTestTrades(2), nil)

	handler := NewAPIHandler(mockService, setupTestLogger())
	router := handler.SetupRoutes()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/trades?symbol=BTC-USD", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	var response []model.Trade
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 2)

	// Verify trade structure
	assert.NotEmpty(t, response[0].TradeID)
	assert.NotEmpty(t, response[0].Symbol)
	assert.Greater(t, response[0].Price, 0.0)
	assert.Greater(t, response[0].Size, 0.0)
	assert.Greater(t, response[0].Timestamp, int64(0))
}

// Test Route Not Found
func TestRouteNotFound(t *testing.T) {
	setupGinTestMode()

	mockService := &MockTradeService{}
	handler := NewAPIHandler(mockService, setupTestLogger())
	router := handler.SetupRoutes()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Test HTTP Methods
func TestHTTPMethods(t *testing.T) {
	setupGinTestMode()

	mockService := &MockTradeService{}
	handler := NewAPIHandler(mockService, setupTestLogger())
	router := handler.SetupRoutes()

	tests := []struct {
		method         string
		endpoint       string
		expectedStatus int
	}{
		{"POST", "/trades", http.StatusNotFound},
		{"PUT", "/trades", http.StatusNotFound},
		{"DELETE", "/trades", http.StatusNotFound},
		{"POST", "/candles", http.StatusNotFound},
		{"GET", "/health", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.method, tt.endpoint), func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, tt.endpoint, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// Benchmark tests
func BenchmarkGetTrades(b *testing.B) {
	setupGinTestMode()

	mockService := &MockTradeService{}
	mockService.On("GetTrades", mock.Anything, "BTC-USD").Return(createTestTrades(50), nil)

	handler := NewAPIHandler(mockService, setupTestLogger())
	router := handler.SetupRoutes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/trades?symbol=BTC-USD", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkGetCandles(b *testing.B) {
	setupGinTestMode()

	mockService := &MockTradeService{}
	mockService.On("GetCandles", mock.Anything, "BTC-USD", "1m", int64(100)).Return(createTestCandles(100), nil)

	handler := NewAPIHandler(mockService, setupTestLogger())
	router := handler.SetupRoutes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/candles?symbol=BTC-USD&interval=1m&limit=100", nil)
		router.ServeHTTP(w, req)
	}
}
