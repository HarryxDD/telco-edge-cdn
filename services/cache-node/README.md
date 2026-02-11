# Cache Node (Edge Cache Service)

The Cache Node is a distributed edge caching service that sits between the load balancer and origin server. It caches HLS video segments and manifests to reduce latency and origin load.

## Features

- **LRU Caching**: Implements Least Recently Used eviction policy
- **Disk-backed Storage**: Uses BadgerDB for persistent caching
- **Origin Failover**: Automatically fetches from origin on cache miss
- **Health Monitoring**: Exposes health endpoint for load balancer
- **Content Validation**: SHA-256 based cache key generation
- **TLS Support**: Can communicate with origin over HTTPS

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
│  │  - /videos/*         │  │
│  │  - /hls/*            │  │
│  └──────────┬───────────┘  │
│             │              │
│  ┌──────────┴───────────┐  │
│  │  EdgeCache Service   │  │
│  │  - LRU eviction      │  │
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

# Origin server
ORIGIN_URL=http://localhost:8443

# Storage
DB_PATH=./data/cache-cache-1

# Cache capacity in bytes (default: 500MB)
CACHE_CAPACITY=524288000
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
  "node": "cache-1"
}
```

### Serve HLS Content
```http
GET /hls/{videoId}/master.m3u8
GET /hls/{videoId}/playlist_{quality}.m3u8
GET /hls/{videoId}/segment_{quality}_{n}.m4s
HEAD /hls/{videoId}/*
```

Serves HLS content from cache or fetches from origin.

### Serve Video (Legacy Path)
```http
GET /videos/{videoId}/master.m3u8
GET /videos/{videoId}/*
HEAD /videos/{videoId}/*
```

Alternative path for video content.

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

### Cache Miss
1. Cache node detects miss
2. Fetches content from origin server
3. Stores content in BadgerDB
4. Returns content to client
5. Applies LRU eviction if cache is full

### Eviction Policy

When cache reaches capacity:
- Least Recently Used (LRU) items are evicted first
- Eviction continues until space is available
- Cache capacity is configurable via `CACHE_CAPACITY`

## Directory Structure

```
services/cache-node/
├── cmd/
│   └── server/
│       └── main.go           # Entry point
├── internal/
│   ├── api/
│   │   └── server.go         # HTTP server and handlers
│   ├── service/
│   │   └── cache.go          # EdgeCache implementation
│   └── lru/
│       └── lru.go            # LRU eviction logic
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
- **Client**: Web frontend for video playback

## References

- [LRU Cache Implementation in Go (Medium)](https://medium.com/@dinesht.bits/implementing-lru-cache-using-golang-7dcea5c3f054)
- [LRU Cache in Go (dev.to)](https://dev.to/johnscode/implement-an-lru-cache-in-go-1hbc)
- [BadgerDB Documentation](https://dgraph.io/docs/badger/)
- [HLS Caching Best Practices](https://www.cloudflare.com/learning/video/what-is-http-live-streaming/)
