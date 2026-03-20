package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/dextr_avs/okx_repo/config"
	"github.com/dextr_avs/okx_repo/models"
	"github.com/dextr_avs/okx_repo/services"
)

type PricingHandler struct {
	cfg    *config.Config
	pricer *services.PricerService
}

func NewPricingHandler(cfg *config.Config, pricer *services.PricerService) *PricingHandler {
	return &PricingHandler{cfg: cfg, pricer: pricer}
}

func (h *PricingHandler) HandlePricing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOKXError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	chainIndex := r.URL.Query().Get("chainIndex")
	if chainIndex == "" {
		writeOKXError(w, http.StatusBadRequest, "missing chainIndex", "")
		return
	}
	if _, ok := h.cfg.FindChain(chainIndex); !ok {
		writeOKXError(w, http.StatusBadRequest, "unsupported chainIndex", chainIndex)
		return
	}

	// Build non-cumulative, level-by-level quotes.
	// This reference impl provides a single level per pair with a fixed quantity.
	const qty = "1000"

	var levelData []models.LevelData
	for _, taker := range h.cfg.Tokens {
		for _, maker := range h.cfg.Tokens {
			if strings.EqualFold(taker.Address, maker.Address) {
				continue
			}
			rate, err := h.pricer.TakerTokenRate(taker.Address, maker.Address)
			if err != nil {
				continue
			}
			levelData = append(levelData, models.LevelData{
				TakerTokenAddress: taker.Address,
				MakerTokenAddress: maker.Address,
				Levels: [][2]string{
					{qty, ratToDecimalString(rate, 18)},
				},
			})
		}
	}

	// Ensure stable output order (helps debugging).
	sort.Slice(levelData, func(i, j int) bool {
		if levelData[i].TakerTokenAddress == levelData[j].TakerTokenAddress {
			return levelData[i].MakerTokenAddress < levelData[j].MakerTokenAddress
		}
		return levelData[i].TakerTokenAddress < levelData[j].TakerTokenAddress
	})

	resp := models.OKXResponse[models.PricingData]{
		Code: "0",
		Msg:  "",
		Data: models.PricingData{
			ChainIndex: chainIndex,
			LevelData:  levelData,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

