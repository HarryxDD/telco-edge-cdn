# Evaluation Methodology (WIP)

This document describes the methodology for evaluating the Telco-Edge CDN system's performance, scalability, and reliability.

## Evaluation Goals

1. **Performance**: Measure latency and throughput
2. **Scalability**: Assess system behavior under increasing load
3. **Reliability**: Test fault tolerance and recovery
4. **Efficiency**: Evaluate cache hit ratio and resource utilization
5. **Comparison**: Benchmark against traditional CDN approaches

---

## Metrics

### Primary Metrics

| Metric | Definition | Target | Measurement Method |
|--------|------------|--------|--------------------|
| **Latency (P50)** | Median response time | <5ms (edge)<br>~50ms (origin) | HTTP request timing |
| **Latency (P95)** | 95th percentile response time | <10ms (edge)<br>~75ms (origin) | HTTP request timing |
| **Latency (P99)** | 99th percentile response time | <20ms (edge)<br>~100ms (origin) | HTTP request timing |
| **Throughput** | Requests per second | >1000 req/s | Load testing (k6) |
| **Cache Hit Ratio** | % of requests served from cache | >90% | Log analysis |
| **Upload Time** | Time to upload + transcode video | <2x video duration | End-to-end timing |
| **Failover Time** | Time to detect and recover from node failure | <30s | Chaos testing |

### Secondary Metrics

| Metric | Definition | Measurement |
|--------|------------|-------------|
| **CPU Usage** | Average CPU utilization | `docker stats`, `top` |
| **Memory Usage** | RAM consumption per service | `docker stats`, `ps` |
| **Disk I/O** | Read/write operations per second | `iostat`, BadgerDB metrics |
| **Network Bandwidth** | Data transferred | `iftop`, `nethogs` |
| **Transcoding Efficiency** | Speed relative to video duration | FFmpeg logs |
| **Cache Efficiency** | Storage used vs videos cached | BadgerDB size / video count |

---

## Test Scenarios

### Scenario 1: Cold Cache Performance

**Objective:** Measure performance with empty cache (origin serving).

**Setup:**
1. Deploy system with clean cache
2. Upload 10 test videos (varying sizes)
3. Wait for transcoding completion

**Test:**
```bash
# Clear all caches
docker-compose down -v
docker-compose up -d

# Request each video's master playlist
for video in video-{1..10}; do
  time curl -o /dev/null -s http://localhost:8090/hls/$video/master.m3u8
done
```

**Expected Results:**
- Latency: 40-60ms (origin latency)
- All requests result in cache miss
- Origin load: 100%

**Data Collection:**
```bash
# Log all request times
for video in video-{1..10}; do
  for i in {1..100}; do
    curl -w "%{time_total}\n" -o /dev/null -s \
      http://localhost:8090/hls/$video/master.m3u8
  done
done > cold-cache-latency.txt

# Analyze results
cat cold-cache-latency.txt | \
  awk '{sum+=$1; sumsq+=$1*$1} END {print "Mean:",sum/NR,"StdDev:",sqrt(sumsq/NR - (sum/NR)^2)}'
```

---

### Scenario 2: Warm Cache Performance

**Objective:** Measure performance with fully warmed cache.

**Setup:**
1. Run Scenario 1 to populate cache
2. Ensure all videos cached

**Test:**
```bash
# Request same videos (now cached)
for video in video-{1..10}; do
  time curl -o /dev/null -s http://localhost:8090/hls/$video/master.m3u8
done
```

**Expected Results:**
- Latency: <5ms (edge latency)
- 100% cache hit rate
- Origin load: 0%

**Comparison:**
```bash
# Compare cold vs warm
echo "Cold cache mean: $(cat cold-cache-latency.txt | awk '{sum+=$1} END {print sum/NR}')s"
echo "Warm cache mean: $(cat warm-cache-latency.txt | awk '{sum+=$1} END {print sum/NR}')s"
```

---

### Scenario 3: Load Testing

**Objective:** Test system under sustained high load.

**Setup:**
1. Deploy with 3 cache nodes
2. Warm cache with 20 videos

**Test:**
```javascript
// k6 load test script
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
  stages: [
    { duration: '1m', target: 100 },   // Ramp up to 100 users
    { duration: '5m', target: 100 },   // Stay at 100 users
    { duration: '1m', target: 500 },   // Spike to 500 users
    { duration: '5m', target: 500 },   // Stay at 500 users
    { duration: '1m', target: 0 },     // Ramp down
  ],
  thresholds: {
    'http_req_duration': ['p(95)<50'],  // 95% < 50ms
    'http_req_failed': ['rate<0.01'],    // <1% errors
  },
};

export default function() {
  const videos = ['video-1', 'video-2', 'video-3'];
  const video = videos[Math.floor(Math.random() * videos.length)];
  
  const res = http.get(`http://localhost:8090/hls/${video}/master.m3u8`);
  
  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 50ms': (r) => r.timings.duration < 50,
  });
  
  sleep(1);
}
```

**Run Test:**
```bash
cd benchmarks/load-testing
k6 run --out json=results.json k6-test.js
```

**Expected Results:**
- Throughput: >1000 req/s sustained
- P95 latency: <50ms
- Error rate: <1%
- Cache hit ratio: >90%

**Analysis:**
```bash
# Parse k6 results
jq '.metrics.http_req_duration' results.json
jq '.metrics.http_reqs' results.json
```

---

### Scenario 4: Cache Eviction Test

**Objective:** Test LRU cache eviction behavior.

**Setup:**
1. Set small cache capacity: `CACHE_CAPACITY=10485760` (10MB)
2. Upload 20 videos (total >10MB)

**Test:**
```bash
# Request videos sequentially
for video in video-{1..20}; do
  curl -o /dev/null -s http://localhost:8090/hls/$video/master.m3u8
  sleep 0.1
done

# Request first 5 videos again
for video in video-{1..5}; do
  curl -o /dev/null -s http://localhost:8090/hls/$video/master.m3u8
done
```

**Expected Behavior:**
1. Videos 1-10 fill cache
2. Videos 11-20 trigger eviction
3. Oldest videos (least recently used) evicted first
4. Re-requesting early videos results in cache miss

**Verify:**
```bash
# Check cache logs for eviction events
docker-compose logs cache-node | grep -i evict

# Expected pattern:
# [EVICT] Removing video-1 (LRU)
# [EVICT] Removing video-2 (LRU)
```

---

### Scenario 5: Fault Tolerance Test

**Objective:** Test system resilience to node failures.

**Setup:**
1. Deploy with 3 cache nodes
2. Generate steady traffic

**Test:**
```bash
# Terminal 1: Generate traffic
while true; do
  curl -o /dev/null -s http://localhost:8090/hls/video-1/master.m3u8
  sleep 0.1
done

# Terminal 2: Kill cache nodes sequentially
docker stop telco-cache-1
sleep 60  # Observe failover
docker start telco-cache-1

docker stop telco-cache-2
sleep 60
docker start telco-cache-2
```

**Measurements:**
- Time to detect failure
- Time to reroute traffic
- Request success rate during failure
- Latency impact

**Expected Results:**
- Detection time: <30s
- Success rate: >99% (brief dip during detection)
- Latency increase: <2x (due to cache miss on new node)

**Data Collection:**
```bash
# Log request success/failure
while true; do
  status=$(curl -o /dev/null -s -w "%{http_code}" http://localhost:8090/hls/video-1/master.m3u8)
  echo "$(date '+%s'),$status" >> failover-test.csv
  sleep 0.1
done
```

---

### Scenario 6: Load Balancing Fairness

**Objective:** Verify load is distributed evenly.

**Setup:**
1. Deploy with 3 cache nodes
2. Upload 30 videos

**Test:**
```bash
# Request all videos
for video in video-{1..30}; do
  curl -o /dev/null -s http://localhost:8090/hls/$video/master.m3u8
done

# Check load distribution
docker-compose logs load-balancer | grep "Routing" | \
  awk '{print $NF}' | sort | uniq -c
```

**Expected Distribution:**
```
  10 cache-1  (33%)
  10 cache-2  (33%)
  10 cache-3  (33%)
```

**Statistical Test:**
```python
import scipy.stats as stats

# Chi-square test for uniformity
observed = [10, 10, 10]
expected = [10, 10, 10]
chi2, p_value = stats.chisquare(observed, expected)
print(f"Chi-square: {chi2}, p-value: {p_value}")
# p-value > 0.05 → distribution is uniform
```

---

## Comparison Benchmarks

### CDN vs Origin Comparison

**Objective:** Quantify benefit of edge caching.

**Test:**
```bash
# Direct origin requests
for i in {1..1000}; do
  time curl -o /dev/null -s http://localhost:8443/hls/video-1/master.m3u8
done > origin-latency.txt

# CDN requests (cached)
for i in {1..1000}; do
  time curl -o /dev/null -s http://localhost:8090/hls/video-1/master.m3u8
done > cdn-latency.txt

# Compare
echo "Origin P95: $(cat origin-latency.txt | sort -n | awk 'NR==int(0.95*950)')"
echo "CDN P95: $(cat cdn-latency.txt | sort -n | awk 'NR==int(0.95*950)')"
```

**Expected Improvement:**
- Latency reduction: 10-20x
- Throughput increase: 5-10x
- Origin load reduction: 90-95%

---

### Bounded-Load vs Round-Robin

**Objective:** Compare routing algorithms.

**Implementation:**
```go
// Temporarily modify load balancer for A/B test
func (s *Server) routeRoundRobin(path string) *Node {
    s.rrIndex = (s.rrIndex + 1) % len(s.nodes)
    return s.nodes[s.rrIndex]
}
```

**Test:**
1. Run load test with round-robin routing
2. Measure cache hit ratio
3. Switch to bounded-load hashing
4. Run same load test
5. Compare results

**Expected:**
- Bounded-load achieves 10-20% higher cache hit ratio
- More uniform load distribution
- Better handling of popular content

---

## Monitoring and Data Collection

### Prometheus Metrics

**Install Prometheus:**
```bash
cd infrastructure/monitoring
docker-compose up -d prometheus grafana
```

**Scrape Configuration:**
```yaml
scrape_configs:
  - job_name: 'origin'
    static_configs:
      - targets: ['origin:8443']
  
  - job_name: 'cache-nodes'
    static_configs:
      - targets: ['cache-1:8081', 'cache-2:8082', 'cache-3:8083']
  
  - job_name: 'load-balancer'
    static_configs:
      - targets: ['load-balancer:8090']
```

**Query Examples:**
```promql
# Average request latency
rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])

# Cache hit ratio
rate(cache_hits_total[5m]) / (rate(cache_hits_total[5m]) + rate(cache_misses_total[5m]))

# Requests per second
rate(http_requests_total[1m])

# Error rate
rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m])
```

### Log Analysis

**Aggregate Cache Stats:**
```bash
# Cache hit/miss ratio
docker-compose logs cache-node | grep CACHE | \
  awk '{print $3}' | sort | uniq -c | \
  awk '{print $2": "$1" ("$1/995*100"%)" }'

# Average response time per service
docker-compose logs | grep "latency_ms" | \
  awk '{sum+=$(NF); count++} END {print "Avg latency: "sum/count"ms"}'
```

---
## Statistical Analysis

### Latency Distribution

```python
import numpy as np
import matplotlib.pyplot as plt

# Load latency data
latencies = np.loadtxt('cdn-latency.txt')

# Calculate percentiles
p50 = np.percentile(latencies, 50)
p95 = np.percentile(latencies, 95)
p99 = np.percentile(latencies, 99)

print(f"P50: {p50:.2f}ms")
print(f"P95: {p95:.2f}ms")
print(f"P99: {p99:.2f}ms")

# Plot histogram
plt.hist(latencies, bins=50, edgecolor='black')
plt.axvline(p50, color='g', linestyle='--', label='P50')
plt.axvline(p95, color='orange', linestyle='--', label='P95')
plt.axvline(p99, color='r', linestyle='--', label='P99')
plt.xlabel('Latency (ms)')
plt.ylabel('Frequency')
plt.legend()
plt.savefig('latency-distribution.png')
```

### Cache Hit Ratio Over Time

```python
import pandas as pd

# Parse logs
df = pd.read_csv('cache-logs.csv', names=['timestamp', 'event', 'path'])

# Calculate rolling hit ratio
df['hit'] = (df['event'] == 'HIT').astype(int)
df['hit_ratio'] = df['hit'].rolling(window=100).mean()

# Plot
plt.plot(df['timestamp'], df['hit_ratio'])
plt.xlabel('Time')
plt.ylabel('Cache Hit Ratio')
plt.title('Cache Hit Ratio Over Time')
plt.savefig('cache-hit-ratio.png')
```

---

## Reporting Results

### Performance Summary Table

| Metric | Target | Achieved | Status |
|--------|--------|----------|--------|
| P50 Latency (Edge) | | | |
| P95 Latency (Edge) | | | |
| P99 Latency (Edge) | | | |
| Throughput | | | |
| Cache Hit Ratio | | | |
| Failover Time | | | |
| Upload + Transcode | | | |

### Visualizations

**Required Graphs:**
1. Latency distribution (histogram)
2. Cache hit ratio over time (line chart)
3. Load distribution across nodes (bar chart)
4. Throughput vs. latency (scatter plot)
5. Failover behavior (timeline)
6. Resource utilization (stacked area chart)

---

## References

- [Performance Testing Best Practices](https://martinfowler.com/articles/practical-test-pyramid.html)
- [k6 Load Testing Documentation](https://k6.io/docs/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
- [CDN Performance Benchmarking (Akamai)](https://www.akamai.com/resources/white-paper/state-of-online-video-2021)
- [Netflix Performance Tuning](https://netflixtechblog.com/netflix-at-velocity-2015-streaming-performance-2c3f4d42faeb)
