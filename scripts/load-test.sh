#!/bin/bash
# Load testing script - simulates concurrent users

set -e

LB_URL=${1:-"http://localhost:8090"}
VIDEO_ID=${2:-"wolf-1770292891"}
CONCURRENT=${3:-10}
REQUESTS=${4:-50}

echo "Load Testing Configuration:"
echo "   Load Balancer: $LB_URL"
echo "   Video ID: $VIDEO_ID"
echo "   Concurrent Users: $CONCURRENT"
echo "   Total Requests: $REQUESTS"
echo ""

# Test endpoints
MASTER_URL="$LB_URL/hls/$VIDEO_ID/master.m3u8"
SEGMENT_URL="$LB_URL/hls/$VIDEO_ID/segment_0000.m4s"

echo "Testing master playlist fetch..."

# Warm-up request
curl -s -o /dev/null -w "Warm-up: %{http_code} in %{time_total}s\n" "$MASTER_URL"

echo "==="
echo "Starting concurrent load test..."

START_TIME=$(date +%s)
SUCCESS=0
FAILED=0

# Generate requests
for i in $(seq 1 $REQUESTS); do
    (
        RESPONSE=$(curl -s -o /dev/null -w "%{http_code}|%{time_total}" "$SEGMENT_URL" 2>/dev/null)
        CODE=$(echo $RESPONSE | cut -d'|' -f1)
        TIME=$(echo $RESPONSE | cut -d'|' -f2)
        
        if [ "$CODE" = "200" ]; then
            echo "✓ Request $i: ${CODE} (${TIME}s)"
        else
            echo "✗ Request $i: ${CODE} (${TIME}s)"
        fi
    ) &
    
    # Limit concurrent requests
    if [ $((i % CONCURRENT)) -eq 0 ]; then
        wait
    fi
done

wait

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo ""
echo "Load Test Complete"
echo "   Duration: ${DURATION}s"
echo "   Requests/sec: $(($REQUESTS / $DURATION))"
echo ""
echo "Check logs with: make logs-cache-all"
