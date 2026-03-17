package services

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsService struct {
	// Trading metrics
	totalHits      prometheus.Counter
	successfulHits prometheus.Counter
	failedHits     prometheus.Counter
	responseTime   prometheus.HistogramVec

	// Token pair metrics
	tokenPairHits    prometheus.CounterVec
	tokenPairSuccess prometheus.CounterVec
	tokenPairFails   prometheus.CounterVec

	// Price metrics
	tokenPrices prometheus.GaugeVec
	pairRates   prometheus.GaugeVec
	priceMarkup prometheus.Gauge

	// System metrics
	startTime        prometheus.Gauge
	hitsPerMinute    prometheus.Gauge
	hitsPerHour      prometheus.Gauge
	lastHitTimestamp prometheus.Gauge

	// HTTP endpoint metrics
	httpRequestsTotal   prometheus.CounterVec
	httpRequestDuration prometheus.HistogramVec

	// Order volume and amount metrics
	orderAmount    prometheus.HistogramVec // Order amounts per token pair
	orderVolumeUSD prometheus.CounterVec   // Cumulative volume in USD per pair
	acceptedPrices prometheus.GaugeVec     // Last accepted price per pair
	totalFeesUSD   prometheus.CounterVec   // Total fees collected per pair

	// Order API error metrics
	orderErrors        prometheus.CounterVec // Order errors by type and status code
	orderErrorsByToken prometheus.CounterVec // Order errors by token pair and error type

	// Service error metrics
	levelsErrors  prometheus.CounterVec // Levels endpoint errors by error type
	balanceErrors prometheus.CounterVec // Balance service errors by error type
	pricerErrors  prometheus.CounterVec // Pricer service errors by error type

	// RPC error metrics
	rpcErrors prometheus.Counter // Total RPC errors (no labels)

	// Oracle API error metrics
	oracleErrors prometheus.Counter // Total Oracle API errors (no labels)

	registry *prometheus.Registry
}

func NewMetricsService() *MetricsService {
	registry := prometheus.NewRegistry()

	ms := &MetricsService{
		// Trading metrics
		totalHits: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "rfq_total_hits",
				Help: "Total number of RFQ hits",
			},
		),
		successfulHits: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "rfq_successful_hits",
				Help: "Number of successful RFQ hits",
			},
		),
		failedHits: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "rfq_failed_hits",
				Help: "Number of failed RFQ hits",
			},
		),
		responseTime: *prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rfq_response_time_seconds",
				Help:    "Response time for RFQ requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{},
		),

		// Token pair metrics
		tokenPairHits: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rfq_token_pair_hits",
				Help: "Total hits per token pair",
			},
			[]string{"token_in", "token_out", "symbol_in", "symbol_out"},
		),
		tokenPairSuccess: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rfq_token_pair_success",
				Help: "Successful hits per token pair",
			},
			[]string{"token_in", "token_out", "symbol_in", "symbol_out"},
		),
		tokenPairFails: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rfq_token_pair_fails",
				Help: "Failed hits per token pair",
			},
			[]string{"token_in", "token_out", "symbol_in", "symbol_out"},
		),

		// Price metrics
		tokenPrices: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rfq_token_price_usd",
				Help: "Current USD price of tokens",
			},
			[]string{"token", "symbol"},
		),
		pairRates: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rfq_pair_exchange_rate",
				Help: "Exchange rate between token pairs (with markup)",
			},
			[]string{"token_in", "token_out", "symbol_in", "symbol_out"},
		),
		priceMarkup: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rfq_price_markup_percent",
				Help: "Current price markup percentage",
			},
		),

		// System metrics
		startTime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rfq_start_time_seconds",
				Help: "Unix timestamp when the service started",
			},
		),
		hitsPerMinute: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rfq_hits_per_minute",
				Help: "Current rate of hits per minute",
			},
		),
		hitsPerHour: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rfq_hits_per_hour",
				Help: "Current rate of hits per hour",
			},
		),
		lastHitTimestamp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rfq_last_hit_timestamp",
				Help: "Timestamp of the last hit",
			},
		),

		// HTTP endpoint metrics
		httpRequestsTotal: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rfq_http_requests_total",
				Help: "Total HTTP requests by endpoint and status",
			},
			[]string{"endpoint", "method", "status"},
		),
		httpRequestDuration: *prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rfq_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: []float64{0.001, 0.01, 0.1, 1, 5, 10},
			},
			[]string{"endpoint", "method"},
		),

		// Order volume and amount metrics
		orderAmount: *prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rfq_order_amount",
				Help:    "Order amounts per token pair",
				Buckets: []float64{0.01, 0.1, 1, 10, 100, 1000, 10000},
			},
			[]string{"token_in", "token_out", "symbol_in", "symbol_out"},
		),
		orderVolumeUSD: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rfq_order_volume_usd_total",
				Help: "Cumulative trading volume in USD per token pair",
			},
			[]string{"token_in", "token_out", "symbol_in", "symbol_out"},
		),
		acceptedPrices: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rfq_accepted_price",
				Help: "Last accepted price (exchange rate) per token pair",
			},
			[]string{"token_in", "token_out", "symbol_in", "symbol_out"},
		),
		totalFeesUSD: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rfq_fees_usd_total",
				Help: "Total fees collected in USD per token pair",
			},
			[]string{"token_in", "token_out", "symbol_in", "symbol_out"},
		),

		// Order API error metrics
		orderErrors: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rfq_order_errors_total",
				Help: "Total order API errors by error type and HTTP status code",
			},
			[]string{"error_type", "status_code"},
		),
		orderErrorsByToken: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rfq_order_errors_by_token_total",
				Help: "Total order API errors by token pair and error type",
			},
			[]string{"token_in", "token_out", "symbol_in", "symbol_out", "error_type"},
		),

		// Service error metrics
		levelsErrors: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rfq_levels_errors_total",
				Help: "Total errors in levels endpoint by error type",
			},
			[]string{"error_type"},
		),
		balanceErrors: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rfq_balance_errors_total",
				Help: "Total errors in balance service by error type",
			},
			[]string{"error_type", "token_symbol"},
		),
		pricerErrors: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rfq_pricer_errors_total",
				Help: "Total errors in pricer service by error type",
			},
			[]string{"error_type"},
		),

		// RPC error metrics
		rpcErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "rfq_rpc_errors_total",
				Help: "Total RPC errors when fetching balances",
			},
		),

		// Oracle API error metrics
		oracleErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "rfq_oracle_errors_total",
				Help: "Total errors from Oracle API endpoint calls",
			},
		),

		registry: registry,
	}

	// Register all metrics
	registry.MustRegister(
		ms.totalHits,
		ms.successfulHits,
		ms.failedHits,
		&ms.responseTime,
		&ms.tokenPairHits,
		&ms.tokenPairSuccess,
		&ms.tokenPairFails,
		&ms.tokenPrices,
		&ms.pairRates,
		ms.priceMarkup,
		ms.startTime,
		ms.hitsPerMinute,
		ms.hitsPerHour,
		ms.lastHitTimestamp,
		&ms.httpRequestsTotal,
		&ms.httpRequestDuration,
		&ms.orderAmount,
		&ms.orderVolumeUSD,
		&ms.acceptedPrices,
		&ms.totalFeesUSD,
		&ms.orderErrors,
		&ms.orderErrorsByToken,
		&ms.levelsErrors,
		&ms.balanceErrors,
		&ms.pricerErrors,
		ms.rpcErrors,
		ms.oracleErrors,
	)

	// Set start time to current time once
	ms.startTime.SetToCurrentTime()

	return ms
}

// Trading metrics methods
func (ms *MetricsService) RecordHit(success bool, responseTime time.Duration) {
	ms.totalHits.Inc()
	if success {
		ms.successfulHits.Inc()
	} else {
		ms.failedHits.Inc()
	}
	ms.responseTime.WithLabelValues().Observe(responseTime.Seconds())
}

func (ms *MetricsService) RecordTokenPairHit(tokenIn, tokenOut, symbolIn, symbolOut string, success bool) {
	ms.tokenPairHits.WithLabelValues(tokenIn, tokenOut, symbolIn, symbolOut).Inc()
	if success {
		ms.tokenPairSuccess.WithLabelValues(tokenIn, tokenOut, symbolIn, symbolOut).Inc()
	} else {
		ms.tokenPairFails.WithLabelValues(tokenIn, tokenOut, symbolIn, symbolOut).Inc()
	}
}

// Price metrics methods
func (ms *MetricsService) UpdateTokenPrice(token, symbol string, price float64) {
	ms.tokenPrices.WithLabelValues(token, symbol).Set(price)
}

func (ms *MetricsService) UpdatePairRate(tokenIn, tokenOut, symbolIn, symbolOut string, rate float64) {
	ms.pairRates.WithLabelValues(tokenIn, tokenOut, symbolIn, symbolOut).Set(rate)
}

func (ms *MetricsService) UpdatePriceMarkup(markupPercent float64) {
	ms.priceMarkup.Set(markupPercent)
}

// System metrics methods
func (ms *MetricsService) UpdateSystemMetrics(hpm, hph float64, lastHit *time.Time) {
	ms.hitsPerMinute.Set(hpm)
	ms.hitsPerHour.Set(hph)

	if lastHit != nil {
		ms.lastHitTimestamp.Set(float64(lastHit.Unix()))
	}
}

// HTTP metrics methods
func (ms *MetricsService) RecordHTTPRequest(endpoint, method, status string, duration time.Duration) {
	ms.httpRequestsTotal.WithLabelValues(endpoint, method, status).Inc()
	ms.httpRequestDuration.WithLabelValues(endpoint, method).Observe(duration.Seconds())
}

// Order volume metrics methods
func (ms *MetricsService) RecordOrderVolume(tokenIn, tokenOut, symbolIn, symbolOut string, amount, volumeUSD, acceptedPrice float64) {
	ms.orderAmount.WithLabelValues(tokenIn, tokenOut, symbolIn, symbolOut).Observe(amount)
	ms.orderVolumeUSD.WithLabelValues(tokenIn, tokenOut, symbolIn, symbolOut).Add(volumeUSD)
	ms.acceptedPrices.WithLabelValues(tokenIn, tokenOut, symbolIn, symbolOut).Set(acceptedPrice)
}

// Order error metrics methods
func (ms *MetricsService) RecordOrderError(errorType, statusCode string) {
	ms.orderErrors.WithLabelValues(errorType, statusCode).Inc()
}

func (ms *MetricsService) RecordOrderErrorByToken(tokenIn, tokenOut, symbolIn, symbolOut, errorType string) {
	ms.orderErrorsByToken.WithLabelValues(tokenIn, tokenOut, symbolIn, symbolOut, errorType).Inc()
}

// Get HTTP handler for Prometheus metrics endpoint
func (ms *MetricsService) GetHandler() http.Handler {
	return promhttp.HandlerFor(ms.registry, promhttp.HandlerOpts{})
}

// Service error metrics methods
func (ms *MetricsService) RecordLevelsError(errorType string) {
	ms.levelsErrors.WithLabelValues(errorType).Inc()
}

func (ms *MetricsService) RecordBalanceError(errorType, tokenSymbol string) {
	ms.balanceErrors.WithLabelValues(errorType, tokenSymbol).Inc()
}

func (ms *MetricsService) RecordPricerError(errorType string) {
	ms.pricerErrors.WithLabelValues(errorType).Inc()
}

// RPC error metrics methods
func (ms *MetricsService) RecordRPCError() {
	ms.rpcErrors.Inc()
}

// Oracle API error metrics methods
func (ms *MetricsService) RecordOracleError() {
	ms.oracleErrors.Inc()
}

// Update all metrics from counter service
func (ms *MetricsService) UpdateFromCounterService(counterService *CounterService) {
	stats := counterService.GetStatistics()

	// Update system metrics (uptime is calculated by Prometheus using start_time)
	ms.UpdateSystemMetrics(stats.HitsPerMinute, stats.HitsPerHour, stats.LastHit)

	// Note: Individual hit metrics are recorded in real-time via RecordHit method
	// Token pair and price metrics are updated in real-time via their respective methods
}

// InitializeCounters initializes all counter metrics to zero so they appear on Grafana dashboard immediately
func (ms *MetricsService) InitializeCounters() {
	// Initialize simple counters (no labels)
	ms.totalHits.Add(0)
	ms.successfulHits.Add(0)
	ms.failedHits.Add(0)

	// Initialize gauges to zero
	ms.hitsPerMinute.Set(0)
	ms.hitsPerHour.Set(0)
	ms.lastHitTimestamp.Set(0)

	// Initialize error counters with common error types so rate() works in Grafana
	// Order API errors by type and status code
	orderErrorTypes := []struct {
		errorType  string
		statusCode string
	}{
		{"price_unavailable", "503"},
		{"balance_service_unavailable", "503"},
		{"insufficient_balance", "422"},
		{"zero_balance", "422"},
		{"invalid_amount_format", "400"},
		{"invalid_amount_value", "400"},
		{"unknown", "500"},
	}
	for _, errType := range orderErrorTypes {
		ms.orderErrors.WithLabelValues(errType.errorType, errType.statusCode).Add(0)
	}

	// Levels endpoint errors
	levelsErrorTypes := []string{
		"method_not_allowed",
		"encoding_failed",
	}
	for _, errType := range levelsErrorTypes {
		ms.levelsErrors.WithLabelValues(errType).Add(0)
	}

	// Balance service errors (initialize with placeholder token symbol)
	balanceErrorTypes := []string{
		"fetch_balance_failed",
		"abi_parse_failed",
		"pack_call_failed",
		"contract_call_failed",
		"unpack_result_failed",
	}
	for _, errType := range balanceErrorTypes {
		ms.balanceErrors.WithLabelValues(errType, "unknown").Add(0)
	}

	// Pricer service errors
	pricerErrorTypes := []string{
		"create_request_failed",
		"api_request_failed",
		"api_non_ok_status",
		"read_response_failed",
		"parse_json_failed",
		"missing_price_data",
	}
	for _, errType := range pricerErrorTypes {
		ms.pricerErrors.WithLabelValues(errType).Add(0)
	}

	// Initialize RPC errors counter
	ms.rpcErrors.Add(0)

	// Initialize Oracle API errors counter
	ms.oracleErrors.Add(0)

	// Note: Token pair metrics (tokenPairHits, tokenPairSuccess, etc.) and
	// orderErrorsByToken will be initialized when first used with specific labels,
	// as they require knowing actual token addresses/symbols from requests.
}
