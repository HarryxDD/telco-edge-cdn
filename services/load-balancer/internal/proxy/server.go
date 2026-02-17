package proxy

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/HarryxDD/telco-edge-cdn/load-balancer/internal/ring"
	"github.com/gin-gonic/gin"
)

type Server struct {
	ring *ring.BoundedLoadHashRing
}

func NewServer(r *ring.BoundedLoadHashRing) *Server {
	return &Server{ring: r}
}

func (s *Server) Start(port string) error {
	go s.ring.HealthCheck()

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"status": "healthy"})
	})

	router.Any("/hls/*path", s.proxyHLSRequest)
	router.Any("/api/*path", s.proxyAPIRequest)

	log.Printf("Load balancer starting on port %s", port)
	return router.Run()
}

func (s *Server) proxyHLSRequest(ctx *gin.Context) {
	path := ctx.Param("path")
	fullPath := "/hls" + path

	node := s.ring.GetNode(fullPath)
	if node == nil {
		ctx.JSON(503, gin.H{"error": "no healthy nodes available"})
		return
	}

	s.ring.IncrementLoad(node.ID)
	defer s.ring.DecrementLoad(node.ID)

	targetURL := "http://" + node.Address + fullPath
	s.forwardRequest(ctx, targetURL, node.ID)
}

func (s *Server) proxyAPIRequest(ctx *gin.Context) {
	path := ctx.Param("path")
	fullPath := "/api" + path

	node := s.ring.GetNode(fullPath)
	if node == nil {
		ctx.JSON(503, gin.H{"error": "no healthy nodes available"})
		return
	}

	targetURL := "http://" + node.Address + fullPath
	s.forwardRequest(ctx, targetURL, node.ID)
}

func (s *Server) forwardRequest(ctx *gin.Context, targetURL, nodeID string) {
	req, err := http.NewRequest(ctx.Request.Method, targetURL, ctx.Request.Body)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	for key, values := range ctx.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error proxying to %s: %v", nodeID, err)
		ctx.JSON(502, gin.H{"error": "failed to reach backend"})
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			ctx.Writer.Header().Add(key, value)
		}
	}

	ctx.Writer.WriteHeader(resp.StatusCode)
	io.Copy(ctx.Writer, resp.Body)
}
