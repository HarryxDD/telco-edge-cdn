# Deployment Guide

This guide covers deploying the Telco-Edge CDN system in various environments.

## Table of Contents

- [Quick Start (Docker Compose)](#quick-start-docker-compose)
- [Development Setup](#development-setup)
- [Production Deployment](#production-deployment)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)

---

## Quick Start (Docker Compose)

### Prerequisites

- Docker 20.10+
- Docker Compose 2.0+
- 4GB+ available RAM
- 10GB+ available disk space

### Deployment Steps

1. **Clone the repository:**
   ```bash
   git clone https://github.com/HarryxDD/telco-edge-cdn.git
   cd telco-edge-cdn
   ```

2. **Navigate to infrastructure directory:**
   ```bash
   cd infrastructure/docker-compose
   ```

3. **Start all services:**
   ```bash
   docker-compose up -d --build
   ```

4. **Verify services are running:**
   ```bash
   docker-compose ps
   ```

   Expected output:
   ```
   NAME              STATUS   PORTS
   telco-origin      Up       0.0.0.0:8443->8443/tcp
   telco-cache-1     Up       0.0.0.0:8081->8081/tcp
   telco-lb          Up       0.0.0.0:8090->8090/tcp
   ```

5. **Check service health:**
   ```bash
   curl http://localhost:8443/health  # Origin
   curl http://localhost:8081/health  # Cache-1
   curl http://localhost:8090/health  # Load Balancer
   ```

6. **Upload a test video:**
   ```bash
   curl -X POST http://localhost:8443/api/upload \
     -F "file=@./test-video.mp4"
   ```

7. **Access the system:**
   - API: `http://localhost:8090`
   - Origin: `http://localhost:8443`
   - Cache Node: `http://localhost:8081`

### Stop Services

```bash
docker-compose down

# Remove volumes (deletes cached data)
docker-compose down -v
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

## Production Deployment (WIP)

---

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

## Monitoring (WIP)

### Prometheus + Grafana

```bash
cd infrastructure/monitoring

# Start monitoring stack
docker-compose up -d

# Access Grafana
open http://localhost:3000
# Default: admin/admin
```

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
