#!/bin/bash
set -e

# Use provided OUTPUT_DIR or default to cache-hit
BASE_DIR="${OUTPUT_DIR:-data/evaluation-results}"
OUTPUT_DIR="${BASE_DIR}/cache-hit"
mkdir -p "$OUTPUT_DIR"

VIDEO_ID="wolf-1770316220"
BASE_URL="http://localhost:8080/hls/${VIDEO_ID}"

echo "Clearing caches..."
docker exec oulu-telco-cdn-oulu-oulu-cache-1 rm -rf /app/data/* 2>/dev/null || true
docker exec oulu-telco-cdn-oulu-oulu-cache-2 rm -rf /app/data/* 2>/dev/null || true
docker exec oulu-telco-cdn-oulu-oulu-cache-3 rm -rf /app/data/* 2>/dev/null || true
sleep 2

echo "Generating 1000 requests (Zipf: 40%/25%/15%/10%/10%)..."

> "$OUTPUT_DIR/requests.log"

for i in $(seq 1 1000); do
    # Generate random number 0-99
    r=$((RANDOM % 100))
    
    if [ $r -lt 40 ]; then
        SEGMENT="segment_0001.m4s"
    elif [ $r -lt 65 ]; then
        SEGMENT="segment_0002.m4s"
    elif [ $r -lt 80 ]; then
        SEGMENT="segment_0003.m4s"
    elif [ $r -lt 90 ]; then
        SEGMENT="segment_0004.m4s"
    else
        # Remaining 10% distributed
        SEG_NUM=$((5 + RANDOM % 6))
        SEGMENT=$(printf "segment_%04d.m4s" $SEG_NUM)
    fi
    
    URL="${BASE_URL}/${SEGMENT}"
    
    # Use temp file for headers
    HEADER_FILE=$(mktemp)
    
    # Measure time and status, save headers to temp file
    TIME=$(curl -o /dev/null -D "$HEADER_FILE" -s -w "%{time_total}" "$URL" 2>/dev/null)
    STATUS=$(curl -o /dev/null -s -w "%{http_code}" "$URL" 2>/dev/null)
    
    # Extract X-Cache header value
    CACHE_STATUS=$(grep -i "^x-cache:" "$HEADER_FILE" | awk '{print $2}' | tr -d '\r' || echo "UNKNOWN")
    
    rm -f "$HEADER_FILE"
    
    echo "${SEGMENT},${STATUS},${TIME},${CACHE_STATUS}" >> "$OUTPUT_DIR/requests.log"
    [ $((i % 100)) -eq 0 ] && echo "  $i/1000"
    sleep 0.01
done

sleep 2

echo "Analyzing..."

python3 - <<EOF
import json
from collections import Counter, defaultdict

segment_counts = Counter()
segment_times = defaultdict(list)
hits = 0
misses = 0
unknown = 0

with open('$OUTPUT_DIR/requests.log') as f:
    for line in f:
        if line.strip():
            parts = line.strip().split(',')
            if len(parts) >= 4:
                segment, status, time_str, cache_status = parts[0], parts[1], parts[2], parts[3]
                segment_counts[segment] += 1
                try:
                    segment_times[segment].append(float(time_str))
                except:
                    pass
                
                # Count actual hits/misses from X-Cache header
                if cache_status == 'HIT':
                    hits += 1
                elif cache_status == 'MISS':
                    misses += 1
                else:
                    unknown += 1

total_requests = sum(segment_counts.values())
hit_ratio = (hits / total_requests * 100) if total_requests > 0 else 0

# Generate segment_stats with count and percent
segment_stats = {}
for segment, count in segment_counts.items():
    segment_stats[segment] = {
        'count': count,
        'percent': round((count / total_requests * 100), 2) if total_requests > 0 else 0
    }

results = {
    'total_requests': total_requests,
    'cache_hits': hits,
    'cache_misses': misses,
    'hit_ratio_percent': round(hit_ratio, 2),
    'segment_stats': segment_stats
}

with open('$OUTPUT_DIR/analysis.json', 'w') as f:
    json.dump(results, f, indent=2)

print(f"Requests: {total_requests}")
print(f"Hits: {hits}, Misses: {misses}", end='')
if unknown > 0:
    print(f", Unknown: {unknown}")
else:
    print()
print(f"Hit Ratio: {hit_ratio:.1f}%")
print(f"Unique segments: {len(segment_counts)}")
EOF

echo "Results: $OUTPUT_DIR"
