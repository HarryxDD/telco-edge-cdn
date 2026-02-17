#!/bin/bash

set -e

LB_URL=${1:-"http://localhost:8090"}
VIDEO_ID=${2:-"wolf-1770316220"}
CONCURRENT=${3:-20}

# Use a valid segment (wolf video has segments 0000-0025)
# Pick a random one from the valid range
RANDOM_SEGMENT=$(printf "segment_%04d.m4s" $((RANDOM % 26)))
SEGMENT_URL="$LB_URL/hls/$VIDEO_ID/$RANDOM_SEGMENT"

echo "Cache Stampede Test"
echo "Test segment: $RANDOM_SEGMENT"
echo "This test fires $CONCURRENT simultaneous requests"
echo "for a cold segment (not in any cache)."
echo ""
echo "Expected behavior:"
echo "  Only ONE origin fetch (stampede prevention working)"
echo "  One cache node gets the lock"
echo "  Others wait or fetch from peer"
echo ""
read -p "Press Enter to start test (Ctrl+C to cancel)..."

# Get current timestamp for log filtering
TEST_START=$(date '+%Y/%m/%d %H:%M')

echo ""
echo "Firing $CONCURRENT concurrent requests for $RANDOM_SEGMENT..."
START_TIME=$(date +%s.%N)

for i in $(seq 1 $CONCURRENT); do
    curl -s -o /dev/null -w "Request $i: %{http_code} in %{time_total}s\n" "$SEGMENT_URL" &
done

wait

END_TIME=$(date +%s.%N)
DURATION=$(echo "$END_TIME - $START_TIME" | bc)

echo ""
echo "All requests completed in ${DURATION}s"
echo ""
echo "Verification (checking last 2 minutes of logs)..."

# Count only actual HTTP GET requests (not all log lines mentioning the segment)
# Origin log format: [request-id] GET /videos/.../segment_XXXX.m4s - 200 (1ms)
ORIGIN_FETCHES=$(cd infrastructure/docker-compose && docker compose logs origin --since 2m 2>/dev/null | grep " GET .*$RANDOM_SEGMENT - " | wc -l)

echo ""
if [ "$ORIGIN_FETCHES" -eq 1 ]; then
    echo "STAMPEDE PREVENTION WORKING"
    echo "   Origin fetches: $ORIGIN_FETCHES (expected: 1)"
elif [ "$ORIGIN_FETCHES" -eq 0 ]; then
    echo "NO ORIGIN FETCH DETECTED"
    echo "   This segment might already be cached or test failed"
    echo "   Try running again with a different segment"
else
    echo "STAMPEDE DETECTED"
    echo "   Origin fetches: $ORIGIN_FETCHES (expected: 1)"
    echo "   Multiple nodes fetched from origin simultaneously!"
fi

echo ""
echo "Detailed logs available:"
echo "   make logs-origin-fetch    # All origin requests"
echo "   make logs-cache-locks     # Lock coordination details"
echo "   make logs-cache-gossip    # Peer discovery & sharing"
echo ""
echo "To check specific segment:"
echo "   make logs-origin-fetch | grep '$RANDOM_SEGMENT'"

