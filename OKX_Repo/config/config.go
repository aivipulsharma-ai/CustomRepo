package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
)

type Token struct {
	Address string `json:"address"`
	Symbol  string `json:"symbol"`
	// Decimals is optional; used for display only in this reference implementation.
	Decimals int `json:"decimals"`
}

type ChainConfig struct {
	ChainIndex       string `json:"chainIndex"`        // OKX chain index (string in schema)
	ChainID          int64  `json:"chainId"`           // EVM chainId for signing
	Settlement       string `json:"settlement"`        // OKX PMM settlement contract (pmmProtocol/verifyingContract)
	Permit2          string `json:"permit2,omitempty"` // optional
	NativeToken      string `json:"nativeToken,omitempty"`
	NativeTokenSymbol string `json:"nativeTokenSymbol,omitempty"`
}

type PricingConfig struct {
	// PriceMarkupBps is applied to the computed takerTokenRate. 30 bps = 0.30%
	PriceMarkupBps int64 `json:"priceMarkupBps"`
	// MinQuoteUSD is informational only in this reference implementation.
	MinQuoteUSD float64 `json:"minQuoteUsd"`
}

type Config struct {
	Server struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"server"`

	LogLevel string `json:"logLevel"`

	Auth struct {
		XAPIKey string `json:"xApiKey"`
	} `json:"auth"`

	Maker struct {
		Address    string `json:"address"`
		PrivateKey string `json:"privateKey"`
	} `json:"maker"`

	Pricing PricingConfig `json:"pricing"`

	Chains []ChainConfig `json:"chains"`
	Tokens []Token       `json:"tokens"`
}

func LoadConfig() (*Config, error) {
	cfg := &Config{}

	configFile := getEnv("CONFIG_FILE", "config.json")
	if _, err := os.Stat(configFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg = defaultConfig()
		} else {
			return nil, err
		}
	} else {
		f, err := os.Open(configFile)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()

		dec := json.NewDecoder(f)
		if err := dec.Decode(cfg); err != nil {
			return nil, err
		}
	}

	overrideWithEnv(cfg)
	return cfg, nil
}

func defaultConfig() *Config {
	cfg := &Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8081
	cfg.LogLevel = "info"
	cfg.Pricing.PriceMarkupBps = 30
	cfg.Pricing.MinQuoteUSD = 200
	cfg.Auth.XAPIKey = ""
	cfg.Maker.Address = ""
	cfg.Maker.PrivateKey = ""
	cfg.Chains = []ChainConfig{
		// Addresses from OKX docs (Market Maker Integration page).
		{ChainIndex: "1", ChainID: 1, Settlement: "0x0Bdf246b4AEF9Cfe4DD6eEf153A1b645aC4BcBb6", Permit2: "0x000000000022D473030F116dDEE9F6B43aC78BA3"},
		{ChainIndex: "42161", ChainID: 42161, Settlement: "0x1ef032a3c471a99cc31578c8007f256d95e89896", Permit2: "0x000000000022D473030F116dDEE9F6B43aC78BA3"},
		{ChainIndex: "8453", ChainID: 8453, Settlement: "0xed97b4331fff9dc8c40936532a04ac1400f273a5", Permit2: "0x000000000022D473030F116dDEE9F6B43aC78BA3"},
		{ChainIndex: "56", ChainID: 56, Settlement: "0x9ff547bbb813a0e5d53742c7a5f7370dcea214a3", Permit2: "0x000000000022D473030F116dDEE9F6B43aC78BA3"},
	}
	cfg.Tokens = []Token{
		{Address: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", Symbol: "WETH", Decimals: 18},
		{Address: "0xdAC17F958D2ee523a2206206994597C13D831ec7", Symbol: "USDT", Decimals: 6},
		{Address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", Symbol: "USDC", Decimals: 6},
	}
	return cfg
}

func overrideWithEnv(cfg *Config) {
	if host := getEnv("HOST", ""); host != "" {
		cfg.Server.Host = host
	}
	if port := getEnv("PORT", ""); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Server.Port = p
		}
	}
	if lvl := getEnv("LOG_LEVEL", ""); lvl != "" {
		cfg.LogLevel = lvl
	}
	if xk := getEnv("X_API_KEY", ""); xk != "" {
		cfg.Auth.XAPIKey = xk
	}
	if makerAddr := getEnv("MAKER_ADDRESS", ""); makerAddr != "" {
		cfg.Maker.Address = makerAddr
	}
	if pk := getEnv("MAKER_PRIVATE_KEY", ""); pk != "" {
		cfg.Maker.PrivateKey = pk
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func (c *Config) FindChain(chainIndex string) (ChainConfig, bool) {
	for _, ch := range c.Chains {
		if ch.ChainIndex == chainIndex {
			return ch, true
		}
	}
	return ChainConfig{}, false
}

func (c *Config) Validate(ctx context.Context) error {
	_ = ctx
	if c.Auth.XAPIKey == "" {
		return errors.New("auth.xApiKey (or env X_API_KEY) is required")
	}
	if c.Maker.Address == "" {
		return errors.New("maker.address (or env MAKER_ADDRESS) is required")
	}
	if c.Maker.PrivateKey == "" {
		return errors.New("maker.privateKey (or env MAKER_PRIVATE_KEY) is required")
	}
	return nil
}

