#!/bin/bash

set -e

LB_URL=${1:-"http://localhost:8090"}
VIDEO_ID=${2:-"wolf-1770292891"}
CONCURRENT=${3:-20}

SEGMENT_URL="$LB_URL/hls/$VIDEO_ID/segment_0005.m4s"

echo "Cache Stampede Test"
echo "This test fires $CONCURRENT simultaneous requests"
echo "for a cold segment (not in any cache)."
echo ""
echo "Expected behavior:"
echo "  Only ONE origin fetch (check origin logs)"
echo "  One cache node gets the lock"
echo "  Others wait or fetch from peer"
echo ""
read -p "Press Enter to start test (Ctrl+C to cancel)..."

echo ""
echo "Firing $CONCURRENT concurrent requests..."
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
echo "Now check the logs to verify stampede prevention:"
echo "   make logs-origin-fetch    # Should show ONLY 1 origin request"
echo "   make logs-cache-locks     # Shows lock coordination"
echo "   make logs-cache-gossip    # Shows peer discovery"
