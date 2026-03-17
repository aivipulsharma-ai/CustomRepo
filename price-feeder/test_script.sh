#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Server URL
SERVER_URL="http://localhost:8080"

echo -e "${BLUE}=====================================${NC}"
echo -e "${BLUE}    1inch RFQ Tester - Test Script    ${NC}"
echo -e "${BLUE}=====================================${NC}"
echo ""

# Function to check if server is running
check_server() {
    echo -e "${YELLOW}Checking if server is running...${NC}"
    if curl -s "${SERVER_URL}/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Server is running${NC}"
        return 0
    else
        echo -e "${RED}✗ Server is not running. Please start it with: go run main.go${NC}"
        exit 1
    fi
}

# Function to test /levels endpoint
test_levels() {
    echo -e "\n${BLUE}1. Testing /levels endpoint${NC}"
    echo -e "${YELLOW}Fetching all token pair levels...${NC}"
    
    LEVELS_RESPONSE=$(curl -s "${SERVER_URL}/levels")
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ /levels endpoint successful${NC}"
        
        # Count number of pairs
        PAIR_COUNT=$(echo "$LEVELS_RESPONSE" | jq 'length')
        echo -e "${BLUE}Number of pairs: ${PAIR_COUNT}${NC}"
        
        echo -e "\n${YELLOW}Token Pairs and their levels:${NC}"
        echo "$LEVELS_RESPONSE" | jq -r 'to_entries[] | "\(.key): \(.value[0][0]) tokens at rate \(.value[0][1])"'
        
        # Save for later use
        echo "$LEVELS_RESPONSE" > /tmp/levels_response.json
        echo -e "${GREEN}✓ Levels data saved for order testing${NC}"
    else
        echo -e "${RED}✗ /levels endpoint failed${NC}"
        return 1
    fi
}

# Function to test /stats endpoint (initial state)
test_initial_stats() {
    echo -e "\n${BLUE}2. Testing /stats endpoint (initial state)${NC}"
    
    STATS_RESPONSE=$(curl -s "${SERVER_URL}/stats")
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ /stats endpoint successful${NC}"
        
        TOTAL_HITS=$(echo "$STATS_RESPONSE" | jq '.totalHits')
        SUCCESSFUL_HITS=$(echo "$STATS_RESPONSE" | jq '.successfulHits')
        FAILED_HITS=$(echo "$STATS_RESPONSE" | jq '.failedHits')
        
        echo -e "${BLUE}Initial Stats:${NC}"
        echo -e "  Total Hits: ${TOTAL_HITS}"
        echo -e "  Successful Hits: ${SUCCESSFUL_HITS}"
        echo -e "  Failed Hits: ${FAILED_HITS}"
        
        # Save initial stats
        echo "$STATS_RESPONSE" > /tmp/initial_stats.json
    else
        echo -e "${RED}✗ /stats endpoint failed${NC}"
        return 1
    fi
}

# Function to create test orders
test_orders() {
    echo -e "\n${BLUE}3. Testing /order endpoint${NC}"
    
    if [ ! -f /tmp/levels_response.json ]; then
        echo -e "${RED}✗ No levels data available. Run levels test first.${NC}"
        return 1
    fi
    
    # Get first two pairs for testing
    PAIR1=$(cat /tmp/levels_response.json | jq -r 'keys[0]')
    PAIR2=$(cat /tmp/levels_response.json | jq -r 'keys[1]')
    
    # Extract token addresses from pair keys
    TOKEN_IN_1=$(echo "$PAIR1" | cut -d'_' -f1)
    TOKEN_OUT_1=$(echo "$PAIR1" | cut -d'_' -f2)
    PRICE_1=$(cat /tmp/levels_response.json | jq -r ".\"$PAIR1\"[0][1]")
    
    TOKEN_IN_2=$(echo "$PAIR2" | cut -d'_' -f1)
    TOKEN_OUT_2=$(echo "$PAIR2" | cut -d'_' -f2)
    PRICE_2=$(cat /tmp/levels_response.json | jq -r ".\"$PAIR2\"[0][1]")
    
    echo -e "${YELLOW}Creating test orders...${NC}"
    
    # Order 1 - Should succeed
    DEADLINE_1=$(python3 -c "import time; print(int(time.time() + 3600))")
    ORDER_1='{
        "tokenIn": "'$TOKEN_IN_1'",
        "tokenOut": "'$TOKEN_OUT_1'",
        "amount": "1000000000000000000",
        "price": "'$PRICE_1'",
        "trader": "0x1234567890123456789012345678901234567890",
        "receiver": "0x0987654321098765432109876543210987654321",
        "nonce": "12345001",
        "deadline": '$DEADLINE_1'
    }'
    
    echo -e "${BLUE}Order 1: ${TOKEN_IN_1} -> ${TOKEN_OUT_1} at price ${PRICE_1}${NC}"
    ORDER_RESPONSE_1=$(curl -s -X POST "${SERVER_URL}/order" -H "Content-Type: application/json" -d "$ORDER_1")
    SUCCESS_1=$(echo "$ORDER_RESPONSE_1" | jq -r '.success')
    
    if [ "$SUCCESS_1" = "true" ]; then
        echo -e "${GREEN}✓ Order 1 successful${NC}"
        ORDER_HASH_1=$(echo "$ORDER_RESPONSE_1" | jq -r '.orderHash')
        echo -e "  Order Hash: ${ORDER_HASH_1}"
    else
        echo -e "${RED}✗ Order 1 failed${NC}"
        echo -e "  Error: $(echo "$ORDER_RESPONSE_1" | jq -r '.error // .message')"
    fi
    
    # Order 2 - Should succeed
    DEADLINE_2=$(python3 -c "import time; print(int(time.time() + 3600))")
    ORDER_2='{
        "tokenIn": "'$TOKEN_IN_2'",
        "tokenOut": "'$TOKEN_OUT_2'",
        "amount": "500000000000000000",
        "price": "'$PRICE_2'",
        "trader": "0x1234567890123456789012345678901234567890",
        "receiver": "0x0987654321098765432109876543210987654321",
        "nonce": "12345002",
        "deadline": '$DEADLINE_2'
    }'
    
    echo -e "${BLUE}Order 2: ${TOKEN_IN_2} -> ${TOKEN_OUT_2} at price ${PRICE_2}${NC}"
    ORDER_RESPONSE_2=$(curl -s -X POST "${SERVER_URL}/order" -H "Content-Type: application/json" -d "$ORDER_2")
    SUCCESS_2=$(echo "$ORDER_RESPONSE_2" | jq -r '.success')
    
    if [ "$SUCCESS_2" = "true" ]; then
        echo -e "${GREEN}✓ Order 2 successful${NC}"
        ORDER_HASH_2=$(echo "$ORDER_RESPONSE_2" | jq -r '.orderHash')
        echo -e "  Order Hash: ${ORDER_HASH_2}"
    else
        echo -e "${RED}✗ Order 2 failed${NC}"
        echo -e "  Error: $(echo "$ORDER_RESPONSE_2" | jq -r '.error // .message')"
    fi
    
    # Order 3 - Should fail (expired deadline)
    EXPIRED_DEADLINE=$(python3 -c "import time; print(int(time.time() - 3600))")
    ORDER_3='{
        "tokenIn": "'$TOKEN_IN_1'",
        "tokenOut": "'$TOKEN_OUT_1'",
        "amount": "1000000000000000000",
        "price": "'$PRICE_1'",
        "trader": "0x1234567890123456789012345678901234567890",
        "receiver": "0x0987654321098765432109876543210987654321",
        "nonce": "12345003",
        "deadline": '$EXPIRED_DEADLINE'
    }'
    
    echo -e "${BLUE}Order 3: (Should fail - expired deadline)${NC}"
    ORDER_RESPONSE_3=$(curl -s -X POST "${SERVER_URL}/order" -H "Content-Type: application/json" -d "$ORDER_3")
    SUCCESS_3=$(echo "$ORDER_RESPONSE_3" | jq -r '.success')
    
    if [ "$SUCCESS_3" = "false" ]; then
        echo -e "${GREEN}✓ Order 3 correctly failed (expired)${NC}"
        echo -e "  Error: $(echo "$ORDER_RESPONSE_3" | jq -r '.error // .message')"
    else
        echo -e "${RED}✗ Order 3 should have failed but succeeded${NC}"
    fi
    
    # Order 4 - Should fail (invalid price)
    INVALID_PRICE=$(python3 -c "print($PRICE_1 * 2)")
    ORDER_4='{
        "tokenIn": "'$TOKEN_IN_1'",
        "tokenOut": "'$TOKEN_OUT_1'",
        "amount": "1000000000000000000",
        "price": "'$INVALID_PRICE'",
        "trader": "0x1234567890123456789012345678901234567890",
        "receiver": "0x0987654321098765432109876543210987654321",
        "nonce": "12345004",
        "deadline": '$DEADLINE_1'
    }'
    
    echo -e "${BLUE}Order 4: (Should fail - invalid price)${NC}"
    ORDER_RESPONSE_4=$(curl -s -X POST "${SERVER_URL}/order" -H "Content-Type: application/json" -d "$ORDER_4")
    SUCCESS_4=$(echo "$ORDER_RESPONSE_4" | jq -r '.success')
    
    if [ "$SUCCESS_4" = "false" ]; then
        echo -e "${GREEN}✓ Order 4 correctly failed (invalid price)${NC}"
        echo -e "  Error: $(echo "$ORDER_RESPONSE_4" | jq -r '.error // .message')"
    else
        echo -e "${RED}✗ Order 4 should have failed but succeeded${NC}"
    fi
    
    # Wait a moment for stats to update
    sleep 1
}

# Function to test /stats endpoint (after orders)
test_final_stats() {
    echo -e "\n${BLUE}4. Testing /stats endpoint (after orders)${NC}"
    
    FINAL_STATS_RESPONSE=$(curl -s "${SERVER_URL}/stats")
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ /stats endpoint successful${NC}"
        
        TOTAL_HITS=$(echo "$FINAL_STATS_RESPONSE" | jq '.totalHits')
        SUCCESSFUL_HITS=$(echo "$FINAL_STATS_RESPONSE" | jq '.successfulHits')
        FAILED_HITS=$(echo "$FINAL_STATS_RESPONSE" | jq '.failedHits')
        HITS_PER_MINUTE=$(echo "$FINAL_STATS_RESPONSE" | jq '.hitsPerMinute')
        AVG_RESPONSE_TIME=$(echo "$FINAL_STATS_RESPONSE" | jq '.averageResponseTime')
        
        echo -e "${BLUE}Final Stats:${NC}"
        echo -e "  Total Hits: ${TOTAL_HITS}"
        echo -e "  Successful Hits: ${SUCCESSFUL_HITS}"
        echo -e "  Failed Hits: ${FAILED_HITS}"
        echo -e "  Hits per Minute: ${HITS_PER_MINUTE}"
        echo -e "  Average Response Time: ${AVG_RESPONSE_TIME} ns"
        
        # Calculate expected values
        EXPECTED_TOTAL=4
        EXPECTED_SUCCESS=2
        EXPECTED_FAILED=2
        
        echo -e "\n${YELLOW}Verification:${NC}"
        
        if [ "$TOTAL_HITS" = "$EXPECTED_TOTAL" ]; then
            echo -e "${GREEN}✓ Total hits correct: ${TOTAL_HITS}/${EXPECTED_TOTAL}${NC}"
        else
            echo -e "${RED}✗ Total hits incorrect: ${TOTAL_HITS}/${EXPECTED_TOTAL}${NC}"
        fi
        
        if [ "$SUCCESSFUL_HITS" = "$EXPECTED_SUCCESS" ]; then
            echo -e "${GREEN}✓ Successful hits correct: ${SUCCESSFUL_HITS}/${EXPECTED_SUCCESS}${NC}"
        else
            echo -e "${RED}✗ Successful hits incorrect: ${SUCCESSFUL_HITS}/${EXPECTED_SUCCESS}${NC}"
        fi
        
        if [ "$FAILED_HITS" = "$EXPECTED_FAILED" ]; then
            echo -e "${GREEN}✓ Failed hits correct: ${FAILED_HITS}/${EXPECTED_FAILED}${NC}"
        else
            echo -e "${RED}✗ Failed hits incorrect: ${FAILED_HITS}/${EXPECTED_FAILED}${NC}"
        fi
        
        # Show token pair stats
        echo -e "\n${YELLOW}Token Pair Statistics:${NC}"
        echo "$FINAL_STATS_RESPONSE" | jq -r '.tokenPairStats | to_entries[] | "\(.key): \(.value.totalHits) hits (\(.value.successfulHits) success, \(.value.failedHits) failed)"'
        
    else
        echo -e "${RED}✗ /stats endpoint failed${NC}"
        return 1
    fi
}

# Function to test /recent-hits endpoint
test_recent_hits() {
    echo -e "\n${BLUE}5. Testing /recent-hits endpoint${NC}"
    
    RECENT_HITS_RESPONSE=$(curl -s "${SERVER_URL}/recent-hits?limit=10")
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ /recent-hits endpoint successful${NC}"
        
        HIT_COUNT=$(echo "$RECENT_HITS_RESPONSE" | jq 'length')
        echo -e "${BLUE}Recent hits count: ${HIT_COUNT}${NC}"
        
        if [ "$HIT_COUNT" -gt 0 ]; then
            echo -e "\n${YELLOW}Recent Hits:${NC}"
            echo "$RECENT_HITS_RESPONSE" | jq -r '.[] | "[\(.timestamp)] \(.tokenIn) -> \(.tokenOut): \(if .success then "SUCCESS" else "FAILED" end) (ID: \(.id))"'
        fi
    else
        echo -e "${RED}✗ /recent-hits endpoint failed${NC}"
        return 1
    fi
}

# Function to calculate and verify price calculations
verify_price_calculations() {
    echo -e "\n${BLUE}6. Price Calculation Verification${NC}"
    
    echo -e "${YELLOW}Fetching current token prices and verifying calculations...${NC}"
    
    # Get levels again to see current prices
    LEVELS=$(curl -s "${SERVER_URL}/levels")
    
    echo -e "\n${BLUE}Price Verification for each pair:${NC}"
    echo "$LEVELS" | jq -r 'to_entries[] | .key as $pair | .value[0] as $level | "Pair: \($pair)\n  Available Quantity: \($level[0])\n  Exchange Rate: \($level[1])\n"'
    
    echo -e "${YELLOW}Note: Verify that:${NC}"
    echo -e "1. Each token has ~50% of \$1M worth available (based on current USD price)"
    echo -e "2. Exchange rates are calculated as price_tokenA / price_tokenB"
    echo -e "3. All 6 pairs are present (3 tokens = 3×2 = 6 combinations)"
}

# Function to test reset functionality
test_reset() {
    echo -e "\n${BLUE}7. Testing /reset endpoint${NC}"
    
    RESET_RESPONSE=$(curl -s -X POST "${SERVER_URL}/reset")
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ /reset endpoint successful${NC}"
        echo -e "Response: $(echo "$RESET_RESPONSE" | jq -r '.message')"
        
        # Check stats after reset
        sleep 1
        RESET_STATS=$(curl -s "${SERVER_URL}/stats")
        TOTAL_AFTER_RESET=$(echo "$RESET_STATS" | jq '.totalHits')
        
        if [ "$TOTAL_AFTER_RESET" = "0" ]; then
            echo -e "${GREEN}✓ Stats successfully reset${NC}"
        else
            echo -e "${RED}✗ Stats not properly reset (still shows ${TOTAL_AFTER_RESET} hits)${NC}"
        fi
    else
        echo -e "${RED}✗ /reset endpoint failed${NC}"
    fi
}

# Main execution
main() {
    check_server
    test_levels
    test_initial_stats
    test_orders
    test_final_stats
    test_recent_hits
    verify_price_calculations
    test_reset
    
    echo -e "\n${BLUE}=====================================${NC}"
    echo -e "${GREEN}    Test Script Complete!    ${NC}"
    echo -e "${BLUE}=====================================${NC}"
    
    # Cleanup temp files
    rm -f /tmp/levels_response.json /tmp/initial_stats.json
}

# Run main function
main