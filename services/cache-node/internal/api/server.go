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

	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/coordination"
	. "github.com/HarryxDD/telco-edge-cdn/cache-node/internal/service"
	"github.com/gin-gonic/gin"
)

type ServerCoordinated struct {
	cache       *EdgeCacheCoordinated
	coordinator *coordination.Coordinator
}

func NewServerCoordinated(cache *EdgeCacheCoordinated, coord *coordination.Coordinator) *ServerCoordinated {
	return &ServerCoordinated{
		cache:       cache,
		coordinator: coord,
	}
}

func (s *ServerCoordinated) Start(port string) error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(func(ctx *gin.Context) {
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.Next()
	})

	// router.GET("/metrics", gin.WrapH())
	router.GET("/health", s.handleHealth)
	router.GET("/coordination/status", s.handleCoordStatus)

	// Client-facing paths (same as origin)
	router.GET("/hls/:videoId/*filepath", s.serveHLS)
	router.HEAD("/hls/:videoId/*filepath", s.serveHLS)

	// API proxy to origin
	router.GET("/api/videos", s.proxyToOrigin)

	log.Printf("Edge cache %s starting on part %s", s.cache.NodeID, port)
	return router.Run(":" + port)
}

func (s *ServerCoordinated) handleHealth(ctx *gin.Context) {
	ctx.JSON(200, gin.H{
		"status": "healthy",
		"node":   s.cache.NodeID,
		// TODO: fix
		// "is_leader": s.coordinator.IsLeader(),
	})
}

func (s *ServerCoordinated) handleCoordStatus(ctx *gin.Context) {
	ctx.JSON(200, gin.H{
		"node": s.cache.NodeID,
		// TODO: fix
		// "is_leader": s.coordinator.IsLeader(),
		// "state":     s.coordinator.GetState(),
	})
}

func (s *ServerCoordinated) serveHLS(ctx *gin.Context) {
	videoId := ctx.Param("videoId")
	filepath := ctx.Param("filepath")

	// Map /hls/{videoId}/file to /videos/{videoId}/file for caching
	requestPath := fmt.Sprintf("/videos/%s%s", videoId, filepath)

	s.serveCachedContent(ctx, requestPath)
}

func (s *ServerCoordinated) serveCachedContent(ctx *gin.Context, requestPath string) {
	cacheKey := hashKey(requestPath)

	// Use coordinated get
	data, found := s.cache.GetCoordinated(cacheKey)
	if !found {
		log.Printf("[%s] COORDINATED MISS: %s", s.cache.NodeID, requestPath)
		ctx.JSON(502, gin.H{"error": "failed to fetch content"})
		return
	}

	log.Printf("[%s] COORDINATED HIT: %s", s.cache.NodeID, requestPath)

	contentType := getContentType(requestPath)
	ctx.Header("Content-Type", contentType)
	ctx.Data(200, contentType, data)
}

// Proxy /api/videos to origin
func (s *ServerCoordinated) proxyToOrigin(ctx *gin.Context) {
	url := s.cache.OriginURL + "/api/videos"

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
