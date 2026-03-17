package handlers

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	"github.com/0xcatalysis/core/go-sdk/errors"
	"github.com/0xcatalysis/core/go-sdk/log"
	"github.com/0xcatalysis/core/go-sdk/z"

	"github.com/dextr_avs/price-feeder/config"
	"github.com/dextr_avs/price-feeder/models"
	"github.com/dextr_avs/price-feeder/services"
)

type OrdersHandler struct {
	pricer  *services.PricerService
	counter *services.CounterService
	metrics *services.MetricsService
	config  *config.Config
}

func NewOrdersHandler(pricer *services.PricerService, counter *services.CounterService, metrics *services.MetricsService, cfg *config.Config) *OrdersHandler {
	return &OrdersHandler{
		pricer:  pricer,
		counter: counter,
		metrics: metrics,
		config:  cfg,
	}
}

func (h *OrdersHandler) HandleOrder(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := log.WithTopic(r.Context(), "order-api")

	if r.Method != http.MethodPost {
		h.writeErrorResponse(ctx, w, nil, "Method not allowed", http.StatusMethodNotAllowed, start)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorResponse(ctx, w, err, "Failed to read request body", http.StatusBadRequest, start)
		return
	}
	defer func() {
		if err = r.Body.Close(); err != nil {
			log.Warn(ctx, "Failed to close request body", err)
		}
	}()

	var orderReq models.OrderRequest
	if err = json.Unmarshal(body, &orderReq); err != nil {
		h.writeErrorResponse(ctx, w, err, "Invalid JSON format", http.StatusBadRequest, start)
		return
	}

	if err = h.validateOrderRequest(&orderReq); err != nil {
		h.writeErrorResponse(ctx, w, err, err.Error(), http.StatusBadRequest, start)
		return
	}

	processResult, processErr := h.processOrder(ctx, &orderReq)
	if processErr != nil {
		log.Error(ctx, "Failed to process order", processErr)
		responseTime := time.Since(start)

		// Map error to appropriate HTTP status code
		statusCode := h.getStatusCodeForError(processErr)
		errorType := h.getErrorType(processErr)

		// Record failed hit
		h.counter.RecordHit(
			orderReq.BaseToken,
			orderReq.QuoteToken,
			orderReq.Amount,
			orderReq.FeeBps,
			orderReq.Taker,
			false,
			responseTime,
			0, 0, 0, 0,
		)

		// Record metrics for failed request
		h.metrics.RecordHit(false, responseTime)
		baseSymbol := h.pricer.GetTokenSymbol(orderReq.BaseToken)
		quoteSymbol := h.pricer.GetTokenSymbol(orderReq.QuoteToken)
		h.metrics.RecordTokenPairHit(orderReq.BaseToken, orderReq.QuoteToken, baseSymbol, quoteSymbol, false)
		h.metrics.RecordHTTPRequest("/order", r.Method, strconv.Itoa(statusCode), responseTime)

		// Record error-specific metrics
		h.metrics.RecordOrderError(errorType, strconv.Itoa(statusCode))
		h.metrics.RecordOrderErrorByToken(orderReq.BaseToken, orderReq.QuoteToken, baseSymbol, quoteSymbol, errorType)

		// Write error response and return
		h.writeErrorResponse(ctx, w, processErr, processErr.Error(), statusCode, start)
		return
	}

	responseTime := time.Since(start)

	// Success case - record metrics
	statusCode := http.StatusOK
	hitID := h.counter.RecordHit(
		orderReq.BaseToken,
		orderReq.QuoteToken,
		orderReq.Amount,
		orderReq.FeeBps,
		orderReq.Taker,
		true,
		responseTime,
		processResult.AcceptedPrice,
		processResult.VolumeUSD,
		processResult.FeeUSD,
		processResult.AmountFloat,
	)

	h.metrics.RecordHit(true, responseTime)
	baseSymbol := h.pricer.GetTokenSymbol(orderReq.BaseToken)
	quoteSymbol := h.pricer.GetTokenSymbol(orderReq.QuoteToken)
	h.metrics.RecordTokenPairHit(orderReq.BaseToken, orderReq.QuoteToken, baseSymbol, quoteSymbol, true)
	h.metrics.RecordOrderVolume(orderReq.BaseToken, orderReq.QuoteToken, baseSymbol, quoteSymbol, processResult.AmountFloat, processResult.VolumeUSD, processResult.AcceptedPrice)

	// Generate 1inch RFQ order structure
	rfqOrder, signature, err := h.generateRFQOrder(&orderReq, processResult)
	if err != nil {
		h.writeErrorResponse(ctx, w, err, "Failed to generate RFQ", http.StatusInternalServerError, start)
		return
	}
	response := models.OrderResponse{
		Order:     rfqOrder,
		Signature: signature,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Response-Time", responseTime.String())
	w.Header().Set("X-Hit-ID", hitID)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(statusCode)

	// Record HTTP metrics
	h.metrics.RecordHTTPRequest("/order", r.Method, strconv.Itoa(statusCode), responseTime)

	if err = json.NewEncoder(w).Encode(response); err != nil {
		h.writeErrorResponse(ctx, w, err, "Failed to encode response", http.StatusInternalServerError, start)
		return
	}
}

func (h *OrdersHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	ctx := log.WithTopic(r.Context(), "stats-api")
	if r.Method != http.MethodGet {
		h.writeErrorResponse(ctx, w, nil, "Method not allowed", http.StatusMethodNotAllowed, time.Now())
		return
	}

	stats := h.counter.GetStatistics()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache")

	if err := json.NewEncoder(w).Encode(stats); err != nil {
		h.writeErrorResponse(ctx, w, err, "Failed to encode statistics", http.StatusInternalServerError, time.Now())
		return
	}
}

func (h *OrdersHandler) HandleRecentHits(w http.ResponseWriter, r *http.Request) {
	ctx := log.WithTopic(r.Context(), "recent-hits-api")
	if r.Method != http.MethodGet {
		h.writeErrorResponse(ctx, w, nil, "Method not allowed", http.StatusMethodNotAllowed, time.Now())
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	recentHits := h.counter.GetRecentHits(limit)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache")

	if err := json.NewEncoder(w).Encode(recentHits); err != nil {
		h.writeErrorResponse(ctx, w, err, "Failed to encode recent hits", http.StatusInternalServerError, time.Now())
		return
	}
}

func (h *OrdersHandler) HandleReset(w http.ResponseWriter, r *http.Request) {
	ctx := log.WithTopic(r.Context(), "reset-api")
	if r.Method != http.MethodPost {
		h.writeErrorResponse(ctx, w, nil, "Method not allowed", http.StatusMethodNotAllowed, time.Now())
		return
	}

	h.counter.Reset()

	response := map[string]interface{}{
		"success":   true,
		"message":   "Statistics reset successfully",
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeErrorResponse(ctx, w, err, "Failed to encode response", http.StatusInternalServerError, time.Now())
		return
	}
}

func (h *OrdersHandler) HandleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.WriteHeader(http.StatusOK)
}

func (h *OrdersHandler) validateOrderRequest(req *models.OrderRequest) error {
	if req.BaseToken == "" {
		return errors.New("baseToken is required")
	}
	if req.QuoteToken == "" {
		return errors.New("quoteToken is required")
	}
	if req.Amount == "" {
		return errors.New("amount is required")
	}
	if req.Taker == "" {
		return errors.New("taker is required")
	}
	if req.FeeBps < 0 {
		return errors.New("feeBps must be non-negative")
	}

	// Case-insensitive comparison for token addresses
	if strings.EqualFold(req.BaseToken, req.QuoteToken) {
		return errors.New("baseToken and quoteToken cannot be the same")
	}

	if _, err := strconv.ParseFloat(req.Amount, 64); err != nil {
		return errors.Wrap(err, "invalid amount format")
	}

	return nil
}

func (h *OrdersHandler) processOrder(ctx context.Context, req *models.OrderRequest) (*models.OrderProcessingResult, error) {
	// Get current exchange rate (baseToken/quoteToken)
	acceptedPrice, priceExists := h.pricer.GetCurrentPrice(req.BaseToken, req.QuoteToken)
	if !priceExists || acceptedPrice == 0 {
		log.Warn(ctx, "Unable to get current price for token pair", nil,
			z.Str("base_token", req.BaseToken),
			z.Str("quote_token", req.QuoteToken),
		)
		return nil, errors.New("price unavailable for token pair",
			z.Str("base_token", req.BaseToken),
			z.Str("quote_token", req.QuoteToken),
		)
	}

	// Parse order amount (in WEI/smallest units)
	amountWei := new(big.Int)
	_, success := amountWei.SetString(req.Amount, 10)
	if !success {
		return nil, errors.New("invalid amount format",
			z.Str("amount", req.Amount),
		)
	}
	if amountWei.Cmp(big.NewInt(0)) <= 0 {
		return nil, errors.New("amount must be greater than zero",
			z.Str("amount", req.Amount),
		)
	}

	// Check if maker has sufficient balance (CRITICAL SECURITY CHECK)
	// Compare WEI to WEI - no conversion needed!
	makerBalanceWei, hasBalance := h.pricer.GetTokenBalanceWei(req.BaseToken)

	if !hasBalance {
		return nil, errors.New("balance service unavailable for token",
			z.Str("token", req.BaseToken),
		)
	}

	if makerBalanceWei.Cmp(big.NewInt(0)) == 0 {
		return nil, errors.New("maker has zero balance for token",
			z.Str("token", req.BaseToken),
		)
	}

	// Compare WEI to WEI directly
	if amountWei.Cmp(makerBalanceWei) > 0 {
		return nil, errors.New("insufficient balance",
			z.Str("token", req.BaseToken),
			z.Str("requested_wei", amountWei.String()),
			z.Str("available_wei", makerBalanceWei.String()),
		)
	}

	// Balance check passed - proceed with order
	log.Info(ctx, "Balance check passed",
		z.Str("requested_wei", amountWei.String()),
		z.Str("available_wei", makerBalanceWei.String()),
		z.Str("token", req.BaseToken),
	)

	// Convert amount to float for USD calculations (only used for metrics/logging)
	baseDecimals := h.getTokenDecimals(req.BaseToken)
	amount := h.convertWeiToFloat(amountWei, baseDecimals)

	// Create result object
	result := &models.OrderProcessingResult{
		Success:       true,
		AcceptedPrice: acceptedPrice,
		AmountFloat:   amount,
	}

	// Get USD prices for both tokens to calculate volume
	baseTokenUSD, quoteTokenUSD := h.pricer.GetTokenUSDPrices(req.BaseToken, req.QuoteToken)
	result.BaseTokenUSD = baseTokenUSD
	result.QuoteTokenUSD = quoteTokenUSD

	// Calculate trade volume in USD (using base token amount * base token USD price)
	if baseTokenUSD > 0 {
		result.VolumeUSD = amount * baseTokenUSD

		// Calculate fee in USD (feeBps is basis points, 1 bps = 0.01%)
		feeFraction := float64(req.FeeBps) / 10000.0
		result.FeeUSD = result.VolumeUSD * feeFraction
	}

	// Get token symbols (case-insensitive lookup via pricer service)
	baseTokenSymbol := h.pricer.GetTokenSymbol(req.BaseToken)
	quoteTokenSymbol := h.pricer.GetTokenSymbol(req.QuoteToken)

	log.Info(ctx, "Order accepted",
		z.Str("base_token", req.BaseToken),
		z.Str("quote_token", req.QuoteToken),
		z.Str("base_token_symbol", baseTokenSymbol),
		z.Str("quote_token_symbol", quoteTokenSymbol),
		z.Str("amount", req.Amount),
		z.F64("price", acceptedPrice),
		z.F64("base_token_price_usd", baseTokenUSD),
		z.F64("quote_token_price_usd", quoteTokenUSD),
		z.F64("volume_usd", result.VolumeUSD),
		z.F64("fee_usd", result.FeeUSD),
		z.Str("taker", req.Taker),
	)

	return result, nil
}

// getStatusCodeForError maps error messages to appropriate HTTP status codes
func (h *OrdersHandler) getStatusCodeForError(err error) int {
	if err == nil {
		return http.StatusOK
	}

	errMsg := err.Error()

	// 503 Service Unavailable - External service issues
	if strings.Contains(errMsg, "price unavailable") ||
		strings.Contains(errMsg, "balance service unavailable") {
		return http.StatusServiceUnavailable
	}

	// 422 Unprocessable Entity - Valid request but cannot be processed
	if strings.Contains(errMsg, "insufficient balance") ||
		strings.Contains(errMsg, "zero balance") {
		return http.StatusUnprocessableEntity
	}

	// 400 Bad Request - Invalid input
	if strings.Contains(errMsg, "invalid amount") ||
		strings.Contains(errMsg, "amount must be") {
		return http.StatusBadRequest
	}

	// Default to 500 Internal Server Error for unexpected errors
	return http.StatusInternalServerError
}

// getErrorType classifies errors into categories for metrics
func (h *OrdersHandler) getErrorType(err error) string {
	if err == nil {
		return "none"
	}

	errMsg := err.Error()

	// Service availability errors
	if strings.Contains(errMsg, "price unavailable") {
		return "price_unavailable"
	}
	if strings.Contains(errMsg, "balance service unavailable") {
		return "balance_service_unavailable"
	}

	// Balance errors
	if strings.Contains(errMsg, "insufficient balance") {
		return "insufficient_balance"
	}
	if strings.Contains(errMsg, "zero balance") {
		return "zero_balance"
	}

	// Input validation errors
	if strings.Contains(errMsg, "invalid amount format") {
		return "invalid_amount_format"
	}
	if strings.Contains(errMsg, "amount must be") {
		return "invalid_amount_value"
	}

	// Unknown/unexpected errors
	return "unknown"
}

// generateRFQOrder creates a 1inch RFQ order structure and signature
func (h *OrdersHandler) generateRFQOrder(req *models.OrderRequest, result *models.OrderProcessingResult) (*models.RFQOrder, string, error) {
	// Get token decimals for proper amount conversion
	baseDecimals := h.getTokenDecimals(req.BaseToken)
	quoteDecimals := h.getTokenDecimals(req.QuoteToken)

	// Use the amount in token units from result (already converted from WEI in processOrder)
	amount := result.AmountFloat

	// Calculate taking amount based on accepted price
	// makingAmount = amount * price
	makingAmountFloat := amount * result.AcceptedPrice

	// Convert to wei/smallest unit for the RFQ order
	takingAmount := h.convertToWei(amount, baseDecimals)
	makingAmount := h.convertToWei(makingAmountFloat, quoteDecimals)

	// Generate random salt (nonce)
	salt := h.generateSalt()

	// Generate makerTraits (encodes expiration, nonce, and other flags)
	// For now, using a standard value with 120 second expiration
	expiration := time.Now().Unix() + 120 // 2 minutes from now
	// Generate nonce for makerTraits (uint40, max value 2^40 - 1)
	// Use timestamp-based nonce to ensure uniqueness
	nonce := time.Now().UnixNano() % (1 << 40) // Keep within uint40 range
	makerTraits := h.generateMakerTraits(expiration, nonce)

	order := &models.RFQOrder{
		Maker:        h.config.MakerAddress,
		MakerAsset:   req.QuoteToken,
		TakerAsset:   req.BaseToken,
		MakerTraits:  makerTraits,
		Salt:         salt,
		MakingAmount: makingAmount,
		TakingAmount: takingAmount,
		Receiver:     "0x0000000000000000000000000000000000000000",
	}

	signature, err := h.generateSignature(order)
	if err != nil {
		return nil, "", errors.Wrap(err, "generate signature")
	}

	return order, signature, nil
}

// getTokenDecimals returns the number of decimals for a token
func (h *OrdersHandler) getTokenDecimals(tokenAddress string) int {
	// Standard token decimals - most tokens use 18, stablecoins use 6
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

	// Normalize address to lowercase for case-insensitive comparison
	tokenAddress = strings.ToLower(tokenAddress)
	for addr, decimals := range decimalsMap {
		if strings.ToLower(addr) == tokenAddress {
			return decimals
		}
	}

	return 18 // default to 18 decimals
}

// convertWeiToFloat converts amount from WEI (big.Int) to token units (float64)
func (h *OrdersHandler) convertWeiToFloat(amountWei *big.Int, decimals int) float64 {
	// Convert big.Int to big.Float
	amountFloat := new(big.Float).SetInt(amountWei)

	// Create divisor: 10^decimals
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))

	// Divide: amount / 10^decimals
	result := new(big.Float).Quo(amountFloat, divisor)

	// Convert to float64
	tokenAmount, _ := result.Float64()
	return tokenAmount
}

// convertFromWei converts amount from WEI/smallest unit to token units (kept for backward compatibility)
func (h *OrdersHandler) convertFromWei(amountWei float64, decimals int) float64 {
	// Divide by 10^decimals
	divisor := new(big.Float).SetFloat64(1)
	for i := 0; i < decimals; i++ {
		divisor.Mul(divisor, big.NewFloat(10))
	}

	result := new(big.Float).Quo(big.NewFloat(amountWei), divisor)
	tokenAmount, _ := result.Float64()

	return tokenAmount
}

// convertToWei converts a float amount to wei/smallest unit
func (h *OrdersHandler) convertToWei(amount float64, decimals int) string {
	// Multiply by 10^decimals
	multiplier := new(big.Float).SetFloat64(1)
	for i := 0; i < decimals; i++ {
		multiplier.Mul(multiplier, big.NewFloat(10))
	}

	result := new(big.Float).Mul(big.NewFloat(amount), multiplier)

	// Convert to big.Int (round down)
	resultInt := new(big.Int)
	result.Int(resultInt)

	return resultInt.String()
}

// generateSalt generates a random nonce for the order
func (h *OrdersHandler) generateSalt() string {
	// Generate random number up to 10^13
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(13), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		// Fallback to timestamp-based salt
		return fmt.Sprintf("%d", time.Now().UnixNano()%1000000000000)
	}
	return n.String()
}

// generateMakerTraits generates the makerTraits field encoding expiration and nonce
// Based on 1inch Limit Order SDK: https://github.com/1inch/limit-order-sdk/blob/master/src/limit-order/maker-traits.ts
func (h *OrdersHandler) generateMakerTraits(expiration int64, nonce int64) string {
	// MakerTraits encoding (256 bits):
	// Low 200 bits are used for allowed sender, expiration, nonceOrEpoch, and series:
	// Bits 0-79:   allowed sender (uint80, last 10 bytes of address, 0 if any sender allowed)
	// Bits 80-119: expiration timestamp (uint40, 0 if no expiration)
	// Bits 120-159: nonce or epoch (uint40)
	// Bits 160-199: series (uint40)
	// High bits (200-255) are used for flags:
	// Bit 255: NO_PARTIAL_FILLS_FLAG
	// Bit 254: ALLOW_MULTIPLE_FILLS_FLAG
	// Bit 252: PRE_INTERACTION_CALL_FLAG
	// Bit 251: POST_INTERACTION_CALL_FLAG
	// Bit 250: NEED_CHECK_EPOCH_MANAGER_FLAG
	// Bit 249: HAS_EXTENSION_FLAG
	// Bit 248: USE_PERMIT2_FLAG
	// Bit 247: UNWRAP_WETH_FLAG

	traits := new(big.Int)

	// Set expiration in bits 80-119 (40 bits for timestamp)
	// Shift expiration left by 80 bits to place it in the correct position
	exp := big.NewInt(expiration)
	exp.Lsh(exp, 80)
	traits.Or(traits, exp)

	// Set nonce in bits 120-159 (40 bits)
	// Shift nonce left by 120 bits to place it in the correct position
	nonceBig := big.NewInt(nonce)
	nonceBig.Lsh(nonceBig, 120)
	traits.Or(traits, nonceBig)

	// For a basic RFQ order, we don't need special flags
	// The resulting number encodes: expiration at bits 80-119, nonce at bits 120-159

	return traits.String()
}

// generateSignature generates an EIP-712 signature for the order
func (h *OrdersHandler) generateSignature(order *models.RFQOrder) (string, error) {
	// If no private key is configured, return a mock signature
	if h.config.MakerPrivateKey == "" {
		return "", errors.New("makerPrivateKey is empty")
	}

	// Parse the private key
	privateKey, err := h.parsePrivateKey(h.config.MakerPrivateKey)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse private key")
	}

	// Create EIP-712 typed data
	typedData := h.createTypedData(order)

	// Hash the typed data
	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return "", errors.Wrap(err, "failed to get domain separator")
	}

	typedDataHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return "", errors.Wrap(err, "failed to hash EIP712Domain")
	}

	// Combine according to EIP-712: "\x19\x01" + domainSeparator + typedDataHash
	rawData := []byte(fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(typedDataHash)))
	hash := crypto.Keccak256Hash(rawData)

	// Sign the hash
	signature, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		return "", errors.Wrap(err, "failed to sign the hash")
	}

	// Adjust V value for Ethereum (add 27)
	if signature[64] < 27 {
		signature[64] += 27
	}

	return "0x" + hex.EncodeToString(signature), nil
}

// parsePrivateKey parses a hex private key string
func (h *OrdersHandler) parsePrivateKey(keyHex string) (*ecdsa.PrivateKey, error) {
	// Remove 0x prefix if present
	keyHex = strings.TrimPrefix(keyHex, "0x")

	privateKeyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, errors.Wrap(err, "invalid hex string")
	}

	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "invalid private key")
	}

	return privateKey, nil
}

// createTypedData creates EIP-712 typed data for the order
func (h *OrdersHandler) createTypedData(order *models.RFQOrder) apitypes.TypedData {
	// 1inch AggregationRouter V6 on Ethereum Mainnet
	// Based on bytecode analysis, the contract uses "1inch Aggregation Router" as the domain name
	const aggregationRouterAddress = "0x111111125421ca6dc452d289314280a0f8842a65"
	const chainID = 1 // Ethereum Mainnet

	return apitypes.TypedData{
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
}

func (h *OrdersHandler) writeErrorResponse(ctx context.Context, w http.ResponseWriter, err error, message string, statusCode int, startTime time.Time) {
	responseTime := time.Since(startTime)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Response-Time", responseTime.String())
	w.WriteHeader(statusCode)

	errorResp := models.ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    statusCode,
	}

	// Log the error here
	if err != nil {
		log.Error(ctx, "Error Processing order", err)
	}

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}
