# Local Testing Guide

## Setup

### 1. Build and Run Locally

```bash
cd price-feeder

# Option A: Run with Go directly
go run main.go

# Option B: Build and run
go build -o price-feeder
./price-feeder
```

### 2. Or Use Docker Compose

```bash
# Build local image first
docker build -t 0xdhruv/pricefeeder:local .

# Update docker-compose.yml to use local image
# Then run:
docker-compose up
```

## Quick Test Commands

### Basic Order Test (3 USDT)

```bash
curl -X POST http://localhost:8080/order \
  -H "Content-Type: application/json" \
  -d '{
    "baseToken": "0xdac17f958d2ee523a2206206994597c13d831ec7",
    "quoteToken": "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
    "amount": "3000000",
    "taker": "0x1234567890123456789012345678901234567890",
    "feeBps": 30
  }'
```

### Understanding the Amount Field

The `amount` field should be in **WEI** (smallest units):

- **USDT (6 decimals)**: 
  - 1 USDT = 1,000,000 WEI
  - 3 USDT = 3,000,000 WEI
  
- **WETH (18 decimals)**:
  - 1 WETH = 1,000,000,000,000,000,000 WEI
  - 0.1 WETH = 100,000,000,000,000,000 WEI

### Token Addresses (Ethereum Mainnet)

```
USDT:   0xdAC17F958D2ee523a2206206994597C13D831ec7
USDC:   0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48
WETH:   0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2
DAI:    0x6B175474E89094C44Da98b954EedeAC495271d0F
WSTETH: 0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0
WBTC:   0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599
```

**Note**: Address case doesn't matter (case-insensitive)

## Run All Tests

```bash
chmod +x test_order_local.sh
./test_order_local.sh
```

## Check Other Endpoints

### Get Statistics
```bash
curl http://localhost:8080/stats | jq '.'
```

### Get Recent Hits
```bash
curl http://localhost:8080/recent-hits?limit=10 | jq '.'
```

### Health Check
```bash
curl http://localhost:8080/health
```

### Prometheus Metrics
```bash
curl http://localhost:9091/metrics
```

## Expected Responses

### Success Response
```json
{
  "order": {
    "maker": "0x4B8fF16B1d97a957A7C649F5B3517199cc8fa358",
    "makerAsset": "0xdac17f958d2ee523a2206206994597c13d831ec7",
    "takerAsset": "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
    "makerTraits": "...",
    "salt": "...",
    "makingAmount": "3000000",
    "takingAmount": "...",
    "receiver": "0x0000000000000000000000000000000000000000"
  },
  "signature": "0x..."
}
```

### Error Responses

#### Insufficient Balance
```json
{
  "error": "Unprocessable Entity",
  "message": "insufficient balance",
  "code": 422
}
```

#### Validation Error (Same Token)
```json
{
  "error": "Bad Request",
  "message": "baseToken and quoteToken cannot be the same",
  "code": 400
}
```

#### Price Unavailable
```json
{
  "error": "Service Unavailable",
  "message": "price unavailable for token pair",
  "code": 503
}
```

## Debugging Tips

1. **Check Logs**: The service logs all requests with timestamps
2. **Check Balance Service**: Make sure the RPC endpoint in `config.json` is working
3. **Check Metrics**: Visit http://localhost:9091/metrics to see balance metrics
4. **Use jq**: Pipe curl output to `jq '.'` for pretty JSON

## Re-enable Auth for Production

Before deploying to production, uncomment the auth middleware in `main.go`:

```go
// Change this:
ordersHandler.HandleOrder(w, r) // Auth disabled for local testing

// Back to this:
authMiddleware.Authenticate(http.HandlerFunc(ordersHandler.HandleOrder)).ServeHTTP(w, r)
```

## Common Issues

### Issue: "balance service unavailable"
- **Cause**: RPC endpoint not responding or balance fetch failed
- **Solution**: Check `eth_rpc_url` in `config.json`

### Issue: "price unavailable"
- **Cause**: Oracle API not responding
- **Solution**: Check `chaos_labs_api_key` in `config.json`

### Issue: Connection refused
- **Cause**: Service not running
- **Solution**: Make sure service is running on port 8080

