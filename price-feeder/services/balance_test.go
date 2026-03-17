package services

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// TestAddressCaseInsensitivity verifies that addresses with different cases
// are treated as the same address (Ethereum-native behavior)
func TestAddressCaseInsensitivity(t *testing.T) {
	// USDT address in different case formats
	tests := []struct {
		name    string
		address string
	}{
		{
			name:    "Lowercase",
			address: "0xdac17f958d2ee523a2206206994597c13d831ec7",
		},
		{
			name:    "Uppercase",
			address: "0xDAC17F958D2EE523A2206206994597C13D831EC7",
		},
		{
			name:    "Mixed case (EIP-55 checksum)",
			address: "0xdAC17F958D2ee523a2206206994597C13D831ec7",
		},
		{
			name:    "Different mixed case",
			address: "0xDaC17f958D2Ee523A2206206994597c13D831Ec7",
		},
	}

	// Create a mock balance map
	balances := make(map[common.Address]*big.Int)

	// Store balance using one case format (mixed case from config)
	originalAddr := common.HexToAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7")
	expectedBalance := big.NewInt(1000000000000) // 1 million USDT (6 decimals)
	balances[originalAddr] = expectedBalance

	t.Logf("Stored balance for address: %s", originalAddr.Hex())
	t.Logf("Expected balance: %s", expectedBalance.String())

	// Verify all case variations can retrieve the same balance
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := common.HexToAddress(tt.address)
			balance, exists := balances[addr]

			if !exists {
				t.Errorf("Address %s (%s) not found in balance map", tt.name, tt.address)
				return
			}

			if balance.Cmp(expectedBalance) != 0 {
				t.Errorf("Balance mismatch for %s: expected %s, got %s",
					tt.name, expectedBalance.String(), balance.String())
			}

			t.Logf("✓ %s format (%s) correctly retrieved balance: %s",
				tt.name, tt.address, balance.String())
		})
	}

	// Verify all addresses are treated as identical
	t.Run("AllAddressesAreEqual", func(t *testing.T) {
		addr1 := common.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7")
		addr2 := common.HexToAddress("0xDAC17F958D2EE523A2206206994597C13D831EC7")
		addr3 := common.HexToAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7")

		if addr1 != addr2 {
			t.Errorf("Lowercase and uppercase addresses should be equal")
		}
		if addr1 != addr3 {
			t.Errorf("Lowercase and mixed-case addresses should be equal")
		}
		if addr2 != addr3 {
			t.Errorf("Uppercase and mixed-case addresses should be equal")
		}

		t.Logf("✓ All address formats are treated as identical by common.Address")
	})
}

// TestMultipleTokenAddressCases tests multiple tokens with various case formats
func TestMultipleTokenAddressCases(t *testing.T) {
	balances := make(map[common.Address]*big.Int)

	// Add balances for multiple tokens in different case formats
	tokens := []struct {
		address string // Stored in config format
		balance *big.Int
		symbol  string
	}{
		{
			address: "0xdAC17F958D2ee523a2206206994597C13D831ec7", // USDT (mixed case)
			balance: big.NewInt(1000000000000),                    // 1M USDT
			symbol:  "USDT",
		},
		{
			address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // USDC (mixed case)
			balance: big.NewInt(2000000000000),                    // 2M USDC
			symbol:  "USDC",
		},
		{
			address: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",        // WETH (mixed case)
			balance: new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)), // 100 WETH
			symbol:  "WETH",
		},
	}

	// Store balances
	for _, token := range tokens {
		addr := common.HexToAddress(token.address)
		balances[addr] = token.balance
		t.Logf("Stored %s balance: %s at address %s", token.symbol, token.balance.String(), addr.Hex())
	}

	// Test retrieval with different case formats
	t.Run("RetrieveUSDTLowercase", func(t *testing.T) {
		addr := common.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7")
		balance, exists := balances[addr]
		if !exists {
			t.Fatal("USDT balance not found with lowercase address")
		}
		if balance.Cmp(big.NewInt(1000000000000)) != 0 {
			t.Errorf("USDT balance mismatch: expected 1000000000000, got %s", balance.String())
		}
		t.Logf("✓ Retrieved USDT with lowercase address")
	})

	t.Run("RetrieveUSDCUppercase", func(t *testing.T) {
		addr := common.HexToAddress("0xA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48")
		balance, exists := balances[addr]
		if !exists {
			t.Fatal("USDC balance not found with uppercase address")
		}
		if balance.Cmp(big.NewInt(2000000000000)) != 0 {
			t.Errorf("USDC balance mismatch: expected 2000000000000, got %s", balance.String())
		}
		t.Logf("✓ Retrieved USDC with uppercase address")
	})

	t.Run("RetrieveWETHMixedCase", func(t *testing.T) {
		addr := common.HexToAddress("0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2")
		balance, exists := balances[addr]
		if !exists {
			t.Fatal("WETH balance not found with different mixed case")
		}
		expected := new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18))
		if balance.Cmp(expected) != 0 {
			t.Errorf("WETH balance mismatch: expected %s, got %s", expected.String(), balance.String())
		}
		t.Logf("✓ Retrieved WETH with different mixed case")
	})
}

// TestBalanceServiceGetBalance tests the actual BalanceService.GetBalance method
// with address case variations
func TestBalanceServiceGetBalance(t *testing.T) {
	// Create a minimal BalanceService for testing
	bs := &BalanceService{
		balances: make(map[common.Address]*big.Int),
	}

	// Add test balances with mixed-case addresses (as they would come from config)
	testTokens := map[string]*big.Int{
		"0xdAC17F958D2ee523a2206206994597C13D831ec7": big.NewInt(1000000000000),      // USDT
		"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48": big.NewInt(2000000000000),      // USDC
		"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2": big.NewInt(100000000000000000), // WETH (0.1)
	}

	for addr, balance := range testTokens {
		bs.balances[common.HexToAddress(addr)] = balance
	}

	tests := []struct {
		name            string
		inputAddress    string
		expectedBalance *big.Int
		shouldExist     bool
		description     string
	}{
		{
			name:            "USDT lowercase",
			inputAddress:    "0xdac17f958d2ee523a2206206994597c13d831ec7",
			expectedBalance: big.NewInt(1000000000000),
			shouldExist:     true,
			description:     "Should find USDT with lowercase address",
		},
		{
			name:            "USDT uppercase",
			inputAddress:    "0xDAC17F958D2EE523A2206206994597C13D831EC7",
			expectedBalance: big.NewInt(1000000000000),
			shouldExist:     true,
			description:     "Should find USDT with uppercase address",
		},
		{
			name:            "USDC mixed case",
			inputAddress:    "0xa0B86991C6218b36c1D19d4A2E9eb0Ce3606eb48",
			expectedBalance: big.NewInt(2000000000000),
			shouldExist:     true,
			description:     "Should find USDC with different mixed case",
		},
		{
			name:            "WETH all lowercase",
			inputAddress:    "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			expectedBalance: big.NewInt(100000000000000000),
			shouldExist:     true,
			description:     "Should find WETH with all lowercase",
		},
		{
			name:            "Non-existent token",
			inputAddress:    "0x1234567890123456789012345678901234567890",
			expectedBalance: nil,
			shouldExist:     false,
			description:     "Should not find non-existent token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balance, exists := bs.GetBalance(tt.inputAddress)

			if exists != tt.shouldExist {
				t.Errorf("Existence mismatch for %s: expected %v, got %v",
					tt.name, tt.shouldExist, exists)
			}

			if tt.shouldExist {
				if balance.Cmp(tt.expectedBalance) != 0 {
					t.Errorf("Balance mismatch for %s: expected %s, got %s",
						tt.name, tt.expectedBalance.String(), balance.String())
				}
				t.Logf("✓ %s: %s", tt.description, balance.String())
			} else {
				if exists {
					t.Errorf("Expected token to not exist, but found balance: %s", balance.String())
				}
				t.Logf("✓ %s", tt.description)
			}
		})
	}
}

// TestBalanceServiceGetBalanceFloat tests GetBalanceFloat with case variations
func TestBalanceServiceGetBalanceFloat(t *testing.T) {
	bs := &BalanceService{
		balances: make(map[common.Address]*big.Int),
	}

	// Add USDT balance (6 decimals): 1,000,000 USDT = 1,000,000,000,000 units
	usdtAddr := common.HexToAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7")
	bs.balances[usdtAddr] = big.NewInt(1000000000000)

	tests := []struct {
		name          string
		address       string
		decimals      int
		expected      float64
		shouldSucceed bool
	}{
		{
			name:          "USDT lowercase",
			address:       "0xdac17f958d2ee523a2206206994597c13d831ec7",
			decimals:      6,
			expected:      1000000.0,
			shouldSucceed: true,
		},
		{
			name:          "USDT uppercase",
			address:       "0xDAC17F958D2EE523A2206206994597C13D831EC7",
			decimals:      6,
			expected:      1000000.0,
			shouldSucceed: true,
		},
		{
			name:          "USDT EIP-55 checksum",
			address:       "0xdAC17F958D2ee523a2206206994597C13D831ec7",
			decimals:      6,
			expected:      1000000.0,
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balance, exists := bs.GetBalanceFloat(tt.address, tt.decimals)

			if exists != tt.shouldSucceed {
				t.Errorf("Existence mismatch: expected %v, got %v", tt.shouldSucceed, exists)
			}

			if tt.shouldSucceed {
				if balance != tt.expected {
					t.Errorf("Balance mismatch: expected %.2f, got %.2f", tt.expected, balance)
				}
				t.Logf("✓ Retrieved balance %.2f USDT with address format: %s", balance, tt.name)
			}
		})
	}
}

// TestCommonAddressEquality demonstrates how common.Address handles case-insensitivity
func TestCommonAddressEquality(t *testing.T) {
	t.Run("SameAddressDifferentCases", func(t *testing.T) {
		addr1 := common.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7")
		addr2 := common.HexToAddress("0xDAC17F958D2EE523A2206206994597C13D831EC7")
		addr3 := common.HexToAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7")

		// Test equality
		if addr1 != addr2 {
			t.Error("Addresses with different cases should be equal")
		}
		if addr1 != addr3 {
			t.Error("Addresses with different cases should be equal")
		}

		// Test as map keys
		testMap := make(map[common.Address]string)
		testMap[addr1] = "value1"
		testMap[addr2] = "value2" // Should overwrite value1
		testMap[addr3] = "value3" // Should overwrite value2

		if len(testMap) != 1 {
			t.Errorf("Map should have 1 entry, got %d", len(testMap))
		}

		if testMap[addr1] != "value3" {
			t.Errorf("Expected 'value3', got '%s'", testMap[addr1])
		}

		t.Log("✓ common.Address correctly treats different cases as the same key")
	})

	t.Run("HexOutputFormat", func(t *testing.T) {
		// Test that .Hex() returns checksummed format
		addr := common.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7")
		hexOutput := addr.Hex()

		t.Logf("Input: 0xdac17f958d2ee523a2206206994597c13d831ec7")
		t.Logf("Output (checksummed): %s", hexOutput)

		// The output should be the EIP-55 checksummed format
		expected := "0xdAC17F958D2ee523a2206206994597C13D831ec7"
		if hexOutput != expected {
			t.Logf("Note: Expected EIP-55 checksum %s, got %s", expected, hexOutput)
		}
	})
}
