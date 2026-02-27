package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	api "github.com/HarryxDD/telco-edge-cdn/cache-node/internal/api"
	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/coordination"
	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/election"
	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/gossip"
	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/logging"
	service "github.com/HarryxDD/telco-edge-cdn/cache-node/internal/service"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	// Parse configuration
	nodeIDStr := getEnv("NODE_ID", "cache-1")
	nodeID, _ := strconv.Atoi(strings.TrimPrefix(nodeIDStr, "cache-"))
	port, _ := strconv.Atoi(getEnv("PORT", "8081"))
	address := getEnv("ADDRESS", "localhost")
	originURL := getEnv("ORIGIN_URL", "https://localhost:8443")
	dbPath := getEnv("DB_PATH", "./data/cache-"+nodeIDStr)

	// Cache capacity in number of items (not bytes)
	cacheCapacityStr := getEnv("CACHE_CAPACITY", "1000")
	cacheCapacity, err := strconv.Atoi(cacheCapacityStr)
	if err != nil {
		log.Fatalf("Invalid CACHE_CAPACITY: %v", err)
	}

	// Parse cluster nodes
	nodesConfig := getEnv("CLUSTER_NODES", "cache-1:localhost:8081,cache-2:localhost:8082,cache-3:localhost:8083")
	electionNodes, gossipPeers := parseClusterNodes(nodesConfig)

	log.Printf("Starting cache node: ID=%d, Port=%d", nodeID, port)
	log.Printf("Cluster nodes: %v", electionNodes)

	logPath := fmt.Sprintf("/app/logs/access_%s.ndjson", nodeIDStr)
	accessLogger, err := logging.NewAccessLogger(logPath, nodeIDStr)
	if err != nil {
		log.Printf("Warning: Failed to create access logger: %v", err)
		accessLogger = nil
	} else {
		defer accessLogger.Close()
		log.Printf("Access logger initialized: %s", logPath)
	}

	elec := election.NewBullyElection(nodeID, address, port, electionNodes)

	gosp := gossip.NewEpidemicGossip(nodeIDStr, address, port, gossipPeers)

	coord := coordination.NewCoordinator(nodeIDStr, elec, gosp)

	cache, err := service.NewEdgeCache(dbPath, originURL, nodeIDStr, cacheCapacity)
	if err != nil {
		log.Fatal(err)
	}

	// Register origin fetch callback with coordinator
	coord.RegisterCallbacks(cache.FetchFromOrigin)

	// Start coordination (after cache and callback are ready)
	if err := coord.Start(); err != nil {
		log.Fatalf("Failed to start coordinator: %v", err)
	}

	// Initialize API server
	srv := api.NewServerCoordinated(cache, coord, accessLogger)

	log.Printf("[CACHE-SERVER] Cache node %s ready on port %d", nodeIDStr, port)

	if err := srv.Start(strconv.Itoa(port)); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseClusterNodes(config string) (map[int]election.NodeInfo, map[string]gossip.NodeInfo) {
	electionNodes := make(map[int]election.NodeInfo)
	gossipPeers := make(map[string]gossip.NodeInfo)

	for _, nodeStr := range strings.Split(config, ",") {
		parts := strings.Split(strings.TrimSpace(nodeStr), ":")
		if len(parts) != 3 {
			continue
		}

		nodeIDStr := parts[0]
		address := parts[1]
		port, _ := strconv.Atoi(parts[2])

		nodeID, _ := strconv.Atoi(strings.TrimPrefix(nodeIDStr, "cache-"))

		electionNodes[nodeID] = election.NodeInfo{
			ID:      nodeID,
			Address: address,
			Port:    port,
		}

		gossipPeers[nodeIDStr] = gossip.NodeInfo{
			ID:      nodeIDStr,
			Address: address,
			Port:    port,
		}
	}

	return electionNodes, gossipPeers
}
