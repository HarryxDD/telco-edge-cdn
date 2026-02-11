# Origin Server

The Origin Server is the central content repository for the CDN. It handles video uploads, performs HLS transcoding using FFmpeg, and serves as the authoritative source for all video content.

## Features

- **Video Upload**: Accept video file uploads via multipart form-data
- **HLS Transcoding**: Automatic conversion to HLS format with multiple quality levels
- **Content Storage**: Persistent storage for uploaded videos and transcoded segments
- **Video Metadata**: JSON-based catalog of available videos
- **Health Monitoring**: Built-in health check endpoint

## Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ POST /api/upload
       ↓
┌────────────────────────────┐
│      Origin Server         │
│                            │
│  ┌──────────────────────┐  │
│  │  API Handler         │  │
│  │  - Upload            │  │
│  │  - List Videos       │  │
│  └──────────┬───────────┘  │
│             │              │
│  ┌──────────┴───────────┐  │
│  │  Encoder Service     │  │
│  │  - FFmpeg wrapper    │  │
│  │  - Async processing  │  │
│  └──────────┬───────────┘  │
│             │              │
│  ┌──────────┴───────────┐  │
│  │  Video Store         │  │
│  │  - Metadata (JSON)   │  │
│  │  - File storage      │  │
│  └──────────────────────┘  │
└────────────────────────────┘
```

## Configuration

Configure via environment variables or `.env` file:

```env
# Server configuration
ADDR=:8443
USE_TLS=false

# Storage paths
UPLOADS_DIR=uploads
HLS_DIR=hls
VIDEOS_PATH=videos.json

# TLS configuration (if USE_TLS=true)
TLS_CERT_FILE=server.crt
TLS_KEY_FILE=server.key
```

## Running Locally

### Prerequisites

- Go 1.25+
- FFmpeg installed and available in PATH
- OpenSSL (for generating TLS certificates, if needed)

### Generate TLS Certificate (Optional)

If running with `USE_TLS=true`, create a self-signed certificate:

```bash
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout server.key -out server.crt \
  -days 365 -subj "/CN=localhost"
```

Place the certificate files in the same directory as the executable.

### Build and Run

From the `services/origin` directory:

```bash
# Install dependencies
go mod download

# Run the server
go run cmd/main.go
```

The server will start on port 8443 (or the port specified in `ADDR`).

## API Endpoints

### Health Check
```http
GET /health
```

Returns server health status.

**Response:**
```json
{
  "status": "healthy",
  "service": "origin",
  "version": "1.0.0"
}
```

### List Videos
```http
GET /api/videos
```

Returns a list of all available videos.

**Response:**
```json
[
  {
    "title": "video-name",
    "duration": 125.5
  }
]
```

### Upload Video
```http
POST /api/upload
Content-Type: multipart/form-data
```

Upload a video file for HLS transcoding.

**Request (Local Development):**
```bash
curl -X POST http://localhost:8443/api/upload \
  -F "file=@/path/to/your/video.mp4"
```

**With Docker (from host machine):**
```bash
curl -X POST http://localhost:8443/api/upload \
  -F "file=@./my-video.mp4"
```

**With HTTPS (if TLS enabled):**
```bash
curl -k -X POST https://localhost:8443/api/upload \
  -F "file=@./video_test_1.mp4"
```

**Windows PowerShell:**
```powershell
curl.exe -X POST http://localhost:8443/api/upload -F "file=@./video.mp4"
```

**Response:**
```json
{
  "status": "processing",
  "title": "video-name"
}
```

The video will be transcoded asynchronously. Check `/api/videos` to see when it's ready.

### Stream Video (HLS)
```http
GET /hls/{videoId}/master.m3u8
GET /hls/{videoId}/playlist_{quality}.m3u8
GET /hls/{videoId}/segment_{quality}_{n}.m4s
```

Serve HLS manifests and segments.

**Example:**
```
http://localhost:8443/hls/video-name/master.m3u8
```

## HLS Transcoding

The encoder automatically:
1. Accepts uploaded video files (mp4, mov, avi, etc.)
2. Generates HLS segments with multiple quality levels:
   - 1080p (if source allows)
   - 720p
   - 480p
   - 360p
3. Creates master and variant playlists
4. Updates the video metadata catalog

Processing happens asynchronously. Monitor logs for transcoding progress.

## Directory Structure

```
services/origin/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── api/
│   │   ├── server.go        # HTTP server setup
│   │   └── handlers.go      # Request handlers
│   ├── service/
│   │   └── encoder.go       # FFmpeg wrapper
│   └── store/
│       └── videostore.go    # Video metadata management
├── uploads/                 # Uploaded source videos
├── hls/                     # Transcoded HLS output
├── videos.json             # Video metadata catalog
├── Dockerfile
└── README.md
```

## Dockerfile

The origin service includes a Dockerfile for containerized deployment:

```bash
docker build -t origin-server .
docker run -p 8443:8443 -v $(pwd)/uploads:/app/uploads -v $(pwd)/hls:/app/hls origin-server
```

## Related Services

- **Cache Node**: Caches HLS segments from origin
- **Load Balancer**: Routes requests to cache nodes
- **Client**: Web frontend for video playback

## References

- [HLS Specification](https://datatracker.ietf.org/doc/html/rfc8216)
- [FFmpeg Documentation](https://ffmpeg.org/documentation.html)
- [Gin Web Framework](https://gin-gonic.com/docs/)
