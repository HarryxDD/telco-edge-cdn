# Evaluation Methodology

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
| **Latency** | Request duration (P50, P95, P99) | <10ms (edge)<br>~200ms (origin) | HTTP request timing |
| **Throughput** | Requests per second | >1000 req/s | Load testing (k6) |
| **Cache Hit Ratio** | % of requests served from cache | >90% | Log analysis / k6 headers |
| **Availability** | Success rate of requests | >99% | Load testing / Chaos testing |
| **Failover Time** | Time to detect and recover from node failure | <10s (target) | Chaos testing (`test-election.sh`) |

### Secondary Metrics

| Metric | Definition | Measurement |
|--------|------------|-------------|
| **CPU Usage** | Average CPU utilization | `docker stats`, Prometheus |
| **Memory Usage** | RAM consumption per service | `docker stats`, Prometheus |
| **Network Latency** | Configured delay between nodes | `containerlab tools netem show` |
| **FL Accuracy** | Machine learning model metrics | ML Aggregator API |

---

## Automated Evaluation Suite

The complete evaluation suite can be run at once using the master script. Results are timestamped and collected automatically.

```bash
# Run all evaluations (Baseline, Cache Hit, k6 Load, Failover/Election, Stampede)
make eval-all
```
This runs `scripts/run-all-tests.sh` which orchestrates the following individual tests and outputs all metrics and reports to `data/evaluation-results/{timestamp}/`.

---

## Test Scenarios

### Scenario 1: Baseline Performance

**Objective:** Measure performance differences between a cold cache, warm cache, and direct origin fetching.

**Test Execution:**
```bash
make eval-baseline
# or directly: ./scripts/test-baseline.sh
```

**Methodology:**
1. Clears data volumes across all 3 cache nodes.
2. Makes a single "cold" request to the LB.
3. Warms the cache with 10 sequential requests.
4. Measures latency of 100 requests against the warm cache.
5. Measures latency of 50 requests directly against the Origin Server.
6. Calculates P50 and P95 statistics using a local Python script snippet.

**Expected Results:**
- Origin Latency (backhaul limit): ~200ms
- Warm Cache Latency (edge): <10ms
- Speedup: 20x+ faster at the edge.

---

### Scenario 2: Cache Hit Ratio

**Objective:** Test the cache efficiency using a Zipf-like content distribution.

**Test Execution:**
```bash
make eval-cache-hit
# or directly: ./scripts/test-cache-hit.sh
```

**Methodology:**
1. Clears data volumes across all 3 cache nodes.
2. Generates 1,000 HTTP requests following a simulated popularity distribution (40% most popular, 25% second, 15% third, 10% fourth, 10% uniformly distributed among the rest).
3. Reads the `X-Cache` HTTP response header to determine local cache success.
4. Calculates the total hit ratio percent across the fleet.

**Expected Results:**
- High cache hit ratio (consistently >80%) as popular segments are cached and served from the edge.

---

### Scenario 3: Load Testing (k6)

**Objective:** Detail system throughput and request duration handling under sustained high load.

**Test Execution:**
```bash
make eval-load
# or directly: cd benchmarks/load-testing && k6 run --out json=results.json k6-test.js
```

**Methodology:**
1. Uses k6 with staged ramps simulating up to 200 concurrent HTTP virtual users fetching 10 different video segments.
2. Checks custom thresholds (`http_req_duration`: p(95) < 100ms, `error_rate`: < 1%).
3. Collects `X-Cache` metrics for total Hits/Misses under load.
4. Outputs final test summary locally.

**Expected Results:**
- P95 latency: <100ms
- Error rate: <1%
- Custom `cache_hits` trend strongly indicates that bounded-load consistent hashing successfully routes repeated segments to warm edge caches.

---

### Scenario 4: Failover and Leader Election

**Objective:** Test system resilience to node failures and ensure the Bully election algorithm successfully promotes a new cache coordinator leader.

**Test Execution:**
```bash
make eval-election
# or directly: ./scripts/test-election.sh
```

**Methodology:**
1. Validates a current cache leader exists via `/coordination/status`.
2. Warms cache heavily against a test track segment.
3. Violently stops the leader docker container (`docker stop`).
4. Runs a continuous poll testing system availability through the LB.
5. Verifies election of a new leader.
6. Analyzes logs mapping Election Time vs Availability drop.
7. Restarts the killed node and ensures clean cluster re-entry.

**Measurements:**
- Time to detect failure and elect a new leader.
- Request success rate via Edge LB during failover duration.

---

### Analytics and Charting

The repository provides analytical tools to parse JSON evaluation outputs and generate visual graphs outlining infrastructure performance. They run automatically during `make eval-all` provided that Python 3 and Matplotlib are present on the host system.

**Commands:**
```bash
# Generate basic result graphs in root analysis output directory
make eval-analyze 

# To run manually against specific result dir
python3 scripts/analyze-results.py data/evaluation-results/YYYYMMDD_HHMMSS
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

- [k6 Load Testing Documentation](https://k6.io/docs/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
