package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/dextr_avs/okx_repo/config"
	"github.com/dextr_avs/okx_repo/models"
)

type XAPIKeyMiddleware struct {
	cfg *config.Config
}

func NewXAPIKeyMiddleware(cfg *config.Config) *XAPIKeyMiddleware {
	return &XAPIKeyMiddleware{cfg: cfg}
}

func (m *XAPIKeyMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.cfg.Auth.XAPIKey == "" {
			writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
				Code: "401",
				Msg:  "X-API-KEY not configured on server",
			})
			return
		}

		if r.Header.Get("X-API-KEY") != m.cfg.Auth.XAPIKey {
			writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
				Code: "401",
				Msg:  "Unauthorized",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

