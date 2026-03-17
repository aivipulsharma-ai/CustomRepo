package services

import (
	"context"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/0xcatalysis/core/go-sdk/errors"
	"github.com/0xcatalysis/core/go-sdk/log"
	"github.com/0xcatalysis/core/go-sdk/z"

	"github.com/dextr_avs/price-feeder/config"
)

// ERC20 ABI for balanceOf function
const erc20ABI = `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"type":"function"}]`

type BalanceService struct {
	config   *config.Config
	client   *ethclient.Client
	balances map[common.Address]*big.Int // token address -> balance (using common.Address for case-insensitive lookup)
	mu       sync.RWMutex
	metrics  *MetricsService
}

func NewBalanceService(ctx context.Context, cfg *config.Config, metrics *MetricsService) (*BalanceService, error) {
	ctx = log.WithTopic(ctx, "balance")
	client, err := ethclient.Dial(cfg.EthRpcUrl)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Ethereum RPC")
	}

	bs := &BalanceService{
		config:   cfg,
		client:   client,
		balances: make(map[common.Address]*big.Int),
		metrics:  metrics,
	}

	// Initial balance fetch
	if err = bs.updateBalances(ctx); err != nil {
		log.Error(ctx, "Failed to update balances", err)
		return nil, err
	}

	// Start background updater
	go bs.startBalanceUpdates(ctx)

	return bs, nil
}

func (bs *BalanceService) startBalanceUpdates(ctx context.Context) {
	// Update balances every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := bs.updateBalances(ctx); err != nil {
				log.Error(ctx, "failed to update balances", err)
			}
		}
	}
}

func (bs *BalanceService) updateBalances(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	newBalances := make(map[common.Address]*big.Int)

	for _, token := range bs.config.Tokens {
		// Use common.Address for case-insensitive lookup (Ethereum-native way)
		tokenAddr := common.HexToAddress(token.BaseToken)

		// Try to fetch balance with retry logic
		balance, err := bs.fetchTokenBalanceWithRetry(ctx, token.BaseToken, token.BaseSymbol)
		if err != nil {
			bs.metrics.RecordBalanceError("fetch_balance_failed", token.BaseSymbol)
			log.Warn(ctx, "Failed to fetch token balance, using previous balance if available", err,
				z.Str("token", token.BaseSymbol),
			)

			// Use previous balance if available instead of failing completely
			bs.mu.RLock()
			if prevBalance, exists := bs.balances[tokenAddr]; exists {
				newBalances[tokenAddr] = prevBalance
				log.Info(ctx, "Using previous balance for token",
					z.Str("symbol", token.BaseSymbol),
					z.Str("balance_wei", prevBalance.String()),
				)
			} else {
				// No previous balance available - don't add this token to the map
				// This will cause GetBalance to return false (service unavailable)
				log.Warn(ctx, "No previous balance available, token will be unavailable for trading", nil,
					z.Str("token", token.BaseSymbol),
				)
			}
			bs.mu.RUnlock()
			continue
		}

		newBalances[tokenAddr] = balance
		log.Debug(ctx, "Updated balance for token",
			z.Str("symbol", token.BaseSymbol),
			z.Str("address", token.BaseToken),
			z.Str("balance_wei", balance.String()),
		)
	}

	// Update balances atomically
	bs.mu.Lock()
	bs.balances = newBalances
	bs.mu.Unlock()

	return nil
}

func (bs *BalanceService) fetchTokenBalanceWithRetry(ctx context.Context, tokenAddress, tokenSymbol string) (*big.Int, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		balance, err := bs.fetchTokenBalance(ctx, tokenAddress, tokenSymbol)
		if err == nil {
			return balance, nil
		}

		lastErr = err

		// Don't retry if context is already cancelled
		if ctx.Err() != nil {
			break
		}

		// Exponential backoff: 1s, 2s, 4s
		if attempt < maxRetries {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Debug(ctx, "Retrying balance fetch after backoff",
				z.Str("token", tokenSymbol),
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

	return nil, errors.Wrap(lastErr, "failed after retries")
}

func (bs *BalanceService) fetchTokenBalance(ctx context.Context, tokenAddress, tokenSymbol string) (*big.Int, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Parse the ERC20 ABI
	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		bs.metrics.RecordBalanceError("abi_parse_failed", tokenSymbol)
		return nil, errors.Wrap(err, "failed to parse ERC20 ABI")
	}

	// Encode the balanceOf call
	data, err := parsedABI.Pack("balanceOf", common.HexToAddress(bs.config.MakerAddress))
	if err != nil {
		bs.metrics.RecordBalanceError("pack_call_failed", tokenSymbol)
		return nil, errors.Wrap(err, "failed to pack balanceOf call")
	}

	// Make the call
	msg := ethereum.CallMsg{
		To:   &[]common.Address{common.HexToAddress(tokenAddress)}[0],
		Data: data,
	}

	result, err := bs.client.CallContract(ctx, msg, nil)
	if err != nil {
		bs.metrics.RecordRPCError()
		bs.metrics.RecordBalanceError("contract_call_failed", tokenSymbol)
		return nil, errors.Wrap(err, "failed to call contract")
	}

	// Decode the result
	var balance *big.Int
	err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		bs.metrics.RecordBalanceError("unpack_result_failed", tokenSymbol)
		return nil, errors.Wrap(err, "failed to unpack result")
	}

	return balance, nil
}

// GetBalance returns the balance for a token in wei/smallest unit
func (bs *BalanceService) GetBalance(tokenAddress string) (*big.Int, bool) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	// Use common.Address for case-insensitive lookup (Ethereum-native way)
	addr := common.HexToAddress(tokenAddress)
	balance, exists := bs.balances[addr]
	if !exists {
		return big.NewInt(0), false
	}

	// Return a copy to prevent modification
	return new(big.Int).Set(balance), true
}

// GetBalanceFloat returns the balance for a token as a float64 (in token units, not wei)
func (bs *BalanceService) GetBalanceFloat(tokenAddress string, decimals int) (float64, bool) {
	balance, exists := bs.GetBalance(tokenAddress)
	if !exists {
		return 0, false
	}

	// Convert from wei to token units
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	balanceFloat := new(big.Float).SetInt(balance)
	result := new(big.Float).Quo(balanceFloat, divisor)

	floatVal, _ := result.Float64()
	return floatVal, true
}
