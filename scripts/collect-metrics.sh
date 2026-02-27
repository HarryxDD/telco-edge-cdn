#!/bin/bash
set -e

DURATION=${1:-60}
OUTPUT_DIR="data/evaluation-results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULT_DIR="${OUTPUT_DIR}/${TIMESTAMP}"

mkdir -p "$RESULT_DIR"

echo "Collecting metrics (${DURATION}s)..."
sleep "$DURATION"

# Check if Prometheus is running (optional)
if curl -s -o /dev/null -w "%{http_code}" "http://localhost:9090/-/healthy" 2>/dev/null | grep -q "200"; then
    echo "Collecting Prometheus metrics..."
    curl -s "http://localhost:9090/api/v1/query?query=sum(rate(cache_hits_total[5m]))/(sum(rate(cache_hits_total[5m]))+sum(rate(cache_misses_total[5m])))" > "$RESULT_DIR/hit-ratio.json"
    curl -s "http://localhost:9090/api/v1/query?query=histogram_quantile(0.50,sum(rate(request_duration_seconds_bucket[5m]))by(le))" > "$RESULT_DIR/latency-p50.json"
    curl -s "http://localhost:9090/api/v1/query?query=histogram_quantile(0.95,sum(rate(request_duration_seconds_bucket[5m]))by(le))" > "$RESULT_DIR/latency-p95.json"
else
    echo "Prometheus not running, skipping Prometheus metrics"
fi

docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" \
  $(docker ps --filter "name=oulu-telco-cdn" --format "{{.Names}}") > "$RESULT_DIR/docker-stats.txt" || true

echo "Results: $RESULT_DIR"
