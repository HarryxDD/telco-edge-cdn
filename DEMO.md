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

## Topology File

Location: `infrastructure/containerlab/topology.yml`

MEC Oulu contains:
- 1 Load Balancer (entry point)
- 3 Cache Nodes (distributed)
- Gossip protocol for coordination
- Bully algorithm for leader election

Future expansion: Add Tampere/Helsinki MECs.
