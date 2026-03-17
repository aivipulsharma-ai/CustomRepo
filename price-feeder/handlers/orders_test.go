package handlers

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/dextr_avs/price-feeder/config"
	"github.com/dextr_avs/price-feeder/models"
)

func TestGetTokenDecimals_CaseInsensitive(t *testing.T) {
	handler := &OrdersHandler{}

	testCases := []struct {
		name             string
		address          string
		expectedDecimals int
	}{
		{
			name:             "USDT - Mixed case (canonical)",
			address:          "0xdAC17F958D2ee523a2206206994597C13D831ec7",
			expectedDecimals: 6,
		},
		{
			name:             "USDT - All lowercase",
			address:          "0xdac17f958d2ee523a2206206994597c13d831ec7",
			expectedDecimals: 6,
		},
		{
			name:             "USDT - All uppercase",
			address:          "0xDAC17F958D2EE523A2206206994597C13D831EC7",
			expectedDecimals: 6,
		},
		{
			name:             "USDC - Mixed case",
			address:          "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
			expectedDecimals: 6,
		},
		{
			name:             "USDC - All lowercase",
			address:          "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
			expectedDecimals: 6,
		},
		{
			name:             "WETH - Mixed case",
			address:          "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
			expectedDecimals: 18,
		},
		{
			name:             "WETH - All lowercase",
			address:          "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			expectedDecimals: 18,
		},
		{
			name:             "WBTC - Mixed case",
			address:          "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599",
			expectedDecimals: 8,
		},
		{
			name:             "WBTC - All lowercase",
			address:          "0x2260fac5e5542a773aa44fbcfedf7c193bc2c599",
			expectedDecimals: 8,
		},
		{
			name:             "Unknown token - should default to 18",
			address:          "0x0000000000000000000000000000000000000000",
			expectedDecimals: 18,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := handler.getTokenDecimals(tc.address)
			if result != tc.expectedDecimals {
				t.Errorf("getTokenDecimals(%s) = %d, expected %d", tc.address, result, tc.expectedDecimals)
			}
		})
	}
}

func TestGetTokenDecimals_AllTokens(t *testing.T) {
	handler := &OrdersHandler{}

	// Test all supported tokens with their expected decimals
	expectedDecimals := map[string]int{
		"0xdAC17F958D2ee523a2206206994597C13D831ec7": 6,  // USDT
		"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48": 6,  // USDC
		"0x6B175474E89094C44Da98b954EedeAC495271d0F": 18, // DAI
		"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2": 18, // WETH
		"0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0": 18, // WSTETH
		"0xcd5fe23c85820f7b72d0926fc9b05b43e359b7ee": 18, // WEETH
		"0xae7ab96520DE3A18E5e111B5EaAb095312D7fE84": 18, // STETH
		"0x4c9edd5852cd905f086c759e8383e09bff1e68b3": 18, // USDE
		"0x8236a87084f8B84306f72007F36F2618A5634494": 8,  // LBTC
		"0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599": 8,  // WBTC
	}

	for address, expected := range expectedDecimals {
		result := handler.getTokenDecimals(address)
		if result != expected {
			t.Errorf("getTokenDecimals(%s) = %d, expected %d", address, result, expected)
		}
	}
}

func TestValidateOrderRequest_SameTokenCaseInsensitive(t *testing.T) {
	handler := &OrdersHandler{
		config: &config.Config{},
	}

	testCases := []struct {
		name        string
		baseToken   string
		quoteToken  string
		shouldError bool
		description string
	}{
		{
			name:        "Same token - exact same case",
			baseToken:   "0xdAC17F958D2ee523a2206206994597C13D831ec7",
			quoteToken:  "0xdAC17F958D2ee523a2206206994597C13D831ec7",
			shouldError: true,
			description: "Should reject same token with same case",
		},
		{
			name:        "Same token - different case (lowercase)",
			baseToken:   "0xdAC17F958D2ee523a2206206994597C13D831ec7",
			quoteToken:  "0xdac17f958d2ee523a2206206994597c13d831ec7",
			shouldError: true,
			description: "Should reject same token with different case",
		},
		{
			name:        "Same token - different case (uppercase)",
			baseToken:   "0xdac17f958d2ee523a2206206994597c13d831ec7",
			quoteToken:  "0xDAC17F958D2EE523A2206206994597C13D831EC7",
			shouldError: true,
			description: "Should reject same token with different case",
		},
		{
			name:        "Different tokens - same case",
			baseToken:   "0xdAC17F958D2ee523a2206206994597C13D831ec7", // USDT
			quoteToken:  "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // WETH
			shouldError: false,
			description: "Should accept different tokens",
		},
		{
			name:        "Different tokens - different case",
			baseToken:   "0xdac17f958d2ee523a2206206994597c13d831ec7", // USDT lowercase
			quoteToken:  "0xC02AAA39B223FE8D0A0E5C4F27EAD9083C756CC2", // WETH uppercase
			shouldError: false,
			description: "Should accept different tokens with different case",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &models.OrderRequest{
				BaseToken:  tc.baseToken,
				QuoteToken: tc.quoteToken,
				Amount:     "100",
				Taker:      "0x0000000000000000000000000000000000000001",
				FeeBps:     0,
			}

			err := handler.validateOrderRequest(req)

			if tc.shouldError && err == nil {
				t.Errorf("%s: expected error but got none", tc.description)
			}
			if !tc.shouldError && err != nil {
				t.Errorf("%s: expected no error but got: %v", tc.description, err)
			}
		})
	}
}

func TestConvertFromWei(t *testing.T) {
	handler := &OrdersHandler{}

	testCases := []struct {
		name           string
		amountWei      float64
		decimals       int
		expectedTokens float64
		description    string
	}{
		{
			name:           "USDT - 3 tokens (3,000,000 smallest units)",
			amountWei:      3000000,
			decimals:       6,
			expectedTokens: 3.0,
			description:    "3 USDT in WEI (6 decimals) should be 3.0 tokens",
		},
		{
			name:           "USDT - 50.41 tokens",
			amountWei:      50410000,
			decimals:       6,
			expectedTokens: 50.41,
			description:    "50.41 USDT in WEI should be 50.41 tokens",
		},
		{
			name:           "USDC - 1000 tokens",
			amountWei:      1000000000,
			decimals:       6,
			expectedTokens: 1000.0,
			description:    "1000 USDC in WEI (6 decimals) should be 1000.0 tokens",
		},
		{
			name:           "WETH - 1 token (18 decimals)",
			amountWei:      1000000000000000000,
			decimals:       18,
			expectedTokens: 1.0,
			description:    "1 WETH in WEI (18 decimals) should be 1.0 tokens",
		},
		{
			name:           "WETH - 0.5 tokens",
			amountWei:      500000000000000000,
			decimals:       18,
			expectedTokens: 0.5,
			description:    "0.5 WETH in WEI should be 0.5 tokens",
		},
		{
			name:           "WBTC - 1 token (8 decimals)",
			amountWei:      100000000,
			decimals:       8,
			expectedTokens: 1.0,
			description:    "1 WBTC in WEI (8 decimals) should be 1.0 tokens",
		},
		{
			name:           "WBTC - 0.01 tokens",
			amountWei:      1000000,
			decimals:       8,
			expectedTokens: 0.01,
			description:    "0.01 WBTC in WEI should be 0.01 tokens",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := handler.convertFromWei(tc.amountWei, tc.decimals)

			// Use a small epsilon for float comparison
			epsilon := 0.0000001
			diff := result - tc.expectedTokens
			if diff < 0 {
				diff = -diff
			}

			if diff > epsilon {
				t.Errorf("%s: convertFromWei(%f, %d) = %f, expected %f",
					tc.description, tc.amountWei, tc.decimals, result, tc.expectedTokens)
			}
		})
	}
}

func TestConvertToWei(t *testing.T) {
	handler := &OrdersHandler{}

	testCases := []struct {
		name         string
		amountTokens float64
		decimals     int
		expectedWei  string
		description  string
	}{
		{
			name:         "USDT - 3 tokens",
			amountTokens: 3.0,
			decimals:     6,
			expectedWei:  "3000000",
			description:  "3.0 USDT should be 3,000,000 in WEI",
		},
		{
			name:         "USDT - 50.41 tokens",
			amountTokens: 50.41,
			decimals:     6,
			expectedWei:  "50410000",
			description:  "50.41 USDT should be 50,410,000 in WEI",
		},
		{
			name:         "WETH - 1 token",
			amountTokens: 1.0,
			decimals:     18,
			expectedWei:  "1000000000000000000",
			description:  "1.0 WETH should be 10^18 in WEI",
		},
		{
			name:         "WBTC - 1 token",
			amountTokens: 1.0,
			decimals:     8,
			expectedWei:  "100000000",
			description:  "1.0 WBTC should be 10^8 in WEI",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := handler.convertToWei(tc.amountTokens, tc.decimals)

			if result != tc.expectedWei {
				t.Errorf("%s: convertToWei(%f, %d) = %s, expected %s",
					tc.description, tc.amountTokens, tc.decimals, result, tc.expectedWei)
			}
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	handler := &OrdersHandler{}

	testCases := []struct {
		name     string
		decimals int
		amounts  []float64
	}{
		{
			name:     "USDT (6 decimals)",
			decimals: 6,
			amounts:  []float64{1.0, 3.0, 50.41, 1000.0, 0.01},
		},
		{
			name:     "WETH (18 decimals)",
			decimals: 18,
			amounts:  []float64{1.0, 0.5, 10.0, 0.001},
		},
		{
			name:     "WBTC (8 decimals)",
			decimals: 8,
			amounts:  []float64{1.0, 0.01, 21.0, 0.001},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, original := range tc.amounts {
				// Convert to WEI
				weiStr := handler.convertToWei(original, tc.decimals)

				// Parse back to float
				weiFloat, _ := strconv.ParseFloat(weiStr, 64)

				// Convert back to token units
				recovered := handler.convertFromWei(weiFloat, tc.decimals)

				// Check if we got back the original value
				epsilon := 0.0000001
				diff := recovered - original
				if diff < 0 {
					diff = -diff
				}

				if diff > epsilon {
					t.Errorf("Round trip failed for %f with %d decimals: got %f",
						original, tc.decimals, recovered)
				}
			}
		})
	}
}

func TestConvertWeiToFloat(t *testing.T) {
	handler := &OrdersHandler{}

	testCases := []struct {
		name           string
		amountWei      string
		decimals       int
		expectedTokens float64
		description    string
	}{
		{
			name:           "USDT - 3 tokens",
			amountWei:      "3000000",
			decimals:       6,
			expectedTokens: 3.0,
			description:    "3,000,000 WEI with 6 decimals should be 3.0 tokens",
		},
		{
			name:           "USDT - 50.41 tokens",
			amountWei:      "50410000",
			decimals:       6,
			expectedTokens: 50.41,
			description:    "50,410,000 WEI with 6 decimals should be 50.41 tokens",
		},
		{
			name:           "WETH - 1 token (large number)",
			amountWei:      "1000000000000000000", // 10^18
			decimals:       18,
			expectedTokens: 1.0,
			description:    "10^18 WEI with 18 decimals should be 1.0 tokens",
		},
		{
			name:           "WETH - 100 tokens (very large number)",
			amountWei:      "100000000000000000000", // 100 * 10^18
			decimals:       18,
			expectedTokens: 100.0,
			description:    "100 * 10^18 WEI should be 100.0 tokens",
		},
		{
			name:           "WBTC - 0.5 tokens",
			amountWei:      "50000000",
			decimals:       8,
			expectedTokens: 0.5,
			description:    "50,000,000 WEI with 8 decimals should be 0.5 tokens",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create big.Int from string
			amountWei := new(big.Int)
			amountWei.SetString(tc.amountWei, 10)

			result := handler.convertWeiToFloat(amountWei, tc.decimals)

			// Use a small epsilon for float comparison
			epsilon := 0.0000001
			diff := result - tc.expectedTokens
			if diff < 0 {
				diff = -diff
			}

			if diff > epsilon {
				t.Errorf("%s: convertWeiToFloat(%s, %d) = %f, expected %f",
					tc.description, tc.amountWei, tc.decimals, result, tc.expectedTokens)
			}
		})
	}
}

func TestWeiToWeiComparison(t *testing.T) {
	testCases := []struct {
		name         string
		requestedWei string
		availableWei string
		shouldPass   bool
		description  string
	}{
		{
			name:         "Request less than balance - should pass",
			requestedWei: "3000000",  // 3 USDT
			availableWei: "50410000", // 50.41 USDT
			shouldPass:   true,
			description:  "3 USDT < 50.41 USDT should pass",
		},
		{
			name:         "Request equal to balance - should pass",
			requestedWei: "1000000", // 1 USDT
			availableWei: "1000000", // 1 USDT
			shouldPass:   true,
			description:  "1 USDT == 1 USDT should pass",
		},
		{
			name:         "Request more than balance - should fail",
			requestedWei: "100000000", // 100 USDT
			availableWei: "50410000",  // 50.41 USDT
			shouldPass:   false,
			description:  "100 USDT > 50.41 USDT should fail",
		},
		{
			name:         "Large WETH amount - request less than balance",
			requestedWei: "1000000000000000000",  // 1 WETH
			availableWei: "10000000000000000000", // 10 WETH
			shouldPass:   true,
			description:  "1 WETH < 10 WETH should pass",
		},
		{
			name:         "Large WETH amount - request more than balance",
			requestedWei: "20000000000000000000", // 20 WETH
			availableWei: "10000000000000000000", // 10 WETH
			shouldPass:   false,
			description:  "20 WETH > 10 WETH should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse amounts as big.Int
			requestedWei := new(big.Int)
			requestedWei.SetString(tc.requestedWei, 10)

			availableWei := new(big.Int)
			availableWei.SetString(tc.availableWei, 10)

			// Perform comparison (same as in processOrder)
			comparisonResult := requestedWei.Cmp(availableWei)
			passed := comparisonResult <= 0

			if passed != tc.shouldPass {
				t.Errorf("%s: expected shouldPass=%v, got %v (comparison result: %d)",
					tc.description, tc.shouldPass, passed, comparisonResult)
			}
		})
	}
}
