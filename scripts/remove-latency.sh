#!/bin/bash
# Script to remove network latency from containers

set -e

CONTAINER=$1

if [ -z "$CONTAINER" ]; then
    echo "Usage: $0 <container-name>"
    echo "Example: $0 telco-cache-1"
    exit 1
fi

echo "Removing latency from $CONTAINER..."

docker exec $CONTAINER tc qdisc del dev eth0 root 2>/dev/null || echo "No latency configured"

echo "Latency removed"
