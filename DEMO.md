# MEC Oulu CDN Demo

## Quick Start

```bash
# 1. Build images
make build

# 2. Deploy MEC Oulu
make clab-up

# 3. Test
make test-fetch
```

## Architecture

```
Cloud:
  ├─ Origin (port 8081)
  └─ ML Service (port 8090)

MEC Oulu (Complete CDN):
  ├─ Load Balancer (port 8080)
  ├─ Cache-1 (port 8001)
  ├─ Cache-2 (port 8002)
  └─ Cache-3 (port 8003)

Client → MEC LB → Cache Nodes → Origin
```

## Network Latencies

- Client → MEC: ~7ms (last mile)
- MEC internal: ~1ms (LB to caches)
- MEC → Origin: 60-70ms (backhaul)
- Inter-cache: <1ms (gossip)

## Available Commands

### Build & Deploy
```bash
make build          # Build Docker images
make clab-up        # Deploy topology
make clab-down      # Destroy topology
make clab-inspect   # Show status
make clab-graph     # Open visual graph
```

### Testing
```bash
make test-fetch     # Test video delivery
make test-latency   # Verify network delays
make test-election  # Check leader election
make test-gossip    # View gossip logs
```

### Logs
```bash
make logs-lb        # Load balancer logs
make logs-cache1    # Cache node 1 logs
make logs-cache2    # Cache node 2 logs
make logs-cache3    # Cache node 3 logs
make logs-origin    # Origin logs
make logs-all       # All logs
```

### Debug
```bash
make shell-lb       # Shell into LB
make shell-cache1   # Shell into cache-1
make shell-client   # Shell into client
make ps             # Show containers
```

## Demo Scenarios

### Demo 1: Basic Fetch
```bash
make demo-1
```
Shows MEC serving video content through load balancer.

### Demo 2: Cache Performance
```bash
make demo-2
```
Compares cache miss (origin fetch) vs cache hit performance.

### Demo 3: Leader Election
```bash
make demo-3
```
Kills a cache node to trigger leader election, then recovers.

## Manual Testing

### Fetch video through MEC
```bash
curl http://localhost:8080/cdn/wolf-1770316220/master.m3u8
curl http://localhost:8080/cdn/wolf-1770316220/segment_0000.m4s
```

### Direct cache access
```bash
curl http://localhost:8001/cache/wolf-1770316220/segment_0000.m4s
curl http://localhost:8002/cache/wolf-1770316220/segment_0000.m4s
curl http://localhost:8003/cache/wolf-1770316220/segment_0000.m4s
```

### Check latency
```bash
# Client to LB (~7ms)
docker exec oulu-telco-cdn-oulu-client ping -c 3 oulu-lb

# Cache to origin (~60ms)
docker exec oulu-telco-cdn-oulu-oulu-cache-1 ping -c 3 origin

# Inter-cache (<1ms)
docker exec oulu-telco-cdn-oulu-oulu-cache-1 ping -c 3 oulu-cache-2
```

## Federated Learning Demo

### Overview
Each MEC site runs a FL client that:
1. **Reads** access logs from all 3 cache nodes (shared volume)
2. **Downloads** global model from cloud ML service
3. **Trains** locally (warm start from global model)
4. **Sends** updated model to aggregator
5. **Aggregator** keeps best model (highest F1 score)

### Generate Traffic for FL Training
```bash
# Generate diverse traffic patterns (need 20+ log entries)
for i in {0..30}; do
  SEGMENT=$(printf "%04d" $((i % 10)))
  curl -s http://localhost:8080/cdn/wolf-1770316220/segment_${SEGMENT}.m4s > /dev/null
  echo "Fetched segment $SEGMENT"
  sleep 0.5
done
```

### Observe FL Training
```bash
# Watch FL client logs (shows training rounds)
docker logs -f oulu-telco-cdn-oulu-oulu-fl-client

# Check FL status
curl http://localhost:8092/status | jq

# Expected output:
# {
#   "fl_round": 3,
#   "last_update": "2025-02-25T...",
#   "participants": {"oulu": {...}},
#   "avg_f1": 0.8234
# }
```

### Verify FL Flow
```bash
# 1. Check logs are being written by all 3 caches
tail -20 data/oulu-logs/access.ndjson

# 2. Watch FL training (should show "Loaded global model")
docker logs oulu-telco-cdn-oulu-oulu-fl-client | grep -A5 "Round"

# 3. Verify model upload
curl http://localhost:8092/status | jq '.participants'

# 4. Download global model (for inspection)
curl http://localhost:8092/model -o global_model.pkl
```

### FL Demo Script
```bash
# Full FL demonstration
echo "=== Generating Initial Traffic ==="
for i in {1..50}; do
  curl -s http://localhost:8080/cdn/wolf-1770316220/segment_000$((i % 10)).m4s > /dev/null
done

echo "=== Waiting for FL Round 1 (120s) ==="
sleep 130

echo "=== Checking FL Status ==="
curl -s http://localhost:8092/status | jq '{round: .fl_round, f1: .avg_f1, sites: .participants | keys}'

echo "=== Generating More Traffic (different pattern) ==="
for i in {1..30}; do
  curl -s http://localhost:8080/cdn/wolf-1770316220/segment_00$(( (i * 2) % 10 )).m4s > /dev/null
done

echo "=== Waiting for FL Round 2 ==="
sleep 130

echo "=== Final Status ==="
curl -s http://localhost:8092/status | jq
```

### Key Metrics to Show
```bash
# Cache node metrics (Prometheus)
curl -s http://localhost:8001/metrics | grep 'cache_hit\|cache_miss\|request_duration'

# ML service metrics
curl -s http://localhost:8092/metrics | grep 'fl_'

# Access log count (should grow over time)
wc -l data/oulu-logs/access.ndjson
```

### Expected FL Behavior

**Round 1:** Train from scratch → F1 ≈ 0.65-0.75
**Round 2:** Warm start from global → F1 ≈ 0.75-0.85 (improves!)
**Round 3:** Continue learning → F1 ≈ 0.80-0.90

### Troubleshooting FL

**"Not enough data to train"**
- Generate more traffic (need 20+ log entries)
- Wait 1-2 minutes for logs to accumulate

**"Could not load global model"**
- Normal for Round 1 (no global model yet)
- If persists after Round 2, check ML service logs: `docker logs oulu-telco-cdn-oulu-ml-service`

**"Model update failed"**
- Check ML service is running: `curl http://localhost:8092/health`
- Check network connectivity from FL client

## Topology File

Location: `infrastructure/containerlab/topology.yml`

MEC Oulu contains:
- 1 Load Balancer (entry point)
- 3 Cache Nodes (distributed)
- 1 FL Client (reads logs from all caches)
- Gossip protocol for coordination
- Bully algorithm for leader election

Cloud contains:
- Origin server (video storage)
- ML aggregator service (FL coordination)

Future expansion: Add Tampere/Helsinki MECs for multi-site FL.
