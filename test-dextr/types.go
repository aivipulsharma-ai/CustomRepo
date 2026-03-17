package main

import (
	"sync"
	"time"
)

// Core Data Structures
type Vault struct {
	Balance       float64 `json:"balance"`
	TargetBalance float64 `json:"targetBalance"`
	MinBalance    float64 `json:"minBalance"`
	MaxBalance    float64 `json:"maxBalance"`
	Decimals      int     `json:"decimals"`
}

type MultiTokenVault struct {
	Balances map[string]float64 `json:"balances"`
	mu       sync.RWMutex
}

type PriceLevel struct {
	Size  string `json:"0"`
	Price string `json:"1"`
}

type TradeDirection int

const (
	Buy TradeDirection = iota
	Sell
)

type TokenPair struct {
	Base  string
	Quote string
}

// Configuration Structs
type InventoryParams struct {
	CriticalLowThreshold  float64
	LowThreshold          float64
	HighThreshold         float64
	CriticalHighThreshold float64
	CriticalLowPenalty    float64
	LowPenalty            float64
	HighBonus             float64
	CriticalHighBonus     float64
}

type SizeImpactParams struct {
	SmallTrade   float64
	MediumTrade  float64
	LargeTrade   float64
	SmallImpact  float64
	MediumImpact float64
	LargeImpact  float64
	MaxImpact    float64
}

type CompetitiveParams struct {
	BaseSpread           float64
	MinSpread            float64
	MaxSpread            float64
	VolatilityMultiplier float64
	GasCostBuffer        float64
}

type RebalanceParams struct {
	EpochDuration      time.Duration
	EmergencyThreshold float64
	TargetDeviation    float64
	RebalanceBuffer    float64
}

// Request/Response Structs
type OrderRequest struct {
	BaseToken  string `json:"baseToken"`
	QuoteToken string `json:"quoteToken"`
	Amount     string `json:"amount"`
	Taker      string `json:"taker"`
	FeeBps     int    `json:"feeBps"`
}

type Order struct {
	Maker        string `json:"maker"`
	MakerAsset   string `json:"makerAsset"`
	TakerAsset   string `json:"takerAsset"`
	MakerTraits  string `json:"makerTraits"`
	Salt         string `json:"salt"`
	MakingAmount string `json:"makingAmount"`
	TakingAmount string `json:"takingAmount"`
	Receiver     string `json:"receiver"`
}

type OrderResponse struct {
	Order     Order  `json:"order"`
	Signature string `json:"signature"`
}

type SwapRequest struct {
	InputToken  string  `json:"inputToken"`
	OutputToken string  `json:"outputToken"`
	Amount      float64 `json:"amount"`
	FeeBps      int     `json:"feeBps"`
}

type SwapResponse struct {
	InputAmount  float64 `json:"inputAmount"`
	OutputAmount float64 `json:"outputAmount"`
	FeeAmount    float64 `json:"feeAmount"`
	ExchangeRate float64 `json:"exchangeRate"`
	PriceUsed    float64 `json:"priceUsed"`
	Success      bool    `json:"success"`
	Message      string  `json:"message"`
}

// Default Parameters
var (
	DefaultInventoryParams = InventoryParams{
		CriticalLowThreshold:  0.1,
		LowThreshold:          0.3,
		HighThreshold:         1.7,
		CriticalHighThreshold: 1.9,
		CriticalLowPenalty:    0.02,
		LowPenalty:            0.005,
		HighBonus:             0.003,
		CriticalHighBonus:     0.01,
	}

	DefaultSizeImpactParams = SizeImpactParams{
		SmallTrade:   0.01,
		MediumTrade:  0.05,
		LargeTrade:   0.15,
		SmallImpact:  0.0005,
		MediumImpact: 0.002,
		LargeImpact:  0.008,
		MaxImpact:    0.02,
	}

	DefaultCompetitiveParams = CompetitiveParams{
		BaseSpread:           0.001,
		MinSpread:            0.0005,
		MaxSpread:            0.005,
		VolatilityMultiplier: 2.0,
		GasCostBuffer:        0.0002,
	}

	DefaultRebalanceParams = RebalanceParams{
		EpochDuration:      2 * time.Minute,
		EmergencyThreshold: 0.05,
		TargetDeviation:    0.02,
		RebalanceBuffer:    0.005,
	}
)

// MultiTokenVault Methods
func NewMultiTokenVault() *MultiTokenVault {
	return &MultiTokenVault{
		Balances: make(map[string]float64),
	}
}

func (mtv *MultiTokenVault) GetBalance(token string) float64 {
	mtv.mu.RLock()
	defer mtv.mu.RUnlock()
	return mtv.Balances[token]
}

func (mtv *MultiTokenVault) UpdateBalance(token string, delta float64) {
	mtv.mu.Lock()
	defer mtv.mu.Unlock()

	if mtv.Balances == nil {
		mtv.Balances = make(map[string]float64)
	}

	mtv.Balances[token] += delta
	if mtv.Balances[token] < 0 {
		mtv.Balances[token] = 0
	}
}

func (mtv *MultiTokenVault) GetAllBalances() map[string]float64 {
	mtv.mu.RLock()
	defer mtv.mu.RUnlock()

	result := make(map[string]float64)
	for token, balance := range mtv.Balances {
		result[token] = balance
	}
	return result
}
