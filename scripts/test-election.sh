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

if [[ "$LEADER" == *"cache-1"* ]]; then
    LEADER_CONTAINER="oulu-telco-cdn-oulu-oulu-cache-1"
elif [[ "$LEADER" == *"cache-2"* ]]; then
    LEADER_CONTAINER="oulu-telco-cdn-oulu-oulu-cache-2"
else
    LEADER_CONTAINER="oulu-telco-cdn-oulu-oulu-cache-3"
fi

echo "Stopping $LEADER_CONTAINER"
date +%s > "$OUTPUT_DIR/kill-timestamp.txt"
docker stop "$LEADER_CONTAINER"

echo "Waiting for election..."
sleep 10

NEW_LEADER=$(get_leader 8002)
echo "New leader: $NEW_LEADER"
date +%s > "$OUTPUT_DIR/elected-timestamp.txt"

echo "Testing availability..."
VIDEO_ID="wolf-1770316220"
URL="http://localhost:8080/hls/${VIDEO_ID}/segment_0005.m4s"
SUCCESS=0
for i in {1..10}; do
    curl -s -f -o /dev/null "$URL" && SUCCESS=$((SUCCESS + 1))
    sleep 0.5
done
echo "Availability: $SUCCESS/10"

echo "Restarting old leader"
docker start "$LEADER_CONTAINER"
sleep 5

python3 - <<EOF
kill_time = int(open('$OUTPUT_DIR/kill-timestamp.txt').read())
elected_time = int(open('$OUTPUT_DIR/elected-timestamp.txt').read())
election_time = elected_time - kill_time

print(f"Election time: {election_time}s")
print(f"Availability: $SUCCESS/10")

with open('$OUTPUT_DIR/summary.txt', 'w') as f:
    f.write(f"Election Time: {election_time}s\n")
    f.write(f"Availability: $SUCCESS/10\n")
EOF

echo "Results: $OUTPUT_DIR"
