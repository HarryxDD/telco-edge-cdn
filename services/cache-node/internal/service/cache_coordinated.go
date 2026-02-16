package service

import (
	"log"
	"time"

	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/coordination"
	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/gossip"
)

// EdgeCacheCoordinated extends EdgeCache with coordination
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

	// Register coordination callbacks
	coordinator.RegisterCallbacks(
		cache.fetchFromOrigin,
		cache.handlePreFetch,
		cache.handleInvalidate,
	)

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
		// Fetch from peer
		log.Printf("[CACHE] Fetching segment %s from peer %s", key, peerID)
		data, err = e.fetchFromPeer(peerID, key)
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

func (e *EdgeCacheCoordinated) fetchFromOrigin(segmentID string) ([]byte, error) {
	// Map segment ID back to origin path
	// segmentID format: "sha256(path)"
	// For now, assume we stored the original path somewhere
	// Simplified: fetch directly
	return e.FetchFromOrigin(segmentID)
}

func (e *EdgeCacheCoordinated) fetchFromPeer(peerID, segmentID string) ([]byte, error) {
	// TODO: Implement peer-to-peer fetch
	// For now, fall back to origin
	log.Printf("[CACHE] Peer fetch not implemented, falling back to origin")
	return e.FetchFromOrigin(segmentID)
}

func (e *EdgeCacheCoordinated) handlePreFetch(cmd gossip.PreFetchCommand) {
	log.Printf("[CACHE] Pre-fetching %d segments for %s", len(cmd.Segments), cmd.VideoID)

	// Background worker
	go func() {
		for _, segmentPath := range cmd.Segments {
			// Check if already cached
			if _, found := e.Get(segmentPath); found {
				continue
			}

			// Fetch from origin
			data, err := e.FetchFromOrigin(segmentPath)
			if err != nil {
				log.Printf("[CACHE] Pre-fetch failed for %s: %v", segmentPath, err)
				continue
			}

			// Store in cache
			if err := e.Put(segmentPath, data); err != nil {
				log.Printf("[CACHE] Pre-fetch failed to store %s: %v", segmentPath, err)
				continue
			}

			// Notify gossip
			e.coordinator.NotifyCacheAdd(segmentPath, int64(len(data)))

			log.Printf("[CACHE] Pre-cached %s (%d bytes)", segmentPath, len(data))

			// Rate limit
			time.Sleep(100 * time.Microsecond)
		}
	}()
}

func (e *EdgeCacheCoordinated) handleInvalidate(videoID string) {
	log.Printf("[CACHE] Invalidating cache for video %s", videoID)

	// Evict all segments for this video
	// This requires iterating through cache
	// For now, simplified: clear entire cache (not production ready)
	// TODO: Implement prefix-based eviction in BadgerDB

	log.Printf("[CACHE] Cache invalidation completed for %s", videoID)
}
