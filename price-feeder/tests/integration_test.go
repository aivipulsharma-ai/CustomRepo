package tests

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	"github.com/dextr_avs/price-feeder/config"
	"github.com/dextr_avs/price-feeder/handlers"
	"github.com/dextr_avs/price-feeder/middleware"
	"github.com/dextr_avs/price-feeder/models"
	"github.com/dextr_avs/price-feeder/services"
)

func setupTestServer() *httptest.Server {
	cfg := &config.Config{
		Server: struct {
			Port int    `json:"port"`
			Host string `json:"host"`
		}{
			Port: 8080,
			Host: "localhost",
		},
		Tokens: []config.Token{
			{
				BaseToken:  "0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0",
				BaseSymbol: "WSTETH",
			},
			{
				BaseToken:  "0x8236a87084f8B84306f72007F36F2618A5634494",
				BaseSymbol: "LBTC",
			},
		},
		Pricing: struct {
			UpdateIntervalSeconds int     `json:"update_interval_seconds"`
			VolatilityFactor      float64 `json:"volatility_factor"`
			DefaultSpread         float64 `json:"default_spread"`
			PriceMarkup           float64 `json:"price_markup"`
			ChaosLabsAPIKey       string  `json:"chaos_labs_api_key"`
		}{
			UpdateIntervalSeconds: 5,
			VolatilityFactor:      0.01,
			DefaultSpread:         0.002,
			PriceMarkup:           0.003,
			ChaosLabsAPIKey:       "6CiESFtsHvrIdswVZhTV9GVftI3JQh6atciFyHNXVIA=",
		},
		MakerAddress:    "0x6eDC317F3208B10c46F4fF97fAa04dD632487408",
		MakerPrivateKey: "", // Empty for tests - will generate mock signatures
		EthRpcUrl:       "https://eth.llamarpc.com",
		OneInchAuth: struct {
			AccessKey  string `json:"access_key"`
			SecretKey  string `json:"secret_key"`
			Passphrase string `json:"passphrase"`
		}{
			AccessKey:  "",
			SecretKey:  "",
			Passphrase: "",
		},
	}

	counterService := services.NewCounterService()
	metricsService := services.NewMetricsService()

	pricerService, err := services.NewPricerService(context.Background(), cfg, metricsService)
	if err != nil {
		panic(fmt.Sprintf("Failed to create pricer service: %v", err))
	}

	levelsHandler := handlers.NewLevelsHandler(pricerService, metricsService)
	ordersHandler := handlers.NewOrdersHandler(pricerService, counterService, metricsService, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/levels", levelsHandler.HandleLevels)
	mux.HandleFunc("/order", ordersHandler.HandleOrder)
	mux.HandleFunc("/stats", ordersHandler.HandleStats)
	mux.HandleFunc("/recent-hits", ordersHandler.HandleRecentHits)
	mux.HandleFunc("/reset", ordersHandler.HandleReset)

	var handler http.Handler = mux
	handler = middleware.CORSMiddleware(handler)
	handler = middleware.HealthCheckMiddleware("/health")(handler)

	return httptest.NewServer(handler)
}

func TestLevelsEndpoint(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	t.Run("Get all levels", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/levels")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Errorf("Failed to close response body: %v", err)
			}
		}()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		var allLevels models.AllLevelsResponse
		if err := json.NewDecoder(resp.Body).Decode(&allLevels); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(allLevels) == 0 {
			t.Error("Expected pairs in response, got none")
		}

		for pairKey, levels := range allLevels {
			if len(levels) == 0 {
				t.Errorf("Expected levels for pair %s, got none", pairKey)
			}

			for _, level := range levels {
				if len(level) != 2 {
					t.Errorf("Expected level array of length 2, got %d", len(level))
				}
			}
		}
	})

	t.Run("Verify parameters are ignored", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/levels?tokenIn=0x123&tokenOut=0x456&amount=999")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Errorf("Failed to close response body: %v", err)
			}
		}()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		var allLevels models.AllLevelsResponse
		if err := json.NewDecoder(resp.Body).Decode(&allLevels); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(allLevels) == 0 {
			t.Error("Expected pairs in response, got none")
		}
	})
}

func TestOrderEndpoint(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	validOrder := models.OrderRequest{
		BaseToken:  "0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0",
		QuoteToken: "0x8236a87084f8B84306f72007F36F2618A5634494",
		Amount:     "1000000000000000000",
		Taker:      "0x1234567890123456789012345678901234567890",
		FeeBps:     10,
	}

	tests := []struct {
		name           string
		order          models.OrderRequest
		expectedStatus int
		expectSuccess  bool
	}{
		{
			name:           "Valid order",
			order:          validOrder,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name: "Missing taker",
			order: models.OrderRequest{
				BaseToken:  validOrder.BaseToken,
				QuoteToken: validOrder.QuoteToken,
				Amount:     validOrder.Amount,
				FeeBps:     validOrder.FeeBps,
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name: "Negative feeBps",
			order: models.OrderRequest{
				BaseToken:  validOrder.BaseToken,
				QuoteToken: validOrder.QuoteToken,
				Amount:     validOrder.Amount,
				Taker:      validOrder.Taker,
				FeeBps:     -5,
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.order)
			resp, err := http.Post(server.URL+"/order", "application/json", bytes.NewBuffer(body))
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			// For successful orders, check that Order and Signature are present
			if tt.expectSuccess {
				var orderResp models.OrderResponse
				if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if orderResp.Order == nil {
					t.Error("Expected order object for successful order")
				} else {
					if orderResp.Order.Maker == "" {
						t.Error("Expected maker address in order")
					}
					if orderResp.Order.MakerAsset == "" {
						t.Error("Expected makerAsset in order")
					}
					if orderResp.Order.TakerAsset == "" {
						t.Error("Expected takerAsset in order")
					}
					if orderResp.Order.MakingAmount == "" {
						t.Error("Expected makingAmount in order")
					}
					if orderResp.Order.TakingAmount == "" {
						t.Error("Expected takingAmount in order")
					}
				}
				if orderResp.Signature == "" {
					t.Error("Expected signature for successful order")
				}
			} else {
				// For failed orders, decode as ErrorResponse
				var errorResp models.ErrorResponse
				if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}
				if errorResp.Code != tt.expectedStatus {
					t.Errorf("Expected error code %d, got %d", tt.expectedStatus, errorResp.Code)
				}
			}
		})
	}
}

func TestStatsEndpoint(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/stats")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var stats models.Statistics
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if stats.TokenPairStats == nil {
		t.Error("Expected TokenPairStats to be initialized")
	}

	if stats.HourlyBreakdown == nil {
		t.Error("Expected HourlyBreakdown to be initialized")
	}
}

func TestRecentHitsEndpoint(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	validOrder := models.OrderRequest{
		BaseToken:  "0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0",
		QuoteToken: "0x8236a87084f8B84306f72007F36F2618A5634494",
		Amount:     "1000000000000000000",
		Taker:      "0x1234567890123456789012345678901234567890",
		FeeBps:     10,
	}

	body, _ := json.Marshal(validOrder)
	_, err := http.Post(server.URL+"/order", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	resp, err := http.Get(server.URL + "/recent-hits?limit=10")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var hits []models.TradeHit
	if err := json.NewDecoder(resp.Body).Decode(&hits); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(hits) == 0 {
		t.Error("Expected at least one hit after creating an order")
	}
}

func TestHealthCheckEndpoint(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if health["status"] != "healthy" {
		t.Error("Expected health status to be 'healthy'")
	}
}

// setupTestServerWithConfig creates a test server using the actual config.json file
func setupTestServerWithConfig(t *testing.T) (*httptest.Server, *config.Config) {
	// Get the path to config.json (assuming tests run from tests/ directory)
	configPath := filepath.Join("..", "config.json")

	// Check if config.json exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skipf("config.json not found at %s, skipping test", configPath)
	}

	// Set CONFIG_FILE env var temporarily
	oldConfigFile := os.Getenv("CONFIG_FILE")
	os.Setenv("CONFIG_FILE", configPath)
	defer os.Setenv("CONFIG_FILE", oldConfigFile)

	// Load actual config
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	counterService := services.NewCounterService()
	metricsService := services.NewMetricsService()
	pricerService, err := services.NewPricerService(context.Background(), cfg, metricsService)
	if err != nil {
		t.Fatalf("Failed to create pricer service: %v", err)
	}
	levelsHandler := handlers.NewLevelsHandler(pricerService, metricsService)
	ordersHandler := handlers.NewOrdersHandler(pricerService, counterService, metricsService, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/levels", levelsHandler.HandleLevels)
	mux.HandleFunc("/order", ordersHandler.HandleOrder)
	mux.HandleFunc("/stats", ordersHandler.HandleStats)
	mux.HandleFunc("/recent-hits", ordersHandler.HandleRecentHits)
	mux.HandleFunc("/reset", ordersHandler.HandleReset)

	var handler http.Handler = mux
	handler = middleware.CORSMiddleware(handler)
	handler = middleware.HealthCheckMiddleware("/health")(handler)

	return httptest.NewServer(handler), cfg
}

// verifyEIP712Signature verifies that the signature was created by the expected signer
func verifyEIP712Signature(order *models.RFQOrder, signature string, expectedSigner string) (bool, error) {
	// Remove 0x prefix from signature
	sigHex := strings.TrimPrefix(signature, "0x")
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %v", err)
	}

	if len(sigBytes) != 65 {
		return false, fmt.Errorf("invalid signature length: expected 65, got %d", len(sigBytes))
	}

	// Adjust V value (Ethereum signatures have V as 27 or 28, we need 0 or 1 for recovery)
	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	// Create EIP-712 typed data (same as in the handler)
	// Based on bytecode analysis, using "1inch Aggregation Router" v6
	const aggregationRouterAddress = "0x111111125421ca6dc452d289314280a0f8842a65"
	const chainID = 1 // Ethereum Mainnet

	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": []apitypes.Type{
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"Order": []apitypes.Type{
				{Name: "salt", Type: "uint256"},
				{Name: "maker", Type: "address"},
				{Name: "receiver", Type: "address"},
				{Name: "makerAsset", Type: "address"},
				{Name: "takerAsset", Type: "address"},
				{Name: "makingAmount", Type: "uint256"},
				{Name: "takingAmount", Type: "uint256"},
				{Name: "makerTraits", Type: "uint256"},
			},
		},
		PrimaryType: "Order",
		Domain: apitypes.TypedDataDomain{
			Name:              "1inch Aggregation Router",
			Version:           "6",
			ChainId:           math.NewHexOrDecimal256(chainID),
			VerifyingContract: aggregationRouterAddress,
		},
		Message: apitypes.TypedDataMessage{
			"salt":         order.Salt,
			"maker":        order.Maker,
			"receiver":     order.Receiver,
			"makerAsset":   order.MakerAsset,
			"takerAsset":   order.TakerAsset,
			"makingAmount": order.MakingAmount,
			"takingAmount": order.TakingAmount,
			"makerTraits":  order.MakerTraits,
		},
	}

	// Hash the typed data
	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return false, fmt.Errorf("error hashing domain: %v", err)
	}

	typedDataHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return false, fmt.Errorf("error hashing message: %v", err)
	}

	// Combine according to EIP-712: "\x19\x01" + domainSeparator + typedDataHash
	rawData := []byte(fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(typedDataHash)))
	hash := crypto.Keccak256Hash(rawData)

	// Recover the public key from the signature
	pubKey, err := crypto.SigToPub(hash.Bytes(), sigBytes)
	if err != nil {
		return false, fmt.Errorf("failed to recover public key: %v", err)
	}

	// Get the address from the public key
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)

	// Compare with expected signer (case-insensitive)
	expectedAddr := common.HexToAddress(expectedSigner)

	if !strings.EqualFold(recoveredAddr.Hex(), expectedAddr.Hex()) {
		return false, fmt.Errorf("signature verification failed: expected %s, recovered %s",
			expectedAddr.Hex(), recoveredAddr.Hex())
	}

	return true, nil
}

// TestOrderEndpointWithActualConfig tests the /order endpoint with actual config.json values
func TestOrderEndpointWithActualConfig(t *testing.T) {
	server, cfg := setupTestServerWithConfig(t)
	defer server.Close()

	t.Run("Verify maker address from config", func(t *testing.T) {
		validOrder := models.OrderRequest{
			BaseToken:  "0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0", // WSTETH
			QuoteToken: "0x8236a87084f8B84306f72007F36F2618A5634494", // LBTC
			Amount:     "0.01",                                       // 0.01 token (well within available balance of 0.029224)
			Taker:      "0x1234567890123456789012345678901234567890",
			FeeBps:     10,
		}

		body, _ := json.Marshal(validOrder)
		resp, err := http.Post(server.URL+"/order", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Errorf("Failed to close response body: %v", err)
			}
		}()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		var orderResp models.OrderResponse
		if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify order object exists
		if orderResp.Order == nil {
			t.Fatal("Expected order object in response")
		}

		// Verify maker address matches config.json
		expectedMaker := strings.ToLower(cfg.MakerAddress)
		actualMaker := strings.ToLower(orderResp.Order.Maker)
		if actualMaker != expectedMaker {
			t.Errorf("Expected maker address %s from config.json, got %s", expectedMaker, actualMaker)
		}
		t.Logf("✓ Maker address matches config.json: %s", cfg.MakerAddress)

		// Verify signature exists
		if orderResp.Signature == "" {
			t.Error("Expected signature in response")
		}

		// Verify signature format (should be 0x + 130 hex characters for ECDSA signature)
		if !strings.HasPrefix(orderResp.Signature, "0x") {
			t.Error("Expected signature to start with 0x")
		}
		sigHex := strings.TrimPrefix(orderResp.Signature, "0x")
		if len(sigHex) != 130 {
			t.Errorf("Expected signature length of 130 hex chars (65 bytes), got %d", len(sigHex))
		}

		// Verify it's valid hex
		if _, err := hex.DecodeString(sigHex); err != nil {
			t.Errorf("Signature is not valid hex: %v", err)
		}

		// If private key is configured, verify it's not a mock signature
		if cfg.MakerPrivateKey != "" {
			t.Logf("✓ Private key is configured in config.json")
			t.Logf("✓ Signature generated successfully: %s", orderResp.Signature[:10]+"...")

			// Verify the signature cryptographically
			valid, err := verifyEIP712Signature(orderResp.Order, orderResp.Signature, cfg.MakerAddress)
			if err != nil {
				t.Errorf("Signature verification error: %v", err)
			}
			if !valid {
				t.Error("Signature verification failed: signature is not valid")
			} else {
				t.Logf("✓ Signature cryptographically verified - signed by maker address: %s", cfg.MakerAddress)
			}
		} else {
			t.Logf("⚠ Private key not configured in config.json, signature may be mock")
		}

		// Verify other order fields
		if orderResp.Order.MakerAsset == "" {
			t.Error("Expected makerAsset in order")
		}
		if orderResp.Order.TakerAsset == "" {
			t.Error("Expected takerAsset in order")
		}
		if orderResp.Order.MakingAmount == "" {
			t.Error("Expected makingAmount in order")
		}
		if orderResp.Order.TakingAmount == "" {
			t.Error("Expected takingAmount in order")
		}
		if orderResp.Order.Salt == "" {
			t.Error("Expected salt in order")
		}
		if orderResp.Order.MakerTraits == "" {
			t.Error("Expected makerTraits in order")
		}

		t.Logf("✓ All order fields present and valid")
	})

	t.Run("Verify response structure", func(t *testing.T) {
		validOrder := models.OrderRequest{
			BaseToken:  "0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0",
			QuoteToken: "0x8236a87084f8B84306f72007F36F2618A5634494",
			Amount:     "0.005", // 0.005 tokens (well within available balance)
			Taker:      "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			FeeBps:     25,
		}

		body, _ := json.Marshal(validOrder)
		resp, err := http.Post(server.URL+"/order", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Errorf("Failed to close response body: %v", err)
			}
		}()

		var orderResp models.OrderResponse
		if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify the response follows the expected RFQ format
		if orderResp.Order == nil {
			t.Fatal("Expected order object")
		}

		// Verify maker asset and taker asset match the request
		expectedMakerAsset := strings.ToLower(validOrder.BaseToken)
		actualMakerAsset := strings.ToLower(orderResp.Order.MakerAsset)
		if actualMakerAsset != expectedMakerAsset {
			t.Errorf("Expected makerAsset %s, got %s", expectedMakerAsset, actualMakerAsset)
		}

		expectedTakerAsset := strings.ToLower(validOrder.QuoteToken)
		actualTakerAsset := strings.ToLower(orderResp.Order.TakerAsset)
		if actualTakerAsset != expectedTakerAsset {
			t.Errorf("Expected takerAsset %s, got %s", expectedTakerAsset, actualTakerAsset)
		}

		// Verify signature if private key is configured
		if cfg.MakerPrivateKey != "" && orderResp.Signature != "" {
			valid, err := verifyEIP712Signature(orderResp.Order, orderResp.Signature, cfg.MakerAddress)
			if err != nil {
				t.Errorf("Signature verification error: %v", err)
			}
			if !valid {
				t.Error("Signature verification failed")
			} else {
				t.Logf("✓ Signature verified for second order")
			}
		}

		t.Logf("✓ Order structure validated successfully")
	})

	t.Run("Display config info", func(t *testing.T) {
		t.Logf("=== Config.json Configuration ===")
		t.Logf("Maker Address: %s", cfg.MakerAddress)
		if cfg.MakerPrivateKey != "" {
			// Only show first and last 4 chars for security
			pkLen := len(cfg.MakerPrivateKey)
			if pkLen > 8 {
				masked := cfg.MakerPrivateKey[:6] + "..." + cfg.MakerPrivateKey[pkLen-4:]
				t.Logf("Private Key: %s (configured)", masked)
			} else {
				t.Logf("Private Key: configured")
			}
		} else {
			t.Logf("Private Key: NOT configured (will use mock signatures)")
		}
		t.Logf("ETH RPC URL: %s", cfg.EthRpcUrl)
		t.Logf("Number of tokens: %d", len(cfg.Tokens))
		t.Logf("================================")
	})
}
