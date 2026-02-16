package service

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/coordination"
	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/gossip"
	"github.com/dgraph-io/badger/v4"
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
	// Construct peer URL from peerID
	// peerID format: "cache-1", "cache-2", "cache-3"
	// Port mapping: cache-1 -> 8081, cache-2 -> 8082, cache-3 -> 8083
	peerURL := fmt.Sprintf("http://%s:8081", peerID)
	if peerID == "cache-2" {
		peerURL = "http://cache-2:8082"
	} else if peerID == "cache-3" {
		peerURL = "http://cache-3:8083"
	}

	// segmentID format: "/videos/wolf-1770292891/segment_0000.m4s"
	// Convert to /hls/ path for peer API: "/hls/wolf-1770292891/segment_0000.m4s"
	path := segmentID
	if len(segmentID) > 7 && segmentID[:7] == "/videos" {
		path = "/hls" + segmentID[7:] // replace /videos with /hls
	}

	url := peerURL + path
	log.Printf("[CACHE] Fetching %s from peer %s: %s", segmentID, peerID, url)

	resp, err := e.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("peer fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("peer returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
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

	// Delete segments matching /videos/{videoID}/
	prefix := fmt.Sprintf("/videos/%s", videoID)

	err := e.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		deleteKeys := make([][]byte, 0)
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			deleteKeys = append(deleteKeys, key)
		}

		// Delete all matching keys
		for _, key := range deleteKeys {
			if err := txn.Delete(key); err != nil {
				return err
			}
			log.Printf("[%s] Delete cache entry: %s", e.NodeID, string(key))
		}

		return nil
	})

	if err != nil {
		log.Printf("[CACHE] Failed to invalidate cache for video %s: %v", videoID, err)
	} else {
		log.Printf("[CACHE] Invalidated cache for video %s", videoID)
	}
}

// RequestFetchLock handles lock requests (leader only)
func (e *EdgeCacheCoordinated) RequestFetchLock(segmentID, nodeID string) (bool, string) {
	return e.coordinator.RequestLeaderLock(segmentID, nodeID)
}

// ReleaseFetchLock handles lock release (leader only)
func (e *EdgeCacheCoordinated) ReleaseFetchLock(segmentID, nodeID string) error {
	return e.coordinator.ReleaseLeaderLock(segmentID, nodeID)
}
