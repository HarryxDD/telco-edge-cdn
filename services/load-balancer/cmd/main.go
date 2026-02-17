package main

import (
	"log"
	"os"
	"strings"

	"github.com/HarryxDD/telco-edge-cdn/load-balancer/internal/proxy"
	"github.com/HarryxDD/telco-edge-cdn/load-balancer/internal/ring"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	_ = godotenv.Load()

	port := getEnv("PORT", "8090")
	cacheNodes := getEnv("CACHE_NODES", "cache-1:cache-1:8081,cache-2:cache-2:8082,cache-3:cache-3:8083")

	hashRing := ring.NewBoundedLoadHashRing(150, 1.25)

	// Parse and add cache nodes
	for _, node := range strings.Split(cacheNodes, ",") {
		parts := strings.Split(strings.TrimSpace(node), ":")
		if len(parts) >= 2 {
			nodeID := parts[0]
			nodeAddr := strings.Join(parts[1:], ":")
			hashRing.AddNode(nodeID, nodeAddr)
			log.Printf("Added cache node: %s -> %s", nodeID, nodeAddr)
		}
	}

	srv := proxy.NewServer(hashRing)

	log.Printf("Load balancer starting on port %s", port)
	if err := srv.Start(port); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
