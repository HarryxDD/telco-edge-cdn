package service

import (
	"log"

	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/coordination"
)

// EdgeCacheCoordinated extends EdgeCache with cluster coordination
type EdgeCacheCoordinated struct {
	*EdgeCache
	coordinator *coordination.Coordinator
}

func NewEdgeCacheCoordinated(
	dbPath,
	originURL,
	nodeID string,
	cacheCapacity int,
	coordinator *coordination.Coordinator,
) (*EdgeCacheCoordinated, error) {
	// Create base cache
	baseCache, err := NewEdgeCache(dbPath, originURL, nodeID, cacheCapacity)
	if err != nil {
		return nil, err
	}

	cache := &EdgeCacheCoordinated{
		EdgeCache:   baseCache,
		coordinator: coordinator,
	}

	// Register origin fetch callback
	coordinator.RegisterCallbacks(baseCache.FetchFromOrigin)

	return cache, nil
}

// GetCoordinated uses coordinated fetch (with leader election + gossip)
func (e *EdgeCacheCoordinated) GetCoordinated(key string) ([]byte, bool) {
	// Try local cache first
	if data, found := e.Get(key); found {
		return data, true
	}

	// Cache miss - coordinate with cluster
	data, peerID, err := e.coordinator.HandleCacheMiss(key)
	if err != nil {
		log.Printf("[CACHE] Coordinated fetch failed: %v", err)
		return nil, false
	}

	if peerID != "" {
		// Fetch from peer using coordination helper
		log.Printf("[CACHE] Fetching segment %s from peer %s", key, peerID)
		data, err = coordination.FetchFromPeer(peerID, key)
		if err != nil {
			log.Printf("[CACHE] Failed to fetch from peer %s: %v", peerID, err)
			return nil, false
		}
	}

	// Store in local cache
	if err := e.Put(key, data); err != nil {
		log.Printf("[CACHE] Failed to store segment %s: %v", key, err)
	}

	return data, true
}

// GetCoordinator exposes coordinator for HTTP handlers
func (e *EdgeCacheCoordinated) GetCoordinator() *coordination.Coordinator {
	return e.coordinator
}
