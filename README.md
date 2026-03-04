# Telco-Edge CDN for Latency-Sensitive Video Streaming

A Content Delivery Network (CDN) system designed for ultra-low latency video streaming at the telecommunications network edge. This project demonstrates edge computing principles applied to video distribution, achieving faster response times through distributed caching and load balancing.

## Project Overview

A distributed video streaming system that brings content delivery to the edge of telecom networks. Unlike traditional CDNs (Cloudflare, Akamai) that operate outside ISP networks, this system deploys cache nodes **inside** the telecommunications infrastructure, dramatically reducing latency and bandwidth costs.

### Key Features

- **HLS Video Streaming**: Adaptive bitrate streaming with FFmpeg transcoding
- **Distributed Caching**: W-TinyLFU cache admission with BadgerDB persistence
- **Smart Load Balancing**: Bounded-load consistent hashing prevents hotspots
- **Fault Tolerance**: Automatic failover, Bully Leader Election, and Gossiping
- **Federated Learning**: On-device model training via XGBoost and Ray to predict edge cache behavior
- **MEC Simulation**: Realistic 5G/MEC topology simulation using Nokia's Containerlab

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
│ BadgerDB│  │ BadgerDB│  │ BadgerDB│  (W-TinyLFU, Gossip
│         │  │         │  │         │   & Leader Election)
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
| **Cache Node** | Go + BadgerDB + W-TinyLFU | Edge caching, Gossip syncing, Bully leader election |
| **Load Balancer** | Go + Consistent Hashing | Request routing with bounded-load awareness |
| **Client** | React + TypeScript + HLS.js | Web frontend for video playback & dashboard |
| **ML Edge** | Python + Ray + XGBoost | Federated Learning model training at the edge |
| **Monitoring** | Prometheus + Grafana | Cluster observability & metric collection |

**Detailed Architecture**: See [docs/architecture.md](docs/architecture.md)

## Quick Start

### Prerequisites

- Docker 20.10+ and Docker Compose 2.0+
- Containerlab (`sudo bash -c "$(curl -sL https://get.containerlab.dev)"`)
- GNU Make
- 4GB+ RAM available

### 1. Clone and Build

```bash
git clone https://github.com/HarryxDD/telco-edge-cdn.git
cd telco-edge-cdn
make build-all
```

### 2. Deploy MEC Topology

```bash
make clab-up
```

This simulates the MEC Oulu topology and starts:
- Origin Server (Cloud Tier)
- 3 Cache Nodes (Edge Tier)
- Load Balancer
- ML Aggregator and FL Clients
- Prometheus & Grafana Monitoring

### 3. Verify Services

```bash
# Check all services are healthy through Load Balancer proxy
curl http://clab-mec-oulu-lb:8090/health
```

### 4. Upload a Video

```bash
curl -X POST http://clab-mec-oulu-lb:8090/api/upload \
  -F "file=@./your-video.mp4"
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
curl http://clab-mec-oulu-lb:8090/api/videos
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

Open the React Client in your browser (default `http://localhost:5173` typically if run locally) or use a video player directly:
```
http://clab-mec-oulu-lb:8090/hls/your-video/master.m3u8
```

### 7. Clean up

To tear down the Containerlab deployment:
```bash
make clab-down
```

## Project Structure

```
telco-cdn-video-streaming/
├── services/                    # Microservices
│   ├── origin/                 # Origin server (Go + FFmpeg)
│   │   ├── cmd/main.go        # Entry point
│   │   └── internal/          # Business logic
│   ├── cache-node/            # Edge cache (Go + BadgerDB + W-TinyLFU)
│   │   ├── cmd/server/main.go
│   │   └── internal/          # Coordination, Gossip, Election
│   ├── load-balancer/         # Load balancer (Go)
│   │   ├── cmd/main.go
│   │   └── internal/ring/     # Bounded-load Consistent hashing
│   ├── client/                # Web frontend (React)
│   │   └── frontend/          # React + TypeScript + HLS.js
│   └── ml/                    # Machine Learning Services
│       ├── aggregator/        # Ray Cluster Head (Global Model)
│       └── fl-client/         # Ray Workers (Local Edge Models)
├── infrastructure/             # Deployment configs
│   ├── topologies/            # Containerlab YAML topologies
│   ├── monitoring/            # Prometheus + Grafana configs
│   └── docker/                # Service Dockerfiles
├── docs/                       # Documentation
│   ├── architecture.md        # System architecture
│   ├── api.md                 # API documentation
│   ├── deployment.md          # Deployment guide
│   ├── demo.md                # Demo script
│   └── evaluation.md          # Performance evaluation
├── benchmarks/                 # Performance tests
│   ├── load-testing/          # k6 load testing scripts
│   ├── dataset/               # Generated test data
│   └── python/                # Analytical chart scripts
├── data/                       # Evaluation & Metric outputs
└── scripts/                    # Makefile utility scripts
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