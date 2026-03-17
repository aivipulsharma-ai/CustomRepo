package handlers

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/0xcatalysis/catalyst-sdk/go-sdk/log"
	"github.com/0xcatalysis/catalyst-sdk/go-sdk/types"
	"github.com/0xcatalysis/catalyst-sdk/go-sdk/z"

	avstypes "github.com/dextr_avs/types"
)

// SwapRequest represents the request data structure for token swaps
type SwapRequest struct {
	InputToken  string  `json:"input_token"`
	OutputToken string  `json:"output_token"`
	Amount      float64 `json:"amount"`
}

// HandleSwapRequest handles HTTP requests for token swap operations
func HandleSwapRequest(avs interface{ ProcessTask(tx types.Task) error }) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only allow POST method
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract transaction ID from URL path
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) < 3 {
			http.Error(w, "Invalid URL path", http.StatusBadRequest)
			return
		}
		txIDStr := pathParts[len(pathParts)-1]

		// Convert txID to big.Int
		txID := new(big.Int)
		_, success := txID.SetString(txIDStr, 10)
		if !success {
			http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
			return
		}

		// Parse request
		var request SwapRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}

		// Create transaction
		tx := types.Task{
			ID:        txID,
			Type:      avstypes.TaskTypeSwap,
			Timestamp: time.Now().UTC(),
		}

		// Serialize request
		data, err := json.Marshal(request)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error serializing request: %v", err), http.StatusInternalServerError)
			return
		}
		tx.Data = data

		ctx := log.WithTopic(r.Context(), "SwapRequest")
		// Process transaction
		log.Info(ctx, "Sending request to swap avs to process task",
			z.I64("tx_id", txID.Int64()),
			z.Str("input_token", request.InputToken),
			z.Str("output_token", request.OutputToken),
			z.F64("amount", request.Amount))
		err = avs.ProcessTask(tx)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error processing transaction: %v", err), http.StatusInternalServerError)
			return
		}

		log.Info(ctx, "Task has been sent to consensus for further processing",
			z.I64("tx_id", txID.Int64()),
			z.Str("input_token", request.InputToken),
			z.Str("output_token", request.OutputToken),
			z.F64("amount", request.Amount))
	}
}
