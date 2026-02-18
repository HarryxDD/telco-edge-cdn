# Telco CDN with MEC (Multi-access Edge Computing)

Distributed CDN system deployed at edge locations using containerlab for network simulation.

## Architecture

```
Cloud Tier:
  ├─ Origin Server (video storage)
  └─ ML Service (analytics)

MEC Oulu (Complete CDN Service):
  ├─ Load Balancer
  ├─ Cache Node 1
  ├─ Cache Node 2
  └─ Cache Node 3

Client → MEC LB → Cache Nodes → Origin
```

## Quick Start

```bash
# 1. Install containerlab (run once)
sudo make clab-install

# 2. Build images
make build

# 3. Deploy MEC Oulu
make clab-up

# 4. Test
make test-fetch
```

## Key Features

- HLS Video Streaming
- Distributed Caching (W-TinyLFU)
- Load Balancing (Bounded-load Consistent Hashing)
- Leader Election (Bully Algorithm)
- Gossip Protocol coordination
- Network latency simulation

## Network Latencies

- Client → MEC: ~7ms (last mile)
- MEC internal: ~1ms (LB to caches)
- MEC → Origin: 60-70ms (backhaul)
- Inter-cache: <1ms (gossip)

## Available Commands

```bash
# Build & Deploy
make build              # Build Docker images
make clab-up            # Deploy MEC topology
make clab-down          # Destroy topology
make clab-inspect       # Show status
make clab-graph         # Visual topology

# Testing
make test-fetch         # Test video delivery
make test-latency       # Verify network delays
make test-election      # Check leader election
make test-gossip        # View gossip logs

# Logs
make logs-lb            # Load balancer logs
make logs-cache1        # Cache-1 logs
make logs-cache2        # Cache-2 logs
make logs-cache3        # Cache-3 logs
make logs-origin        # Origin logs

# Debug
make shell-cache1       # Shell into cache-1
make shell-lb           # Shell into LB
make ps                 # Show containers

# Demo
make demo-1             # Basic fetch demo
make demo-2             # Cache performance demo
make demo-3             # Leader election demo
```

## Documentation

- [DEMO.md](DEMO.md) - Demo scenarios
- [docs/architecture.md](docs/architecture.md) - System architecture
- [docs/deployment.md](docs/deployment.md) - Deployment guide
- [infrastructure/containerlab/](infrastructure/containerlab/) - Topology config

## Project Structure

```
telco-edge-cdn/
├── services/
│   ├── origin/              # Origin server (Go)
│   ├── cache-node/          # Cache nodes (Go + W-TinyLFU)
│   └── load-balancer/       # Load balancer (Go)
├── infrastructure/
│   └── containerlab/        # Network topology
├── ml/                      # ML service
├── scripts/                 # Helper scripts
└── docs/                    # Documentation
```

## Access Points

After `make clab-up`:

- Load Balancer: http://localhost:8080
- Origin Server: http://localhost:8081
- Cache Node 1: http://localhost:8001
- Cache Node 2: http://localhost:8002
- Cache Node 3: http://localhost:8003

## Manual Testing

```bash
# Fetch video through LB
curl http://localhost:8080/cdn/wolf-1770316220/master.m3u8

# Direct cache access
curl http://localhost:8001/cache/wolf-1770316220/segment_0000.m4s

# Check latency
docker exec oulu-telco-cdn-oulu-client ping -c 3 oulu-lb
docker exec oulu-telco-cdn-oulu-oulu-cache-1 ping -c 3 origin
```

## License

MIT License
