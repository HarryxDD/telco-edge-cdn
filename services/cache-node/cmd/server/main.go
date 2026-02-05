package main

import (
	"log"
	"os"
	"strconv"

	api "github.com/HarryxDD/telco-edge-cdn/cache-node/internal/api"
	service "github.com/HarryxDD/telco-edge-cdn/cache-node/internal/service"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	nodeID := getEnv("NODE_ID", "edge-1")
	port := getEnv("PORT", "8081")
	originURL := getEnv("ORIGIN_URL", "https://localhost:8443")
	dbPath := getEnv("DB_PATH", "./data/cache-"+nodeID)
	cacheCapacityStr := getEnv("CACHE_CAPACITY", "524288000")
	cacheCapacity, err := strconv.ParseInt(cacheCapacityStr, 10, 64)
    if err != nil {
        cacheCapacity = 524288000
    }

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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
