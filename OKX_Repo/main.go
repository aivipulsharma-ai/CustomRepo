package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dextr_avs/okx_repo/config"
	"github.com/dextr_avs/okx_repo/handlers"
	"github.com/dextr_avs/okx_repo/middleware"
	"github.com/dextr_avs/okx_repo/services"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("failed to load config: %v", err)
		os.Exit(1)
	}
	if err := cfg.Validate(ctx); err != nil {
		log.Printf("invalid config: %v", err)
		os.Exit(1)
	}

	pricer := services.NewPricerService(cfg)
	signer := services.NewEVMSigner(cfg)

	pricingHandler := handlers.NewPricingHandler(cfg, pricer)
	firmHandler := handlers.NewFirmOrderHandler(cfg, pricer, signer)
	auth := middleware.NewXAPIKeyMiddleware(cfg)

	mux := http.NewServeMux()
	mux.Handle("/OKXDEX/rfq/pricing", auth.Authenticate(http.HandlerFunc(pricingHandler.HandlePricing)))
	mux.Handle("/OKXDEX/rfq/firm-order", auth.Authenticate(http.HandlerFunc(firmHandler.HandleFirmOrder)))

	handler := middleware.CORSMiddleware(mux)
	handler = middleware.HealthCheckMiddleware("/health")(handler)
	handler = middleware.LoggingMiddleware(handler)

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("OKX MM server listening on %s", server.Addr)
		log.Printf("Endpoints:")
		log.Printf("  GET  /OKXDEX/rfq/pricing?chainIndex=<chainIndex>")
		log.Printf("  POST /OKXDEX/rfq/firm-order")
		log.Printf("  GET  /health")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	_ = server.Shutdown(shutdownCtx)
	log.Printf("shutdown complete")
}

