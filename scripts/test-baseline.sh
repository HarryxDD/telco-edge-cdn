#!/bin/bash
set -e

# Use provided OUTPUT_DIR or default to baseline
BASE_DIR="${OUTPUT_DIR:-data/evaluation-results}"
OUTPUT_DIR="${BASE_DIR}/baseline"
mkdir -p "$OUTPUT_DIR"

VIDEO_ID="wolf-1770316220"
SEGMENT="segment_0005.m4s"
URL="http://localhost:8080/hls/${VIDEO_ID}/${SEGMENT}"

measure_latency() {
    local output_file=$1
    local iterations=$2
    > "$output_file"
    for i in $(seq 1 $iterations); do
        time_ms=$(curl -o /dev/null -s -w "%{time_total}" "$URL")
        echo "$time_ms * 1000" | bc >> "$output_file"
    done
}

echo "Clearing caches..."
docker exec oulu-telco-cdn-oulu-oulu-cache-1 rm -rf /app/data/* 2>/dev/null || true
docker exec oulu-telco-cdn-oulu-oulu-cache-2 rm -rf /app/data/* 2>/dev/null || true
docker exec oulu-telco-cdn-oulu-oulu-cache-3 rm -rf /app/data/* 2>/dev/null || true
sleep 2

echo "Testing cold cache..."
first_request_time=$(curl -o /dev/null -s -w "%{time_total}" "$URL")
first_request_ms=$(echo "$first_request_time * 1000" | bc)
echo "$first_request_ms" > "$OUTPUT_DIR/first-request.txt"

echo "Testing warm cache (100 requests)..."
measure_latency "$OUTPUT_DIR/warm-cache-latency.txt" 100

echo "Testing direct origin (50 requests)..."
ORIGIN_URL="http://localhost:8081/hls/${VIDEO_ID}/${SEGMENT}"
> "$OUTPUT_DIR/origin-latency.txt"
for i in $(seq 1 50); do
    time_ms=$(curl -o /dev/null -s -w "%{time_total}" "$ORIGIN_URL")
    echo "$time_ms * 1000" | bc >> "$OUTPUT_DIR/origin-latency.txt"
done

echo "Analyzing results..."
python3 - <<EOF
import numpy as np
import json

def load_data(file):
    with open(file) as f:
        return [float(line.strip()) for line in f if line.strip()]

def calculate_stats(data, name):
    data = np.array(data)
    stats = {
        'name': name,
        'count': len(data),
        'mean': float(np.mean(data)),
        'median': float(np.median(data)),
        'std': float(np.std(data)),
        'min': float(np.min(data)),
        'max': float(np.max(data)),
        'p50': float(np.percentile(data, 50)),
        'p95': float(np.percentile(data, 95)),
        'p99': float(np.percentile(data, 99))
    }
    return stats

# Load data
first_req = load_data('$OUTPUT_DIR/first-request.txt')
warm = load_data('$OUTPUT_DIR/warm-cache-latency.txt')
origin = load_data('$OUTPUT_DIR/origin-latency.txt')

# Calculate stats
results = {
    'first_request': calculate_stats(first_req, 'First Request (Cold)'),
    'warm_cache': calculate_stats(warm, 'Warm Cache'),
    'direct_origin': calculate_stats(origin, 'Direct Origin')
}

# Save JSON
with open('$OUTPUT_DIR/baseline-stats.json', 'w') as f:
    json.dump(results, f, indent=2)

for key, stats in results.items():
    print(f"{stats['name']}: P50={stats['p50']:.1f}ms, P95={stats['p95']:.1f}ms")

cold_mean = results['first_request']['mean']
warm_mean = results['warm_cache']['mean']
print(f"\nSpeedup: {cold_mean / warm_mean:.1f}x faster")
EOF

echo "Results: $OUTPUT_DIR"
