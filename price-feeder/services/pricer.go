package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/0xcatalysis/core/go-sdk/errors"
	"github.com/0xcatalysis/core/go-sdk/log"
	"github.com/0xcatalysis/core/go-sdk/z"

	"github.com/dextr_avs/price-feeder/config"
	"github.com/dextr_avs/price-feeder/models"
)

// ChaosLabsPrice represents a single price in the API response
type ChaosLabsPrice struct {
	FeedID    string `json:"feedId"`
	Price     int64  `json:"price"`
	Timestamp int64  `json:"ts"`
	Expo      int    `json:"expo"`
	Signature string `json:"signature"`
}

// ChaosLabsMultiResponse represents the multi-feed API response structure
type ChaosLabsMultiResponse struct {
	Prices []ChaosLabsPrice `json:"prices"`
}

type PricerService struct {
	config         *config.Config
	prices         map[string]float64
	mu             sync.RWMutex
	httpClient     *http.Client
	metrics        *MetricsService
	balanceService *BalanceService
}

func NewPricerService(ctx context.Context, cfg *config.Config, metrics *MetricsService) (*PricerService, error) {
	ctx = log.WithTopic(ctx, "pricer")
	// Initialize balance service
	balanceService, err := NewBalanceService(ctx, cfg, metrics)
	if err != nil {
		log.Warn(ctx, "Failed to initialize balance service, tokens without balance will not be quoted", err)
		return nil, err
	}

	ps := &PricerService{
		config:         cfg,
		prices:         make(map[string]float64),
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		metrics:        metrics,
		balanceService: balanceService,
	}

	if err = ps.updatePrices(ctx); err != nil {
		log.Error(ctx, "Failed to update prices", err)
		return nil, err
	}
	go ps.startPriceUpdates(ctx)

	return ps, nil
}

func (ps *PricerService) fetchAllPricesFromAPIWithRetry(ctx context.Context) (map[string]float64, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		prices, err := ps.fetchAllPricesFromAPI(ctx)
		if err == nil {
			return prices, nil
		}

		lastErr = err

		// Don't retry if context is already cancelled
		if ctx.Err() != nil {
			break
		}

		// Exponential backoff: 2s, 4s, 8s
		if attempt < maxRetries {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			log.Debug(ctx, "Retrying Oracle API fetch after backoff",
				z.Int("attempt", attempt),
				z.Any("backoff", backoff),
			)

			select {
			case <-time.After(backoff):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return nil, errors.Wrap(lastErr, "failed to fetch prices after retries")
}

func (ps *PricerService) fetchAllPricesFromAPI(ctx context.Context) (map[string]float64, error) {
	// Build feedIds parameter from all tokens
	var feedIDs []string
	for _, token := range ps.config.Tokens {
		// Map WETH to ETH since they have the same price and API provides ETH
		symbol := token.BaseSymbol
		if strings.ToUpper(symbol) == "WETH" {
			symbol = "ETH"
		}
		feedID := strings.ToUpper(symbol) + "USD"
		feedIDs = append(feedIDs, feedID)
	}

	// Use the multi-feed endpoint
	url := "https://oracle-staging.chaoslabs.co/prices/evm/crypto"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		ps.metrics.RecordPricerError("create_request_failed")
		return nil, errors.Wrap(err, "failed to create request")
	}

	// Add feedIds as query parameter
	q := req.URL.Query()
	q.Add("feedIds", strings.Join(feedIDs, ","))
	req.URL.RawQuery = q.Encode()

	// Use Authorization header
	req.Header.Set("Authorization", ps.config.Pricing.ChaosLabsAPIKey)

	resp, err := ps.httpClient.Do(req)
	if err != nil {
		ps.metrics.RecordOracleError()
		ps.metrics.RecordPricerError("api_request_failed")
		return nil, errors.Wrap(err, "failed to fetch prices")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warn(context.Background(), "Failed to close response body", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		ps.metrics.RecordOracleError()
		ps.metrics.RecordPricerError("api_non_ok_status")
		return nil, errors.New("API returned non-OK status", z.Int("status", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ps.metrics.RecordOracleError()
		ps.metrics.RecordPricerError("read_response_failed")
		return nil, errors.Wrap(err, "failed to read response body")
	}

	var apiResponse ChaosLabsMultiResponse
	if err = json.Unmarshal(body, &apiResponse); err != nil {
		ps.metrics.RecordOracleError()
		ps.metrics.RecordPricerError("parse_json_failed")
		return nil, errors.Wrap(err, "failed to parse JSON response")
	}

	// Convert prices and map them to tokens
	prices := make(map[string]float64)
	for _, priceData := range apiResponse.Prices {
		// Convert price using the exponent
		// Price = price * 10^(-expo)
		actualPrice := float64(priceData.Price) / math.Pow10(-priceData.Expo)

		// Find the corresponding token for this feedId
		feedSymbol := strings.TrimSuffix(strings.ToUpper(priceData.FeedID), "USD")
		for _, token := range ps.config.Tokens {
			tokenSymbol := strings.ToUpper(token.BaseSymbol)
			// Map ETH price to WETH tokens
			if tokenSymbol == "WETH" && feedSymbol == "ETH" {
				prices[token.BaseSymbol] = actualPrice
				break
			}
			if tokenSymbol == feedSymbol {
				prices[token.BaseSymbol] = actualPrice
				break
			}
		}
	}

	return prices, nil
}

func (ps *PricerService) startPriceUpdates(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(ps.config.Pricing.UpdateIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := ps.updatePrices(ctx); err != nil {
				log.Error(ctx, "Failed to update prices", err)
			}
		}
	}
}

func (ps *PricerService) updatePrices(ctx context.Context) error {
	// Fetch all prices in a single API call WITHOUT holding the lock
	// This prevents blocking /levels requests during network I/O
	newPrices, err := ps.fetchAllPricesFromAPIWithRetry(ctx)
	if err != nil {
		log.Warn(ctx, "Failed to fetch prices from API, using previous prices if available", err)

		// Check if we have previous prices to fall back on
		ps.mu.RLock()
		hasPrices := len(ps.prices) > 0
		ps.mu.RUnlock()

		if hasPrices {
			log.Info(ctx, "Using previous prices due to API fetch failure")
			return nil // Don't fail, just use stale prices
		}

		// No previous prices available, this is critical
		return err
	}

	// NOW acquire the lock only for the fast in-memory update
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Update prices and metrics (fast operation ~5-10ms)
	for _, token := range ps.config.Tokens {
		price, exists := newPrices[token.BaseSymbol]
		if !exists {
			ps.metrics.RecordOracleError()
			ps.metrics.RecordPricerError("missing_price_data")
			log.Warn(ctx, "No price data received for token, keeping previous price", nil,
				z.Str("symbol", token.BaseSymbol),
			)
			// Keep previous price if available, don't fail
			continue
		}
		ps.prices[token.BaseSymbol] = price
		log.Debug(ctx, "Updated token price",
			z.Str("symbol", token.BaseSymbol),
			z.Str("address", token.BaseToken),
			z.F64("price_usd", price),
		)

		// Update metrics
		ps.metrics.UpdateTokenPrice(token.BaseToken, token.BaseSymbol, price)
	}

	// Update pair rates in metrics
	for i, tokenA := range ps.config.Tokens {
		for j, tokenB := range ps.config.Tokens {
			if i == j {
				continue
			}

			priceA, existsA := ps.prices[tokenA.BaseSymbol]
			priceB, existsB := ps.prices[tokenB.BaseSymbol]

			if existsA && existsB {
				pairRate := priceA / priceB
				// Apply markup
				markupMultiplier := 1 + (ps.config.Pricing.PriceMarkup / 100.0)
				markedUpPairRate := pairRate * markupMultiplier

				ps.metrics.UpdatePairRate(tokenA.BaseToken, tokenB.BaseToken, tokenA.BaseSymbol, tokenB.BaseSymbol, markedUpPairRate)
			}
		}
	}

	return nil
}

func (ps *PricerService) GetAllLevels(ctx context.Context) models.AllLevelsResponse {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	result := make(models.AllLevelsResponse)

	for i, tokenA := range ps.config.Tokens {
		for j, tokenB := range ps.config.Tokens {
			if i == j {
				continue
			}

			priceA, existsA := ps.prices[tokenA.BaseSymbol]
			priceB, existsB := ps.prices[tokenB.BaseSymbol]

			if !existsA || !existsB {
				continue
			}

			pairRate := priceA / priceB

			// Apply price markup from config
			markupMultiplier := 1 + (ps.config.Pricing.PriceMarkup / 100.0)
			markedUpPairRate := pairRate * markupMultiplier

			levels := ps.generateTokenPairLevels(ctx, tokenA, tokenB, markedUpPairRate)

			pairKey := fmt.Sprintf("%s_%s", strings.ToLower(tokenA.BaseToken), strings.ToLower(tokenB.BaseToken))
			result[pairKey] = levels
		}
	}

	return result
}

func (ps *PricerService) GetCurrentPrice(tokenIn, tokenOut string) (float64, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	tokenA := ps.findToken(tokenIn)
	tokenB := ps.findToken(tokenOut)

	if tokenA == nil || tokenB == nil {
		return 0, false
	}

	priceA, existsA := ps.prices[tokenA.BaseSymbol]
	priceB, existsB := ps.prices[tokenB.BaseSymbol]

	if !existsA || !existsB {
		return 0, false
	}

	return priceA / priceB, true
}

// GetTokenUSDPrices returns the USD prices for both tokens
func (ps *PricerService) GetTokenUSDPrices(tokenIn, tokenOut string) (float64, float64) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	tokenA := ps.findToken(tokenIn)
	tokenB := ps.findToken(tokenOut)

	var priceA, priceB float64
	if tokenA != nil {
		priceA = ps.prices[tokenA.BaseSymbol]
	}
	if tokenB != nil {
		priceB = ps.prices[tokenB.BaseSymbol]
	}

	return priceA, priceB
}

func (ps *PricerService) findToken(tokenAddress string) *config.Token {
	for _, token := range ps.config.Tokens {
		if strings.EqualFold(token.BaseToken, tokenAddress) {
			return &token
		}
	}
	return nil
}

// GetTokenSymbol returns the symbol for a given token address
func (ps *PricerService) GetTokenSymbol(tokenAddress string) string {
	token := ps.findToken(tokenAddress)
	if token == nil {
		return ""
	}
	return token.BaseSymbol
}

func (ps *PricerService) generateTokenPairLevels(ctx context.Context, tokenA, tokenB config.Token, pairRate float64) [][2]string {
	var levels [][2]string

	// Only quote if we have a balance service and can get actual balance
	if ps.balanceService == nil {
		log.Warn(ctx, "Balance service not available, skipping token pair",
			nil,
			z.Str("base_token", tokenA.BaseSymbol),
			z.Str("quote_token", tokenB.BaseSymbol),
		)
		return levels
	}

	// Get token decimals
	decimals := ps.getTokenDecimals(tokenA.BaseToken)

	// Get actual balance from blockchain
	balance, exists := ps.balanceService.GetBalanceFloat(tokenA.BaseToken, decimals)
	if !exists || balance <= 0 {
		log.Debug(ctx, "No balance found for token, skipping pair",
			z.Str("token", tokenA.BaseSymbol),
			z.Str("base_token", tokenA.BaseSymbol),
			z.Str("quote_token", tokenB.BaseSymbol),
		)
		return levels
	}

	// Use 100% of actual balance as available liquidity
	availableLiquidityA := balance
	log.Debug(ctx, "Using actual balance for token",
		z.Str("token", tokenA.BaseSymbol),
		z.F64("available", availableLiquidityA),
		z.F64("total_balance", balance),
	)

	levels = append(levels, [2]string{
		fmt.Sprintf("%.6f", availableLiquidityA),
		fmt.Sprintf("%.6f", pairRate),
	})

	return levels
}

// getTokenDecimals returns the number of decimals for a token
func (ps *PricerService) getTokenDecimals(tokenAddress string) int {
	// Standard token decimals
	decimalsMap := map[string]int{
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

	// Normalize address to lowercase for comparison
	tokenAddress = strings.ToLower(tokenAddress)
	for addr, decimals := range decimalsMap {
		if strings.ToLower(addr) == tokenAddress {
			return decimals
		}
	}

	return 18 // default
}

// GetTokenBalance returns the actual balance for a token in token units (for use in order validation)
func (ps *PricerService) GetTokenBalance(tokenAddress string, decimals int) (float64, bool) {
	if ps.balanceService == nil {
		return 0, false
	}
	return ps.balanceService.GetBalanceFloat(tokenAddress, decimals)
}

// GetTokenBalanceWei returns the actual balance for a token in WEI (for use in order validation)
func (ps *PricerService) GetTokenBalanceWei(tokenAddress string) (*big.Int, bool) {
	if ps.balanceService == nil {
		return big.NewInt(0), false
	}
	return ps.balanceService.GetBalance(tokenAddress)
}
