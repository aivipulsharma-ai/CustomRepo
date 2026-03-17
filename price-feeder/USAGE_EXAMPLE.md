# Usage Example: Balance-Based Quoting

## Quick Example

### Setup
```bash
cd price-feeder

# Configure RPC endpoint
export ETH_RPC_URL="https://eth.llamarpc.com"
export MAKER_ADDRESS="0x6eDC317F3208B10c46F4fF97fAa04dD632487408"

# Run
go run main.go
```

### Expected Output (Startup)
```
[1INCH-RFQ] Starting 1inch RFQ Tester server on 0.0.0.0:8080
Updated price for WSTETH (0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0): $5020.00
Updated price for USDT (0xdAC17F958D2ee523a2206206994597C13D831ec7): $1.00
Updated balance for WSTETH (0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0): 1500000000000000000
Updated balance for USDT (0xdAC17F958D2ee523a2206206994597C13D831ec7): 10000000000
Using actual balance for WSTETH: 0.750000 (50% of 1.500000)
Using actual balance for USDT: 5000.000000 (50% of 10000.000000)
```

**Translation:**
- Maker has 1.5 WSTETH → Quote up to 0.75 WSTETH
- Maker has 10,000 USDT → Quote up to 5,000 USDT

## Example 1: /levels Endpoint

### Request
```bash
curl http://localhost:8080/levels | jq '.'
```

### Response
```json
{
  "0x7f39c581f595b53c5cb19bd0b3f8da6c935e2ca0_0xdac17f958d2ee523a2206206994597c13d831ec7": [
    ["0.750000", "5020.000000"]
  ],
  "0xdac17f958d2ee523a2206206994597c13d831ec7_0x7f39c581f595b53c5cb19bd0b3f8da6c935e2ca0": [
    ["5000.000000", "0.000199"]
  ]
}
```

**Interpretation:**
- WSTETH→USDT: Can sell up to **0.75 WSTETH** at ~5020 USDT each
- USDT→WSTETH: Can sell up to **5000 USDT** at ~0.000199 WSTETH each

## Example 2: /order with Sufficient Balance

### Request
```bash
curl -X POST http://localhost:8080/order \
  -H "Content-Type: application/json" \
  -d '{
    "baseToken": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
    "quoteToken": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
    "amount": "100",
    "taker": "0x1234567890123456789012345678901234567890",
    "feeBps": 10
  }'
```

### Server Logs
```
Balance check passed: requested=100.000000, available=10000.000000 USDT
Order accepted for USDT/WETH, amount: 100, price: 0.000320, volumeUSD: $100.00
```

### Response
```json
{
  "order": {
    "maker": "0x6eDC317F3208B10c46F4fF97fAa04dD632487408",
    "makerAsset": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
    "takerAsset": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
    "makerTraits": "186375555395584000000000",
    "salt": "9876543210987",
    "makingAmount": "100000000",
    "takingAmount": "32000000000000000",
    "receiver": "0x0000000000000000000000000000000000000000"
  },
  "signature": "0x..."
}
```

**Result:** ✅ Order created successfully (maker has enough USDT)

## Example 3: /order with Insufficient Balance

### Request
```bash
curl -X POST http://localhost:8080/order \
  -H "Content-Type: application/json" \
  -d '{
    "baseToken": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
    "quoteToken": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
    "amount": "50000",
    "taker": "0x1234567890123456789012345678901234567890",
    "feeBps": 10
  }'
```

### Server Logs
```
Insufficient balance: requested=50000.000000, available=10000.000000 USDT
Order processing failed
```

### Response
```json
{
  "success": false,
  "error": "Order processing failed",
  "message": "Price validation failed or order expired"
}
```

**Result:** ❌ Order rejected (maker doesn't have 50,000 USDT)

## Example 4: Balance Updates

### After 30 Seconds
```
Updated balance for WSTETH (0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0): 1450000000000000000
Updated balance for USDT (0xdAC17F958D2ee523a2206206994597C13D831ec7): 9900000000
Using actual balance for WSTETH: 0.725000 (50% of 1.450000)
Using actual balance for USDT: 4950.000000 (50% of 9900.000000)
```

**Balance decreased:**
- WSTETH: 1.5 → 1.45 (used 0.05 in a trade)
- USDT: 10,000 → 9,900 (used 100 in a trade)

**New quotes:**
- WSTETH: Quote up to 0.725 (instead of 0.75)
- USDT: Quote up to 4,950 (instead of 5,000)

## Example 5: No Balance Available

### Without RPC URL or No Balance
```bash
# Don't set ETH_RPC_URL
unset ETH_RPC_URL
go run main.go
```

### Output
```
Warning: Failed to initialize balance service: failed to connect to Ethereum RPC
Tokens without balance will not be quoted
Updated price for WSTETH: $5020.00
Updated price for USDT: $1.00
Balance service not available, skipping WSTETH/USDT pair
Balance service not available, skipping USDT/WSTETH pair
```

### /levels Response
```json
{}
```

**Result:** ❌ No price levels quoted (RPC not available, cannot determine balance)

## Configuration Examples

### Minimal (Free RPC)
```json
{
  "maker_address": "0x6eDC317F3208B10c46F4fF97fAa04dD632487408",
  "eth_rpc_url": "https://eth.llamarpc.com"
}
```

### With Infura
```json
{
  "maker_address": "0x6eDC317F3208B10c46F4fF97fAa04dD632487408",
  "eth_rpc_url": "https://mainnet.infura.io/v3/YOUR_INFURA_KEY"
}
```

### With Environment Variable
```bash
export ETH_RPC_URL="https://eth-mainnet.alchemyapi.io/v2/YOUR_ALCHEMY_KEY"
export MAKER_ADDRESS="0x6eDC317F3208B10c46F4fF97fAa04dD632487408"
```

## Monitoring Balances

### Check Current Balances
```bash
# Watch balance updates
tail -f logs/app.log | grep "Updated balance"

# Output every 30 seconds:
Updated balance for USDT (0xdAC..): 10000000000
Updated balance for WETH (0xC02..): 5000000000000000000
```

### Check via Stats API
```bash
curl http://localhost:8080/stats | jq '.tokenPairStats'
```

## Real-World Scenario

### Initial State
```
Maker wallet contains:
- 100 USDT
- 0.05 WETH
- 0.1 WSTETH
```

### System Behavior

**1. Startup**
```
Updated balance for USDT: 100000000
Updated balance for WETH: 50000000000000000
Updated balance for WSTETH: 100000000000000000
```

**2. Available in /levels**
```
USDT: 50 (50% of 100)
WETH: 0.025 (50% of 0.05)
WSTETH: 0.05 (50% of 0.1)
```

**3. Order Request for 30 USDT**
```
Balance check: 30 < 100 ✅
Order created successfully
```

**4. Order Request for 150 USDT**
```
Balance check: 150 > 100 ❌
Order rejected: Insufficient balance
```

**5. After 30 seconds (if balance changed)**
```
Updated balance for USDT: 70000000  (after using 30)
New available: 35 USDT (50% of 70)
```

## Summary

**Key Points:**

1. **Dynamic Quotes**: Based on real on-chain balances
2. **Automatic Updates**: Balances refresh every 30 seconds
3. **Balance Validation**: Orders rejected if insufficient funds
4. **50% Rule**: Only quote 50% of balance for safety
5. **Balance Required**: No quotes if balance service unavailable or balance is zero

**Benefits:**
- ✅ Accurate, real-time liquidity
- ✅ Prevents over-commitment
- ✅ Automatic balance tracking
- ✅ No manual updates needed
- ✅ Only quotes when actual funds are available

**Next Steps:**
1. Configure your RPC endpoint
2. Set your maker address
3. Run the service
4. Watch balances update automatically!

