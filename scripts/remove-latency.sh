#!/bin/bash
# Remove network latencies using containerlab's built-in netem tools

set -e

PREFIX="oulu"
LAB_NAME="telco-cdn-oulu"

remove_delay() {
    local node=$1
    echo "  [✓] ${node}: removing latency"
    containerlab tools netem set -n "${PREFIX}-${LAB_NAME}-${node}" -i eth0 --delay 0ms 2>/dev/null || \
        echo "  [!] Failed to set 0ms on ${node}, skipping"
}

echo "Removing latencies using containerlab netem:"
echo ""

remove_delay "client"
remove_delay "oulu-cache-1"
remove_delay "oulu-cache-2"
remove_delay "oulu-cache-3"
remove_delay "origin"

echo ""
echo "[✓] Latencies removed!"
