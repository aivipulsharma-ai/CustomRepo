package models

import (
	"time"
)

type LevelsRequest struct {
	TokenIn  string `json:"tokenIn" validate:"required"`
	TokenOut string `json:"tokenOut" validate:"required"`
	Amount   string `json:"amount" validate:"required"`
}

type Level struct {
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
}

type LevelsResponse struct {
	Levels []Level `json:"levels"`
	TTL    int64   `json:"ttl"`
}

type AllLevelsResponse map[string][][2]string

type OrderRequest struct {
	BaseToken  string `json:"baseToken" validate:"required"`
	QuoteToken string `json:"quoteToken" validate:"required"`
	Amount     string `json:"amount" validate:"required"`
	Taker      string `json:"taker" validate:"required"`
	FeeBps     int    `json:"feeBps" validate:"required"`
}

// RFQOrder represents the 1inch RFQ order structure
type RFQOrder struct {
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
	Order     *RFQOrder `json:"order,omitempty"`
	Signature string    `json:"signature,omitempty"`
}

// OrderProcessingResult contains detailed information about order processing
type OrderProcessingResult struct {
	Success       bool
	AcceptedPrice float64 // The exchange rate at which order was accepted
	BaseTokenUSD  float64 // USD price of base token
	QuoteTokenUSD float64 // USD price of quote token
	AmountFloat   float64 // Order amount as float
	VolumeUSD     float64 // Total trade volume in USD
	FeeUSD        float64 // Fee amount in USD
}

type TradeHit struct {
	ID            string    `json:"id"`
	BaseToken     string    `json:"baseToken"`
	QuoteToken    string    `json:"quoteToken"`
	Amount        string    `json:"amount"`
	AcceptedPrice string    `json:"acceptedPrice"` // The price at which order was accepted
	VolumeUSD     string    `json:"volumeUsd"`     // Trade volume in USD
	Taker         string    `json:"taker"`
	FeeBps        int       `json:"feeBps"`
	FeeAmount     string    `json:"feeAmount"` // Calculated fee amount
	Timestamp     time.Time `json:"timestamp"`
	Success       bool      `json:"success"`
}

type Statistics struct {
	TotalHits           int64                 `json:"totalHits"`
	SuccessfulHits      int64                 `json:"successfulHits"`
	FailedHits          int64                 `json:"failedHits"`
	HitsPerMinute       float64               `json:"hitsPerMinute"`
	HitsPerHour         float64               `json:"hitsPerHour"`
	AverageResponseTime time.Duration         `json:"averageResponseTime"`
	LastHit             *time.Time            `json:"lastHit,omitempty"`
	TokenPairStats      map[string]*PairStats `json:"tokenPairStats"`
	HourlyBreakdown     map[string]int64      `json:"hourlyBreakdown"`
	Uptime              time.Duration         `json:"uptime"`
	TotalVolumeUSD      float64               `json:"totalVolumeUsd"`      // Total trading volume across all pairs
	TotalFeesUSD        float64               `json:"totalFeesUsd"`        // Total fees collected
	AverageOrderSizeUSD float64               `json:"averageOrderSizeUsd"` // Average order size in USD
}

type PairStats struct {
	TotalHits        int64     `json:"totalHits"`
	SuccessfulHits   int64     `json:"successfulHits"`
	FailedHits       int64     `json:"failedHits"`
	LastPrice        string    `json:"lastPrice"` // Last accepted price
	LastHit          time.Time `json:"lastHit"`
	TotalVolumeUSD   float64   `json:"totalVolumeUsd"`   // Cumulative volume in USD
	AverageTradeSize float64   `json:"averageTradeSize"` // Average order size in base token
	TotalFeesUSD     float64   `json:"totalFeesUsd"`     // Total fees collected in USD
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}
