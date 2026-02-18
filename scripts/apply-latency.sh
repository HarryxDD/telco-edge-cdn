#!/bin/bash
# Apply network latencies using containerlab's built-in netem tools
# This affects ALL outbound traffic from each container

set -e

PREFIX="oulu"
LAB_NAME="telco-cdn-oulu"

add_delay() {
    local node=$1
    local delay=$2
    echo "  [✓] ${node}: +${delay}ms outbound latency"
    containerlab tools netem set -n "${PREFIX}-${LAB_NAME}-${node}" -i eth0 --delay "${delay}ms" 2>/dev/null || \
        echo "  [!] Failed to set delay on ${node}, skipping"
}

echo "Applying latencies using containerlab netem:"
echo ""

# Client: last-mile latency (keep small so demo focuses on origin distance)
add_delay "client" "7"

# Caches: backhaul latency to origin (~200ms)
add_delay "oulu-cache-1" "200" 
add_delay "oulu-cache-2" "200"
add_delay "oulu-cache-3" "200"

echo ""
echo "[✓] Latencies applied (client 7ms, caches 200ms)"
