package api

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/coordination"
	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/logging"
	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/metrics"
	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type ServerCoordinated struct {
	cache        *service.EdgeCache
	coordinator  *coordination.Coordinator
	accessLogger *logging.AccessLogger
}

func NewServerCoordinated(cache *service.EdgeCache, coord *coordination.Coordinator, logger *logging.AccessLogger) *ServerCoordinated {
	return &ServerCoordinated{
		cache:        cache,
		coordinator:  coord,
		accessLogger: logger,
	}
}

func (s *ServerCoordinated) Start(port string) error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(func(ctx *gin.Context) {
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.Next()
	})

	router.GET("/health", s.handleHealth)
	router.GET("/coordination/status", s.handleCoordStatus)
	router.POST("/coordination/request-lock", s.handleRequestLock)
	router.POST("/coordination/release-lock", s.handleReleaseLock)
	router.GET("/hls/:videoId/*filepath", s.serveHLS)
	router.HEAD("/hls/:videoId/*filepath", s.serveHLS)
	router.GET("/api/videos", s.proxyToOrigin)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	log.Printf("edge cache %s starting on port %s", s.cache.NodeID, port)
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
	start := time.Now()
	videoID := ctx.Param("videoId")
	filepath := ctx.Param("filepath")
	requestPath := fmt.Sprintf("/videos/%s%s", videoID, filepath)
	cacheKey := requestPath

	var cacheHit bool
	statusCode := 200

	data, found := s.cache.Get(cacheKey)
	cacheHit = found

	if found {
		log.Printf("[%s] HIT (local): %s", s.cache.NodeID, requestPath)
		metrics.CacheHits.WithLabelValues(s.cache.NodeID, videoID).Inc()
	} else {
		log.Printf("[%s] MISS: %s", s.cache.NodeID, requestPath)
		metrics.CacheMisses.WithLabelValues(s.cache.NodeID, videoID).Inc()

		var peerID string
		var err error

		data, peerID, err = s.coordinator.HandleCacheMiss(cacheKey)
		if err != nil {
			log.Printf("[%s] cache miss handling failed for %s: %v", s.cache.NodeID, requestPath, err)
			metrics.ErrorResponses.WithLabelValues(s.cache.NodeID, "502").Inc()
			ctx.JSON(502, gin.H{"error": "failed to fetch content"})
			return
		}

		if peerID != "" {
			data, err = coordination.FetchFromPeer(peerID, cacheKey)
			if err != nil {
				log.Printf("[%s] peer fetch failed from %s: %v", s.cache.NodeID, peerID, err)
				metrics.ErrorResponses.WithLabelValues(s.cache.NodeID, "502").Inc()
				ctx.JSON(502, gin.H{"error": "failed to fetch content"})
				return
			}
		}

		if err := s.cache.Put(cacheKey, data); err != nil {
			log.Printf("[%s] cache store failed for %s: %v", s.cache.NodeID, cacheKey, err)
		}
	}

	duration := time.Since(start)

	// track non-200 status codes
	if statusCode != 200 {
		metrics.ErrorResponses.WithLabelValues(s.cache.NodeID, strconv.Itoa(statusCode)).Inc()
	}

	metrics.RequestDuration.WithLabelValues(s.cache.NodeID, fmt.Sprintf("%v", cacheHit)).Observe(duration.Seconds())
	metrics.BytesServed.WithLabelValues(s.cache.NodeID).Add(float64(len(data)))
	metrics.RequestsTotal.WithLabelValues(s.cache.NodeID, videoID).Inc()

	if s.accessLogger != nil {
		s.logAccess(ctx, videoID, filepath, cacheHit, duration, int64(len(data)), statusCode)
	}

	contentType := getContentType(requestPath)
	ctx.Header("Content-Type", contentType)
	ctx.Data(200, contentType, data)
}

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
		log.Printf("origin proxy failed: %v", err)
		metrics.ErrorResponses.WithLabelValues(s.cache.NodeID, "502").Inc()
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

	if err := s.coordinator.ReleaseLeaderLock(req.SegmentID, req.NodeID); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "released"})
}

func (s *ServerCoordinated) logAccess(ctx *gin.Context, videoID, filepath string, cacheHit bool, duration time.Duration, bytes int64, statusCode int) {
	clientIP := ctx.ClientIP()
	userAgent := ctx.Request.UserAgent()
	now := time.Now()

	segmentNumber := extractSegmentNumber(filepath)
	bitrateKbps := inferBitrate(filepath)

	// determine if this is a manifest or segment request
	requestType := "segment"
	if strings.HasSuffix(filepath, ".m3u8") {
		requestType = "manifest"
	}

	rebuffer := duration.Milliseconds() > 2000
	if rebuffer {
		metrics.RebufferEvents.WithLabelValues(s.cache.NodeID, strconv.FormatInt(bitrateKbps, 10)).Inc()
	}

	metrics.BitrateRequested.WithLabelValues(s.cache.NodeID).Observe(float64(bitrateKbps))

	s.accessLogger.Log(logging.AccessLog{
		Timestamp:      now.UTC().Format("2006-01-02T15:04:05.000Z"),
		EdgeNodeID:     s.cache.NodeID,
		ClientID:       clientIP,
		SessionID:      logging.GenerateSessionID(clientIP, userAgent, now),
		VideoID:        videoID,
		VideoCategory:  "general",
		SegmentNumber:  segmentNumber,
		RequestType:    requestType, // now correctly set
		CacheHit:       cacheHit,
		ResponseTimeMs: float64(duration.Milliseconds()),
		BytesSent:      bytes,
		ClientRegion:   "FI-OUL",
		Protocol:       "HTTP3",
		BitrateKbps:    bitrateKbps,
		RebufferEvent:  rebuffer,
		StatusCode:     statusCode,
	})
}

// try to parse bitrate from segment filename like seg_4500k_7.m4s
// falls back to 2000 kbps if pattern is not found
func inferBitrate(filepath string) int64 {
	// var bitrate int64
	// if _, err := fmt.Sscanf(filepath, "/seg_%dk", &bitrate); err == nil {
	// 	return bitrate
	// }
	return 2000
}

// parse segment index from pattern like /segment_0001.m4s
func extractSegmentNumber(filepath string) int {
	var n int
	if _, err := fmt.Sscanf(filepath, "/segment_%d.m4s", &n); err == nil {
		return n
	}
	return 0
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
