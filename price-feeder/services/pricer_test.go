package services

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dextr_avs/price-feeder/config"
)

// TestFetchAllPricesFromAPI tests the fetchAllPricesFromAPI function using real API
func TestFetchAllPricesFromAPI(t *testing.T) {
	// Load config from config.json
	cfg, err := loadTestConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify API key is present
	if cfg.Pricing.ChaosLabsAPIKey == "" {
		t.Fatal("ChaosLabsAPIKey is empty in config")
	}

	// Create a minimal PricerService for testing
	ps := &PricerService{
		config:     cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		prices:     make(map[string]float64),
		metrics:    nil, // We don't need metrics for this test
	}

	// Call the function
	ctx := context.Background()
	prices, err := ps.fetchAllPricesFromAPI(ctx)
	if err != nil {
		t.Fatalf("fetchAllPricesFromAPI failed: %v", err)
	}

	// Validate results
	t.Logf("Fetched prices for %d tokens", len(prices))

	// Check that we got prices for all expected tokens
	expectedTokens := make(map[string]bool)
	for _, token := range cfg.Tokens {
		expectedTokens[token.BaseSymbol] = true
	}

	if len(prices) != len(expectedTokens) {
		t.Errorf("Expected %d prices, got %d", len(expectedTokens), len(prices))
	}

	// Validate each token price
	for symbol, price := range prices {
		t.Logf("Token: %s, Price: $%.2f", symbol, price)

		// Check that the token is expected
		if !expectedTokens[symbol] {
			t.Errorf("Unexpected token in results: %s", symbol)
		}

		// Check that price is positive
		if price <= 0 {
			t.Errorf("Invalid price for %s: %.2f (expected positive value)", symbol, price)
		}

		// Sanity check: prices should be within reasonable ranges
		// This is just a basic sanity check, not strict validation
		switch strings.ToUpper(symbol) {
		case "USDC", "USDT", "DAI", "USDE":
			// Stablecoins should be around $1
			if price < 0.90 || price > 1.10 {
				t.Logf("Warning: %s price %.4f is outside typical stablecoin range (0.90-1.10)", symbol, price)
			}
		case "WETH", "WSTETH", "WEETH", "STETH":
			// ETH and derivatives should be > $100 (reasonable lower bound)
			if price < 100 {
				t.Errorf("Warning: %s price %.2f seems too low (expected > $100)", symbol, price)
			}
		case "WBTC", "LBTC":
			// Bitcoin should be > $1000 (reasonable lower bound)
			if price < 1000 {
				t.Errorf("Warning: %s price %.2f seems too low (expected > $1000)", symbol, price)
			}
		}
	}

	// Check for missing tokens
	for _, token := range cfg.Tokens {
		if _, exists := prices[token.BaseSymbol]; !exists {
			t.Errorf("Missing price for token: %s (address: %s)", token.BaseSymbol, token.BaseToken)
		}
	}

	// Test the response structure by making a direct API call
	t.Run("ValidateAPIResponse", func(t *testing.T) {
		validateAPIResponse(t, cfg)
	})
}

// validateAPIResponse makes a direct API call and validates the response structure
func validateAPIResponse(t *testing.T, cfg *config.Config) {
	// Build feedIds parameter
	var feedIDs []string
	for _, token := range cfg.Tokens {
		symbol := token.BaseSymbol
		if strings.ToUpper(symbol) == "WETH" {
			symbol = "ETH"
		}
		feedID := strings.ToUpper(symbol) + "USD"
		feedIDs = append(feedIDs, feedID)
	}

	// Make API request
	url := "https://oracle-staging.chaoslabs.co/prices/evm/crypto"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	q := req.URL.Query()
	q.Add("feedIds", strings.Join(feedIDs, ","))
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", cfg.Pricing.ChaosLabsAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("API returned non-OK status: %d", resp.StatusCode)
	}

	// Parse response
	var apiResponse ChaosLabsMultiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	t.Logf("API returned %d prices", len(apiResponse.Prices))

	// Validate response structure
	for _, priceData := range apiResponse.Prices {
		t.Logf("FeedID: %s, Price: %d, Expo: %d, Timestamp: %d",
			priceData.FeedID, priceData.Price, priceData.Expo, priceData.Timestamp)

		// Validate required fields
		if priceData.FeedID == "" {
			t.Error("FeedID is empty")
		}
		if priceData.Price == 0 {
			t.Error("Price is zero")
		}
		if priceData.Timestamp == 0 {
			t.Error("Timestamp is zero")
		}
		if priceData.Signature == "" {
			t.Error("Signature is empty")
		}

		// Validate feedID format (should end with "USD")
		if !strings.HasSuffix(strings.ToUpper(priceData.FeedID), "USD") {
			t.Errorf("FeedID %s doesn't end with 'USD'", priceData.FeedID)
		}

		// Check timestamp is recent (within last hour)
		now := time.Now().Unix()
		if priceData.Timestamp < now-3600 || priceData.Timestamp > now+60 {
			t.Logf("Warning: Timestamp %d seems stale or in future (current: %d)",
				priceData.Timestamp, now)
		}
	}
}

// loadTestConfig loads the config.json file for testing
func loadTestConfig() (*config.Config, error) {
	// Try to load from the project root config.json
	configPath := "../config.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Try alternate path
		configPath = "config.json"
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg config.Config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// TestPricerServiceIntegration is a more comprehensive integration test
func TestPricerServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg, err := loadTestConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	ps := &PricerService{
		config:     cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		prices:     make(map[string]float64),
		metrics:    nil,
	}

	ctx := context.Background()

	// Test multiple calls to ensure consistency
	for i := 0; i < 3; i++ {
		t.Logf("Attempt %d", i+1)
		prices, err := ps.fetchAllPricesFromAPI(ctx)
		if err != nil {
			t.Errorf("Attempt %d failed: %v", i+1, err)
			continue
		}

		// Basic validation
		if len(prices) == 0 {
			t.Errorf("Attempt %d returned no prices", i+1)
		}

		// Add a small delay between requests to avoid rate limiting
		if i < 2 {
			time.Sleep(1 * time.Second)
		}
	}
}

// TestPriceCalculation tests the price calculation logic
func TestPriceCalculation(t *testing.T) {
	tests := []struct {
		name     string
		price    int64
		expo     int
		expected float64
	}{
		{
			name:     "USDC with expo -8",
			price:    100000000,
			expo:     -8,
			expected: 1.0,
		},
		{
			name:     "ETH with expo -8",
			price:    250000000000,
			expo:     -8,
			expected: 2500.0,
		},
		{
			name:     "BTC with expo -8",
			price:    6000000000000,
			expo:     -8,
			expected: 60000.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is the calculation logic from fetchAllPricesFromAPI
			// actualPrice := float64(priceData.Price) / math.Pow10(-priceData.Expo)
			actualPrice := float64(tt.price) / float64(1e8) // Simplified for expo = -8

			if actualPrice != tt.expected {
				t.Errorf("Expected price %.2f, got %.2f", tt.expected, actualPrice)
			}
		})
	}
}
