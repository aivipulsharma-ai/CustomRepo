package main

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"sync"
)

type VaultManager struct {
	Vaults             map[string]*Vault `json:"vaults"`
	BalancerVault      *MultiTokenVault  `json:"balancerVault"`
	FeeCollectionVault *MultiTokenVault  `json:"feeCollectionVault"`
	mu                 sync.RWMutex
}

func NewVaultManager() *VaultManager {
	vaults := map[string]*Vault{
		"BTC": {
			Balance:       10.0,
			TargetBalance: 10.0,
			MinBalance:    8.0,
			MaxBalance:    12.0,
			Decimals:      8,
		},
		"ETH": {
			Balance:       150.0,
			TargetBalance: 150.0,
			MinBalance:    120.0,
			MaxBalance:    180.0,
			Decimals:      18,
		},
		"USDC": {
			Balance:       500000.0,
			TargetBalance: 500000.0,
			MinBalance:    400000.0,
			MaxBalance:    600000.0,
			Decimals:      6,
		},
		"USDT": {
			Balance:       300000.0,
			TargetBalance: 300000.0,
			MinBalance:    240000.0,
			MaxBalance:    360000.0,
			Decimals:      6,
		},
	}

	// Initialize balancer vault with 10% of each vault's initial balance
	balancerVault := NewMultiTokenVault()
	for token, vault := range vaults {
		initialBalancerAmount := vault.TargetBalance * 0.3
		balancerVault.UpdateBalance(token, initialBalancerAmount)
	}

	return &VaultManager{
		Vaults:             vaults,
		BalancerVault:      balancerVault,
		FeeCollectionVault: NewMultiTokenVault(),
	}
}

func (vm *VaultManager) GetVault(token string) *Vault {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.Vaults[token]
}

func (vm *VaultManager) UpdateBalance(token string, delta float64) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	if vault, exists := vm.Vaults[token]; exists {
		vault.Balance += delta
		log.Printf("Updated %s vault balance: %.6f (delta: %.6f)", token, vault.Balance, delta)
	}
}

func (vm *VaultManager) CanFulfillSwap(inputToken string, outputToken string, inputAmount float64) (bool, float64, error) {
	balancerBalance := vm.BalancerVault.GetBalance(outputToken)
	mainVaultBalance := 0.0

	if vault := vm.GetVault(outputToken); vault != nil {
		mainVaultBalance = vault.Balance
	}

	totalAvailable := balancerBalance + mainVaultBalance
	return totalAvailable > 0, totalAvailable, nil
}

func (vm *VaultManager) findPriceFromLevels(priceLevels []PriceLevel, amount float64) (float64, error) {
	if len(priceLevels) == 0 {
		return 0, fmt.Errorf("no price levels available")
	}

	// Find the level with size closest to our amount
	bestLevel := priceLevels[0]
	bestDiff := math.MaxFloat64

	for _, level := range priceLevels {
		levelSize, err := strconv.ParseFloat(level.Size, 64)
		if err != nil {
			continue
		}

		diff := math.Abs(levelSize - amount)
		if diff < bestDiff {
			bestDiff = diff
			bestLevel = level
		}
	}

	price, err := strconv.ParseFloat(bestLevel.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price in level: %v", err)
	}

	return price, nil
}

func (vm *VaultManager) ExecuteSwap(inputToken, outputToken string, inputAmount, feeAmount float64, priceLevels []PriceLevel) (float64, error) {
	// Calculate net input after fee deduction
	netInputAmount := inputAmount - feeAmount

	// Find the appropriate price from price levels
	price, err := vm.findPriceFromLevels(priceLevels, netInputAmount)
	if err != nil {
		return 0, fmt.Errorf("cannot find price for amount %.6f: %v", netInputAmount, err)
	}

	outputAmount := netInputAmount * price

	// Check if we have enough output tokens
	balancerBalance := vm.BalancerVault.GetBalance(outputToken)
	mainVaultBalance := 0.0
	if vault := vm.GetVault(outputToken); vault != nil {
		mainVaultBalance = vault.Balance
	}
	totalAvailable := balancerBalance + mainVaultBalance

	if totalAvailable < outputAmount {
		return 0, fmt.Errorf("insufficient liquidity: need %.6f %s, have %.6f", outputAmount, outputToken, totalAvailable)
	}

	// 1. Collect fee from input token
	vm.FeeCollectionVault.UpdateBalance(inputToken, feeAmount)
	log.Printf("Fee collected: %.6f %s", feeAmount, inputToken)

	// 2. Add net input tokens to balancer vault
	vm.BalancerVault.UpdateBalance(inputToken, netInputAmount)
	log.Printf("Balancer vault received %.6f %s (after fee)", netInputAmount, inputToken)

	// 3. Provide output tokens to user (from balancer vault first, then main vault)
	if balancerBalance >= outputAmount {
		vm.BalancerVault.UpdateBalance(outputToken, -outputAmount)
		log.Printf("Balancer vault provided %.6f %s", outputAmount, outputToken)
	} else {
		if balancerBalance > 0 {
			vm.BalancerVault.UpdateBalance(outputToken, -balancerBalance)
			log.Printf("Balancer vault provided %.6f %s", balancerBalance, outputToken)
		}

		needed := outputAmount - balancerBalance
		vm.UpdateBalance(outputToken, -needed)
		log.Printf("Main vault provided %.6f %s", needed, outputToken)
	}

	log.Printf("Swap executed at price %.6f: %.6f %s -> %.6f %s",
		price, netInputAmount, inputToken, outputAmount, outputToken)

	return outputAmount, nil
}

func (vm *VaultManager) GetStatus() map[string]interface{} {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	status := make(map[string]interface{})

	// Main vaults status
	mainVaults := make(map[string]interface{})
	for token, vault := range vm.Vaults {
		deviation := (vault.Balance - vault.TargetBalance) / vault.TargetBalance
		mainVaults[token] = map[string]interface{}{
			"balance":      vault.Balance,
			"target":       vault.TargetBalance,
			"deviation":    deviation,
			"deviationPct": deviation * 100,
		}
	}

	status["mainVaults"] = mainVaults
	status["balancerVault"] = vm.BalancerVault.GetAllBalances()
	status["feeCollectionVault"] = vm.FeeCollectionVault.GetAllBalances()

	return status
}
