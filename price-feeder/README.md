# 1inch RFQ Tester

A Go application for testing 1inch v6 Embedded RFQ API capacity with comprehensive monitoring via Prometheus and Grafana.

## Features

- **Mock Price Level Generation**: Realistic bid/ask spreads for configured token pairs
- **Trade Hit Counting**: Detailed statistics tracking with real-time monitoring
- **Prometheus Metrics**: Comprehensive metrics export for monitoring
- **Grafana Dashboard**: Pre-configured dashboard with time-series analysis
- **RESTful API**: Clean HTTP endpoints for integration

## Quick Setup

### 1. Start Everything
```bash
# Start RFQ Tester with metrics
go run main.go

# In another terminal, start monitoring stack
docker-compose up -d
```

### 2. Access Monitoring
- **RFQ Tester**: http://localhost:8080
- **Grafana Dashboard**: http://localhost:3000 (admin/admin123)
- **Prometheus**: http://localhost:9090

## API Endpoints

**⚠️ Authentication Required**: All endpoints now require 1inch HMAC-SHA256 authentication headers.

## Easy Testing (For Non-Technical Users)
**Simple commands anyone can run:**
```bash
# Make script executable (run once)
chmod +x easy-test.sh

# Get price levels
./easy-test.sh levels

# Send a test order
./easy-test.sh order

# Show help
./easy-test.sh help
```

### `/levels` (GET) - Get Price Levels
**With Authentication (Advanced):**
```bash
# Test authenticated endpoint (replace with your server IP)
TIMESTAMP=$(python3 -c "import time; print(int(time.time() * 1000))") && SIGNATURE=$(echo -n "${TIMESTAMP}GET/levels" | openssl dgst -sha256 -hmac "sk_1inch_7f3e9a8b2c5d4e6f9a1b3c7e8f2a5b9c4d7e0f3a6b9c2e5f8a1b4d7e0f3a6b9c2" -hex | cut -d' ' -f2) && curl -i -H "INCH-ACCESS-KEY: 1inch_access_key_8a9f2e5c7b4d6891" -H "INCH-ACCESS-TIMESTAMP: $TIMESTAMP" -H "INCH-ACCESS-SIGN: $SIGNATURE" -H "INCH-ACCESS-PASSPHRASE: secure_1inch_passphrase_2024_x9k7m3n8" http://34.10.232.3:8080/levels
```

### `/order` (POST) - Submit Order
**With Authentication:**
```bash
# Generate signature for POST request
TIMESTAMP=$(python3 -c "import time; print(int(time.time() * 1000))")
BODY_HASH=$(echo -n "amount=1000000000000000000&baseToken=0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0&feeBps=10&quoteToken=0x8236a87084f8B84306f72007F36F2618A5634494&taker=0x1234567890123456789012345678901234567890" | tr -d '\n')
SIGNATURE=$(echo -n "${TIMESTAMP}POST/order${BODY_HASH}" | openssl dgst -sha256 -hmac "sk_1inch_7f3e9a8b2c5d4e6f9a1b3c7e8f2a5b9c4d7e0f3a6b9c2e5f8a1b4d7e0f3a6b9c2" -hex | cut -d' ' -f2)

curl -X POST http://34.10.232.3:8080/order \
  -H "Content-Type: application/json" \
  -H "INCH-ACCESS-KEY: 1inch_access_key_8a9f2e5c7b4d6891" \
  -H "INCH-ACCESS-TIMESTAMP: 1758713279237" \
  -H "INCH-ACCESS-SIGN: 357e2540fbe87cfb1cd62f8b3eda6423fdae964cd652415afb4e529d68ed7a13" \
  -H "INCH-ACCESS-PASSPHRASE: secure_1inch_passphrase_2024_x9k7m3n8" \
  -d '{
    "baseToken": "0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0",
    "quoteToken": "0x8236a87084f8B84306f72007F36F2618A5634494",
    "amount": "1000000000000000000",
    "taker": "0x1234567890123456789012345678901234567890",
    "feeBps": 10
  }'
```

### `/stats` (GET) - Get Statistics
### `/health` (GET) - Health Check
### `/reset` (POST) - Reset Counters

## Configuration

### Authentication Setup
Set environment variables for 1inch authentication:
```bash
export ONEINCH_ACCESS_KEY="1inch_access_key_8a9f2e5c7b4d6891"
export ONEINCH_SECRET_KEY="sk_1inch_7f3e9a8b2c5d4e6f9a1b3c7e8f2a5b9c4d7e0f3a6b9c2e5f8a1b4d7e0f3a6b9c2"
export ONEINCH_PASSPHRASE="secure_1inch_passphrase_2024_x9k7m3n8"
export PORT=8080
export HOST=0.0.0.0
```

### Essential config in `config.json`:
```json
{
  "server": {"port": 8080, "host": "0.0.0.0"},
  "metrics": {
    "enable_prometheus": true,
    "metrics_port": 9091
  },
  "oneinch_auth": {
    "access_key": "",
    "secret_key": "",
    "passphrase": ""
  },
  "token_pairs": [...]
}
```

## Dashboard Metrics

The Grafana dashboard displays:

- **Endpoint Call Rates**: Requests/minute for `/levels` and `/order`
- **Token Prices**: Real-time USD prices (WSTETH, LBTC, METH)
- **Exchange Rates**: Cross-token rates with markup
- **HTTP Performance**: Response times, success rates
- **System Health**: Uptime, hit counters

### Key PromQL Queries
```promql
# Requests per minute by endpoint
rate(rfq_http_requests_total[5m]) * 60

# Token prices
rfq_token_price_usd

# Success rate
(rfq_successful_hits / rfq_total_hits) * 100
```

## Load Testing

```bash
# Test /levels endpoint
hey -n 1000 -c 50 "http://localhost:8080/levels?tokenIn=0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0&tokenOut=0x8236a87084f8B84306f72007F36F2618A5634494&amount=1000000000000000000"

# Test /order endpoint
hey -n 1000 -c 50 -m POST -H "Content-Type: application/json" -d '{"baseToken":"0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0","quoteToken":"0x8236a87084f8B84306f72007F36F2618A5634494","amount":"1000000000000000000","taker":"0x1234567890123456789012345678901234567890","feeBps":10}' http://localhost:8080/order
```

## Troubleshooting

### Port Conflicts
If Prometheus fails to start, check port conflicts:
```bash
# Check what's using port 9090
lsof -i :9090

# Update config.json to use different port
"metrics_port": 9091
```

### Verify Setup
```bash
# Check RFQ metrics
curl http://localhost:8080/metrics | grep rfq_

# Check Prometheus targets
curl http://localhost:9090/api/v1/targets

# Check Grafana dashboards
curl -u admin:admin123 http://localhost:3000/api/search
```