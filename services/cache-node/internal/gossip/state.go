package gossip

import (
	"sync"
	"time"
)

type GossipState struct {
	mu          sync.RWMutex
	nodeID      string
	inventory   map[string]*CacheInventory // nodeID -> inventory
	vectorClock map[string]int             // Clock per node
	lastUpdate  map[string]time.Time       // Last update from each node
}

func NewGossipState(nodeID string) *GossipState {
	return &GossipState{
		nodeID:      nodeID,
		inventory:   make(map[string]*CacheInventory),
		vectorClock: make(map[string]int),
		lastUpdate:  make(map[string]time.Time),
	}
}

func (gs *GossipState) GetDigest() map[string]int {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	digest := make(map[string]int)
	for nodeID, clock := range gs.vectorClock {
		digest[nodeID] = clock
	}

	return digest
}

func (gs *GossipState) GetInventory(nodeID string) (*CacheInventory, bool) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	inv, exists := gs.inventory[nodeID]
	return inv, exists
}

func (gs *GossipState) UpdateInventory(nodeID string, inv *CacheInventory) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.inventory[nodeID] = inv
	gs.vectorClock[nodeID] = inv.Version
	gs.lastUpdate[nodeID] = time.Now()
}

func (gs *GossipState) AddSegmentToInventory(nodeID, segmentID string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if _, exists := gs.inventory[nodeID]; !exists {
		gs.inventory[nodeID] = &CacheInventory{
			NodeID:   nodeID,
			Segments: make(map[string]bool),
			Version:  0,
		}
	}

	gs.inventory[nodeID].Segments[segmentID] = true
	gs.inventory[nodeID].Version++
	gs.vectorClock[nodeID] = gs.inventory[nodeID].Version
	gs.lastUpdate[nodeID] = time.Now()
}

func (gs *GossipState) FindNodesWithSegment(segmentID string) []string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	nodes := make([]string, 0)
	for nodeID, inv := range gs.inventory {
		if inv.Segments[segmentID] {
			nodes = append(nodes, nodeID)
		}
	}

	return nodes
}

func (gs *GossipState) CompareTo(remoteDigest map[string]int) []string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	diff := make([]string, 0)

	// Find nodes where remote is ahead
	for nodeID, remoteVersion := range remoteDigest {
		localVersion, exists := gs.vectorClock[nodeID]
		if !exists || remoteVersion > localVersion {
			diff = append(diff, nodeID)
		}
	}

	return diff
}

func (gs *GossipState) GetMyInventory() *CacheInventory {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	inv, exists := gs.inventory[gs.nodeID]
	if !exists {
		return &CacheInventory{
			NodeID:   gs.nodeID,
			Segments: make(map[string]bool),
			Load:     0.0,
			Version:  0,
		}
	}

	return inv
}

func (gs *GossipState) SetMyInventory(segments map[string]bool, load float64) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if _, exists := gs.inventory[gs.nodeID]; !exists {
		gs.inventory[gs.nodeID] = &CacheInventory{
			NodeID:   gs.nodeID,
			Segments: make(map[string]bool),
			Version:  0,
		}
	}

	gs.inventory[gs.nodeID].Segments = segments
	gs.inventory[gs.nodeID].Load = load
	gs.inventory[gs.nodeID].Version++
	gs.vectorClock[gs.nodeID] = gs.inventory[gs.nodeID].Version
	gs.lastUpdate[gs.nodeID] = time.Now()
}

func (gs *GossipState) CleanupStaleEntries(timeout time.Duration) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	now := time.Now()
	for nodeID, last := range gs.lastUpdate {
		if now.Sub(last) > timeout {
			delete(gs.inventory, nodeID)
			delete(gs.vectorClock, nodeID)
			delete(gs.lastUpdate, nodeID)
		}
	}
}
