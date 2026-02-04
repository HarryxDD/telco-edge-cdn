package main

// Origin server entry point
import (
	"log"

	"net/http"
	"os"
)

// load existing videos.json file (if present), otherwise starts with empty video list.
func NewVideoStore(path string) (*VideoStore, error) {
	s := &VideoStore{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// bootstrap server
func main() {
	uploadsDir := "uploads"
	hlsDir := "hls"
	videosPath := "videos.json"
	addr := ":8443"

	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		log.Fatalf("create uploads dir: %v", err)
	}
	if err := os.MkdirAll(hlsDir, 0o755); err != nil {
		log.Fatalf("create hls dir: %v", err)
	}

	store, err := NewVideoStore(videosPath)
	if err != nil {
		log.Fatalf("load video store: %v", err)
	}

	handler := buildMux(store, uploadsDir, hlsDir)

	// certFile/keyFile for secure connection
	certFile := os.Getenv("TLS_CERT_FILE")
	if certFile == "" {
		certFile = "server.crt"
	}
	keyFile := os.Getenv("TLS_KEY_FILE")
	if keyFile == "" {
		keyFile = "server.key"
	}

	log.Printf("Sample CDN Streaming App backend listening on https://localhost%v", addr)
	log.Printf("Using TLS cert=%s key=%s", certFile, keyFile)
	if err := http.ListenAndServeTLS(addr, certFile, keyFile, handler); err != nil {
		log.Fatal(err)
	}
}
