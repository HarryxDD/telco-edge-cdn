# Load Balancer

The Load Balancer is a smart request router that distributes incoming CDN requests across multiple cache nodes using bounded-load consistent hashing. It monitors node health and adapts to failures.

## Features

- **Bounded-Load Consistent Hashing**: Distributes load evenly while maintaining cache affinity
- **Health Monitoring**: Automatic health checks for all cache nodes
- **Failover**: Removes unhealthy nodes from rotation
- **Content-aware Routing**: Routes based on video ID for cache efficiency
- **Multi-protocol Support**: Handles /hls/*, /videos/*, and /api/* paths

## Architecture

```
┌─────────────┐
│   Clients   │
└──────┬──────┘
       │
       ↓
┌────────────────────────────────────┐
│        Load Balancer               │
│                                    │
│  │  API Server          │  │
│  │  - /health           │  │
│  │  - /debug/ring       │  │
│  │  - /api/* (Proxy)    │  │
│  │  - /hls/* (Route)    │  │
│  └──────────┬───────────┘  │
│             │              │
│  ┌──────────┴───────────┐  │
│  │  Bounded-Load Ring   │  │
│  │  - Consistent hashing│  │
│  │  - Rate Calculation  │  │
│  └──────────┬───────────┘  │
│             │              │
│  ┌──────────┴───────────┐  │
│  │  Health Checker      │  │
│  │  - Periodic polls    │  │
│  │  - Node state manage │  │
│  └──────────────────────┘  │
└────────────────────────────┘
       │
       ├─────────┬─────────┬─────────┐
       ↓         ↓         ↓         ↓
  ┌────────┐ ┌────────┐ ┌────────┐ ...
  │Cache-1 │ │Cache-2 │ │Cache-3 │
  └────────┘ └────────┘ └────────┘
```

## Configuration

Configure via environment variables or `.env` file:

```env
# Server configuration
PORT=8090

# Cache nodes (comma-separated)
# Format: nodeId:url,nodeId:url,...
CACHE_NODES=cache-1:http://localhost:8081,cache-2:http://localhost:8082,cache-3:http://localhost:8083
```

## Running Locally

### Prerequisites

- Go 1.25+
- At least one cache node running

### Build and Run

From the `services/load-balancer` directory:

```bash
# Install dependencies
go mod download

# Run the load balancer
go run cmd/main.go
```

The server will start on port 8090 (or the port specified in `PORT`).

## API Endpoints

### Health Check
```http
GET /health
```

Returns load balancer health.

**Response:**
```json
{
  "status": "healthy"
}
```

### Debug Hash Ring
```http
GET /debug/ring
```

Dumps current health and load status of all tracked virtual nodes.

### Video Streaming (HLS)
```http
GET /hls/{videoId}/*path
```

Routes HLS requests to appropriate cache node cleanly.

### API Proxy
```http
GET /api/videos
POST /api/upload
```

Proxies API requests to cache nodes (which forward to origin).

## Routing Algorithm

### Bounded-Load Consistent Hashing

1. **Hash Calculation**: Generates `fnv.New32a()` hash from request path (`/hls/video-123/master.m3u8`)
2. **Node Selection**: Maps hash to a cache node via binary search of virtual ring
3. **Load Check**: Calculates moving average request rate in 10-second windows and verifies node load is below `maxLoadFactor × averageRate`
4. **Health Check**: Ensures selected node is healthy
5. **Fallback**: If primary node is overloaded, tries next node in ring until an acceptable rate is met (or selects the absolute least loaded node globally)

### Parameters

- **Virtual Nodes**: Default 150 (increases distribution uniformity)
- **Max Load Factor**: Default 1.25 (allows 25% above average load per 10-second window)

### Benefits

- **Cache Affinity**: Same video always routes to same cache node (higher hit ratio)
- **Load Balancing**: Prevents hot spots by enforcing load bounds
- **Fault Tolerance**: Automatically removes unhealthy nodes
- **Scalability**: Add/remove nodes without full rehashing

## Health Monitoring

### Health Check Mechanism

- **Interval**: Every 5 seconds
- **Endpoint**: `GET /health` on each registered cache node
- **Timeout**: 5 seconds
- **Action**: Marks node `Healthy: false` on connection refused or non-200 responses

### Node States

- **Healthy**: Node responds to health checks
- **Unhealthy**: Node fails health check or times out

Unhealthy nodes are excluded from routing until they recover.

## Load Tracking

### Request Flow

1. Load balancer increments node's load counter
2. Request is forwarded to cache node
3. Response is returned to client
4. Load counter is decremented

### Load Balancing

If a node's load exceeds `maxLoadFactor × avgLoad`:
- Request is routed to next available node
- Maintains cache affinity when possible
- Prevents cascading overload

## Directory Structure

```
services/load-balancer/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── proxy/
│   │   └── server.go        # HTTP proxy server
│   ├── ring/
│   │   └── ring.go          # Bounded-load hash ring
│   └── health/
│       └── checker.go       # Health monitoring (if exists)
├── Dockerfile
├── go.mod
└── README.md
```

## Performance Tuning

### Virtual Nodes

Increase for better distribution:
```go
NewBoundedLoadHashRing(300, 1.25)  // More uniform distribution
```

Decrease for faster lookups:
```go
NewBoundedLoadHashRing(50, 1.25)   // Faster, less uniform
```

### Max Load Factor

Allow more imbalance:
```go
NewBoundedLoadHashRing(150, 2.0)   // More imbalance tolerance
```

Enforce stricter balance:
```go
NewBoundedLoadHashRing(150, 1.1)   // Stricter load distribution
```

## Monitoring

Key metrics to monitor:
- Request distribution across nodes
- Node health status
- Load per node
- Failover events
- Response latencies

Logs include:
```
Added cache node: cache-1 -> http://localhost:8081
[Health Check] cache-1: healthy
[Health Check] cache-2: unhealthy (connection refused)
```

## Related Services

- **Cache Nodes**: Backend servers that cache video content
- **Origin Server**: Ultimate source of video content
- **Client**: Web frontend for video playback

## References

- [Consistent Hashing](https://en.wikipedia.org/wiki/Consistent_hashing)
- [Bounded-Load Hashing Paper](https://ai.googleblog.com/2017/04/consistent-hashing-with-bounded-loads.html)
- [Load Balancing Algorithms](https://www.nginx.com/blog/choosing-nginx-plus-load-balancing-techniques/)
