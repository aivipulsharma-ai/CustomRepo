package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
)

type OracleClient struct {
	basePrices map[string]float64
	prices     map[string]float64
	mu         sync.RWMutex
}

func NewOracleClient() *OracleClient {
	// Base prices that will fluctuate ±2%
	basePrices := map[string]float64{
		"BTC_USDC":  97500.0,
		"BTC_USDT":  97450.0,
		"ETH_USDC":  3650.0,
		"ETH_USDT":  3648.0,
		"BTC_ETH":   26.71,
		"USDC_USDT": 1.0002,
	}

	// Initialize current prices with base prices
	currentPrices := make(map[string]float64)
	for pair, price := range basePrices {
		currentPrices[pair] = price
	}

	return &OracleClient{
		basePrices: basePrices,
		prices:     currentPrices,
	}
}

func (oc *OracleClient) GetPrice(baseToken, quoteToken string) (float64, error) {
	oc.mu.RLock()
	defer oc.mu.RUnlock()

	key := fmt.Sprintf("%s_%s", baseToken, quoteToken)
	if price, exists := oc.prices[key]; exists {
		return price, nil
	}

	// Try reverse pair
	reverseKey := fmt.Sprintf("%s_%s", quoteToken, baseToken)
	if price, exists := oc.prices[reverseKey]; exists {
		return 1.0 / price, nil
	}

	return 0, fmt.Errorf("price not found for pair %s_%s", baseToken, quoteToken)
}

func (oc *OracleClient) UpdatePrices() {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	// Update prices to fluctuate ±2% around base prices
	for pair, basePrice := range oc.basePrices {
		// Generate random change between -2% and +2%
		changePercent := (rand.Float64() - 0.5) * 0.04 // ±2%
		newPrice := basePrice * (1 + changePercent)

		// Ensure price doesn't go below a reasonable minimum
		minPrice := basePrice * 0.98
		maxPrice := basePrice * 1.02

		if newPrice < minPrice {
			newPrice = minPrice
		} else if newPrice > maxPrice {
			newPrice = maxPrice
		}

		oc.prices[pair] = newPrice
	}

	log.Printf("Prices updated - BTC/USDC: %.2f, ETH/USDC: %.2f",
		oc.prices["BTC_USDC"], oc.prices["ETH_USDC"])
}
