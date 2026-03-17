package config

import (
	"context"
	"encoding/json"
	"os"
	"strconv"

	"github.com/0xcatalysis/core/go-sdk/errors"
	"github.com/0xcatalysis/core/go-sdk/log"
)

type Token struct {
	BaseToken  string `json:"base_token"`
	BaseSymbol string `json:"base_symbol"`
}

type Config struct {
	Server struct {
		Port int    `json:"port"`
		Host string `json:"host"`
	} `json:"server"`

	Tokens []Token `json:"tokens"`

	Pricing struct {
		UpdateIntervalSeconds int     `json:"update_interval_seconds"`
		VolatilityFactor      float64 `json:"volatility_factor"`
		DefaultSpread         float64 `json:"default_spread"`
		PriceMarkup           float64 `json:"price_markup"`
		ChaosLabsAPIKey       string  `json:"chaos_labs_api_key"`
	} `json:"pricing"`

	Metrics struct {
		MetricsPort int    `json:"metrics_port"`
		LogLevel    string `json:"log_level"`
	} `json:"metrics"`

	MakerAddress    string `json:"maker_address"`
	MakerPrivateKey string `json:"maker_private_key"` // Private key for signing orders (without 0x prefix)
	EthRpcUrl       string `json:"eth_rpc_url"`       // Ethereum RPC URL for fetching balances
	LogLevel        string `json:"log_level"`

	OneInchAuth struct {
		AccessKey  string `json:"access_key"`
		SecretKey  string `json:"secret_key"`
		Passphrase string `json:"passphrase"`
	} `json:"oneinch_auth"`
}

func LoadConfig() (*Config, error) {
	config := &Config{}

	configFile := getEnv("CONFIG_FILE", "config.json")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		config = getDefaultConfig()
	} else {
		file, err := os.Open(configFile)
		if err != nil {
			return nil, errors.Wrap(err, "error opening config file")
		}
		defer func() {
			if err := file.Close(); err != nil {
				log.Warn(context.Background(), "Failed to close config file", err)
			}
		}()

		decoder := json.NewDecoder(file)
		if err := decoder.Decode(config); err != nil {
			return nil, errors.Wrap(err, "error decoding config")
		}
	}

	overrideWithEnv(config)

	return config, nil
}

func getDefaultConfig() *Config {
	return &Config{
		Server: struct {
			Port int    `json:"port"`
			Host string `json:"host"`
		}{
			Port: 8080,
			Host: "0.0.0.0",
		},
		Tokens: []Token{
			{
				BaseToken:  "0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0",
				BaseSymbol: "WSTETH",
			},
			{
				BaseToken:  "0x8236a87084f8B84306f72007F36F2618A5634494",
				BaseSymbol: "LBTC",
			},
			{
				BaseToken:  "0xd5F7838F5C461fefF7FE49ea5ebaF7728bB0ADfa",
				BaseSymbol: "METH",
			},
		},
		Pricing: struct {
			UpdateIntervalSeconds int     `json:"update_interval_seconds"`
			VolatilityFactor      float64 `json:"volatility_factor"`
			DefaultSpread         float64 `json:"default_spread"`
			PriceMarkup           float64 `json:"price_markup"`
			ChaosLabsAPIKey       string  `json:"chaos_labs_api_key"`
		}{
			UpdateIntervalSeconds: 5,
			VolatilityFactor:      0.01,
			DefaultSpread:         0.002,
			PriceMarkup:           0.3,
			ChaosLabsAPIKey:       "6CiESFtsHvrIdswVZhTV9GVftI3JQh6atciFyHNXVIA=",
		},
		Metrics: struct {
			MetricsPort int    `json:"metrics_port"`
			LogLevel    string `json:"log_level"`
		}{
			MetricsPort: 9090,
			LogLevel:    "info",
		},
		MakerAddress:    "0x6eDC317F3208B10c46F4fF97fAa04dD632487408",
		MakerPrivateKey: "", // Must be set via env var MAKER_PRIVATE_KEY
		EthRpcUrl:       "https://eth.llamarpc.com",
		OneInchAuth: struct {
			AccessKey  string `json:"access_key"`
			SecretKey  string `json:"secret_key"`
			Passphrase string `json:"passphrase"`
		}{
			AccessKey:  "",
			SecretKey:  "",
			Passphrase: "",
		},
		LogLevel: "info",
	}
}

func overrideWithEnv(config *Config) {
	if port := getEnv("PORT", ""); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}

	if host := getEnv("HOST", ""); host != "" {
		config.Server.Host = host
	}

	if logLevel := getEnv("LOG_LEVEL", ""); logLevel != "" {
		config.Metrics.LogLevel = logLevel
	}

	if accessKey := getEnv("ONEINCH_ACCESS_KEY", ""); accessKey != "" {
		config.OneInchAuth.AccessKey = accessKey
	}

	if secretKey := getEnv("ONEINCH_SECRET_KEY", ""); secretKey != "" {
		config.OneInchAuth.SecretKey = secretKey
	}

	if passphrase := getEnv("ONEINCH_PASSPHRASE", ""); passphrase != "" {
		config.OneInchAuth.Passphrase = passphrase
	}

	if makerAddress := getEnv("MAKER_ADDRESS", ""); makerAddress != "" {
		config.MakerAddress = makerAddress
	}

	if makerPrivateKey := getEnv("MAKER_PRIVATE_KEY", ""); makerPrivateKey != "" {
		config.MakerPrivateKey = makerPrivateKey
	}

	if ethRpcUrl := getEnv("ETH_RPC_URL", ""); ethRpcUrl != "" {
		config.EthRpcUrl = ethRpcUrl
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
