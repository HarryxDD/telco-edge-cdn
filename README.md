# Telco-Edge CDN for Latency-Sensitive Video Streaming

A Content Delivery Network (CDN) system designed for ultra-low latency video streaming at the telecommunications network edge. This project demonstrates edge computing principles applied to video distribution, achieving faster response times through distributed caching and load balancing.

## Project Overview

A distributed video streaming system that brings content delivery to the edge of telecom networks. Unlike traditional CDNs (Cloudflare, Akamai) that operate outside ISP networks, this system deploys cache nodes **inside** the telecommunications infrastructure, dramatically reducing latency and bandwidth costs.

### Key Features

- **HLS Video Streaming**: Adaptive bitrate streaming with FFmpeg transcoding
- **Distributed Caching**: LRU cache with BadgerDB persistence
-  **Smart Load Balancing**: Bounded-load consistent hashing prevents hotspots
- **Fault Tolerance**: Automatic failover on node failure
- **High Cache Hit Ratio**: >90% cache hit rate in typical scenarios
- **Horizontal Scalability**: Add cache nodes without downtime

##  Architecture

```
┌─────────────────────────────────────────────────┐
│                   Client                        │
│              (React + HLS.js)                   │
└─────────────────────┬───────────────────────────┘
                      │ HTTP
                      ↓
┌─────────────────────────────────────────────────┐
│              Load Balancer                      │
│        Bounded-Load Consistent Hashing          │
│        Virtual Nodes: 150                       │
│        Max Load Factor: 1.25                    │
└──────────┬─────────────────────┬────────────────┘
           │                     │
    ┌──────┴──────┐       ┌──────┴──────┐
    ↓             ↓       ↓             ↓
┌─────────┐  ┌─────────┐  ┌─────────┐
│ Cache-1 │  │ Cache-2 │  │ Cache-3 │  ... Edge Nodes
│ BadgerDB│  │ BadgerDB│  │ BadgerDB│  (LRU Eviction)
│  500MB  │  │  500MB  │  │  500MB  │
└────┬────┘  └────┬────┘  └────┬────┘
     │            │            │
     └────────────┴────────────┘
                  │ (on cache miss)
                  ↓
         ┌─────────────────┐
         │  Origin Server  │
         │  FFmpeg HLS     │
         │  Transcoding    │
         └─────────────────┘
```

### Component Overview

| Component | Technology | Purpose |
|-----------|------------|---------|
| **Origin Server** | Go + FFmpeg + Gin | Video upload, HLS transcoding, master content storage |
| **Cache Node** | Go + BadgerDB | Edge caching with LRU eviction |
| **Load Balancer** | Go + Consistent Hashing | Request routing with bounded-load awareness |
| **Client** | React + TypeScript + HLS.js | Web frontend for video playback |

**Detailed Architecture**: See [docs/architecture.md](docs/architecture.md)

## Quick Start

### Prerequisites

- Docker 20.10+ and Docker Compose 2.0+
- 4GB+ RAM available
- 10GB+ disk space

### 1. Clone and Navigate

```bash
git clone https://github.com/HarryxDD/telco-edge-cdn.git
cd telco-edge-cdn/infrastructure/docker-compose
```

### 2. Start Services

```bash
docker-compose up -d --build
```

This starts:
- Origin server on port 8443
- Cache node on port 8081
- Load balancer on port 8090

### 3. Verify Services

```bash
# Check all services are healthy
curl http://localhost:8443/health  # Origin
curl http://localhost:8081/health  # Cache
curl http://localhost:8090/health  # Load Balancer
```

### 4. Upload a Video

```bash
curl -X POST http://localhost:8443/api/upload \\
  -F \"file=@./your-video.mp4\"
```

Response:
```json
{
  \"status\": \"processing\",
  \"title\": \"your-video\"
}
```

### 5. Check Video List

```bash
curl http://localhost:8443/api/videos
```

Response:
```json
[
  {
    \"title\": \"your-video\",
    \"duration\": 125.5
  }
]
```

### 6. Stream Video

Open your browser or use a video player:
```
http://localhost:8090/hls/your-video/master.m3u8
```

Or with curl:
```bash
curl http://localhost:8090/hls/your-video/master.m3u8
```

### 7. Test Cache Performance

```bash
# First request (cache miss ~50ms)
time curl -o /dev/null -s http://localhost:8090/hls/your-video/master.m3u8

# Second request (cache hit <5ms)
time curl -o /dev/null -s http://localhost:8090/hls/your-video/master.m3u8
```

## Project Structure

```
telco-cdn-video-streaming/
├── services/                    # Microservices
│   ├── origin/                 # Origin server (Go + FFmpeg)
│   │   ├── cmd/main.go        # Entry point
│   │   └── internal/          # Business logic
│   ├── cache-node/            # Edge cache (Go + BadgerDB)
│   │   ├── cmd/server/main.go
│   │   └── internal/          # Cache logic + LRU
│   ├── load-balancer/         # Load balancer (Go)
│   │   ├── cmd/main.go
│   │   └── internal/ring/     # Consistent hashing
│   └── client/                # Web frontend (React)
│       └── frontend/          # React + TypeScript + HLS.js
├── infrastructure/             # Deployment configs
│   ├── docker-compose/        # Docker Compose files
│   │   ├── docker-compose.yml
│   │   └── docker-compose.dev.yml
│   ├── kubernetes/            # K8s manifests (future)
│   └── monitoring/            # Prometheus + Grafana
├── docs/                       # Documentation
│   ├── architecture.md        # System architecture
│   ├── api.md                 # API documentation
│   ├── deployment.md          # Deployment guide
│   ├── demo.md                # Demo script
│   └── evaluation.md          # Performance evaluation
├── benchmarks/                 # Performance tests
│   ├── load-testing/          # k6 load tests
│   └── comparison/            # Benchmark comparisons
├── ml/                         # ML components (future)
└── scripts/                    # Utility scripts
```

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | System design, components, data flow |
| [API Reference](docs/api.md) | REST API endpoints and usage |
| [Deployment Guide](docs/deployment.md) | Docker, Kubernetes, cloud deployment |
| [Evaluation](docs/evaluation.md) | Performance testing methodology |
| [Origin README](services/origin/README.md) | Origin server setup and usage |
| [Cache Node README](services/cache-node/README.md) | Cache node configuration |
| [Load Balancer README](services/load-balancer/README.md) | Load balancer details |
| [Client README](services/client/README.md) | Frontend application |

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.