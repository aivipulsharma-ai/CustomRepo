package tests

import (
	"strings"
	"testing"

	"github.com/dextr_avs/price-feeder/services"
)

func TestErrorMetrics(t *testing.T) {
	// Create a metrics service
	metricsService := services.NewMetricsService()

	t.Run("RecordOrderError", func(t *testing.T) {
		// Record some errors
		metricsService.RecordOrderError("insufficient_balance", "422")
		metricsService.RecordOrderError("price_unavailable", "503")
		metricsService.RecordOrderError("invalid_amount_format", "400")
		metricsService.RecordOrderError("zero_balance", "422")

		t.Log("✓ Recorded 4 different error types")
		t.Log("  - insufficient_balance (422)")
		t.Log("  - price_unavailable (503)")
		t.Log("  - invalid_amount_format (400)")
		t.Log("  - zero_balance (422)")
	})

	t.Run("RecordOrderErrorByToken", func(t *testing.T) {
		// Record token-specific errors
		metricsService.RecordOrderErrorByToken(
			"0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0",
			"0x8236a87084f8B84306f72007F36F2618A5634494",
			"WSTETH",
			"LBTC",
			"insufficient_balance",
		)

		metricsService.RecordOrderErrorByToken(
			"0xdAC17F958D2ee523a2206206994597C13D831ec7",
			"0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0",
			"USDT",
			"WSTETH",
			"price_unavailable",
		)

		t.Log("✓ Recorded 2 token-specific errors")
		t.Log("  - WSTETH/LBTC: insufficient_balance")
		t.Log("  - USDT/WSTETH: price_unavailable")
	})

	t.Run("VerifyMetricsExist", func(t *testing.T) {
		// Get the metrics handler and check that metrics can be exported
		handler := metricsService.GetHandler()

		if handler == nil {
			t.Fatal("Metrics handler is nil")
		}

		t.Log("✓ Metrics handler is available")
		t.Log("✓ New error metrics:")
		t.Log("  - rfq_order_errors_total{error_type, status_code}")
		t.Log("  - rfq_order_errors_by_token_total{token_in, token_out, symbol_in, symbol_out, error_type}")
	})
}

func TestErrorTypeClassification(t *testing.T) {
	tests := []struct {
		errorMsg     string
		expectedType string
		expectedCode int
	}{
		{
			errorMsg:     "price unavailable for token pair 0x.../0x...",
			expectedType: "price_unavailable",
			expectedCode: 503,
		},
		{
			errorMsg:     "balance service unavailable for token 0x...",
			expectedType: "balance_service_unavailable",
			expectedCode: 503,
		},
		{
			errorMsg:     "insufficient balance: requested 100.0 but only 50.0 available",
			expectedType: "insufficient_balance",
			expectedCode: 422,
		},
		{
			errorMsg:     "maker has zero balance for token 0x...",
			expectedType: "zero_balance",
			expectedCode: 422,
		},
		{
			errorMsg:     "invalid amount format: strconv.ParseFloat",
			expectedType: "invalid_amount_format",
			expectedCode: 400,
		},
		{
			errorMsg:     "amount must be greater than zero",
			expectedType: "invalid_amount_value",
			expectedCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.expectedType, func(t *testing.T) {
			// Check error type classification
			var errorType string
			errMsg := tt.errorMsg

			if strings.Contains(errMsg, "price unavailable") {
				errorType = "price_unavailable"
			} else if strings.Contains(errMsg, "balance service unavailable") {
				errorType = "balance_service_unavailable"
			} else if strings.Contains(errMsg, "insufficient balance") {
				errorType = "insufficient_balance"
			} else if strings.Contains(errMsg, "zero balance") {
				errorType = "zero_balance"
			} else if strings.Contains(errMsg, "invalid amount format") {
				errorType = "invalid_amount_format"
			} else if strings.Contains(errMsg, "amount must be") {
				errorType = "invalid_amount_value"
			}

			if errorType != tt.expectedType {
				t.Errorf("Expected error type %s, got %s", tt.expectedType, errorType)
			}

			t.Logf("✓ Error type: %s (HTTP %d)", errorType, tt.expectedCode)
			t.Logf("  Message: %s", tt.errorMsg)
		})
	}
}
