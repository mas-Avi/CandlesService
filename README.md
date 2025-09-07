# CandleService - Real-Time Trading Data API

## Table of Contents
- [Overview](#overview)
- [System Architecture](#system-architecture)
- [Main Components](#main-components)
- [Data Flow](#data-flow)
- [Running with Docker](#running-with-docker)
- [Local Development Setup](#local-development-setup)
- [Supported Symbols](#supported-symbols)
- [Data Generation](#data-generation)
- [API Endpoints](#api-endpoints)
- [System Shortcomings](#system-shortcomings)

## Overview

CandleService is a high-performance Go-based REST API service designed to provide real-time and historical trading data. The service generates mock trading data and serves it through well-defined endpoints, supporting both individual trade data and aggregated OHLCV (Open, High, Low, Close, Volume) candle data across multiple time intervals.

The system is built with a clean, layered architecture that emphasizes testability, maintainability, and performance. It uses in-memory storage for fast data access and includes comprehensive middleware for logging, validation, and request tracking.

## System Architecture

### Layered Design Philosophy

The system follows a clean architecture pattern with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                    API Layer (HTTP)                         │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐│
│  │   Router    │ │ Middleware  │ │      Handlers           ││
│  │    (Gin)    │ │    Stack    │ │   (Business Logic)      ││
│  └─────────────┘ └─────────────┘ └─────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Service Layer                             │
│  ┌─────────────────────────────────────────────────────────┐│
│  │              Trade Service                              ││
│  │        (Business Logic & Aggregation)                  ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                    Data Layer                               │
│  ┌─────────────────────────────────────────────────────────┐│
│  │           In-Memory Trade Storage                       ││
│  │        (Thread-Safe Data Operations)                   ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                 Core/Ingestion Layer                        │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐│
│  │   Trade     │ │   Data      │ │      Mock Data          ││
│  │ Ingestion   │ │  Storage    │ │     Generator           ││
│  │  Service    │ │  Interface  │ │    (Simulation)         ││
│  └─────────────┘ └─────────────┘ └─────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

### Design Principles

1. **Dependency Injection**: All components receive their dependencies through constructors
2. **Interface Segregation**: Small, focused interfaces for easy testing and swapping implementations
3. **Single Responsibility**: Each component has a clear, single purpose
4. **Concurrent Safety**: Thread-safe operations using Go's synchronization primitives
5. **Graceful Shutdown**: Proper context-based cancellation for clean service shutdown

## Main Components

### 1. API Layer (`api/`)
- **Purpose**: HTTP request handling, routing, and response formatting
- **Key Files**:
  - `api.go`: Main API handler struct and dependencies
  - `handler.go`: HTTP endpoint implementations
  - `middleware.go`: CORS, logging, request ID propagation
  - `validator.go`: Input validation and sanitization
- **Framework**: Gin HTTP router for high performance
- **Features**: Request timeout handling, structured logging, error handling

### 2. Service Layer (`internal/service/`)
- **Purpose**: Business logic implementation and data aggregation
- **Key Component**: `TradeService`
- **Responsibilities**:
  - Fetching trade data from storage
  - Aggregating trades into OHLCV candles
  - Supporting multiple time intervals (1m, 5m, 15m, 1h)
  - Data validation and transformation

### 3. Data Layer (`internal/data/`)
- **Purpose**: Data storage and retrieval operations
- **Implementation**: In-memory storage with concurrent safety
- **Features**:
  - Thread-safe read/write operations using RWMutex
  - Efficient data structures for fast lookups
  - Time-based data organization
  - Automatic candle pre-computation

### 4. Core/Ingestion Layer (`internal/core/`)
- **Purpose**: Data ingestion and processing pipeline
- **Key Component**: `TradeIngestionService`
- **Features**:
  - Channel-based data ingestion
  - Concurrent processing
  - Context-aware shutdown
  - Real-time data processing

### 5. Mock Data Generation (`internal/mock/`)
- **Purpose**: Realistic trading data simulation
- **Features**:
  - Configurable volatility and price movements
  - Historical data generation
  - Multiple symbol support
  - Realistic trading patterns

### 6. Models (`internal/model/`)
- **Purpose**: Core data structures
- **Key Types**:
  - `Trade`: Individual trade data
  - `Candle`: OHLCV aggregated data

## Data Flow

### Request Processing Flow
```
1. HTTP Request → Gin Router
2. Middleware Stack Processing:
   ├── CORS Headers
   ├── Request ID Generation
   ├── Request Logging
   └── Context Propagation
3. Route Handler Execution:
   ├── Input Validation
   ├── Parameter Sanitization
   └── Context Timeout Setup
4. Service Layer Call:
   ├── Business Logic Processing
   ├── Data Aggregation (if needed)
   └── Error Handling
5. Data Layer Access:
   ├── Thread-Safe Storage Access
   ├── Data Retrieval/Filtering
   └── Result Preparation
6. Response Generation:
   ├── JSON Serialization
   ├── HTTP Status Setting
   └── Response Headers
```

### Data Ingestion Flow
```
1. Mock Data Generator
   ├── Historical Data Generation (1 hour)
   ├── Real-time Data Generation (every 2 seconds)
   └── Price Volatility Simulation
2. Trade Channel
   ├── Buffered Channel Communication
   └── Concurrent Processing
3. Trade Ingestion Service
   ├── Channel Data Reading
   ├── Data Validation
   └── Storage Writing
4. In-Memory Storage
   ├── Trade Data Storage
   ├── Candle Pre-computation
   └── Index Maintenance
```

## Running with Docker

### Prerequisites
- Docker 20.10+
- Docker Compose 2.0+

### Quick Start (Production)
```bash
# Clone the repository
git clone <repository-url>
cd CandleService

# Build and start the service
docker-compose up -d

# Verify the service is running
curl http://localhost:8080/health

# View logs
docker-compose logs -f candle-service

# Stop the service
docker-compose down
```

### Docker Configuration Details

**Services:**
- `candle-service`: Main application (port 8080)

**Networks:**
- `candle-network`: Isolated bridge network for service communication

**Health Checks:**
- Automatic health monitoring every 30 seconds
- Service restart on health check failures

## Local Development Setup

### Prerequisites
- Go 1.24+
- Git

### Setup Steps
```bash
# 1. Clone the repository
git clone <repository-url>
cd CandleService

# 2. Install dependencies
go mod tidy

# 3. Run tests
go test ./...

# 4. Run benchmarks
go test -bench=.

# 5. Start the service
go run cmd/main.go
```

### Development Workflow
```bash
# Run tests with coverage
go test -cover ./...

# Run specific test packages
go test ./internal/service/...

# Build the binary
go build -o candle-service cmd/main.go

# Run with custom configuration
PORT=9090 ./candle-service
```

### IDE Setup
- **VS Code**: Install Go extension for syntax highlighting and debugging
- **GoLand**: Native Go support with debugging and testing integration
- **Vim/Neovim**: Use vim-go plugin for Go development

## Supported Symbols

### Default Symbols
The system comes pre-configured with realistic cryptocurrency perpetual futures symbols:

| Symbol    | Description          | Base Price | Volatility |
|-----------|---------------------|------------|------------|
| BTC-PERP  | Bitcoin Perpetual   | $50,000    | 1%         |
| ETH-PERP  | Ethereum Perpetual  | $3,000     | 1%         |
| SOL-PERP  | Solana Perpetual    | $100       | 1%         |

### Symbol Configuration
Symbols are configured in `internal/mock/trade_data_generator.go`:

```go
DefaultGeneratorConfig() GeneratorConfig {
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
```

### Adding New Symbols
To add new symbols, modify the configuration and restart the service:
1. Add symbol to `Symbols` slice
2. Add base price to `BasePrices` map
3. Restart the service

## Data Generation

### Mock Data Generator Features

**Historical Data Generation:**
- Generates 1 hour of historical trade data on startup
- 1-10 trades per minute with realistic time distribution
- Maintains chronological order within each minute
- Price movements follow realistic volatility patterns

**Real-Time Data Generation:**
- Generates new trades every 2 seconds
- Simulates realistic price movements using random walk
- Configurable volatility (default: 1%)
- Multiple symbols generated concurrently

**Price Movement Algorithm:**
```go
// Simplified price movement calculation
percentChange := (rand.Float64() - 0.5) * 2 * volatility
newPrice := currentPrice * (1 + percentChange)
```

**Trade Size Generation:**
- Random trade sizes between 0.1 and 10.0 units
- Realistic distribution favoring smaller trade sizes
- Each trade gets a unique sequential ID

**Configurable Parameters:**
- `Interval`: Time between trade generations (default: 2s)
- `Volatility`: Price movement percentage (default: 1%)
- `HistoryHours`: Historical data period (default: 1 hour)
- `BasePrices`: Starting prices for each symbol

## API Endpoints

### Health Check
```http
GET /health
```
**Response:**
```json
{
  "status": "OK",
  "service": "trade-candle-service",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.0.0"
}
```

### Get Trades
```http
GET /api/v1/trades?symbol=BTC-PERP
```
**Parameters:**
- `symbol` (required): Trading symbol (e.g., BTC-PERP)

**Response:**
```json
[
  {
    "trade_id": "123456",
    "symbol": "BTC-PERP",
    "price": 50250.50,
    "size": 1.5,
    "timestamp": 1705312200000
  }
]
```

### Get Candles
```http
GET /api/v1/candles?symbol=BTC-PERP&interval=1m&limit=100
```
**Parameters:**
- `symbol` (required): Trading symbol
- `interval` (optional): Time interval (1m, 5m, 15m, 1h) - default: 1m
- `limit` (optional): Number of candles (1-1000) - default: 100

**Response:**
```json
[
  {
    "timestamp": 1705312200000,
    "open": 50200.00,
    "high": 50300.00,
    "low": 50180.00,
    "close": 50250.00,
    "volume": 15.75
  }
]
```

### Error Responses
```json
{
  "error": "Invalid symbol parameter",
  "message": "Symbol must be provided and contain only alphanumeric characters and hyphens"
}
```

## System Shortcomings

### Current Limitations

**1. Data Persistence**
- **Issue**: All data is stored in memory and lost on restart
- **Impact**: No historical data persistence across service restarts
- **Mitigation**: Could be addressed by adding database integration (PostgreSQL, ClickHouse)

**2. No Authentication/Authorization**
- **Issue**: All endpoints are publicly accessible
- **Impact**: No rate limiting or access control
- **Mitigation**: Could add JWT authentication and rate limiting middleware

**3. Error Handling Coverage**
- **Issue**: Limited error scenarios covered
- **Impact**: May not handle all edge cases gracefully
- **Mitigation**: Could add more comprehensive error handling and circuit breakers

**4. Configuration Management**
- **Issue**: Configuration is hardcoded in the application
- **Impact**: Requires code changes for configuration updates
- **Mitigation**: Could implement environment-based configuration

**5. Monitoring and Observability**
- **Issue**: Basic logging, no metrics or tracing
- **Impact**: Limited operational visibility
- **Mitigation**: Could add Prometheus metrics, distributed tracing

### Performance Considerations

**Memory Usage:**
- Grows linearly with trade volume
- No automatic data cleanup or archival
- Estimated ~1MB per 10,000 trades

**Concurrency:**
- Read-heavy workloads perform well with RWMutex
- Write operations may become bottleneck under high load
- Single goroutine for data ingestion

**Network:**
- JSON serialization overhead for large responses
- No response compression
- No caching headers

### Testing Strategy
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run benchmarks
go test -bench=.

# Test specific packages
go test ./internal/service/
```
