package types

// Common constants used throughout the application
const (
	// TaskTypeSwap represents the token swap task type
	TaskTypeSwap = "swap"
)

// Transfer represents a temporary transfer during a swap path
type Transfer struct {
	FromToken string  `json:"from_token"`
	ToToken   string  `json:"to_token"`
	Amount    float64 `json:"amount"`
}
