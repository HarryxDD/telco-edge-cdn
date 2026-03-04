package main

// Origin server entry point
import (
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/HarryxDD/telco-edge-cdn/origin/internal/api"
	"github.com/HarryxDD/telco-edge-cdn/origin/internal/service"
	"github.com/HarryxDD/telco-edge-cdn/origin/internal/store"
)

func main() {
	_ = godotenv.Load()

	// Configuration from environment
	uploadsDir := getEnv("UPLOADS_DIR", "uploads")
	hlsDir := getEnv("HLS_DIR", "hls")
	videosPath := getEnv("VIDEOS_PATH", "videos.json")
	addr := getEnv("ADDR", ":8443")
	certFile := getEnv("TLS_CERT_FILE", "server.crt")
	keyFile := getEnv("TLS_KEY_FILE", "server.key")
	useTLS := getEnv("USE_TLS", "true")

	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		log.Fatalf("create uploads dir: %v", err)
	}
	if err := os.MkdirAll(hlsDir, 0o755); err != nil {
		log.Fatalf("create hls dir: %v", err)
	}

	// Initialize video store
	videoStore, err := store.NewVideoStore(videosPath)
	if err != nil {
		log.Fatalf("load video store: %v", err)
	}

	// Initialize encoder service
	encoder := service.NewEncoder(uploadsDir, hlsDir, videoStore)

	// Create and start API server
	server := api.NewServer(videoStore, encoder, hlsDir)

	log.Printf("Origin server starting on http%s://0.0.0.0%s",
		map[bool]string{true: "s", false: ""}[useTLS == "true"], addr)
	log.Printf("Uploads: %s, HLS: %s", uploadsDir, hlsDir)

	if useTLS == "true" {
		log.Printf("TLS cert=%s key=%s", certFile, keyFile)
		if err := server.StartTLS(addr, certFile, keyFile); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("TLS disabled (internal network)")
		if err := server.Start(addr); err != nil {
			log.Fatal(err)
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}
