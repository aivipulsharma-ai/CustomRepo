package services

import (
	"errors"
	"math/big"
	"strings"
	"sync"

	"github.com/dextr_avs/okx_repo/config"
)

// PricerService is intentionally simple: it derives a takerTokenRate from a
// small set of stable assumptions:
// - USDT and USDC are treated as 1 USD (for example purposes)
// - everything else defaults to 0 (unknown), so those pairs will not be quoted
//
// Replace this with your real pricing engine / oracle for production use.
type PricerService struct {
	cfg *config.Config
	mu  sync.RWMutex

	// usdPriceBySymbol is an optional in-memory override.
	usdPriceBySymbol map[string]*big.Rat
}

func NewPricerService(cfg *config.Config) *PricerService {
	s := &PricerService{
		cfg:              cfg,
		usdPriceBySymbol: map[string]*big.Rat{},
	}
	// default stablecoins
	s.usdPriceBySymbol["USDT"] = big.NewRat(1, 1)
	s.usdPriceBySymbol["USDC"] = big.NewRat(1, 1)
	// default majors (reference-only; replace with a real oracle)
	s.usdPriceBySymbol["WETH"] = big.NewRat(3000, 1)
	return s
}

func (s *PricerService) SetUSDPrice(symbol string, price *big.Rat) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usdPriceBySymbol[strings.ToUpper(symbol)] = new(big.Rat).Set(price)
}

func (s *PricerService) FindToken(address string) (config.Token, bool) {
	for _, t := range s.cfg.Tokens {
		if strings.EqualFold(t.Address, address) {
			return t, true
		}
	}
	return config.Token{}, false
}

// TakerTokenRate returns the taker/maker exchange rate:
//   1 takerToken = rate makerToken
func (s *PricerService) TakerTokenRate(takerTokenAddr, makerTokenAddr string) (*big.Rat, error) {
	taker, ok := s.FindToken(takerTokenAddr)
	if !ok {
		return nil, errors.New("unknown taker token")
	}
	maker, ok := s.FindToken(makerTokenAddr)
	if !ok {
		return nil, errors.New("unknown maker token")
	}

	takerUSD, ok := s.getUSDPrice(strings.ToUpper(taker.Symbol))
	if !ok || takerUSD.Sign() == 0 {
		return nil, errors.New("missing USD price for taker token")
	}
	makerUSD, ok := s.getUSDPrice(strings.ToUpper(maker.Symbol))
	if !ok || makerUSD.Sign() == 0 {
		return nil, errors.New("missing USD price for maker token")
	}

	// rate = takerUSD / makerUSD
	rate := new(big.Rat).Quo(takerUSD, makerUSD)

	// Apply markup in bps (increase rate slightly to be conservative for maker).
	if s.cfg.Pricing.PriceMarkupBps != 0 {
		markup := big.NewRat(10000+s.cfg.Pricing.PriceMarkupBps, 10000)
		rate.Mul(rate, markup)
	}

	return rate, nil
}

func (s *PricerService) getUSDPrice(symbol string) (*big.Rat, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.usdPriceBySymbol[symbol]
	if !ok {
		return nil, false
	}
	return new(big.Rat).Set(p), true
}

