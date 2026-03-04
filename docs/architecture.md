# Architecture Documentation

## System Overview

The Telco-Edge CDN is a distributed video streaming system designed for ultra-low latency delivery. It leverages edge computing principles to cache video content close to end users, reducing latency compared to traditional CDN.

## High-Level Architecture

```
┌─────────────────────────────────────────────────┐
│                   Client                        │
│              (React + HLS.js)                   │
└─────────────────────┬───────────────────────────┘
                      │ HTTP
                      ↓
┌─────────────────────────────────────────────────┐
│              Load Balancer (C1)                 │
│        Bounded-Load Consistent Hashing          │
└──────────┬──────────────────────┬───────────────┘
           │                      │
    ┌──────┴──────┐       ┌──────┴──────┐
    ↓             ↓       ↓             ↓
┌─────────┐  ┌─────────┐  ┌─────────┐
│ Cache-1 │  │ Cache-2 │  │ Cache-3 │  ... Edge Nodes
└────┬────┘  └────┬────┘  └────┬────┘
     │            │            │
     └────────────┴────────────┘
                  │ (on cache miss)
                  ↓
         ┌─────────────────┐
         │  Origin Server  │
         │  (HLS Encoding) │
         └─────────────────┘
```

## Components

### 1. Origin Server

**Purpose:** Central content repository and transcoding service

**Technology:**
- Language: Go
- Web Framework: Gin
- Video Processing: FFmpeg
- Storage: Local filesystem
- Protocol: HTTP/HTTPS

**Responsibilities:**
- Accept video uploads via REST API
- Transcode videos to HLS format (multiple quality levels)
- Store master copies of all video content
- Serve video segments on cache miss
- Maintain video metadata catalog

**Key Features:**
- Asynchronous transcoding
- Multi-bitrate HLS generation (360p, 480p, 720p, 1080p)
- RESTful API for video management
- Health monitoring endpoint

**Storage Structure:**
```
origin/
├── uploads/          # Original uploaded videos
├── hls/             # Transcoded HLS content
│   └── {videoId}/
│       ├── master.m3u8
│       ├── playlist_360p.m3u8
│       ├── playlist_720p.m3u8
│       └── segment_*.m4s
└── videos.json      # Video metadata
```

### 2. Cache Node (Edge Cache)

**Purpose:** Distributed caching layer at the network edge

**Technology:**
- Language: Go
- Storage: BadgerDB (embedded key-value store)
- Eviction Policy: LRU (Least Recently Used)
- Protocol: HTTP

**Responsibilities:**
- Cache video segments and manifests
- Serve cached content with <5ms latency
- Forward cache misses to origin
- Implement LRU eviction when capacity reached
- Health status reporting

**Key Features:**
- Disk-backed persistent cache
- SHA-256 content hashing
- Automatic cache warming
- Configurable capacity limits
- Origin failover on errors

**Cache Architecture:**
```
Cache Node
├── API Server (Gin)
│   ├── /health
│   ├── /hls/{videoId}/*
│   └── /videos/{videoId}/*
│
├── EdgeCache Service
│   ├── Get(key) → data | miss
│   ├── Put(key, data) → evict if full
│   └── FetchFromOrigin(path) → data
│
└── BadgerDB
    └── Key-Value Store (on disk)
```

### 3. Load Balancer

**Purpose:** Intelligent request routing with load awareness

**Technology:**
- Language: Go
- Algorithm: Bounded-Load Consistent Hashing
- Health Checks: HTTP polling

**Responsibilities:**
- Distribute requests across cache nodes
- Maintain cache affinity (same video → same node)
- Monitor cache node health
- Automatic failover on node failure
- Load tracking and balancing

**Routing Algorithm:**

**Bounded-Load Consistent Hashing** balances two goals:
1. **Consistency:** Route same video to same cache node (cache efficiency)
2. **Balance:** Prevent overloading any single node

**Algorithm Steps:**
```
1. Hash the request path (e.g., /hls/video-123/segment_0.m4s)
2. Map hash to cache node using consistent hashing ring
3. Check if selected node's load < maxLoadFactor × avgLoad
4. If yes: route to that node
5. If no: try next node in ring
6. Fallback: route to least loaded healthy node
```

**Parameters:**
- Virtual Nodes: 150 (for uniform distribution)
- Max Load Factor: 1.25 (allows 25% above average)
- Health Check Interval: 30 seconds

**Benefits:**
- High cache hit ratio (same video always goes to same cache)
- Prevents hotspots (bounded load constraint)
- Fast failover (automatic unhealthy node exclusion)
- Scalable (add/remove nodes without full rehash)

### 4. Client (Frontend)

**Purpose:** User interface for video browsing and playback

**Technology:**
- Framework: React 19
- Language: TypeScript
- Build Tool: Vite
- Streaming: HLS.js
- Styling: CSS

**Responsibilities:**
- Display available videos
- Stream video using HLS
- Adaptive bitrate selection
- User interaction handling

**Key Features:**
- Responsive design
- Automatic quality adaptation
- Real-time video catalog updates
- Browser-native video controls

## Data Flow

### Video Upload Flow

```
1. User uploads video to origin
   POST /api/upload
   └─> Origin receives file

2. Origin saves raw video
   └─> uploads/video-name.mp4

3. Origin starts async transcoding
   └─> FFmpeg generates HLS
       ├─> hls/video-name/master.m3u8
       ├─> hls/video-name/playlist_*.m3u8
       └─> hls/video-name/segment_*.m4s

4. Origin updates catalog
   └─> videos.json += {title, duration}

5. Client can now stream video
```

### Video Streaming Flow

```
1. Client requests video list
   GET /api/videos
   └─> Load Balancer
       └─> Cache Node
           └─> Origin (proxy)
               └─> Returns video catalog

2. User selects video
   └─> VideoPlayer loads /hls/video-name/master.m3u8

3. HLS.js parses master playlist
   └─> Selects quality level
       └─> Loads playlist_720p.m3u8

4. HLS.js requests segments sequentially
   GET /hls/video-name/segment_720p_0.m4s
   └─> Load Balancer
       └─> Hash(video-name) → Cache Node 2
           └─> Cache Node 2 checks cache
               ├─> [HIT] Return from BadgerDB (fast)
               └─> [MISS] Fetch from origin
                   └─> Save to BadgerDB
                   └─> Return to client

5. Playback continues
   └─> Subsequent segments likely cached (high hit ratio)
```

### Cache Miss Flow

```
Client Request
    ↓
Load Balancer
    ↓
Cache Node (MISS)
    ↓
1. Cache generates request path
2. Makes HTTP request to Origin
3. Receives video segment
4. Stores in BadgerDB
5. Returns to client
    ↓
Client receives video segment
```

### Cache Eviction Flow

```
Cache Full Event
    ↓
LRU Eviction
1. Sort cached items by last access time
2. Remove oldest item
3. Repeat until space available
    ↓
New item stored
```

## Scalability Considerations

### Horizontal Scaling

**Cache Nodes:**
- Add more nodes to handle increased traffic
- Load balancer automatically distributes load
- Each node operates independently
- Linear scalability (2x nodes → 2x capacity)

**Origin Server:**
- Currently single instance
- Can be scaled with:
  - Read replicas for segment serving
  - Separate transcoding workers
  - Distributed storage (S3, Ceph)

### Vertical Scaling

**Cache Node Capacity:**
- Increase `CACHE_CAPACITY` for larger cache
- Use faster storage (NVMe SSD)
- More RAM for BadgerDB caching

**Origin Throughput:**
- More CPU cores for FFmpeg parallelization
- GPU acceleration for transcoding
- Faster disk for segment I/O

### Health Checks

**Endpoints:**
- `GET /health` on all services
- Returns: `{"status": "healthy", "node": "cache-1"}`

**Monitoring:**
- Prometheus scrapes health endpoints
- Alerts on consecutive failures
- Grafana visualizes status

## References

- [HTTP Live Streaming RFC 8216](https://datatracker.ietf.org/doc/html/rfc8216)
- [Consistent Hashing and Random Trees (Karger et al.)](https://www.akamai.com/us/en/multimedia/documents/technical-publication/consistent-hashing-and-random-trees-distributed-caching-protocols-for-relieving-hot-spots-on-the-world-wide-web-technical-publication.pdf)
- [Consistent Hashing with Bounded Loads (Mirrokni et al., Google)](https://ai.googleblog.com/2017/04/consistent-hashing-with-bounded-loads.html)
- [Edge Computing for CDN (Akamai)](https://www.akamai.com/our-thinking/cdn/what-is-edge-computing)
