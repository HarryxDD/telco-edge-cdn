package api

import (
	"log"
	"time"

	"github.com/HarryxDD/telco-edge-cdn/origin/internal/service"
	"github.com/HarryxDD/telco-edge-cdn/origin/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Server struct {
	store   *store.VideoStore
	encoder *service.Encoder
	hlsDir  string
	router  *gin.Engine
}

func NewServer(videoStore *store.VideoStore, encoder *service.Encoder, hlsDir string) *Server {
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		store:   videoStore,
		encoder: encoder,
		hlsDir:  hlsDir,
		router:  gin.New(),
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware() {

	// Recovery middleware
	s.router.Use(gin.Recovery())

	// Request ID middleware
	s.router.Use(func(c *gin.Context) {
		requestID := uuid.New().String()
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	})

	// Logging middleware
	s.router.Use(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		method := c.Request.Method

		log.Printf("[%s] %s %s - %d (%v)",
			c.GetString("request_id"), method, path, statusCode, latency.Round(time.Millisecond))
	})

	// CORS middleware
	s.router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})
}

func (s *Server) setupRoutes() {
	s.router.GET("/health", s.handleHealth)

	s.router.GET("/metrics", s.handleMetrics)

	api := s.router.Group("/api")
	{
		api.GET("/videos", s.handleGetVideos)
		api.POST("/upload", s.handleUpload)
	}

	s.router.GET("/hls/*filepath", s.handleHLS)
	s.router.HEAD("/hls/*filepath", s.handleHLS)

	s.router.GET("/videos/:videoId/*filepath", s.handleVideosAlias)
	s.router.HEAD("/videos/:videoId/*filepath", s.handleVideosAlias)
}

func (s *Server) StartTLS(addr, certFile, keyFile string) error {
	return s.router.RunTLS(addr, certFile, keyFile)
}

func (s *Server) Start(addr string) error {
	return s.router.Run(addr)
}
