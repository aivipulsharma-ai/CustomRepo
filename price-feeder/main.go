package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xcatalysis/core/go-sdk/log"
	"github.com/0xcatalysis/core/go-sdk/z"

	"github.com/dextr_avs/price-feeder/config"
	"github.com/dextr_avs/price-feeder/handlers"
	"github.com/dextr_avs/price-feeder/middleware"
	"github.com/dextr_avs/price-feeder/services"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Error(ctx, "failed to load config", err)
		os.Exit(1)
	}

	// Initialize logger with default config
	logConfig := log.DefaultConfig()
	logConfig.Level = cfg.LogLevel
	if err := log.InitLogger(logConfig); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}

	loggingMiddleware := middleware.NewLoggingMiddleware()
	authMiddleware := middleware.NewOneInchAuthMiddleware(cfg)

	counterService := services.NewCounterService()

	// Metrics are mandatory - always initialized
	metricsService := services.NewMetricsService()
	metricsService.UpdatePriceMarkup(cfg.Pricing.PriceMarkup)

	// Initialize all counter metrics to zero so they appear on Grafana dashboard immediately
	metricsService.InitializeCounters()

	// Pass metrics to pricer service at construction time
	pricerService, err := services.NewPricerService(ctx, cfg, metricsService)
	if err != nil {
		log.Error(ctx, "failed to create pricer service", err)
		os.Exit(1)
	}

	levelsHandler := handlers.NewLevelsHandler(pricerService, metricsService)
	ordersHandler := handlers.NewOrdersHandler(pricerService, counterService, metricsService, cfg)

	mux := http.NewServeMux()

	// Apply authentication middleware to protected endpoints
	// Handle OPTIONS requests without authentication for CORS preflight
	mux.HandleFunc("/levels", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			levelsHandler.HandleOptions(w, r)
			return
		}
		authMiddleware.Authenticate(http.HandlerFunc(levelsHandler.HandleLevels)).ServeHTTP(w, r)
	})

	mux.HandleFunc("/order", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			ordersHandler.HandleOptions(w, r)
			return
		}
		// TODO: Re-enable auth for production
		authMiddleware.Authenticate(http.HandlerFunc(ordersHandler.HandleOrder)).ServeHTTP(w, r)
		// ordersHandler.HandleOrder(w, r) // Auth disabled for local testing
	})

	mux.HandleFunc("/stats", ordersHandler.HandleStats)
	mux.HandleFunc("/recent-hits", ordersHandler.HandleRecentHits)
	mux.HandleFunc("/reset", ordersHandler.HandleReset)

	// Start a separate metrics server on the configured port
	metricsServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Metrics.MetricsPort),
		Handler:      metricsService.GetHandler(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		log.Info(ctx, "Starting Prometheus metrics server", z.Str("addr", metricsServer.Addr))
		if err = metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error(ctx, "Metrics server error", err)
		}
	}()

	handler := middleware.CORSMiddleware(mux)
	handler = middleware.HealthCheckMiddleware("/health")(handler)
	handler = loggingMiddleware.LogRequest(handler)

	if cfg.Server.Port == 8080 {
		handler = middleware.RateLimitMiddleware(1000)(handler)
	}

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info(ctx, "Starting 1inch RFQ Tester server", z.Str("addr", server.Addr))
		log.Info(ctx, "Endpoints available:")
		log.Info(ctx, "  GET  /levels?tokenIn=<address>&tokenOut=<address>&amount=<amount>")
		log.Info(ctx, "  POST /order")
		log.Info(ctx, "  GET  /stats")
		log.Info(ctx, "  GET  /recent-hits?limit=<number>")
		log.Info(ctx, "  POST /reset")
		log.Info(ctx, "  GET  /health")

		if err = server.ListenAndServe(); err != nil && !errors.Is(http.ErrServerClosed, err) {
			log.Error(ctx, "Server failed to start", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info(ctx, "Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err = server.Shutdown(shutdownCtx); err != nil {
		log.Error(ctx, "Server forced to shutdown", err)
		os.Exit(1)
	}

	if err = metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Error(ctx, "Metrics server forced to shutdown", err)
		os.Exit(1)
	}

	log.Info(ctx, "Server shut down successfully")
}
