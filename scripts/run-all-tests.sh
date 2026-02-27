#!/bin/bash
set -e

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OUTPUT_DIR="data/evaluation-results/${TIMESTAMP}"
mkdir -p "${OUTPUT_DIR}"

echo "Starting evaluation - Results: $OUTPUT_DIR"

if ! docker ps | grep -q "oulu-telco-cdn-oulu-oulu-lb"; then
    echo "ERROR: System not running. Run 'make clab-up' first."
    exit 1
fi

echo "\nVerifying network latencies..."
if ! sudo containerlab tools netem show -n oulu-telco-cdn-oulu-origin 2>/dev/null | grep -q "delay"; then
    echo "WARNING: Latencies not applied. Applying now..."
    sudo bash scripts/apply-latency.sh
    sleep 2
else
    echo "Latencies already applied"
fi

chmod +x scripts/*.sh

# Export OUTPUT_DIR so test scripts use the timestamped directory
export OUTPUT_DIR

echo "\n[1/5] Baseline test"
./scripts/test-baseline.sh

echo "\n[2/5] Cache hit ratio"
./scripts/test-cache-hit.sh

if command -v k6 &> /dev/null; then
    echo "\n[3/5] Load testing (takes ~12 minutes)"
    cd benchmarks/load-testing
    k6 run --out json="../../${OUTPUT_DIR}/k6-results.json" k6-test.js | tee "../../${OUTPUT_DIR}/k6.log"
    cd ../..
else
    echo "\n[3/5] Skipping k6 (not installed)"
fi

read -p "\n[4/5] Run election test (stops a node)? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    ./scripts/test-election.sh
fi

echo "\n[5/5] Stampede test"
make test-stampede 2>&1 | tee "${OUTPUT_DIR}/stampede.log"

echo "\nCollecting metrics..."
./scripts/collect-metrics.sh 10 > "${OUTPUT_DIR}/metrics.txt" 2>&1

if command -v python3 &> /dev/null && python3 -c "import matplotlib" 2>/dev/null; then
    echo "\nGenerating graphs..."
    python3 scripts/analyze-results.py "${OUTPUT_DIR}"
fi

echo "\nDone! Results in: ${OUTPUT_DIR}"
