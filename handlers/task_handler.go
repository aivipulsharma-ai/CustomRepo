package handlers

import (
	"context"
	"encoding/json"

	"github.com/0xcatalysis/catalyst-sdk/go-sdk/errors"
	"github.com/0xcatalysis/catalyst-sdk/go-sdk/log"
	"github.com/0xcatalysis/catalyst-sdk/go-sdk/types"
	"github.com/0xcatalysis/catalyst-sdk/go-sdk/z"

	avstypes "github.com/dextr_avs/types"
	"github.com/dextr_avs/utils"
)

// SwapResponse represents the response data structure for token swaps
type SwapResponse struct {
	InputToken   string              `json:"input_token"`
	OutputToken  string              `json:"output_token"`
	Amount       float64             `json:"amount"`
	Path         []avstypes.Transfer `json:"path"`
	Success      bool                `json:"success"`
	ExecutionLog []string            `json:"execution_log"`
}

// Vault represents a token pair with their respective balances
type Vault struct {
	TokenA   string  `json:"token_a"`
	TokenB   string  `json:"token_b"`
	BalanceA float64 `json:"balance_a"`
	BalanceB float64 `json:"balance_b"`
}

// VaultMap maps token pair keys to their vaults
type VaultMap map[string]*Vault

// SwapHandler implements the TransactionHandler interface for token swap calculations
type SwapHandler struct{}

// vaultKey creates a consistent key for token pairs
func vaultKey(tokenA, tokenB string) string {
	if tokenA < tokenB {
		return tokenA + "/" + tokenB
	}
	return tokenB + "/" + tokenA
}

// printVaults logs the vault balances
func printVaults(ctx context.Context, vaults VaultMap, title string) string {
	log.Info(ctx, "Vault state", z.Str("title", title))

	logStr := "\n=== " + title + " ===\n"
	for key, v := range vaults {
		logStr += key + ": " + v.TokenA + "=" + string(rune(int(v.BalanceA))) + ", " + v.TokenB + "=" + string(rune(int(v.BalanceB))) + "\n"
		log.Info(ctx, "Vault balance",
			z.Str("vault", key),
			z.Str("token_a", v.TokenA),
			z.F64("balance_a", v.BalanceA),
			z.Str("token_b", v.TokenB),
			z.F64("balance_b", v.BalanceB))
	}
	return logStr
}

// printSwapPath logs the swap path details
func printSwapPath(ctx context.Context, path []avstypes.Transfer, inputToken, outputToken string, amount float64) string {
	log.Info(ctx, "Swap path details",
		z.Str("input_token", inputToken),
		z.Str("output_token", outputToken),
		z.F64("amount", amount),
		z.Int("path_length", len(path)))

	logStr := "\n=== SWAP PATH DETAILS ===\n"
	logStr += "Swapping " + string(rune(int(amount))) + " " + inputToken + " for " + outputToken + "\n"
	logStr += "Path length: " + string(rune(len(path))) + " steps\n"
	logStr += "Path:\n"

	for i, transfer := range path {
		logStr += "  Step " + string(rune(i+1)) + ": " + transfer.FromToken + " → " + transfer.ToToken + " (via vault " + vaultKey(transfer.FromToken, transfer.ToToken) + ")\n"
		log.Info(ctx, "Swap step",
			z.Int("step", i+1),
			z.Str("from_token", transfer.FromToken),
			z.Str("to_token", transfer.ToToken),
			z.Str("vault", vaultKey(transfer.FromToken, transfer.ToToken)))
	}

	// Show the complete path as a chain
	logStr += "Complete chain: " + inputToken
	for _, transfer := range path {
		logStr += " → " + transfer.ToToken
	}
	logStr += "\n"
	return logStr
}

// printStepExecution logs detailed step-by-step execution
func printStepExecution(ctx context.Context, vaults VaultMap, path []avstypes.Transfer, amount float64, phase string) string {
	log.Info(ctx, "Step execution", z.Str("phase", phase), z.F64("amount", amount))

	logStr := "\n=== " + phase + " EXECUTION ===\n"
	currAmount := amount

	for i, transfer := range path {
		key := vaultKey(transfer.FromToken, transfer.ToToken)
		vault := vaults[key]

		if vault == nil {
			log.Error(ctx, "Vault not found", errors.New("vault not found"), z.Str("key", key))
			logStr += "ERROR: Vault not found for key: " + key + "\n"
			logStr += "Available vaults:\n"
			for k := range vaults {
				logStr += "  " + k + "\n"
			}
			return logStr
		}

		log.Info(ctx, "Processing swap step",
			z.Int("step", i+1),
			z.Str("from_token", transfer.FromToken),
			z.Str("to_token", transfer.ToToken),
			z.F64("amount", currAmount),
			z.Str("vault", key),
			z.F64("balance_a", vault.BalanceA),
			z.F64("balance_b", vault.BalanceB))

		logStr += "Step " + string(rune(i+1)) + ": Processing " + transfer.FromToken + " → " + transfer.ToToken + " (Amount: " + string(rune(int(currAmount))) + ")\n"
		logStr += "  Vault " + key + " before: " + vault.TokenA + "=" + string(rune(int(vault.BalanceA))) + ", " + vault.TokenB + "=" + string(rune(int(vault.BalanceB))) + "\n"

		// Show the actual balance changes
		if vault.TokenA == transfer.FromToken {
			if phase == "FORWARD" {
				logStr += "  Adding " + string(rune(int(currAmount))) + " to " + vault.TokenA + ", removing " + string(rune(int(currAmount))) + " from " + vault.TokenB + "\n"
			} else {
				logStr += "  Removing " + string(rune(int(currAmount))) + " from " + vault.TokenA + ", adding " + string(rune(int(currAmount))) + " to " + vault.TokenB + "\n"
			}
		} else {
			if phase == "FORWARD" {
				logStr += "  Adding " + string(rune(int(currAmount))) + " to " + vault.TokenB + ", removing " + string(rune(int(currAmount))) + " from " + vault.TokenA + "\n"
			} else {
				logStr += "  Removing " + string(rune(int(currAmount))) + " from " + vault.TokenB + ", adding " + string(rune(int(currAmount))) + " to " + vault.TokenA + "\n"
			}
		}
	}
	return logStr
}

// FindPath uses BFS to find a valid token swap path from input to output
func FindPath(vaults VaultMap, inputToken, outputToken string) ([]avstypes.Transfer, bool) {
	type Node struct {
		Token   string
		Path    []avstypes.Transfer
		Visited map[string]bool
	}

	queue := []Node{{Token: inputToken, Path: []avstypes.Transfer{}, Visited: map[string]bool{inputToken: true}}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, v := range vaults {
			var fromToken, toToken string

			// Check both directions
			if v.TokenA == curr.Token && !curr.Visited[v.TokenB] {
				fromToken = v.TokenA
				toToken = v.TokenB
			} else if v.TokenB == curr.Token && !curr.Visited[v.TokenA] {
				fromToken = v.TokenB
				toToken = v.TokenA
			} else {
				continue
			}

			newPath := append([]avstypes.Transfer{}, curr.Path...)
			newPath = append(newPath, avstypes.Transfer{FromToken: fromToken, ToToken: toToken, Amount: 0}) // Placeholder

			if toToken == outputToken {
				return newPath, true
			}

			// Clone visited map
			newVisited := map[string]bool{}
			for k, v := range curr.Visited {
				newVisited[k] = v
			}
			newVisited[toToken] = true

			queue = append(queue, Node{Token: toToken, Path: newPath, Visited: newVisited})
		}
	}

	return nil, false
}

// PerformSwap executes a virtual swap and ensures vaults are balanced at the end
func PerformSwap(ctx context.Context, vaults VaultMap, inputToken, outputToken string, amount float64) ([]string, bool) {
	executionLog := []string{}

	log.Info(ctx, "Starting swap execution",
		z.Str("input_token", inputToken),
		z.Str("output_token", outputToken),
		z.F64("amount", amount))

	// Save original state
	original := make(map[string][2]float64)
	for key, v := range vaults {
		original[key] = [2]float64{v.BalanceA, v.BalanceB}
	}

	// Find the path
	path, found := FindPath(vaults, inputToken, outputToken)
	if !found {
		log.Error(ctx, "No valid swap path found",
			errors.New("no valid swap path found"),
			z.Str("input_token", inputToken),
			z.Str("output_token", outputToken))
		executionLog = append(executionLog, "No valid path found.")
		return executionLog, false
	}

	// Print path details
	executionLog = append(executionLog, printSwapPath(ctx, path, inputToken, outputToken, amount))

	// Forward transfer with detailed execution
	executionLog = append(executionLog, printStepExecution(ctx, vaults, path, amount, "FORWARD"))
	currAmount := amount
	for _, t := range path {
		key := vaultKey(t.FromToken, t.ToToken)
		vault := vaults[key]

		if vault == nil {
			log.Error(ctx, "Vault not found during forward execution",
				errors.New("vault not found during forward execution"),
				z.Str("key", key))
			executionLog = append(executionLog, "ERROR: Vault not found for key: "+key)
			return executionLog, false
		}

		if vault.TokenA == t.FromToken {
			vault.BalanceA += currAmount
			vault.BalanceB -= currAmount
		} else {
			vault.BalanceB += currAmount
			vault.BalanceA -= currAmount
		}
	}

	executionLog = append(executionLog, printVaults(ctx, vaults, "VAULT STATE AFTER FORWARD TRANSFER"))

	// Reverse to rebalance with detailed execution
	executionLog = append(executionLog, printStepExecution(ctx, vaults, path, amount, "REVERSE"))
	currAmount = amount
	for i := len(path) - 1; i >= 0; i-- {
		t := path[i]
		key := vaultKey(t.FromToken, t.ToToken)
		vault := vaults[key]

		if vault == nil {
			log.Error(ctx, "Vault not found during reverse execution",
				errors.New("vault not found during reverse execution"),
				z.Str("key", key))
			executionLog = append(executionLog, "ERROR: Vault not found for key: "+key)
			return executionLog, false
		}

		if vault.TokenA == t.FromToken {
			vault.BalanceA -= currAmount
			vault.BalanceB += currAmount
		} else {
			vault.BalanceB -= currAmount
			vault.BalanceA += currAmount
		}
	}

	executionLog = append(executionLog, printVaults(ctx, vaults, "FINAL VAULT STATE (AFTER SWAP AND REBALANCE)"))

	log.Info(ctx, "Swap execution completed successfully")
	return executionLog, true
}

// Execute performs the token swap calculation
func (h *SwapHandler) Execute(ctx context.Context, tx types.Task) (types.TaskResult, error) {
	ctx = log.WithTopic(ctx, "SwapHandler.Execute")

	// Parse request
	var request SwapRequest
	if err := json.Unmarshal(tx.Data, &request); err != nil {
		log.Error(ctx, "Failed to unmarshal swap request", err)
		return types.TaskResult{}, errors.Wrap(err, "failed to unmarshal swap request")
	}

	log.Info(ctx, "Processing swap request",
		z.Str("input_token", request.InputToken),
		z.Str("output_token", request.OutputToken),
		z.F64("amount", request.Amount))

	// Validate input using utility function
	if err := utils.ValidateSwapInput(request.InputToken, request.OutputToken, request.Amount); err != nil {
		log.Error(ctx, "Swap input validation failed", err)
		return types.TaskResult{}, errors.Wrap(err, "swap input validation failed")
	}

	// Initialize vaults with sample data (in a real implementation, this would come from blockchain state)
	vaults := VaultMap{
		"ETH/USDC": &Vault{TokenA: "ETH", TokenB: "USDC", BalanceA: 100, BalanceB: 1000},
		"BNB/ETH":  &Vault{TokenA: "BNB", TokenB: "ETH", BalanceA: 250, BalanceB: 50},
		"BNB/USDC": &Vault{TokenA: "BNB", TokenB: "USDC", BalanceA: 200, BalanceB: 1200},
		"BTC/ETH":  &Vault{TokenA: "BTC", TokenB: "ETH", BalanceA: 10, BalanceB: 100},
		"BNB/BTC":  &Vault{TokenA: "BNB", TokenB: "BTC", BalanceA: 400, BalanceB: 20},
	}

	// Find the swap path
	path, found := FindPath(vaults, request.InputToken, request.OutputToken)
	if !found {
		log.Error(ctx, "No valid swap path found",
			errors.New("no valid swap path found"),
			z.Str("input_token", request.InputToken),
			z.Str("output_token", request.OutputToken))
		return types.TaskResult{}, errors.New("no valid swap path found")
	}

	// Validate the swap path
	if err := utils.ValidateSwapPath(path, request.InputToken, request.OutputToken); err != nil {
		log.Error(ctx, "Swap path validation failed", err)
		return types.TaskResult{}, errors.Wrap(err, "swap path validation failed")
	}

	// Perform the actual swap execution with forward and reverse phases
	executionLog, success := PerformSwap(ctx, vaults, request.InputToken, request.OutputToken, request.Amount)
	if !success {
		log.Error(ctx, "Swap execution failed", errors.New("swap execution failed"))
		return types.TaskResult{}, errors.New("swap execution failed")
	}

	// Create response
	result := SwapResponse{
		InputToken:   request.InputToken,
		OutputToken:  request.OutputToken,
		Amount:       request.Amount,
		Path:         path,
		Success:      true,
		ExecutionLog: executionLog,
	}

	// Serialize response
	resultData, err := json.Marshal(result)
	if err != nil {
		log.Error(ctx, "Failed to marshal swap response", err)
		return types.TaskResult{}, errors.Wrap(err, "failed to marshal swap response")
	}

	log.Info(ctx, "Swap execution completed successfully",
		z.Str("input_token", request.InputToken),
		z.Str("output_token", request.OutputToken),
		z.F64("amount", request.Amount),
		z.Int("path_length", len(path)))

	// Create result
	return types.TaskResult{
		Task: &types.Task{
			ID:        tx.ID,
			Type:      avstypes.TaskTypeSwap,
			Data:      tx.Data,
			Metadata:  map[string]string{},
			Timestamp: tx.Timestamp,
		},
		Result: resultData,
	}, nil
}

// Verify validates the swap calculation result
func (h *SwapHandler) Verify(ctx context.Context, result types.SignedResult) error {
	ctx = log.WithTopic(ctx, "SwapHandler.Verify")

	// Parse original request
	var request SwapRequest
	if err := json.Unmarshal(result.Message.Task.Data, &request); err != nil {
		log.Error(ctx, "Failed to unmarshal original request during verification", err)
		return errors.Wrap(err, "failed to unmarshal original request during verification")
	}

	// Parse response
	var response SwapResponse
	if err := json.Unmarshal(result.Message.Result, &response); err != nil {
		log.Error(ctx, "Failed to unmarshal response during verification", err)
		return errors.Wrap(err, "failed to unmarshal response during verification")
	}

	// Verify the input parameters match
	if request.InputToken != response.InputToken || request.OutputToken != response.OutputToken || request.Amount != response.Amount {
		log.Error(ctx, "Response parameters don't match request",
			errors.New("response parameters don't match request"),
			z.Str("request_input", request.InputToken),
			z.Str("response_input", response.InputToken),
			z.Str("request_output", request.OutputToken),
			z.Str("response_output", response.OutputToken),
			z.F64("request_amount", request.Amount),
			z.F64("response_amount", response.Amount))
		return errors.New("response parameters don't match request")
	}

	// Verify the swap path is valid
	if err := utils.ValidateSwapPath(response.Path, request.InputToken, request.OutputToken); err != nil {
		log.Error(ctx, "Swap path validation failed during verification", err)
		return errors.Wrap(err, "swap path validation failed during verification")
	}

	// Verify the swap was successful
	if !response.Success {
		log.Error(ctx, "Swap was not successful during verification", errors.New("swap was not successful"))
		return errors.New("swap was not successful")
	}

	log.Info(ctx, "Swap verification completed successfully")
	return nil
}
