package api

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	. "github.com/HarryxDD/telco-edge-cdn/cache-node/internal/service"
	"github.com/gin-gonic/gin"
)

type Server struct {
	cache *EdgeCache
}

func NewServer(cache *EdgeCache) *Server {
	return &Server{
		cache: cache,
	}
}

func (e *Server) Start(port string) error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(func(ctx *gin.Context) {
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.Next()
	})

	// router.GET("/metrics", gin.WrapH())
	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"status": "healthy", "node": e.cache.NodeID})
	})

	// Cache paths
	router.GET("/videos/:videoId/*filepath", e.serveVideo)
	router.HEAD("/videos/:videoId/*filepath", e.serveVideo)

	// Client-facing paths (same as origin)
	router.GET("/hls/:videoId/*filepath", e.serveHLS)
	router.HEAD("/hls/:videoId/*filepath", e.serveHLS)

	// API proxy to origin
	router.GET("/api/videos", e.proxyToOrigin)

	log.Printf("Edge cache %s starting on part %s", e.cache.NodeID, port)
	return router.Run(":" + port)
}

func (e *Server) serveHLS(ctx *gin.Context) {
	videoId := ctx.Param("videoId")
	filepath := ctx.Param("filepath")

	// Map /hls/{videoId}/file to /videos/{videoId}/file for caching
	requestPath := fmt.Sprintf("/videos/%s%s", videoId, filepath)

	e.serveCachedContent(ctx, requestPath)
}

func (e *Server) serveVideo(ctx *gin.Context) {
	videoId := ctx.Param("videoId")
	filepath := ctx.Param("filepath")
	requestPath := fmt.Sprintf("/videos/%s%s", videoId, filepath)

	e.serveCachedContent(ctx, requestPath)
}

func (e *Server) serveCachedContent(ctx *gin.Context, requestPath string) {
	cacheKey := hashKey(requestPath)

	// Check cache
	if data, found := e.cache.Get(cacheKey); found {
		log.Printf("[%s] CACHE HIT: %s", e.cache.NodeID, requestPath)

		contentType := getContentType(requestPath)
		ctx.Header("Content-Type", contentType)
		ctx.Data(200, contentType, data)
		return
	}

	// Cache miss - fetch from origin
	log.Printf("[%s] CACHE MISS: %s", e.cache.NodeID, requestPath)

	// Map back to /hls/* for origin
	originPath := requestPath
	if len(requestPath) > 7 && requestPath[:8] == "/videos/" {
		originPath = "/hls" + requestPath[7:] // /videos/wolf-123 → /hls/wolf-123
	}

	data, err := e.cache.FetchFromOrigin(originPath)
	if err != nil {
		log.Printf("Error fetching from origin: %v", err)
		ctx.JSON(502, gin.H{"error": "failed to fetch from origin"})
		return
	}

	// Store in cache
	if err := e.cache.Put(cacheKey, data); err != nil {
		log.Printf("Failed to cache: %v", err)
	}

	contentType := getContentType(requestPath)
	ctx.Header("Content-Type", contentType)
	ctx.Data(200, contentType, data)
}

// Proxy /api/videos to origin
func (e *Server) proxyToOrigin(ctx *gin.Context) {
    url := e.cache.OriginURL + "/api/videos"
    
    client := &http.Client{
        Timeout: 5 * time.Second,
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
        },
    }
    
    resp, err := client.Get(url)
    if err != nil {
        log.Printf("Error proxying /api/videos to origin: %v", err)
        ctx.JSON(502, gin.H{"error": "origin unreachable"})
        return
    }
    defer resp.Body.Close()
    
    data, _ := io.ReadAll(resp.Body)
    ctx.Data(resp.StatusCode, resp.Header.Get("Content-Type"), data)
}

func hashKey(key string) string {
	h := sha256.New()
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

func getContentType(path string) string {
	if len(path) < 5 {
		return "application/octet-stream"
	}

	ext := path[len(path)-5:]
	switch {
	case contains(ext, ".m3u8"):
		return "application/vnd.apple.mpegurl"
	case contains(ext, ".m4s"):
		return "video/iso.segment"
	case contains(ext, ".mp4"):
		return "video/mp4"
	case contains(ext, ".ts"):
		return "video/mp2t"
	default:
		return "application/octet-stream"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}
