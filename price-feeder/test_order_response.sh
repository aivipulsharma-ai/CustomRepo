#!/bin/bash

# Test script to demonstrate the new /order API response format
# This shows the 1inch RFQ order structure

echo "Testing /order API with new 1inch RFQ order format..."
echo ""

# Test order request
curl -X POST http://localhost:8080/order \
  -H "Content-Type: application/json" \
  -d '{
    "baseToken": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
    "quoteToken": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
    "amount": "100",
    "taker": "0x1234567890123456789012345678901234567890",
    "feeBps": 10
  }' | jq '.'

echo ""
echo "Response format includes:"
echo "  - order.maker: The maker address"
echo "  - order.makerAsset: The base token (what maker is providing)"
echo "  - order.takerAsset: The quote token (what maker wants to receive)"
echo "  - order.makerTraits: Encoded expiration and flags"
echo "  - order.salt: Random nonce"
echo "  - order.makingAmount: Amount in wei/smallest unit"
echo "  - order.takingAmount: Amount in wei/smallest unit"
echo "  - order.receiver: Zero address"
echo "  - signature: EIP-712 signature (currently mock)"

