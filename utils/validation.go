package utils

import (
	"github.com/0xcatalysis/catalyst-sdk/go-sdk/errors"
	"github.com/dextr_avs/types"
)

// ValidateSwapInput validates the input for token swap calculation
func ValidateSwapInput(inputToken, outputToken string, amount float64) error {
	if inputToken == "" {
		return errors.New("input token cannot be empty")
	}
	if outputToken == "" {
		return errors.New("output token cannot be empty")
	}
	if inputToken == outputToken {
		return errors.New("input and output tokens cannot be the same")
	}
	if amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	return nil
}

// ValidateSwapPath validates the swap path
func ValidateSwapPath(path []types.Transfer, inputToken, outputToken string) error {
	if len(path) == 0 {
		return errors.New("swap path cannot be empty")
	}

	// Check if path starts with input token
	if path[0].FromToken != inputToken {
		return errors.New("swap path must start with input token")
	}

	// Check if path ends with output token
	if path[len(path)-1].ToToken != outputToken {
		return errors.New("swap path must end with output token")
	}

	// Check if path is continuous
	for i := 0; i < len(path)-1; i++ {
		if path[i].ToToken != path[i+1].FromToken {
			return errors.New("swap path must be continuous")
		}
	}

	return nil
}
