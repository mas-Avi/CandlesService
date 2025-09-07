package main

import (
	"CandleService/api"
	core2 "CandleService/internal/core"
	"CandleService/internal/data"
	"CandleService/internal/mock"
	"CandleService/internal/service"
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal, stopping services...")
		cancel() // Cancel the context to stop all services
	}()

	// 1. Create trade storage (pluggable - can be replaced with database, etc.)
	tradeStorage := data.NewInMemoryTradeStorage()

	// 2. Create trade ingestion service (reads from channel and stores trades)
	tradeIngestion := core2.NewTradeIngestionService(tradeStorage)

	// 3. Create trade service (reads from storage to serve API)
	tradeService := service.NewTradeService(tradeStorage)

	// Start trade ingestion service with context
	tradeIngestion.Start(ctx)

	// Create and start trade data generator with context
	tdg := mock.NewTradeDataGenerator(tradeIngestion.GetTradeChannel())
	tdg.Start(ctx)

	// Create API handler with the new architecture
	apiHandler := api.NewAPIHandler(tradeService, slog.Default())

	// Start server
	port := 8080
	fmt.Printf("Trade service starting on port %d\n", port)
	fmt.Printf("Endpoints:\n")
	fmt.Printf("  GET /api/v1/trades?symbol=BTC-PERP\n")
	fmt.Printf("  GET /api/v1/candles?symbol=BTC-PERP&interval=1m\n")
	fmt.Printf("  GET /health\n")
	fmt.Printf("Press Ctrl+C to gracefully shutdown\n")

	log.Fatal(apiHandler.StartServer(port))
}
