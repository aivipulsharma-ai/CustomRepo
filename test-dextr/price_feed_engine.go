package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"sync"
)

type PriceFeedEngine struct {
	vaultManager       *VaultManager
	oracleClient       *OracleClient
	inventoryParams    InventoryParams
	sizeImpactParams   SizeImpactParams
	competitiveParams  CompetitiveParams
	rebalanceParams    RebalanceParams
	currentPriceLevels map[string][]PriceLevel
	priceLevelsMu      sync.RWMutex
}

func NewPriceFeedEngine(vm *VaultManager, oc *OracleClient) *PriceFeedEngine {
	return &PriceFeedEngine{
		vaultManager:       vm,
		oracleClient:       oc,
		inventoryParams:    DefaultInventoryParams,
		sizeImpactParams:   DefaultSizeImpactParams,
		competitiveParams:  DefaultCompetitiveParams,
		rebalanceParams:    DefaultRebalanceParams,
		currentPriceLevels: make(map[string][]PriceLevel),
	}
}

func (pfe *PriceFeedEngine) calculateInventoryBias(baseVault, quoteVault *Vault, direction TradeDirection) float64 {
	var targetVault *Vault
	if direction == Buy {
		targetVault = baseVault
	} else {
		targetVault = quoteVault
	}

	ratio := targetVault.Balance / targetVault.TargetBalance

	switch {
	case ratio < pfe.inventoryParams.CriticalLowThreshold:
		return pfe.inventoryParams.CriticalLowPenalty
	case ratio < pfe.inventoryParams.LowThreshold:
		return pfe.inventoryParams.LowPenalty
	case ratio > pfe.inventoryParams.CriticalHighThreshold:
		return -pfe.inventoryParams.CriticalHighBonus
	case ratio > pfe.inventoryParams.HighThreshold:
		return -pfe.inventoryParams.HighBonus
	default:
		return 0
	}
}

func (pfe *PriceFeedEngine) calculateSizeImpact(tradeSize, vaultBalance float64) float64 {
	ratio := tradeSize / vaultBalance

	switch {
	case ratio < pfe.sizeImpactParams.SmallTrade:
		return pfe.sizeImpactParams.SmallImpact
	case ratio < pfe.sizeImpactParams.MediumTrade:
		return pfe.sizeImpactParams.MediumImpact
	case ratio < pfe.sizeImpactParams.LargeTrade:
		return pfe.sizeImpactParams.LargeImpact
	default:
		return pfe.sizeImpactParams.MaxImpact
	}
}

func (pfe *PriceFeedEngine) generateSizeLevels(maxSize float64) []float64 {
	return []float64{
		maxSize * 0.01,
		maxSize * 0.05,
		maxSize * 0.1,
		maxSize * 0.2,
		maxSize * 0.3,
	}
}

// KEY FIX: Generate price levels with proper direction awareness
func (pfe *PriceFeedEngine) GeneratePriceLevels() map[string][]PriceLevel {
	levels := make(map[string][]PriceLevel)

	// Define the canonical pairs we support (base comes first)
	canonicalPairs := []TokenPair{
		{"BTC", "USDC"}, {"BTC", "USDT"}, {"BTC", "ETH"},
		{"ETH", "USDC"}, {"ETH", "USDT"}, {"USDC", "USDT"},
	}

	for _, pair := range canonicalPairs {
		baseVault := pfe.vaultManager.GetVault(pair.Base)
		quoteVault := pfe.vaultManager.GetVault(pair.Quote)

		if baseVault == nil || quoteVault == nil {
			continue
		}

		// Generate levels for canonical direction (e.g., BTC_USDC)
		buyLevels := pfe.calculateLevelsForDirection(pair.Base, pair.Quote, Buy)

		// Generate levels for reverse direction (e.g., USDC_BTC) by inverting prices
		sellLevels := pfe.calculateReverseLevels(pair.Base, pair.Quote)

		// Store both directions
		levels[fmt.Sprintf("%s_%s", pair.Base, pair.Quote)] = buyLevels
		levels[fmt.Sprintf("%s_%s", pair.Quote, pair.Base)] = sellLevels
	}

	// Cache the current price levels
	pfe.priceLevelsMu.Lock()
	pfe.currentPriceLevels = levels
	pfe.priceLevelsMu.Unlock()

	return levels
}

func (pfe *PriceFeedEngine) calculateLevelsForDirection(baseToken, quoteToken string, direction TradeDirection) []PriceLevel {
	baseVault := pfe.vaultManager.GetVault(baseToken)
	quoteVault := pfe.vaultManager.GetVault(quoteToken)

	oraclePrice, err := pfe.oracleClient.GetPrice(baseToken, quoteToken)
	if err != nil {
		log.Printf("Error getting price for %s_%s: %v", baseToken, quoteToken, err)
		return []PriceLevel{}
	}

	inventoryBias := pfe.calculateInventoryBias(baseVault, quoteVault, direction)

	balancerBalance := pfe.vaultManager.BalancerVault.GetBalance(quoteToken)
	totalAvailable := balancerBalance + quoteVault.Balance
	maxTradeSize := math.Min(baseVault.Balance*0.3, totalAvailable/(oraclePrice*0.3))

	sizeLevels := pfe.generateSizeLevels(maxTradeSize)
	var priceLevels []PriceLevel

	for _, size := range sizeLevels {
		sizeImpact := pfe.calculateSizeImpact(size, baseVault.Balance)
		competitiveSpread := pfe.competitiveParams.BaseSpread

		adjustedPrice := oraclePrice * (1 + inventoryBias + sizeImpact + competitiveSpread)

		priceLevels = append(priceLevels, PriceLevel{
			Size:  fmt.Sprintf("%.8f", size),
			Price: fmt.Sprintf("%.8f", adjustedPrice),
		})
	}

	return priceLevels
}

// KEY FIX: Calculate reverse levels properly
func (pfe *PriceFeedEngine) calculateReverseLevels(baseToken, quoteToken string) []PriceLevel {
	// Get the canonical direction levels first
	canonicalLevels := pfe.calculateLevelsForDirection(baseToken, quoteToken, Buy)

	// Get base price for reverse calculation
	basePrice, err := pfe.oracleClient.GetPrice(baseToken, quoteToken)
	if err != nil {
		return []PriceLevel{}
	}

	var reverseLevels []PriceLevel

	for _, level := range canonicalLevels {
		// For reverse direction, we need to:
		// 1. Invert the price (USDC/BTC instead of BTC/USDC)
		// 2. Calculate appropriate sizes in the quote token

		price, _ := strconv.ParseFloat(level.Price, 64)
		size, _ := strconv.ParseFloat(level.Size, 64)

		reversePrice := 1.0 / price
		// Size should be in quote token units (e.g., if level is 0.1 BTC,
		// reverse should be 0.1 * price USDC)
		reverseSize := size * basePrice

		reverseLevels = append(reverseLevels, PriceLevel{
			Size:  fmt.Sprintf("%.8f", reverseSize),
			Price: fmt.Sprintf("%.8f", reversePrice),
		})
	}

	return reverseLevels
}

func (pfe *PriceFeedEngine) GetPriceLevelsForPair(baseToken, quoteToken string) ([]PriceLevel, error) {
	pfe.priceLevelsMu.RLock()
	defer pfe.priceLevelsMu.RUnlock()

	key := fmt.Sprintf("%s_%s", baseToken, quoteToken)
	if levels, exists := pfe.currentPriceLevels[key]; exists {
		return levels, nil
	}

	return nil, fmt.Errorf("price levels not found for pair %s_%s", baseToken, quoteToken)
}

func (pfe *PriceFeedEngine) ProcessOrder(req OrderRequest) (*OrderResponse, error) {
	inputToken := req.BaseToken
	outputToken := req.QuoteToken

	amount, err := strconv.ParseFloat(req.Amount, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %v", err)
	}

	feeAmount := amount * float64(req.FeeBps) / 10000

	// KEY FIX: Get price levels for the correct direction
	priceLevels, err := pfe.GetPriceLevelsForPair(inputToken, outputToken)
	if err != nil {
		return nil, fmt.Errorf("cannot get price levels: %v", err)
	}

	canFulfill, _, err := pfe.vaultManager.CanFulfillSwap(inputToken, outputToken, amount-feeAmount)
	if !canFulfill {
		return nil, fmt.Errorf("cannot fulfill swap: %v", err)
	}

	actualOutput, err := pfe.vaultManager.ExecuteSwap(inputToken, outputToken, amount, feeAmount, priceLevels)
	if err != nil {
		return nil, fmt.Errorf("swap execution failed: %v", err)
	}

	log.Printf("Order processed: %.6f %s -> %.6f %s (fee: %.6f %s)",
		amount, inputToken, actualOutput, outputToken, feeAmount, inputToken)

	order := Order{
		Maker:        "0x6edc317f3208b10c46f4ff97faa04dd632487408",
		MakerAsset:   outputToken,
		TakerAsset:   inputToken,
		MakerTraits:  "10583315370300944271581153473907065910280",
		Salt:         fmt.Sprintf("%d", rand.Int63()),
		MakingAmount: fmt.Sprintf("%.0f", actualOutput*1e6),
		TakingAmount: fmt.Sprintf("%.0f", amount*1e18),
		Receiver:     "0x0000000000000000000000000000000000000000",
	}

	return &OrderResponse{
		Order:     order,
		Signature: "0xe07cb361731ce320289ca0502910423e6e0201c1fa2f5efbc5c9287855ca5abe330eb0c6bb9b7d08ccab305b3be702353f87952ff2769277fc4c4f85dd123d531b",
	}, nil
}

// Rebalancing methods
func (pfe *PriceFeedEngine) ExecuteRebalance() {
	log.Println("=== Starting Rebalance ===")
	pfe.rebalanceFromBalancerToMain()
	pfe.fillGapsFromBalancer()
	log.Println("=== Rebalance Complete ===")
}

func (pfe *PriceFeedEngine) rebalanceFromBalancerToMain() {
	log.Println("Phase 1: Moving tokens from balancer to main vaults")

	pfe.vaultManager.mu.Lock()
	defer pfe.vaultManager.mu.Unlock()

	for token, vault := range pfe.vaultManager.Vaults {
		deficit := vault.TargetBalance - vault.Balance

		if deficit > 0 {
			balancerBalance := pfe.vaultManager.BalancerVault.GetBalance(token)
			transferAmount := math.Min(deficit, balancerBalance)

			if transferAmount > 0 {
				pfe.vaultManager.BalancerVault.UpdateBalance(token, -transferAmount)
				vault.Balance += transferAmount
				log.Printf("Moved %.6f %s from balancer to main vault (deficit: %.6f)",
					transferAmount, token, deficit)
			}
		}
	}
}

func (pfe *PriceFeedEngine) fillGapsFromBalancer() {
	log.Println("Phase 2: Filling remaining gaps using balancer vault")

	pfe.vaultManager.mu.Lock()
	defer pfe.vaultManager.mu.Unlock()

	rebalanceActions := []string{}

	for token, vault := range pfe.vaultManager.Vaults {
		deviation := vault.Balance - vault.TargetBalance
		deviationPct := deviation / vault.TargetBalance

		if math.Abs(deviationPct) > pfe.rebalanceParams.RebalanceBuffer {
			if deviation > 0 {
				excessAmount := deviation * 0.5
				vault.Balance -= excessAmount
				pfe.vaultManager.BalancerVault.UpdateBalance(token, excessAmount)
				rebalanceActions = append(rebalanceActions,
					fmt.Sprintf("MOVED %.6f %s to balancer (excess: %.2f%%)",
						excessAmount, token, deviationPct*100))
			} else {
				deficitAmount := -deviation
				balancerBalance := pfe.vaultManager.BalancerVault.GetBalance(token)
				transferAmount := math.Min(deficitAmount*0.5, balancerBalance)

				if transferAmount > 0 {
					pfe.vaultManager.BalancerVault.UpdateBalance(token, -transferAmount)
					vault.Balance += transferAmount
					rebalanceActions = append(rebalanceActions,
						fmt.Sprintf("FILLED %.6f %s from balancer (deficit: %.2f%%)",
							transferAmount, token, deviationPct*100))
				} else {
					rebalanceActions = append(rebalanceActions,
						fmt.Sprintf("CANNOT FILL %s deficit (%.2f%%) - insufficient balancer balance",
							token, deviationPct*100))
				}
			}
		}
	}

	if len(rebalanceActions) > 0 {
		log.Printf("Rebalance actions: %v", rebalanceActions)
	} else {
		log.Println("No rebalancing needed - all vaults within tolerance")
	}
}
