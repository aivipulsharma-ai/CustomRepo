package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/0xcatalysis/catalyst-sdk/go-sdk/errors"
	"github.com/0xcatalysis/catalyst-sdk/go-sdk/z"

	"github.com/0xcatalysis/catalyst-sdk/go-sdk/log"
	"github.com/spf13/cobra"

	"github.com/dextr_avs/handlers"
)

func NewSwapCmd() *cobra.Command {
	swapCmd := &cobra.Command{
		Use:   "swap [input_token] [output_token] [amount] [tx_id]",
		Short: "Perform a token swap operation",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := log.WithTopic(cmd.Context(), "swap")

			// Parse the input token
			inputToken := args[0]

			// Parse the output token
			outputToken := args[1]

			// Parse the amount
			amount, err := strconv.ParseFloat(args[2], 64)
			if err != nil {
				return errors.Wrap(err, "invalid amount")
			}

			// Get server URL from flags or use default
			serverURL, _ := cmd.Flags().GetString("server")

			// Get transaction ID from args
			txID := args[3]

			// Create request
			request := handlers.SwapRequest{
				InputToken:  inputToken,
				OutputToken: outputToken,
				Amount:      amount,
			}
			requestData, err := json.Marshal(request)
			if err != nil {
				return errors.Wrap(err, "error encoding request")
			}

			// Send request to server
			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Post(serverURL+"/swap/"+txID, "application/json", bytes.NewBuffer(requestData))
			if err != nil {
				return errors.Wrap(err, "error sending request")
			}
			defer func(Body io.ReadCloser) {
				err = Body.Close()
				if err != nil {
					log.Error(ctx, "error closing response body", err)
					return
				}
			}(resp.Body)

			// Check response status
			if resp.StatusCode != http.StatusOK {
				return errors.New("server returned status code", z.Int("responseCode", resp.StatusCode))
			}

			log.Info(ctx, "Swap request sent successfully",
				z.Str("input_token", inputToken),
				z.Str("output_token", outputToken),
				z.F64("amount", amount),
				z.Str("tx_id", txID))
			return nil
		},
	}

	// Add flags
	swapCmd.Flags().String("server", "http://localhost:8080", "Server URL")

	return swapCmd
}
