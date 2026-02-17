#!/bin/bash
# Inject network latency to ORIGIN server (simulates origin in another country)

set -e

CONTAINER=${1:-telco-origin}  # Default to origin server
DELAY=${2:-2000}  # Default 2000ms (2 seconds) for demo
JITTER=${3:-200}  # Default 200ms

if [ -z "$1" ]; then
    echo "Usage: $0 [container-name] [delay-ms] [jitter-ms]"
    echo ""
    echo "Examples:"
    echo "  $0 telco-origin 2000 200    # Origin has 2s delay"
    echo "  $0 telco-origin 100 20      # Origin has 100ms delay (realistic)"
    echo ""
    echo "This simulates origin server in another country/continent."
    echo "   Cache nodes will be slow fetching from origin, but clients stay fast"
    exit 1
fi

echo "Simulating distant origin server..."
echo "   Container: $CONTAINER"
echo "   Delay: ${DELAY}ms ±${JITTER}ms"
echo ""

# Check if tc is available in container, install if needed
echo "Checking dependencies..."
docker exec $CONTAINER sh -c 'command -v tc >/dev/null 2>&1 || apk add --no-cache iproute2'

# Add latency using netem (applies to incoming traffic from cache nodes)
echo "Injecting latency..."
docker exec $CONTAINER tc qdisc add dev eth0 root netem delay ${DELAY}ms ${JITTER}ms distribution normal

echo ""
echo "Latency injected successfully!"
echo ""
echo "To remove: ./remove-latency.sh $CONTAINER"

