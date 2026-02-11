# Infrastructure

This directory contains all infrastructure-related configurations for deploying the Telco-Edge CDN system.

## Directory Structure

```
infrastructure/
├── docker-compose/       # Docker Compose configurations
│   ├── docker-compose.yml      # Main production configuration
│   └── docker-compose.dev.yml  # Development overrides
├── containerlab/         # ContainerLab network topology
│   ├── topology.yml
│   └── configs/
└── monitoring/           # Monitoring stack
    ├── prometheus/       # Prometheus configuration
    ├── grafana/         # Grafana dashboards
    └── loki/            # Loki logging
```

## Quick Start with Docker Compose

### Prerequisites

- Docker and Docker Compose installed
- Go 1.21+ (for building services)
- FFmpeg (for video transcoding in origin service)

### Running the System

1. **Navigate to the docker-compose directory:**
   ```bash
   cd infrastructure/docker-compose
   ```

2. **Start all services:**
   ```bash
   docker-compose up --build
   ```

   Or run in detached mode:
   ```bash
   docker-compose up -d --build
   ```

3. **Verify services are running:**
   ```bash
   docker-compose ps
   ```

### Service Endpoints

Once running, the following services will be available:

- **Origin Server**: `http://localhost:8443`
  - Upload API: `POST http://localhost:8443/api/upload`
  - Videos API: `GET http://localhost:8443/api/videos`
  - HLS Endpoint: `http://localhost:8443/hls/{videoId}/master.m3u8`

- **Cache Node**: `http://localhost:8081`
  - Health Check: `GET http://localhost:8081/health`
  - Video Proxy: `http://localhost:8081/videos/{videoId}/*`

- **Load Balancer**: `http://localhost:8090`
  - Health Check: `GET http://localhost:8090/health`
  - CDN Entry Point: `http://localhost:8090/hls/{videoId}/*`

### Configuration

Environment variables can be configured in service-specific `.env` files:
- `services/origin/.env`
- `services/cache-node/.env`
- `services/load-balancer/.env`

### Common Commands

**Stop all services:**
```bash
docker-compose down
```

**Stop and remove volumes:**
```bash
docker-compose down -v
```

**View logs:**
```bash
docker-compose logs -f [service-name]
```

**Rebuild specific service:**
```bash
docker-compose up -d --build origin
```

## Monitoring

The monitoring stack includes:
- **Prometheus**: Metrics collection and storage
- **Grafana**: Visualization and dashboards
- **Loki**: Log aggregation

See `monitoring/` directory for configuration details.

## Network Topology

For advanced network simulation with ContainerLab, see `containerlab/README.md`.

## Development

For development with hot-reload and additional debugging tools, use:

```bash
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up
```

