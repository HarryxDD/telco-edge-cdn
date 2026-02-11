# Client (Web Frontend)

The Client is a React-based web application for browsing and streaming videos from the CDN. It uses HLS.js for adaptive bitrate streaming and provides a clean, responsive interface.

## Features

- **Video Library**: Browse available videos
- **HLS Playback**: Adaptive bitrate streaming with HLS.js
- **Responsive Design**: Works on desktop and mobile
- **Real-time Updates**: Fetches latest video catalog
- **Quality Selection**: Automatic quality adaptation based on bandwidth

## Architecture

```
┌──────────────────────────────────┐
│         React Frontend           │
│                                  │
│  ┌────────────────────────────┐  │
│  │  App.tsx                   │  │
│  │  - Main component          │  │
│  │  - Video list state        │  │
│  └────┬────────────────┬──────┘  │
│       │                │         │
│  ┌────┴──────┐    ┌────┴──────┐  │
│  │VideoList  │    │VideoPlayer│  │
│  │.tsx       │    │.tsx       │  │
│  └───────────┘    └───────────┘  │
│                        │         │
│                    ┌───┴──────┐  │
│                    │ HLS.js   │  │
│                    └──────────┘  │
└──────────────────────────────────┘
       │                  │
       ↓                  ↓
  GET /api/videos   GET /hls/{id}/*.m3u8
       │                  │
       ↓                  ↓
┌────────────────────────────────────┐
│       Load Balancer / CDN          │
└────────────────────────────────────┘
```

## Technology Stack

- **React 19**: UI framework
- **TypeScript**: Type-safe development
- **Vite**: Fast build tool and dev server
- **HLS.js**: HTTP Live Streaming playback
- **CSS**: Custom styling

## Running Locally

### Prerequisites

- Node.js 18+ and npm
- Load balancer or origin server running

### Setup and Run

From the `services/client/frontend` directory:

```bash
# Install dependencies
npm install

# Start development server
npm run dev
```

The app will open at `http://localhost:5173` (or the next available port).

### Build for Production

```bash
# Create optimized production build
npm run build

# Preview production build
npm run preview
```

Production files will be in the `dist/` directory.

## Configuration

### API Proxy

In `vite.config.ts`, configure the proxy to your CDN endpoint:

```typescript
export default defineConfig({
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8090',  // Load balancer
        changeOrigin: true,
      },
      '/hls': {
        target: 'http://localhost:8090',  // Load balancer
        changeOrigin: true,
      },
    },
  },
});
```

This allows the frontend to make requests to `/api/videos` and `/hls/*` without CORS issues.

## Components

### App.tsx

Main application component:
- Fetches video list from `/api/videos`
- Manages selected video state
- Renders video list and player

### VideoList.tsx

Displays available videos:
- Shows video title and duration
- Handles video selection
- Highlights selected video

**Props:**
```typescript
interface VideoListProps {
  videos: VideoMeta[];
  onSelect: (video: VideoMeta) => void;
  selectedTitle?: string;
}
```

### VideoPlayer.tsx

HLS video player:
- Initializes HLS.js
- Loads and plays selected video
- Handles quality switching
- Displays playback controls

**Props:**
```typescript
interface VideoPlayerProps {
  title?: string;  // Video ID to play
}
```

## Video Playback Flow

1. User selects video from list
2. `VideoPlayer` receives video `title`
3. Constructs HLS URL: `/hls/${title}/master.m3u8`
4. HLS.js loads master playlist
5. HLS.js selects appropriate quality based on bandwidth
6. Video segments are fetched and played
7. Quality adapts automatically during playback

## Directory Structure

```
services/client/frontend/
├── public/
│   ├── index.html
│   ├── manifest.json
│   └── robots.txt
├── src/
│   ├── App.tsx              # Main component
│   ├── App.css              # App styles
│   ├── VideoList.tsx        # Video list component
│   ├── VideoPlayer.tsx      # HLS player component
│   ├── index.tsx            # Entry point
│   ├── index.css            # Global styles
│   └── react-app-env.d.ts   # Type definitions
├── package.json
├── tsconfig.json
├── vite.config.ts           # Vite configuration
└── README.md
```

## Deployment

### Static Hosting

Build and deploy to any static hosting:

```bash
npm run build
# Upload dist/ to hosting service
```

Compatible with:
- Netlify
- Vercel
- GitHub Pages
- AWS S3 + CloudFront
- Nginx

### Docker

Create a Dockerfile:

```dockerfile
FROM node:18-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

### Environment Variables

For different environments, use Vite's env files:

```bash
# .env.development
VITE_API_URL=http://localhost:8090

# .env.production
VITE_API_URL=https://cdn.example.com
```

## References

- [HLS.js Documentation](https://github.com/video-dev/hls.js/)
- [React Documentation](https://react.dev/)
- [Vite Documentation](https://vitejs.dev/)
- [HLS Specification](https://datatracker.ietf.org/doc/html/rfc8216)
- [TypeScript Documentation](https://www.typescriptlang.org/docs/)
