#!/bin/bash

# Local Order Testing Script
# Tests the order flow with various scenarios

BASE_URL="http://localhost:8080"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "======================================================================"
echo "           Testing Order Flow (Auth Disabled)"
echo "======================================================================"
echo ""

# Test 1: Valid USDT -> WETH order (lowercase addresses)
echo -e "${YELLOW}Test 1: Valid USDT -> WETH order (3 USDT)${NC}"
echo "Request: 3,000,000 WEI (3 USDT with 6 decimals)"
echo ""

curl -X POST "$BASE_URL/order" \
  -H "Content-Type: application/json" \
  -d '{
    "baseToken": "0xdac17f958d2ee523a2206206994597c13d831ec7",
    "quoteToken": "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
    "amount": "3000000",
    "taker": "0x1234567890123456789012345678901234567890",
    "feeBps": 30
  }' | jq '.'

echo ""
echo "----------------------------------------------------------------------"
echo ""

# Test 2: Valid WETH -> USDT order (large amount)
echo -e "${YELLOW}Test 2: Valid WETH -> USDT order (0.1 WETH)${NC}"
echo "Request: 100,000,000,000,000,000 WEI (0.1 WETH with 18 decimals)"
echo ""

curl -X POST "$BASE_URL/order" \
  -H "Content-Type: application/json" \
  -d '{
    "baseToken": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
    "quoteToken": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
    "amount": "100000000000000000",
    "taker": "0x1234567890123456789012345678901234567890",
    "feeBps": 30
  }' | jq '.'

echo ""
echo "----------------------------------------------------------------------"
echo ""

# Test 3: Insufficient balance (should fail)
echo -e "${YELLOW}Test 3: Insufficient balance test (1000 WETH - should fail)${NC}"
echo "Request: 1000 WETH (more than available)"
echo ""

curl -X POST "$BASE_URL/order" \
  -H "Content-Type: application/json" \
  -d '{
    "baseToken": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
    "quoteToken": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
    "amount": "1000000000000000000000",
    "taker": "0x1234567890123456789012345678901234567890",
    "feeBps": 30
  }' | jq '.'

echo ""
echo "----------------------------------------------------------------------"
echo ""

# Test 4: Invalid - same token (should fail validation)
echo -e "${YELLOW}Test 4: Same token validation (should fail)${NC}"
echo "Request: USDT -> USDT (different case)"
echo ""

curl -X POST "$BASE_URL/order" \
  -H "Content-Type: application/json" \
  -d '{
    "baseToken": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
    "quoteToken": "0xdac17f958d2ee523a2206206994597c13d831ec7",
    "amount": "1000000",
    "taker": "0x1234567890123456789012345678901234567890",
    "feeBps": 30
  }' | jq '.'

echo ""
echo "----------------------------------------------------------------------"
echo ""

# Test 5: Check stats
echo -e "${YELLOW}Test 5: Get statistics${NC}"
curl -X GET "$BASE_URL/stats" | jq '.'

echo ""
echo "======================================================================"
echo "                      Tests Complete"
echo "======================================================================"

