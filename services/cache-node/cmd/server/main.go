package main

import (
	"log"
	"os"

	api "github.com/HarryxDD/telco-edge-cdn/cache-node/internal/api"
	service "github.com/HarryxDD/telco-edge-cdn/cache-node/internal/service"
)

func main() {
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		nodeID = "edge-1"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	originURL := os.Getenv("ORIGIN_URL")

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/cache-" + nodeID
	}

	cacheCapacity := int64(1024 * 1024 * 500) // 500MB

	cache, err := service.NewEdgeCache(dbPath, originURL, nodeID, cacheCapacity)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the API Server
	srv := api.NewServer(cache)

	if err := srv.Start(port); err != nil {
		log.Fatal(err)
	}
}
