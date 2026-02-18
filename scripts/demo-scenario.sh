#!/bin/bash
# Complete demo scenario - shows all key features

set -e

LB_URL=${1:-"http://localhost:8090"}

echo "DEMO SCENARIO"
echo ""

# 1: Check cluster health
echo "Checking Cluster Health..."
for node in cache-1 cache-2 cache-3; do
    STATUS=$(docker exec telco-$node curl -s http://localhost:808${node: -1}/coordination/status 2>/dev/null || echo "down")
    echo "   $node: $STATUS"
done
echo ""
sleep 2

# 2: Basic video fetch
echo "Basic Video Fetch (should be fast on repeat)..."
VIDEO_URL="$LB_URL/hls/wolf-1770292891/master.m3u8"
for i in 1 2; do
    TIME=$(curl -s -o /dev/null -w "%{time_total}s" "$VIDEO_URL")
    echo "   Fetch $i: $TIME"
done
echo ""
sleep 2

# 3: Leader check
echo "Leader Status..."
LEADER=$(docker exec telco-cache-3 curl -s http://localhost:8083/coordination/status 2>/dev/null | grep -o '"state":"[^"]*"' || echo "checking...")
echo "   Current leader: $LEADER"
echo ""
sleep 2

# 4: Kill a follower
echo "Simulating Node Failure (killing cache-2)..."
docker stop telco-cache-2 > /dev/null 2>&1
echo "   cache-2 stopped"
echo "   Video should still work..."
TIME=$(curl -s -o /dev/null -w "%{time_total}s" "$VIDEO_URL" 2>/dev/null || echo "failed")
echo "   Fetch time: $TIME"
echo ""
sleep 3

# 5: Restart node
echo "Recovering Node..."
docker start telco-cache-2 > /dev/null 2>&1
echo "   cache-2 restarted!"
echo "   Waiting for it to rejoin cluster..."
sleep 5
echo ""

# 6: Load test
echo "Light Load Test (10 concurrent requests)..."
echo "---"
SEGMENT_URL="$LB_URL/hls/wolf-1770292891/segment_0000.m4s"
for i in $(seq 1 10); do
    curl -s -o /dev/null -w "✓" "$SEGMENT_URL" &
done
wait
echo " All completed"
echo ""

echo "Next steps:"
echo "  • Run 'make stampede-test' to see lock coordination"
echo "  • Run 'make latency-test' to see network delay handling"
echo "  • Check logs with 'make logs-cache-all'"
