package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xcatalysis/core/go-sdk/log"

	"github.com/dextr_avs/price-feeder/models"
)

type CounterService struct {
	totalHits       int64        // Atomic: no lock needed
	successfulHits  int64        // Atomic: no lock needed
	failedHits      int64        // Atomic: no lock needed
	startTime       time.Time    // Immutable after init: no lock needed
	lastHit         atomic.Value // Stores *time.Time, lock-free with atomic.Value
	mu              sync.RWMutex // Protects: tokenPairStats, hourlyBreakdown
	tokenPairStats  map[string]*models.PairStats
	hourlyBreakdown map[string]int64
	responseTimes   []time.Duration
	responseTimesMu sync.RWMutex // Protects: responseTimes
	recentHits      []models.TradeHit
	recentHitsMu    sync.RWMutex // Protects: recentHits
	maxRecentHits   int
}

func NewCounterService() *CounterService {
	return &CounterService{
		startTime:       time.Now(),
		tokenPairStats:  make(map[string]*models.PairStats),
		hourlyBreakdown: make(map[string]int64),
		responseTimes:   make([]time.Duration, 0),
		recentHits:      make([]models.TradeHit, 0),
		maxRecentHits:   1000,
	}
}

func (cs *CounterService) RecordHit(tokenIn, tokenOut, amount string, feeBps int, trader string, success bool, responseTime time.Duration, acceptedPrice, volumeUSD, feeUSD, amountFloat float64) string {
	hitID := cs.generateHitID()
	now := time.Now()

	// Fast atomic operations (no locks needed)
	atomic.AddInt64(&cs.totalHits, 1)
	if success {
		atomic.AddInt64(&cs.successfulHits, 1)
	} else {
		atomic.AddInt64(&cs.failedHits, 1)
	}

	// Update lastHit atomically (no lock needed!)
	cs.lastHit.Store(&now)

	// Lock ONLY for map operations (tokenPairStats, hourlyBreakdown)
	cs.mu.Lock()

	// Update token pair stats with volume and price tracking
	pairKey := cs.getPairKey(tokenIn, tokenOut)
	stats, exists := cs.tokenPairStats[pairKey]
	if !exists {
		stats = &models.PairStats{}
		cs.tokenPairStats[pairKey] = stats
	}
	stats.TotalHits++
	if success {
		stats.SuccessfulHits++
		// Track volume and fees only for successful orders
		stats.TotalVolumeUSD += volumeUSD
		stats.TotalFeesUSD += feeUSD

		// Calculate running average trade size
		if stats.SuccessfulHits > 0 {
			stats.AverageTradeSize = ((stats.AverageTradeSize * float64(stats.SuccessfulHits-1)) + amountFloat) / float64(stats.SuccessfulHits)
		}
	} else {
		stats.FailedHits++
	}
	stats.LastPrice = fmt.Sprintf("%.6f", acceptedPrice)
	stats.LastHit = now

	// Update hourly breakdown (previously in updateHourlyBreakdown)
	hourKey := now.Format("2006-01-02-15")
	cs.hourlyBreakdown[hourKey]++
	if len(cs.hourlyBreakdown) > 168 {
		oldestTime := now.Add(-168 * time.Hour)
		oldestKey := oldestTime.Format("2006-01-02-15")
		delete(cs.hourlyBreakdown, oldestKey)
	}

	cs.mu.Unlock()

	// These use separate locks, so can be done outside cs.mu
	cs.recordResponseTime(responseTime)
	cs.addRecentHit(models.TradeHit{
		ID:            hitID,
		BaseToken:     tokenIn,
		QuoteToken:    tokenOut,
		Amount:        amount,
		AcceptedPrice: fmt.Sprintf("%.6f", acceptedPrice),
		VolumeUSD:     fmt.Sprintf("%.2f", volumeUSD),
		Taker:         trader,
		FeeBps:        feeBps,
		FeeAmount:     fmt.Sprintf("%.2f", feeUSD),
		Timestamp:     now,
		Success:       success,
	})

	return hitID
}

func (cs *CounterService) GetStatistics() *models.Statistics {
	// Read atomic values WITHOUT lock (thread-safe atomic operations)
	totalHits := atomic.LoadInt64(&cs.totalHits)
	successfulHits := atomic.LoadInt64(&cs.successfulHits)
	failedHits := atomic.LoadInt64(&cs.failedHits)

	// Read immutable startTime (no lock needed)
	uptime := time.Since(cs.startTime)
	hitsPerMinute := float64(totalHits) / uptime.Minutes()
	hitsPerHour := float64(totalHits) / uptime.Hours()

	// Read from separate lock (responseTimesMu)
	avgResponseTime := cs.calculateAverageResponseTime()

	// Read lastHit atomically (no lock needed!)
	var lastHit *time.Time
	if val := cs.lastHit.Load(); val != nil {
		lastHit = val.(*time.Time)
	}

	// Lock ONLY for map iteration (significantly reduced lock hold time)
	cs.mu.RLock()
	tokenPairStatsCopy := make(map[string]*models.PairStats)
	var totalVolumeUSD, totalFeesUSD float64
	for k, v := range cs.tokenPairStats {
		tokenPairStatsCopy[k] = &models.PairStats{
			TotalHits:        v.TotalHits,
			SuccessfulHits:   v.SuccessfulHits,
			FailedHits:       v.FailedHits,
			LastPrice:        v.LastPrice,
			LastHit:          v.LastHit,
			TotalVolumeUSD:   v.TotalVolumeUSD,
			AverageTradeSize: v.AverageTradeSize,
			TotalFeesUSD:     v.TotalFeesUSD,
		}
		// Aggregate global volume and fees
		totalVolumeUSD += v.TotalVolumeUSD
		totalFeesUSD += v.TotalFeesUSD
	}

	hourlyBreakdownCopy := make(map[string]int64)
	for k, v := range cs.hourlyBreakdown {
		hourlyBreakdownCopy[k] = v
	}
	cs.mu.RUnlock()

	// Calculate average order size in USD
	var averageOrderSizeUSD float64
	if successfulHits > 0 {
		averageOrderSizeUSD = totalVolumeUSD / float64(successfulHits)
	}

	return &models.Statistics{
		TotalHits:           totalHits,
		SuccessfulHits:      successfulHits,
		FailedHits:          failedHits,
		HitsPerMinute:       hitsPerMinute,
		HitsPerHour:         hitsPerHour,
		AverageResponseTime: avgResponseTime,
		LastHit:             lastHit,
		TokenPairStats:      tokenPairStatsCopy,
		HourlyBreakdown:     hourlyBreakdownCopy,
		Uptime:              uptime,
		TotalVolumeUSD:      totalVolumeUSD,
		TotalFeesUSD:        totalFeesUSD,
		AverageOrderSizeUSD: averageOrderSizeUSD,
	}
}

func (cs *CounterService) GetRecentHits(limit int) []models.TradeHit {
	cs.recentHitsMu.RLock()
	defer cs.recentHitsMu.RUnlock()

	if limit <= 0 || limit > len(cs.recentHits) {
		limit = len(cs.recentHits)
	}

	result := make([]models.TradeHit, limit)
	copy(result, cs.recentHits[len(cs.recentHits)-limit:])

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

func (cs *CounterService) Reset() {
	// Reset atomic counters (no lock needed)
	atomic.StoreInt64(&cs.totalHits, 0)
	atomic.StoreInt64(&cs.successfulHits, 0)
	atomic.StoreInt64(&cs.failedHits, 0)

	// Reset lastHit atomically (no lock needed!)
	cs.lastHit.Store((*time.Time)(nil))

	// Lock only for map operations
	cs.mu.Lock()
	cs.startTime = time.Now()
	cs.tokenPairStats = make(map[string]*models.PairStats)
	cs.hourlyBreakdown = make(map[string]int64)
	cs.mu.Unlock()

	// Reset response times with separate lock
	cs.responseTimesMu.Lock()
	cs.responseTimes = cs.responseTimes[:0]
	cs.responseTimesMu.Unlock()

	// Reset recent hits with separate lock
	cs.recentHitsMu.Lock()
	cs.recentHits = cs.recentHits[:0]
	cs.recentHitsMu.Unlock()
}

// updateTokenPairStats and updateHourlyBreakdown have been inlined into RecordHit
// for better performance (reduced lock contention from 3 acquisitions to 1)

func (cs *CounterService) recordResponseTime(responseTime time.Duration) {
	cs.responseTimesMu.Lock()
	defer cs.responseTimesMu.Unlock()

	cs.responseTimes = append(cs.responseTimes, responseTime)

	if len(cs.responseTimes) > 10000 {
		cs.responseTimes = cs.responseTimes[1000:]
	}
}

func (cs *CounterService) calculateAverageResponseTime() time.Duration {
	cs.responseTimesMu.RLock()
	defer cs.responseTimesMu.RUnlock()

	if len(cs.responseTimes) == 0 {
		return 0
	}

	var total time.Duration
	for _, rt := range cs.responseTimes {
		total += rt
	}

	return total / time.Duration(len(cs.responseTimes))
}

func (cs *CounterService) addRecentHit(hit models.TradeHit) {
	cs.recentHitsMu.Lock()
	defer cs.recentHitsMu.Unlock()

	cs.recentHits = append(cs.recentHits, hit)

	if len(cs.recentHits) > cs.maxRecentHits {
		cs.recentHits = cs.recentHits[1:]
	}
}

func (cs *CounterService) generateHitID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		log.Warn(context.Background(), "Failed to generate random bytes for hit ID", err)
	}
	return hex.EncodeToString(bytes)
}

func (cs *CounterService) getPairKey(tokenA, tokenB string) string {
	if tokenA < tokenB {
		return fmt.Sprintf("%s-%s", tokenA, tokenB)
	}
	return fmt.Sprintf("%s-%s", tokenB, tokenA)
}
