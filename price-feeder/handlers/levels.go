package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/0xcatalysis/core/go-sdk/log"
	"github.com/dextr_avs/price-feeder/models"
	"github.com/dextr_avs/price-feeder/services"
)

type LevelsHandler struct {
	pricer  *services.PricerService
	metrics *services.MetricsService
}

func NewLevelsHandler(pricer *services.PricerService, metrics *services.MetricsService) *LevelsHandler {
	return &LevelsHandler{
		pricer:  pricer,
		metrics: metrics,
	}
}

func (h *LevelsHandler) HandleLevels(w http.ResponseWriter, r *http.Request) {
	ctx := log.WithTopic(r.Context(), "levels-api")
	start := time.Now()

	if r.Method != http.MethodGet {
		h.metrics.RecordHTTPRequest("/levels", r.Method, "405", time.Since(start))
		h.metrics.RecordLevelsError("method_not_allowed")
		h.writeErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allLevels := h.pricer.GetAllLevels(ctx)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Response-Time", time.Since(start).String())
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if err := json.NewEncoder(w).Encode(allLevels); err != nil {
		h.metrics.RecordHTTPRequest("/levels", r.Method, "500", time.Since(start))
		h.metrics.RecordLevelsError("encoding_failed")
		h.writeErrorResponse(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	// Record successful request
	h.metrics.RecordHTTPRequest("/levels", r.Method, "200", time.Since(start))
}

func (h *LevelsHandler) HandleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.WriteHeader(http.StatusOK)
}

func (h *LevelsHandler) writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(statusCode)

	errorResp := models.ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    statusCode,
	}

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}
