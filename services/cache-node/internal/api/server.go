package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

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

	router.GET("/videos/:videoId/*filepath", e.serveVideo)

	log.Printf("Edge cache %s starting on part %s", e.cache.NodeID, port)
	return router.Run(":" + port)
}

func (e *Server) serveVideo(ctx *gin.Context) {
	videoId := ctx.Param("videoId")
	filepath := ctx.Param("filepath")
	requestPath := fmt.Sprintf("/videos/%s%s", videoId, filepath)

	// Try cache first
	cacheKey := hashKey(requestPath)
	if data, found := e.cache.Get(cacheKey); found {
		log.Printf("CACHE HIT: %s", requestPath)
		ctx.Data(200, "application/octet-stream", data)
		return
	}

	// Cache miss - fetch from origin
	log.Printf("CACHE MISS: %s", requestPath)
	data, err := e.cache.FetchFromOrigin(requestPath)
	if err != nil {
		log.Printf("Error fetching from origin: %v", err)
		ctx.JSON(502, gin.H{"error": "failed to fetch from origin"})
		return
	}

	// Store in cache
	if err := e.cache.Put(cacheKey, data); err != nil {
		log.Printf("Failed to cache: %v", err)
	}

	ctx.Data(200, "application/octet-stream", data)
}

func hashKey(key string) string {
	h := sha256.New()
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}
