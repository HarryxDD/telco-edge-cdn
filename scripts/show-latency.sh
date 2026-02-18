#!/bin/bash
# Show network latencies using containerlab's built-in netem tools

set -e

PREFIX="oulu"
LAB_NAME="telco-cdn-oulu"

show_delay() {
    local node=$1
    echo "=== ${node} ==="
    containerlab tools netem show -n "${PREFIX}-${LAB_NAME}-${node}" 2>/dev/null || \
        echo "  [!] Failed to show ${node}"
    echo ""
}

echo "Current network impairments:"
echo ""

show_delay "client"
show_delay "oulu-cache-1"
show_delay "oulu-cache-2"
show_delay "oulu-cache-3"
