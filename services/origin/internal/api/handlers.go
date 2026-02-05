package api

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "healthy",
		"service": "origin",
		"version": "1.0.0",
	})
}

func (s *Server) handleMetrics(c *gin.Context) {
	// TODO: Integrate Prometheus metrics
	c.String(200, "# Prometheus metrics will be here\n")
}

func (s *Server) handleGetVideos(c *gin.Context) {
	videos := s.store.GetAll()
	c.JSON(200, videos)
}

func (s *Server) handleUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<30)

	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("FormFile error: %v", err)
		c.JSON(400, gin.H{"error": "file field is required"})
		return
	}

	title, err := s.encoder.ProcessUpload(file)
	if err != nil {
		log.Printf("ProcessUpload error: %v", err)
		c.JSON(500, gin.H{"error": "failed to process upload"})
		return
	}

	c.JSON(202, gin.H{
		"status": "processing",
		"title":  title,
	})
}

func (s *Server) handleHLS(c *gin.Context) {
	// Client uses: /hls/{title}/master.m3u8
	filePath := c.Param("filepath")
	filePath = strings.TrimPrefix(filePath, "/")
	filePath = cleanPath(filePath)

	fullPath := filepath_Join(s.hlsDir, filePath)

	// Debug logging
	log.Printf("[DEBUG] hlsDir=%s, filePath=%s, fullPath=%s", s.hlsDir, filePath, fullPath)

	// Security: prevent path traversal
	absHlsDir, err := filepath.Abs(s.hlsDir)
	if err != nil {
		log.Printf("[ERROR] Failed to get absolute path for hlsDir: %v", err)
		c.JSON(500, gin.H{"error": "internal error"})
		return
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		log.Printf("[ERROR] Failed to get absolute path for fullPath: %v", err)
		c.JSON(500, gin.H{"error": "internal error"})
		return
	}

	log.Printf("[DEBUG] absHlsDir=%s, absFullPath=%s", absHlsDir, absFullPath)

	if !strings.HasPrefix(absFullPath, absHlsDir) {
		log.Printf("[SECURITY] Path traversal attempt blocked: %s not under %s", absFullPath, absHlsDir)
		c.JSON(403, gin.H{"error": "forbidden"})
		return
	}

	// Set correct content type
	contentType := getContentType(filePath)

	// Log for ML training
	log.Printf("[ACCESS] HLS path=%s", filePath)

	c.Header("Content-Type", contentType)
	c.File(absFullPath)
}

func (s *Server) handleVideosAlias(c *gin.Context) {
	// Cache-node uses: /videos/{videoId}/master.m3u8
	// Map to: /hls/{videoId}/master.m3u8
	videoId := c.Param("videoId")
	filepath := c.Param("filepath")

	// Reconstruct as HLS path
	hlsPath := fmt.Sprintf("/%s%s", videoId, filepath)

	// Reuse HLS handler logic
	c.Params = gin.Params{
		{Key: "filepath", Value: hlsPath},
	}
	s.handleHLS(c)
}

// Helper functions

func filepath_Join(elem ...string) string {
	return filepath.Join(elem...)
}

func cleanPath(p string) string {
	return filepath.Clean(p)
}

func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".m4s":
		return "video/iso.segment"
	case ".mp4":
		return "video/mp4"
	case ".ts":
		return "video/mp2t"
	default:
		return "application/octet-stream"
	}
}
