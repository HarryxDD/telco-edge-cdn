# Cache Node (Edge Cache Service)

The Cache Node is a distributed edge caching service that sits between the load balancer and origin server. It caches HLS video segments and manifests to reduce latency and origin load.

## Features

- **W-TinyLFU Caching**: Advanced cache eviction policy (Window TinyLFU) for high hit rates
- **Gossip Protocol**: Epidemic state synchronization across the cache cluster
- **Leader Election**: Automated Bully Algorithm to select a coordinator
- **Disk-backed Storage**: Uses BadgerDB for persistent caching
- **Origin Failover**: Automatically fetches from origin on cache miss
- **Metrics Export**: Exposes Prometheus metrics for observability
- **Content Validation**: SHA-256 based cache key generation

## Architecture

```
┌──────────────┐
│Load Balancer │
└──────┬───────┘
       │
       ↓
┌────────────────────────────┐
│      Cache Node            │
│                            │
│  ┌──────────────────────┐  │
│  │  API Server          │  │
│  │  - /health           │  │
│  │  - /metrics          │  │
│  │  - /coordination/*   │  │
│  │  - /hls/*            │  │
│  └──────────┬───────────┘  │
│             │              │
│  ┌──────────┴───────────┐  │
│  │  Coordinator Layer   │  │
│  │  - Leader Election   │  │
│  │  - Gossip Protocol   │  │
│  └──────────┬───────────┘  │
│             │              │
│  ┌──────────┴───────────┐  │
│  │  EdgeCache Service   │  │
│  │  - W-TinyLFU evict   │  │
│  │  - Origin fetcher    │  │
│  └──────────┬───────────┘  │
│             │              │
│  ┌──────────┴───────────┐  │
│  │  BadgerDB (Disk)     │  │
│  │  - Key-value store   │  │
│  └──────────────────────┘  │
└────────────────────────────┘
       │
       ↓ (on cache miss)
┌──────────────┐
│Origin Server │
└──────────────┘
```

## Configuration

Configure via environment variables or `.env` file:

```env
# Node identification
NODE_ID=cache-1

# Server configuration
PORT=8081
ADDRESS=localhost

# Cluster configuration
CLUSTER_NODES=cache-1:localhost:8081,cache-2:localhost:8082,cache-3:localhost:8083

# Origin server
ORIGIN_URL=http://localhost:8443

# Storage
DB_PATH=./data/cache-cache-1

# Cache capacity in number of items
CACHE_CAPACITY=1000

# Logging
LOG_DIR=/app/logs
```

## Running Locally

### Prerequisites

- Go 1.25+
- Origin server running

### Build and Run

From the `services/cache-node` directory:

```bash
# Install dependencies
go mod download

# Run the cache node
go run cmd/server/main.go
```

The server will start on port 8081 (or the port specified in `PORT`).

## API Endpoints

### Health Check
```http
GET /health
```

Returns node health and ID.

**Response:**
```json
{
  "status": "healthy",
  "node": "cache-1",
  "is_leader": true
}
```

### Prometheus Metrics
```http
GET /metrics
```

Returns Prometheus-formatted metrics (hit/miss counters, active sessions, rebuffering events, bytes served, etc).

### Coordination Endpoints
```http
GET  /coordination/status
POST /coordination/request-lock
POST /coordination/release-lock
```

Internal cluster communication for leader lock management and state introspection.

### Serve HLS Content
```http
GET /hls/{videoId}/master.m3u8
GET /hls/{videoId}/playlist_{quality}.m3u8
GET /hls/{videoId}/segment_{quality}_{n}.m4s
HEAD /hls/{videoId}/*
```

Serves HLS content from cache or fetches from origin.

Serves HLS content from cache or fetches from origin. Records access analytics locally.

### Proxy Video List
```http
GET /api/videos
```

Proxies the video list request to the origin server.

## Caching Behavior

### Cache Hit
1. Client requests video segment
2. Cache node generates SHA-256 hash of the request path
3. Checks BadgerDB for cached data
4. Returns cached data with appropriate Content-Type header

### Cache Miss & Stampede Protection
1. Cache node detects miss
2. Uses standard cluster coordination to ask Leader for fetch lock
3. Fetches content from origin server (or peer if someone else downloaded it via gossip)
4. Stores content in BadgerDB
5. Returns content to client
6. May trigger W-TinyLFU eviction if maximum segments are reached

### Eviction Policy (W-TinyLFU)

When cache reaches segment capacity (`CACHE_CAPACITY`):
- Uses access frequencies via Count-Min Sketch.
- Small Window Cache accepts bursty newly-admitted segments.
- Main Cache operates SLRU (Segmented LRU) evaluating Frequency (LFU) against Recency (LRU).
- Safely evicts stale segments to free up constraints.

## Directory Structure

```
services/cache-node/
├── cmd/
│   └── server/
│       └── main.go           # Entry point
├── internal/
│   ├── api/
│   │   └── server.go         # HTTP server and handlers
│   ├── coordination/         # Orchestrates cluster features
│   ├── election/             # Bully Election algorithm
│   ├── gossip/               # Epidemic state syncing
│   ├── metrics/              # Prometheus Prometheus integration
│   ├── logging/              # Local access NDJSON logging
│   ├── service/
│   │   └── cache.go          # EdgeCache implementation
│   └── wtinylfu/
│       └── wtinylfu.go       # Frequency/Recency eviction algorithm
├── data/                     # Cache storage (created at runtime)
├── Dockerfile
├── go.mod
└── README.md
```

## Performance Considerations

### Cache Hit Ratio
- Monitor logs for `[CACHE HIT]` vs `[CACHE MISS]` messages
- Higher hit ratio = better performance
- Typical production systems aim for >90% hit ratio

### Storage Management
- BadgerDB automatically manages disk I/O
- Default capacity: 500MB (configurable)
- Consider SSD storage for better performance

### Network Optimization
- Keep cache nodes close to users (edge deployment)
- Use persistent connections to origin
- Enable HTTP/2 for multiplexing

## Monitoring

Key metrics to monitor:
- Cache hit/miss ratio
- Response latency
- Cache capacity utilization
- Origin fetch errors
- Node health status

Logs include:
```
[cache-1] CACHE HIT: /videos/video-name/master.m3u8
[cache-1] CACHE MISS: /videos/video-name/segment_720p_0.m4s
```

## Scaling

Multiple cache nodes can run in parallel:

```bash
# Node 1
NODE_ID=cache-1 PORT=8081 DB_PATH=./data/cache-1 go run cmd/server/main.go

# Node 2
NODE_ID=cache-2 PORT=8082 DB_PATH=./data/cache-2 go run cmd/server/main.go

# Node 3
NODE_ID=cache-3 PORT=8083 DB_PATH=./data/cache-3 go run cmd/server/main.go
```

The load balancer distributes requests across all healthy nodes.

## Related Services

- **Origin Server**: Provides source video content
- **Load Balancer**: Distributes requests to cache nodes
- **Federated Learning Orchestrator / Metric Syncing**: The NDJSON logs produced heavily feed into edge-machine-learning workflows locally.

## References

- [BadgerDB Documentation](https://dgraph.io/docs/badger/)
- [TinyLFU: A Highly Efficient Cache Admission Policy](https://arxiv.org/abs/1512.00727)
