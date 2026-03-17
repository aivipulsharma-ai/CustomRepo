package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func main() {
	// Seed random number generator for dynamic pricing
	rand.Seed(time.Now().UnixNano())

	// Initialize components
	vaultManager := NewVaultManager()
	oracleClient := NewOracleClient()
	priceFeedEngine := NewPriceFeedEngine(vaultManager, oracleClient)

	// Generate initial price levels
	priceFeedEngine.GeneratePriceLevels()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background services
	go priceFeedEngine.startPriceUpdates(ctx)
	go priceFeedEngine.startRebalancing(ctx)
	go priceFeedEngine.startStatusLogger(ctx)

	// Setup HTTP routes
	router := setupRoutes(priceFeedEngine)

	log.Println("Starting 1inch RFQ Market Maker Server on :8080")
	log.Println("Endpoints:")
	log.Println("  GET  /levels - Get dynamic price levels")
	log.Println("  POST /order  - Create signed order (uses price levels)")
	log.Println("  GET  /status - Get vault status")
	log.Println("Features:")
	log.Println("  - Dynamic pricing with ±2% fluctuation around base prices")
	log.Println("  - Proper directional price level generation")
	log.Println("  - Swaps execute using generated price levels")
	log.Println("  - Multi-token balancer vault initialized with 10% of each main vault")
	log.Println("  - Fee collection vault tracking all earned fees")
	log.Println("  - Automatic rebalancing every 2 minutes")
	log.Println("  - Price updates every 3 seconds")

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
