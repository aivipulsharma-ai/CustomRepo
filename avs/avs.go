package avs

import (
	"github.com/0xcatalysis/catalyst-sdk/go-sdk/baseapp"
	"github.com/dextr_avs/handlers"
)

// DextrAVS is an AVS that handles token swap operations
type DextrAVS struct {
	*baseapp.BaseApp
}

// NewDextrAVS creates a new instance of DextrAVS with customizable options
func NewDextrAVS(config *baseapp.Config) (*DextrAVS, error) {
	// Create base app with HTTP server options
	baseApp, err := baseapp.NewBaseApp(config)
	if err != nil {
		return nil, err
	}

	// Create AVS
	avs := &DextrAVS{
		BaseApp: baseApp,
	}

	// Register HTTP handler for token swap endpoint
	avs.RegisterHTTPHandler("/swap/{tx_id}", handlers.HandleSwapRequest(avs))

	// Register task handler for token swap calculations
	avs.RegisterHandler("swap", &handlers.SwapHandler{})

	return avs, nil
}
