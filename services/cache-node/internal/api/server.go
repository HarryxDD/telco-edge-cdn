package api

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/coordination"
	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/service"
	"github.com/gin-gonic/gin"
)

type ServerCoordinated struct {
	cache       *service.EdgeCache
	coordinator *coordination.Coordinator
}

func NewServerCoordinated(cache *service.EdgeCache, coord *coordination.Coordinator) *ServerCoordinated {
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

	// Coordination endpoints (leader only)
	router.POST("/coordination/request-lock", s.handleRequestLock)
	router.POST("/coordination/release-lock", s.handleReleaseLock)

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
		"status":    "healthy",
		"node":      s.cache.NodeID,
		"is_leader": s.coordinator.IsLeader(),
	})
}

func (s *ServerCoordinated) handleCoordStatus(ctx *gin.Context) {
	ctx.JSON(200, gin.H{
		"node":      s.cache.NodeID,
		"is_leader": s.coordinator.IsLeader(),
		"state":     s.coordinator.GetState(),
	})
}

func (s *ServerCoordinated) serveHLS(ctx *gin.Context) {
	videoId := ctx.Param("videoId")
	filepath := ctx.Param("filepath")

	// Map /hls/{videoId}/file to /videos/{videoId}/file for caching
	requestPath := fmt.Sprintf("/videos/%s%s", videoId, filepath)
	cacheKey := requestPath

	// First try local cache
	data, found := s.cache.Get(cacheKey)
	if found {
		log.Printf("[%s] COORDINATED HIT (local): %s", s.cache.NodeID, requestPath)
	} else {
		// Cache miss - coordinate with cluster
		log.Printf("[%s] COORDINATED MISS: %s", s.cache.NodeID, requestPath)
		var peerID string
		var err error

		data, peerID, err = s.coordinator.HandleCacheMiss(cacheKey)
		if err != nil {
			log.Printf("[%s] HandleCacheMiss failed for %s: %v", s.cache.NodeID, requestPath, err)
			ctx.JSON(502, gin.H{"error": "failed to fetch content"})
			return
		}

		if peerID != "" {
			// Fetch from peer using coordination helper
			log.Printf("[%s] Fetching segment %s from peer %s", s.cache.NodeID, cacheKey, peerID)
			data, err = coordination.FetchFromPeer(peerID, cacheKey)
			if err != nil {
				log.Printf("[%s] Failed to fetch from peer %s: %v", s.cache.NodeID, peerID, err)
				ctx.JSON(502, gin.H{"error": "failed to fetch content"})
				return
			}
		}

		// Store in local cache
		if err := s.cache.Put(cacheKey, data); err != nil {
			log.Printf("[%s] Failed to store segment %s: %v", s.cache.NodeID, cacheKey, err)
		}
	}

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

func (s *ServerCoordinated) handleRequestLock(ctx *gin.Context) {
	var req struct {
		SegmentID string `json:"segment_id"`
		NodeID    string `json:"node_id"`
	}

	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !s.coordinator.IsLeader() {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error":     "not leader",
			"leader_id": s.coordinator.GetLeaderID(),
		})
		return
	}

	// Call coordinator directly
	granted, fetchingNode := s.coordinator.RequestLeaderLock(req.SegmentID, req.NodeID)

	ctx.JSON(http.StatusOK, gin.H{
		"granted":       granted,
		"fetching_node": fetchingNode,
	})
}

func (s *ServerCoordinated) handleReleaseLock(ctx *gin.Context) {
	var req struct {
		SegmentID string `json:"segment_id"`
		NodeID    string `json:"node_id"`
	}

	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !s.coordinator.IsLeader() {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "not leader"})
		return
	}

	// Call coordinator directly
	err := s.coordinator.ReleaseLeaderLock(req.SegmentID, req.NodeID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "released"})
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
