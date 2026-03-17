package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/0xcatalysis/catalyst-sdk/go-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	avstypes "github.com/dextr_avs/types"
)

func TestSwapHandler_Execute(t *testing.T) {
	handler := &SwapHandler{}
	ctx := context.Background()

	tests := []struct {
		name            string
		inputToken      string
		outputToken     string
		amount          float64
		expectedSuccess bool
		expectedPath    int // expected path length
		description     string
	}{
		{
			name:            "BTC to USDC swap",
			inputToken:      "BTC",
			outputToken:     "USDC",
			amount:          2.0,
			expectedSuccess: true,
			expectedPath:    2, // BTC -> ETH -> USDC
			description:     "Should find path BTC -> ETH -> USDC",
		},
		{
			name:            "ETH to BNB swap",
			inputToken:      "ETH",
			outputToken:     "BNB",
			amount:          5.0,
			expectedSuccess: true,
			expectedPath:    1, // ETH -> BNB (direct)
			description:     "Should find direct path ETH -> BNB",
		},
		{
			name:            "BNB to USDC swap",
			inputToken:      "BNB",
			outputToken:     "USDC",
			amount:          10.0,
			expectedSuccess: true,
			expectedPath:    1, // BNB -> USDC (direct)
			description:     "Should find direct path BNB -> USDC",
		},
		{
			name:            "BTC to BNB swap",
			inputToken:      "BTC",
			outputToken:     "BNB",
			amount:          1.0,
			expectedSuccess: true,
			expectedPath:    1, // BTC -> BNB (direct)
			description:     "Should find direct path BTC -> BNB",
		},
		{
			name:            "Invalid swap - same tokens",
			inputToken:      "BTC",
			outputToken:     "BTC",
			amount:          1.0,
			expectedSuccess: false,
			expectedPath:    0,
			description:     "Should fail when input and output tokens are the same",
		},
		{
			name:            "Invalid swap - zero amount",
			inputToken:      "BTC",
			outputToken:     "USDC",
			amount:          0.0,
			expectedSuccess: false,
			expectedPath:    0,
			description:     "Should fail when amount is zero",
		},
		{
			name:            "Invalid swap - negative amount",
			inputToken:      "BTC",
			outputToken:     "USDC",
			amount:          -1.0,
			expectedSuccess: false,
			expectedPath:    0,
			description:     "Should fail when amount is negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create swap request
			request := SwapRequest{
				InputToken:  tt.inputToken,
				OutputToken: tt.outputToken,
				Amount:      tt.amount,
			}

			// Marshal request to simulate task data
			requestData, err := json.Marshal(request)
			require.NoError(t, err)

			// Create task
			task := types.Task{
				ID:        nil, // Will be set by the system
				Type:      avstypes.TaskTypeSwap,
				Data:      requestData,
				Timestamp: time.Now().UTC(),
			}

			// Execute the swap
			result, err := handler.Execute(ctx, task)

			if tt.expectedSuccess {
				// Should succeed
				require.NoError(t, err)
				require.NotNil(t, result)

				// Parse the response
				var response SwapResponse
				err = json.Unmarshal(result.Result, &response)
				require.NoError(t, err)

				// Verify response structure
				assert.Equal(t, tt.inputToken, response.InputToken)
				assert.Equal(t, tt.outputToken, response.OutputToken)
				assert.Equal(t, tt.amount, response.Amount)
				assert.True(t, response.Success)
				assert.Len(t, response.Path, tt.expectedPath)

				// Verify execution log is present
				assert.NotEmpty(t, response.ExecutionLog)

				// Verify path starts and ends correctly
				if len(response.Path) > 0 {
					assert.Equal(t, tt.inputToken, response.Path[0].FromToken)
					assert.Equal(t, tt.outputToken, response.Path[len(response.Path)-1].ToToken)
				}

				t.Logf("✅ %s", tt.description)
				t.Logf("   Path: %v", response.Path)
				t.Logf("   Execution log entries: %d", len(response.ExecutionLog))

			} else {
				// Should fail
				assert.Error(t, err)
				t.Logf("✅ %s", tt.description)
			}
		})
	}
}

func TestFindPath(t *testing.T) {
	// Initialize test vaults
	vaults := VaultMap{
		"ETH/USDC": &Vault{TokenA: "ETH", TokenB: "USDC", BalanceA: 100, BalanceB: 1000},
		"BNB/ETH":  &Vault{TokenA: "BNB", TokenB: "ETH", BalanceA: 250, BalanceB: 50},
		"BNB/USDC": &Vault{TokenA: "BNB", TokenB: "USDC", BalanceA: 200, BalanceB: 1200},
		"BTC/ETH":  &Vault{TokenA: "BTC", TokenB: "ETH", BalanceA: 10, BalanceB: 100},
		"BNB/BTC":  &Vault{TokenA: "BNB", TokenB: "BTC", BalanceA: 400, BalanceB: 20},
	}

	tests := []struct {
		name           string
		inputToken     string
		outputToken    string
		expectedFound  bool
		expectedLength int
		description    string
	}{
		{
			name:           "Direct path BTC -> ETH",
			inputToken:     "BTC",
			outputToken:    "ETH",
			expectedFound:  true,
			expectedLength: 1,
			description:    "Should find direct path BTC -> ETH",
		},
		{
			name:           "Multi-hop path BTC -> USDC",
			inputToken:     "BTC",
			outputToken:    "USDC",
			expectedFound:  true,
			expectedLength: 2, // BTC -> ETH -> USDC
			description:    "Should find path BTC -> ETH -> USDC",
		},
		{
			name:           "Direct path ETH -> BNB",
			inputToken:     "ETH",
			outputToken:    "BNB",
			expectedFound:  true,
			expectedLength: 1,
			description:    "Should find direct path ETH -> BNB",
		},
		{
			name:           "No path available",
			inputToken:     "BTC",
			outputToken:    "INVALID",
			expectedFound:  false,
			expectedLength: 0,
			description:    "Should not find path for invalid token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, found := FindPath(vaults, tt.inputToken, tt.outputToken)

			assert.Equal(t, tt.expectedFound, found)
			if found {
				assert.Len(t, path, tt.expectedLength)

				// Verify path starts and ends correctly
				if len(path) > 0 {
					assert.Equal(t, tt.inputToken, path[0].FromToken)
					assert.Equal(t, tt.outputToken, path[len(path)-1].ToToken)
				}

				// Verify path continuity
				for i := 0; i < len(path)-1; i++ {
					assert.Equal(t, path[i].ToToken, path[i+1].FromToken)
				}
			}

			t.Logf("✅ %s", tt.description)
			if found {
				t.Logf("   Path: %v", path)
			}
		})
	}
}

func TestPerformSwap(t *testing.T) {
	ctx := context.Background()

	// Initialize test vaults
	vaults := VaultMap{
		"ETH/USDC": &Vault{TokenA: "ETH", TokenB: "USDC", BalanceA: 100, BalanceB: 1000},
		"BNB/ETH":  &Vault{TokenA: "BNB", TokenB: "ETH", BalanceA: 250, BalanceB: 50},
		"BNB/USDC": &Vault{TokenA: "BNB", TokenB: "USDC", BalanceA: 200, BalanceB: 1200},
		"BTC/ETH":  &Vault{TokenA: "BTC", TokenB: "ETH", BalanceA: 10, BalanceB: 100},
		"BNB/BTC":  &Vault{TokenA: "BNB", TokenB: "BTC", BalanceA: 400, BalanceB: 20},
	}

	// Save original balances for verification
	originalBalances := make(map[string][2]float64)
	for key, v := range vaults {
		originalBalances[key] = [2]float64{v.BalanceA, v.BalanceB}
	}

	t.Run("Successful BTC to USDC swap", func(t *testing.T) {
		// Perform swap
		executionLog, success := PerformSwap(ctx, vaults, "BTC", "USDC", 2.0)

		// Verify success
		assert.True(t, success)
		assert.NotEmpty(t, executionLog)

		// Verify vaults are rebalanced (should be back to original state)
		for key, v := range vaults {
			original := originalBalances[key]
			assert.Equal(t, original[0], v.BalanceA, "Vault %s TokenA balance should be restored", key)
			assert.Equal(t, original[1], v.BalanceB, "Vault %s TokenB balance should be restored", key)
		}

		t.Logf("✅ BTC to USDC swap completed successfully")
		t.Logf("   Execution log entries: %d", len(executionLog))
	})

	t.Run("Failed swap - invalid path", func(t *testing.T) {
		// Try to swap to a non-existent token
		executionLog, success := PerformSwap(ctx, vaults, "BTC", "INVALID", 2.0)

		// Verify failure
		assert.False(t, success)
		assert.NotEmpty(t, executionLog)

		t.Logf("✅ Invalid swap correctly failed")
	})
}

func TestVaultKey(t *testing.T) {
	tests := []struct {
		tokenA      string
		tokenB      string
		expected    string
		description string
	}{
		{"BTC", "ETH", "BTC/ETH", "Should return BTC/ETH when BTC < ETH"},
		{"ETH", "BTC", "BTC/ETH", "Should return BTC/ETH when ETH > BTC"},
		{"USDC", "ETH", "ETH/USDC", "Should return ETH/USDC when USDC < ETH"},
		{"ETH", "USDC", "ETH/USDC", "Should return ETH/USDC when ETH > USDC"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := vaultKey(tt.tokenA, tt.tokenB)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSwapHandler_Verify(t *testing.T) {
	handler := &SwapHandler{}
	ctx := context.Background()

	t.Run("Valid swap verification", func(t *testing.T) {
		// Create a valid swap request
		request := SwapRequest{
			InputToken:  "BTC",
			OutputToken: "USDC",
			Amount:      2.0,
		}

		// Create a valid response
		response := SwapResponse{
			InputToken:  "BTC",
			OutputToken: "USDC",
			Amount:      2.0,
			Path: []avstypes.Transfer{
				{FromToken: "BTC", ToToken: "ETH", Amount: 2.0},
				{FromToken: "ETH", ToToken: "USDC", Amount: 2.0},
			},
			Success: true,
		}

		// Create signed result
		requestData, _ := json.Marshal(request)
		responseData, _ := json.Marshal(response)

		signedResult := types.SignedResult{
			Message: &types.TaskResult{
				Task: &types.Task{
					Data: requestData,
				},
				Result: responseData,
			},
		}

		// Verify should succeed
		err := handler.Verify(ctx, signedResult)
		assert.NoError(t, err)

		t.Logf("✅ Valid swap verification passed")
	})

	t.Run("Invalid swap verification - mismatched parameters", func(t *testing.T) {
		// Create a request
		request := SwapRequest{
			InputToken:  "BTC",
			OutputToken: "USDC",
			Amount:      2.0,
		}

		// Create a response with mismatched parameters
		response := SwapResponse{
			InputToken:  "BTC",
			OutputToken: "ETH", // Different output token
			Amount:      2.0,
			Path: []avstypes.Transfer{
				{FromToken: "BTC", ToToken: "ETH", Amount: 2.0},
			},
			Success: true,
		}

		// Create signed result
		requestData, _ := json.Marshal(request)
		responseData, _ := json.Marshal(response)

		signedResult := types.SignedResult{
			Message: &types.TaskResult{
				Task: &types.Task{
					Data: requestData,
				},
				Result: responseData,
			},
		}

		// Verify should fail
		err := handler.Verify(ctx, signedResult)
		assert.Error(t, err)

		t.Logf("✅ Invalid swap verification correctly failed")
	})
}

// Benchmark tests for performance
func BenchmarkSwapHandler_Execute(b *testing.B) {
	handler := &SwapHandler{}
	ctx := context.Background()

	request := SwapRequest{
		InputToken:  "BTC",
		OutputToken: "USDC",
		Amount:      2.0,
	}

	requestData, _ := json.Marshal(request)
	task := types.Task{
		Type: avstypes.TaskTypeSwap,
		Data: requestData,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := handler.Execute(ctx, task)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindPath(b *testing.B) {
	vaults := VaultMap{
		"ETH/USDC": &Vault{TokenA: "ETH", TokenB: "USDC", BalanceA: 100, BalanceB: 1000},
		"BNB/ETH":  &Vault{TokenA: "BNB", TokenB: "ETH", BalanceA: 250, BalanceB: 50},
		"BNB/USDC": &Vault{TokenA: "BNB", TokenB: "USDC", BalanceA: 200, BalanceB: 1200},
		"BTC/ETH":  &Vault{TokenA: "BTC", TokenB: "ETH", BalanceA: 10, BalanceB: 100},
		"BNB/BTC":  &Vault{TokenA: "BNB", TokenB: "BTC", BalanceA: 400, BalanceB: 20},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FindPath(vaults, "BTC", "USDC")
	}
}
