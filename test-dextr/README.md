# 1inch RFQ Market Maker Server

A sophisticated market maker server implementing dynamic pricing algorithms for decentralized exchange operations. This server provides RFQ (Request for Quote) functionality with intelligent inventory management and automated rebalancing.

## Overview

This market maker server supports trading pairs including BTC, ETH, USDC, and USDT with:
- Dynamic price level generation
- Inventory-aware pricing
- Multi-vault liquidity management
- Automatic rebalancing
- Fee collection and tracking

## Algorithm Details

### Core Architecture

The system consists of four main components:

1. **Vault Manager** (`vault_manager.go`) - Manages liquidity across multiple vaults
2. **Price Feed Engine** (`price_feed_engine.go`) - Generates dynamic price levels
3. **Oracle Client** (`oracle_client.go`) - Provides base price data
4. **HTTP Handlers** (`handlers.go`) - Exposes REST API endpoints

### Pricing Algorithm

#### 1. Base Price Discovery
- Oracle client maintains base prices for major pairs (BTC/USDC, ETH/USDC, etc.)
- Prices fluctuate ±2% around base values to simulate market volatility
- Base prices are updated every 3 seconds

#### 2. Dynamic Price Level Generation

The price feed engine calculates adjusted prices using multiple factors:

```
Adjusted Price = Oracle Price × (1 + Inventory Bias + Size Impact + Competitive Spread)
```

**Inventory Bias Calculation:**
- Monitors vault balance ratios against target balances
- Applies penalties when inventory is low (discourages trades that further deplete inventory)
- Provides bonuses when inventory is high (encourages trades that reduce excess inventory)
- Critical thresholds: 10% (critical low), 30% (low), 170% (high), 190% (critical high)

**Size Impact Calculation:**
- Small trades (< 1% of vault): 0.05% impact
- Medium trades (1-5% of vault): 0.2% impact  
- Large trades (5-15% of vault): 0.8% impact
- Very large trades (> 15% of vault): 2% impact

**Competitive Spread:**
- Base spread of 0.1% applied to all trades
- Additional gas cost buffer of 0.02%

#### 3. Bidirectional Price Level Generation

The system generates price levels for both canonical pairs (e.g., BTC/USDC) and reverse pairs (e.g., USDC/BTC):

- **Canonical Direction**: Direct calculation using base token as input
- **Reverse Direction**: Price inversion with proper size adjustments
- Supports 5 size levels per direction (1%, 5%, 10%, 20%, 30% of available liquidity)

### Vault Management System

#### Three-Vault Architecture

1. **Main Vaults**: Primary liquidity pools per token with target balances
2. **Balancer Vault**: Multi-token buffer (30% of each main vault initially)
3. **Fee Collection Vault**: Tracks all collected trading fees

#### Liquidity Management

**Swap Execution Process:**
1. Collect trading fee (in input token)
2. Add net input amount to balancer vault
3. Provide output tokens (balancer vault first, then main vault if needed)
4. Update all balances atomically

**Rebalancing Algorithm:**

The system performs automatic rebalancing every 2 minutes with two phases:

**Phase 1: Deficit Filling**
- Identify main vaults below target balance
- Transfer tokens from balancer vault to fill deficits
- Prioritizes maintaining main vault target balances

**Phase 2: Excess Management**
- Calculate deviation percentages from target balances
- Move 50% of excess tokens to balancer vault (if > 0.5% deviation)
- Fill 50% of deficits from balancer vault (if available)

### Risk Management

#### Inventory Protection
- Real-time monitoring of vault balance ratios
- Dynamic price adjustments to discourage imbalanced trades
- Emergency thresholds with high penalty factors

#### Position Limits
- Maximum trade size: 30% of vault balance
- Minimum vault thresholds prevent complete depletion
- Balancer vault acts as overflow/emergency liquidity

## API Endpoints

### GET `/levels`
Returns dynamic price levels for all supported pairs.

**Response:**
```json
{
  "BTC_USDC": [
    {"0": "0.10000000", "1": "97575.12000000"},
    {"0": "0.50000000", "1": "97580.24000000"}
  ],
  "USDC_BTC": [
    {"0": "9750.00000000", "1": "0.00001024"},
    {"0": "48750.00000000", "1": "0.00001024"}
  ]
}
```

### POST `/order`
Creates a signed order using current price levels.

**Request:**
```json
{
  "baseToken": "BTC",
  "quoteToken": "USDC", 
  "amount": "0.1",
  "taker": "0x...",
  "feeBps": 30
}
```

### GET `/status`
Returns current vault balances and health metrics.

## CLI Commands

### Build and Run

```bash
# Build the application
go build -o market-maker

# Run the server
./market-maker
```

The server starts on port 8080 and logs all endpoints and features.

### Testing Price Levels

```bash
# Get current price levels
curl -X GET http://localhost:8080/levels

# Get vault status
curl -X GET http://localhost:8080/status
```

### Create Signed Orders

```bash
# Create order for BTC/USDC pair
curl -X POST http://localhost:8080/order \
  -H "Content-Type: application/json" \
  -d '{
    "baseToken": "BTC",
    "quoteToken": "USDC",
    "amount": "0.05",
    "taker": "0x742d35Cc6634C0532925a3b8D93B5D6E25DB0a2E",
    "feeBps": 30
  }'
```

### Monitor Real-time Updates

The server provides continuous logging of:
- Price updates (every 3 seconds)
- Rebalancing operations (every 2 minutes)  
- Trade executions and fee collection
- Vault balance changes

```bash
# Watch logs in real-time
./market-maker 2>&1 | grep -E "(Price|Rebalance|Swap|Fee)"
```

## Configuration

The system uses default parameters defined in `types.go`:

- **Inventory Thresholds**: Critical low (10%), Low (30%), High (170%), Critical high (190%)
- **Size Impact Brackets**: Small (1%), Medium (5%), Large (15%)
- **Rebalance Frequency**: Every 2 minutes
- **Price Update Frequency**: Every 3 seconds
- **Base Spread**: 0.1%

## Supported Trading Pairs

- BTC/USDC, BTC/USDT, BTC/ETH
- ETH/USDC, ETH/USDT  
- USDC/USDT
- All reverse pairs (e.g., USDC/BTC, USDT/BTC, etc.)

## Logging and Monitoring

The server provides comprehensive logging for:
- Trade executions with exchange rates
- Vault balance updates
- Rebalancing operations
- Fee collections
- Price level generations
- Error conditions and warnings