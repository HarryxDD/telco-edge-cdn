#!/bin/bash
set -e

# Use provided OUTPUT_DIR or default to election
BASE_DIR="${OUTPUT_DIR:-data/evaluation-results}"
OUTPUT_DIR="${BASE_DIR}/election"
mkdir -p "$OUTPUT_DIR"

get_leader() {
    # Check all cache nodes to find the leader
    for port in 8001 8002 8003; do
        RESPONSE=$(curl -s "http://localhost:${port}/coordination/status" 2>/dev/null)
        IS_LEADER=$(echo "$RESPONSE" | jq -r '.is_leader' 2>/dev/null)
        if [ "$IS_LEADER" = "true" ]; then
            echo "$RESPONSE" | jq -r '.node' 2>/dev/null
            return
        fi
    done
    echo "none"
}

echo "Initial leader: $(get_leader)"
get_leader > "$OUTPUT_DIR/initial.txt"

LEADER=$(get_leader)
echo "Leader is: $LEADER"

if [ "$LEADER" = "none" ]; then
    echo "WARNING: No leader elected yet. Waiting 5 seconds..."
    sleep 5
    LEADER=$(get_leader)
    if [ "$LEADER" = "none" ]; then
        echo "ERROR: Still no leader. Election may not be working."
        echo "Defaulting to cache-1 for testing purposes."
        LEADER="cache-1"
    fi
fi

# Warm up cache before testing - ensure segment is cached on all nodes
echo "Warming up cache with test segment..."
VIDEO_ID="wolf-1770316220"
URL="http://localhost:8080/hls/${VIDEO_ID}/segment_0005.m4s"

# Send 15 requests to ensure all cache nodes have the segment
for i in {1..15}; do
    curl -s -f -o /dev/null "$URL" 2>&1 || true
    sleep 0.1
done
sleep 2
echo "Cache warmed up. All nodes should have segment cached."

if [[ "$LEADER" == *"cache-1"* ]]; then
    LEADER_CONTAINER="oulu-telco-cdn-oulu-oulu-cache-1"
elif [[ "$LEADER" == *"cache-2"* ]]; then
    LEADER_CONTAINER="oulu-telco-cdn-oulu-oulu-cache-2"
else
    LEADER_CONTAINER="oulu-telco-cdn-oulu-oulu-cache-3"
fi

echo "Stopping $LEADER_CONTAINER"
KILL_TIME=$(date +%s.%N)
echo "$KILL_TIME" > "$OUTPUT_DIR/kill-timestamp.txt"
docker stop "$LEADER_CONTAINER" > /dev/null 2>&1

# Start availability test immediately in background
echo "Testing availability during election (background process)..."
(
    SUCCESS=0
    for i in {1..10}; do
        if curl -s -f -o /dev/null "$URL" 2>&1; then
            SUCCESS=$((SUCCESS + 1))
        fi
        sleep 0.5
    done
    echo "$SUCCESS" > "$OUTPUT_DIR/availability.txt"
) &
AVAILABILITY_PID=$!

echo "Waiting for election (polling every 0.5s)..."
ELECTION_COMPLETE=false
MAX_WAIT=30  # Maximum 30 seconds
ELAPSED=0

while [ "$ELECTION_COMPLETE" = false ] && [ $ELAPSED -lt $MAX_WAIT ]; do
    sleep 0.5
    ELAPSED=$((ELAPSED + 1))
    
    NEW_LEADER=$(get_leader)
    
    # Check if we have a new leader (not the old one and not "none")
    if [ "$NEW_LEADER" != "none" ] && [ "$NEW_LEADER" != "$LEADER" ]; then
        ELECTED_TIME=$(date +%s.%N)
        echo "$ELECTED_TIME" > "$OUTPUT_DIR/elected-timestamp.txt"
        echo "New leader elected: $NEW_LEADER (after ${ELAPSED}×0.5s = $((ELAPSED/2))s)"
        ELECTION_COMPLETE=true
        break
    fi
    
    if [ $((ELAPSED % 4)) -eq 0 ]; then
        echo "  Still waiting... (${ELAPSED}/2 = $((ELAPSED/2))s elapsed)"
    fi
done

if [ "$ELECTION_COMPLETE" = false ]; then
    echo "WARNING: No new leader elected after ${MAX_WAIT}s!"
    NEW_LEADER="none"
    date +%s.%N > "$OUTPUT_DIR/elected-timestamp.txt"
else
    # Determine new leader container name
    if [[ "$NEW_LEADER" == *"cache-1"* ]]; then
        NEW_LEADER_CONTAINER="oulu-telco-cdn-oulu-oulu-cache-1"
    elif [[ "$NEW_LEADER" == *"cache-2"* ]]; then
        NEW_LEADER_CONTAINER="oulu-telco-cdn-oulu-oulu-cache-2"
    else
        NEW_LEADER_CONTAINER="oulu-telco-cdn-oulu-oulu-cache-3"
    fi
    
    # Capture the actual log messages showing election
    echo "Capturing election logs from $NEW_LEADER_CONTAINER..."
    docker logs "$NEW_LEADER_CONTAINER" 2>&1 | grep -i "became leader\|acknowledges leader" | tail -10 > "$OUTPUT_DIR/election-logs.txt" || true
fi

# Wait for availability test to complete
echo "Waiting for availability test to complete..."
wait $AVAILABILITY_PID
SUCCESS=$(cat "$OUTPUT_DIR/availability.txt" 2>/dev/null || echo "0")
echo "Availability during election: $SUCCESS/10"

echo "Restarting old leader"
docker start "$LEADER_CONTAINER" > /dev/null
sleep 5

python3 - <<EOF
kill_time = float(open('$OUTPUT_DIR/kill-timestamp.txt').read())
elected_time = float(open('$OUTPUT_DIR/elected-timestamp.txt').read())
election_time = elected_time - kill_time

print(f"Election time: {election_time:.2f}s")
print(f"Availability: $SUCCESS/10")

# Read and display election logs if available
import os
log_file = '$OUTPUT_DIR/election-logs.txt'
if os.path.exists(log_file):
    print("\nElection log evidence:")
    with open(log_file) as f:
        logs = f.read().strip()
        if logs:
            for line in logs.split('\n')[:3]:  # Show first 3 lines
                print(f"  {line}")

with open('$OUTPUT_DIR/summary.txt', 'w') as f:
    f.write(f"Election Time: {int(election_time)}s\n")
    f.write(f"Availability: $SUCCESS/10\n")
EOF

echo "Results: $OUTPUT_DIR"
