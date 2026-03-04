# Deployment Guide

This guide covers deploying the Telco-Edge CDN system in various environments.

## Table of Contents

- [Quick Start (Docker Compose)](#quick-start-docker-compose)
- [Development Setup](#development-setup)
- [Production Deployment](#production-deployment)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)

---

## Quick Start (Containerlab)

The recommended deployment method for the edge topology (MEC Oulu) is using [Containerlab](https://containerlab.dev/), which allows us to simulate realistic network latencies and topologies.

### Prerequisites

- Linux host (Containerlab requires a Linux kernel)
- Docker 20.10+
- `sudo` privileges
- 8GB+ available RAM
- 20GB+ available disk space

### Deployment Steps

1. **Clone the repository:**
   ```bash
   git clone https://github.com/HarryxDD/telco-edge-cdn.git
   cd telco-edge-cdn
   ```

2. **Install Containerlab (if not already installed):**
   ```bash
   make clab-install
   ```

3. **Build the Docker images:**
   ```bash
   make build-all
   ```
   *This builds the Origin, Cache, Load Balancer, ML Aggregator, and FL Client images, and creates the required data directories.*

4. **Deploy the MEC Oulu Topology:**
   ```bash
   make clab-up
   ```
   *This deploys the topology defined in `infrastructure/containerlab/topology.yml` and automatically applies network latencies (e.g., 200ms to the origin server).*

5. **Verify the deployment:**
   ```bash
   make status
   ```
   *This checks the health of the core services, Cache coordination, ML Aggregator, and Monitoring stack.*

6. **Access the system:**
   - Load Balancer / API: `http://localhost:8080`
   - Origin Server: `http://localhost:8081`
   - ML Aggregator: `http://localhost:8092`
   - Cache-1: `http://localhost:8001`
   - Cache-2: `http://localhost:8002`
   - Cache-3: `http://localhost:8003`
   - Prometheus: `http://localhost:9090`
   - Grafana: `http://localhost:3000`

### Manage Network Latencies

The `clab-up` command automatically applies network latencies based on `scripts/apply-latency.sh`. You can manage these parameters during runtime:

```bash
# Apply programmed network latencies
make network-apply

# Verify current network impairments
make network-show

# Remove all network latencies
make network-remove
```

### Stop Services

```bash
# Destroy the Containerlab topology
make clab-down

# Full cleanup (destroys topology and removes all related Docker containers/networks)
make clean-all
```

---

## Development Setup

### Local Development (Without Docker)

#### Prerequisites

- Go 1.21+
- Node.js 18+
- FFmpeg 4.0+
- Git

#### 1. Setup Origin Server

```bash
cd services/origin

# Create .env file
cat > .env << EOF
ADDR=:8443
USE_TLS=false
UPLOADS_DIR=uploads
HLS_DIR=hls
VIDEOS_PATH=videos.json
EOF

# Install dependencies
go mod download

# Run server
go run cmd/main.go
```

#### 2. Setup Cache Node

```bash
cd services/cache-node

# Create .env file
cat > .env << EOF
NODE_ID=cache-1
PORT=8081
ORIGIN_URL=http://localhost:8443
DB_PATH=./data/cache-1
CACHE_CAPACITY=524288000
EOF

# Install dependencies
go mod download

# Run server
go run cmd/server/main.go
```

#### 3. Setup Load Balancer

```bash
cd services/load-balancer

# Create .env file
cat > .env << EOF
PORT=8090
CACHE_NODES=cache-1:http://localhost:8081
EOF

# Install dependencies
go mod download

# Run server
go run cmd/main.go
```

#### 4. Setup Client (Optional)

```bash
cd services/client/frontend

# Install dependencies
npm install

# Start dev server
npm run dev
```

Access at `http://localhost:5173`

### Running Multiple Cache Nodes

Terminal 1:
```bash
NODE_ID=cache-1 PORT=8081 DB_PATH=./data/cache-1 \
  go run services/cache-node/cmd/server/main.go
```

Terminal 2:
```bash
NODE_ID=cache-2 PORT=8082 DB_PATH=./data/cache-2 \
  go run services/cache-node/cmd/server/main.go
```

Terminal 3:
```bash
NODE_ID=cache-3 PORT=8083 DB_PATH=./data/cache-3 \
  go run services/cache-node/cmd/server/main.go
```

Terminal 4 (Load Balancer):
```bash
CACHE_NODES=cache-1:http://localhost:8081,cache-2:http://localhost:8082,cache-3:http://localhost:8083 \
  go run services/load-balancer/cmd/main.go
```

---

## Network Topology Details

Our system is structured into two main tiers defined in `infrastructure/containerlab/topology.yml`:

### Cloud Tier (finland-central)
- **Origin Server (`172.26.26.10`)**: Transcoding and video database.
- **ML Aggregator (`172.26.26.11`)**: Centralized Federated Learning coordinator.
- **Monitoring**: Prometheus (`172.26.26.12`) and Grafana (`172.26.26.13`).

### Edge Tier (mec-oulu)
- **Load Balancer (`172.26.26.20`)**: Distributed request routing.
- **Cache Cluster (`172.26.26.21-23`)**: 3 edge cache nodes simulating the MEC facilities.
- **FL Clients (`172.26.26.31-33`)**: 3 lightweight sidecar clients corresponding to each cache node for local ML training. 

*Network links and delays are configured explicitly via the `apply-latency.sh` script to simulate the distance between the geographical sites.*

## Configuration

### Environment Variables

#### Origin Server

```env
# Server
ADDR=:8443
USE_TLS=false

# Storage
UPLOADS_DIR=/app/uploads
HLS_DIR=/app/hls
VIDEOS_PATH=/app/videos.json

# TLS (if USE_TLS=true)
TLS_CERT_FILE=/certs/server.crt
TLS_KEY_FILE=/certs/server.key
```

#### Cache Node

```env
# Identity
NODE_ID=cache-1

# Server
PORT=8081

# Origin
ORIGIN_URL=http://origin:8443

# Storage
DB_PATH=/app/data/cache
CACHE_CAPACITY=524288000  # 500MB in bytes
```

#### Load Balancer

```env
# Server
PORT=8090

# Cache nodes (comma-separated)
CACHE_NODES=cache-1:http://cache-1:8081,cache-2:http://cache-2:8081,cache-3:http://cache-3:8081
```

---

## Monitoring

### Prometheus + Grafana

The monitoring stack launches automatically with `make clab-up`.

- **Access Grafana**: `http://localhost:3000`
- **Default Credentials**: admin/admin (or bypass login depending on configuration in Docker Hub image)

### Metrics to Monitor

**Origin Server:**
- Upload rate
- Transcoding queue length
- Storage usage
- Request latency

**Cache Nodes:**
- Cache hit ratio
- Cache size
- Eviction rate
- Request latency

**Load Balancer:**
- Requests per node
- Node health status
- Load distribution
- Failover events

---

### Generate TLS Certificates

**Self-signed (Development):**
```bash
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout server.key -out server.crt \
  -days 365 -subj "/CN=localhost"
```

**Let's Encrypt (Production):**
```bash
certbot certonly --standalone \
  -d cdn.example.com \
  --email admin@example.com
```

---

## References

- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [AWS ECS Best Practices](https://docs.aws.amazon.com/AmazonECS/latest/bestpracticesguide/intro.html)
- [Production-Ready Microservices (Susan Fowler)](https://www.oreilly.com/library/view/production-ready-microservices/9781491965962/)
