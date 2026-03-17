package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func (pfe *PriceFeedEngine) handleLevels(w http.ResponseWriter, r *http.Request) {
	levels := pfe.GeneratePriceLevels()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(levels)
}

func (pfe *PriceFeedEngine) handleOrder(w http.ResponseWriter, r *http.Request) {
	var req OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	orderResp, err := pfe.ProcessOrder(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orderResp)
}

func (pfe *PriceFeedEngine) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := pfe.vaultManager.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Background Services
func (pfe *PriceFeedEngine) startPriceUpdates(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pfe.oracleClient.UpdatePrices()
			pfe.GeneratePriceLevels()
		}
	}
}

func (pfe *PriceFeedEngine) startRebalancing(ctx context.Context) {
	ticker := time.NewTicker(pfe.rebalanceParams.EpochDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pfe.ExecuteRebalance()
		}
	}
}

func (pfe *PriceFeedEngine) startStatusLogger(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := pfe.vaultManager.GetStatus()
			log.Println("=== System Status ===")

			if mainVaults, ok := status["mainVaults"].(map[string]interface{}); ok {
				log.Println("Main Vaults:")
				for token, s := range mainVaults {
					statusMap := s.(map[string]interface{})
					log.Printf("  %s: Balance=%.6f, Target=%.6f, Deviation=%.2f%%",
						token,
						statusMap["balance"].(float64),
						statusMap["target"].(float64),
						statusMap["deviationPct"].(float64))
				}
			}

			if balancerVault, ok := status["balancerVault"].(map[string]float64); ok {
				log.Println("Balancer Vault:")
				if len(balancerVault) == 0 {
					log.Println("  Empty")
				} else {
					for token, balance := range balancerVault {
						log.Printf("  %s: %.6f", token, balance)
					}
				}
			}

			if feeVault, ok := status["feeCollectionVault"].(map[string]float64); ok {
				log.Println("Fee Collection Vault:")
				if len(feeVault) == 0 {
					log.Println("  Empty")
				} else {
					totalFeeValue := 0.0
					for token, balance := range feeVault {
						log.Printf("  %s: %.6f", token, balance)
						if token == "USDC" || token == "USDT" {
							totalFeeValue += balance
						} else if token == "BTC" {
							if price, err := pfe.oracleClient.GetPrice("BTC", "USDC"); err == nil {
								totalFeeValue += balance * price
							}
						} else if token == "ETH" {
							if price, err := pfe.oracleClient.GetPrice("ETH", "USDC"); err == nil {
								totalFeeValue += balance * price
							}
						}
					}
					log.Printf("  Total Fee Value (USD approx): %.2f", totalFeeValue)
				}
			}

			if btcPrice, err := pfe.oracleClient.GetPrice("BTC", "USDC"); err == nil {
				if ethPrice, err := pfe.oracleClient.GetPrice("ETH", "USDC"); err == nil {
					log.Printf("Current Prices: BTC/USDC=%.2f, ETH/USDC=%.2f", btcPrice, ethPrice)
				}
			}

			log.Println("========================")
		}
	}
}

func setupRoutes(pfe *PriceFeedEngine) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/levels", pfe.handleLevels).Methods("GET")
	r.HandleFunc("/order", pfe.handleOrder).Methods("POST")
	r.HandleFunc("/status", pfe.handleStatus).Methods("GET")

	// Add CORS headers
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	return r
}
