# API Documentation

This document describes the REST API endpoints for the Telco-Edge CDN system.

## Base URLs

### Development
- **Origin Server**: `http://localhost:8443`
- **Cache Node**: `http://localhost:8081`
- **Load Balancer**: `http://localhost:8090`

### Production
- **CDN Entry Point**: `https://cdn.example.com`

## API Overview

```
┌─────────────────────────────────────────┐
│          Client Application             │
└────────────────┬────────────────────────┘
                 │
                 ↓
┌─────────────────────────────────────────┐
│         Load Balancer API               │
│  - Video Streaming                      │
│  - Video List (proxied)                 │
│  - Health Check                         │
└────────────────┬────────────────────────┘
                 │
     ┌───────────┴───────────┐
     ↓                       ↓
┌─────────────┐      ┌─────────────────┐
│ Cache Nodes │      │ Origin Server   │
│  - Caching  │      │  - Upload       │
│  - Health   │      │  - Transcode    │
└─────────────┘      │  - Storage      │
                     └─────────────────┘
```

## Origin Server API

### Health Check

**Endpoint:** `GET /health`

**Description:** Check if the origin server is running and healthy.

**Request:**
```http
GET /health HTTP/1.1
Host: localhost:8443
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "status": "healthy",
  "service": "origin",
  "version": "1.0.0"
}
```

**Status Codes:**
- `200`: Service is healthy
- `503`: Service is unavailable

---

### List Videos

**Endpoint:** `GET /api/videos`

**Description:** Retrieve a list of all available videos with metadata.

**Request:**
```http
GET /api/videos HTTP/1.1
Host: localhost:8443
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

[
  {
    "title": "nature-documentary",
    "duration": 142.5
  },
  {
    "title": "tech-presentation",
    "duration": 1800.0
  }
]
```

**Response Fields:**
- `title` (string): Unique video identifier (sanitized filename)
- `duration` (number): Video duration in seconds (0 if still processing)

**Status Codes:**
- `200`: Success
- `500`: Internal server error

---

### Upload Video

**Endpoint:** `POST /api/upload`

**Description:** Upload a video file for HLS transcoding.

**Request:**
```http
POST /api/upload HTTP/1.1
Host: localhost:8443
Content-Type: multipart/form-data; boundary=----WebKitFormBoundary

------WebKitFormBoundary
Content-Disposition: form-data; name="file"; filename="video.mp4"
Content-Type: video/mp4

<binary video data>
------WebKitFormBoundary--
```

**Form Fields:**
- `file` (file, required): Video file to upload

**Supported Formats:**
- MP4 (`.mp4`)
- MOV (`.mov`)
- AVI (`.avi`)
- MKV (`.mkv`)

**Max File Size:** 1 GB

**Response (Accepted):**
```http
HTTP/1.1 202 Accepted
Content-Type: application/json

{
  "status": "processing",
  "title": "video"
}
```

**Response Fields:**
- `status` (string): Processing status ("processing")
- `title` (string): Generated video identifier

**Status Codes:**
- `202`: Upload accepted, transcoding in progress
- `400`: Bad request (missing file, invalid format)
- `413`: Payload too large (>1GB)
- `500`: Internal server error (transcoding failure)

**Example (curl):**
```bash
# Linux/Mac
curl -X POST http://localhost:8443/api/upload \
  -F "file=@./my-video.mp4"

# Windows PowerShell
curl.exe -X POST http://localhost:8443/api/upload -F "file=@./my-video.mp4"

# With progress bar
curl --progress-bar -X POST http://localhost:8443/api/upload \
  -F "file=@./my-video.mp4" \
  -o /dev/null
```

**Transcoding Process:**
1. Upload accepted immediately (202 response)
2. Transcoding happens asynchronously
3. Video appears in `/api/videos` with `duration: 0`
4. After transcoding completes, duration is updated
5. Video is ready for streaming

---

### Stream Video (HLS Master Playlist)

**Endpoint:** `GET /hls/{videoId}/master.m3u8`

**Description:** Retrieve the HLS master playlist for a video.

**Request:**
```http
GET /hls/nature-documentary/master.m3u8 HTTP/1.1
Host: localhost:8443
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/vnd.apple.mpegurl

#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360
playlist_360p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=1400000,RESOLUTION=854x480
playlist_480p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2800000,RESOLUTION=1280x720
playlist_720p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080
playlist_1080p.m3u8
```

**Status Codes:**
- `200`: Success
- `404`: Video not found
- `403`: Forbidden (path traversal attempt)

---

### Stream Video (HLS Variant Playlist)

**Endpoint:** `GET /hls/{videoId}/playlist_{quality}.m3u8`

**Description:** Retrieve the variant playlist for a specific quality level.

**Request:**
```http
GET /hls/nature-documentary/playlist_720p.m3u8 HTTP/1.1
Host: localhost:8443
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/vnd.apple.mpegurl

#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:10.0,
segment_720p_0.m4s
#EXTINF:10.0,
segment_720p_1.m4s
#EXTINF:10.0,
segment_720p_2.m4s
#EXT-X-ENDLIST
```

**Status Codes:**
- `200`: Success
- `404`: Playlist not found

---

### Stream Video (HLS Segment)

**Endpoint:** `GET /hls/{videoId}/segment_{quality}_{n}.m4s`

**Description:** Retrieve a specific video segment.

**Request:**
```http
GET /hls/nature-documentary/segment_720p_5.m4s HTTP/1.1
Host: localhost:8443
Range: bytes=0-
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: video/iso.segment
Content-Length: 524288

<binary video segment data>
```

**Status Codes:**
- `200`: Success
- `206`: Partial content (range request)
- `404`: Segment not found

---

## Cache Node API

### Health Check

**Endpoint:** `GET /health`

**Description:** Check if the cache node is healthy.

**Request:**
```http
GET /health HTTP/1.1
Host: localhost:8081
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "status": "healthy",
  "node": "cache-1"
}
```

**Response Fields:**
- `status` (string): Health status
- `node` (string): Cache node identifier

---

### Serve Cached Video Content

**Endpoint:** `GET /hls/{videoId}/*`

**Description:** Serve video content from cache or fetch from origin.

**Request:**
```http
GET /hls/nature-documentary/master.m3u8 HTTP/1.1
Host: localhost:8081
```

**Response (Cache Hit):**
```http
HTTP/1.1 200 OK
Content-Type: application/vnd.apple.mpegurl
X-Cache-Status: HIT

<cached content>
```

**Response (Cache Miss):**
```http
HTTP/1.1 200 OK
Content-Type: application/vnd.apple.mpegurl
X-Cache-Status: MISS

<content fetched from origin>
```

**Status Codes:**
- `200`: Success (from cache or origin)
- `502`: Bad gateway (origin unreachable)
- `404`: Content not found

---

### Proxy Video List

**Endpoint:** `GET /api/videos`

**Description:** Proxy the video list request to the origin server.

**Request:**
```http
GET /api/videos HTTP/1.1
Host: localhost:8081
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

[...]
```

Same response format as origin server's `/api/videos`.

---

## Load Balancer API

### Health Check

**Endpoint:** `GET /health`

**Description:** Check if the load balancer is healthy.

**Request:**
```http
GET /health HTTP/1.1
Host: localhost:8090
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "status": "healthy"
}
```

---

### Route Video Content

**Endpoint:** `GET /hls/{videoId}/*`

**Description:** Route video requests to appropriate cache node using consistent hashing.

**Request:**
```http
GET /hls/nature-documentary/segment_720p_5.m4s HTTP/1.1
Host: localhost:8090
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: video/iso.segment
X-Served-By: cache-2

<video segment data>
```

**Status Codes:**
- `200`: Success
- `503`: No healthy cache nodes available
- `502`: Backend error

**Routing Behavior:**
- Same video ID always routes to same cache node (consistent hashing)
- Automatic failover if selected node is unhealthy
- Load-aware routing prevents overload

---

### Proxy API Requests

**Endpoint:** `GET /api/*`

**Description:** Proxy API requests to a cache node (which forwards to origin).

**Request:**
```http
GET /api/videos HTTP/1.1
Host: localhost:8090
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

[...]
```

---

## Error Responses

### Standard Error Format

```json
{
  "error": "error message describing what went wrong"
}
```

### Common HTTP Status Codes

| Code | Meaning | Description |
|------|---------|-------------|
| 200 | OK | Request successful |
| 202 | Accepted | Request accepted for async processing |
| 206 | Partial Content | Range request successful |
| 400 | Bad Request | Invalid request parameters |
| 403 | Forbidden | Access denied (e.g., path traversal) |
| 404 | Not Found | Resource doesn't exist |
| 413 | Payload Too Large | Upload exceeds size limit |
| 500 | Internal Server Error | Server-side error |
| 502 | Bad Gateway | Upstream service error |
| 503 | Service Unavailable | Service temporarily unavailable |

---

## cURL Examples

### Upload a Video
```bash
curl -X POST http://localhost:8443/api/upload \
  -F "file=@./video.mp4"
```

### List Available Videos
```bash
curl http://localhost:8443/api/videos
```

### Get Video Master Playlist (via Load Balancer)
```bash
curl http://localhost:8090/hls/video-name/master.m3u8
```

### Check Origin Health
```bash
curl http://localhost:8443/health
```

### Check Cache Node Health
```bash
curl http://localhost:8081/health
```

### Check Load Balancer Health
```bash
curl http://localhost:8090/health
```

---

## Client Integration

### JavaScript/TypeScript Example

```typescript
// Fetch video list
const response = await fetch('/api/videos');
const videos = await response.json();

// Upload video
const formData = new FormData();
formData.append('file', videoFile);

const uploadResponse = await fetch('/api/upload', {
  method: 'POST',
  body: formData,
});

const result = await uploadResponse.json();
console.log('Video title:', result.title);

// Stream video with HLS.js
import Hls from 'hls.js';

if (Hls.isSupported()) {
  const video = document.getElementById('video');
  const hls = new Hls();
  hls.loadSource(`/hls/${videoTitle}/master.m3u8`);
  hls.attachMedia(video);
  hls.on(Hls.Events.MANIFEST_PARSED, () => {
    video.play();
  });
}
```

### React Example

```tsx
import React, { useEffect, useState } from 'react';

function VideoList() {
  const [videos, setVideos] = useState([]);

  useEffect(() => {
    fetch('/api/videos')
      .then(res => res.json())
      .then(data => setVideos(data));
  }, []);

  return (
    <ul>
      {videos.map(video => (
        <li key={video.title}>
          {video.title} - {video.duration}s
        </li>
      ))}
    </ul>
  );
}
```

---

## References

- [HTTP/1.1 RFC 9110](https://www.rfc-editor.org/rfc/rfc9110.html)
- [HLS RFC 8216](https://datatracker.ietf.org/doc/html/rfc8216)
- [REST API Design Best Practices](https://restfulapi.net/)
- [Gin Web Framework Documentation](https://gin-gonic.com/docs/)
