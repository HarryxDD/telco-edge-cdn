package ring

import (
	"fmt"
	"hash/fnv"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"
)

type Node struct {
	ID           string
	Address      string
	Load         int // Active connections (kept for backward compatibility)
	Healthy      bool
	requestCount int64     // Total requests in current window
	windowStart  time.Time // Window start time
}

type NodeStatus struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Load    int    `json:"load"`
	Healthy bool   `json:"healthy"`
}

// VirtualNode represents a position on the hash ring
type VirtualNode struct {
	hash uint32
	node *Node
}

type BoundedLoadHashRing struct {
	mu            sync.RWMutex
	nodes         []*Node
	ring          []VirtualNode // Sorted ring of virtual nodes
	virtualNodes  int
	maxLoadFactor float64
	httpClient    *http.Client
	windowSize    time.Duration // Time window for rate calculation
}

const (
	defaultWindowSize = 10 * time.Second // Track load over 10-second windows
)

func NewBoundedLoadHashRing(virtualNodes int, maxLoadFactor float64) *BoundedLoadHashRing {
	return &BoundedLoadHashRing{
		nodes:         make([]*Node, 0),
		ring:          make([]VirtualNode, 0),
		virtualNodes:  virtualNodes,
		maxLoadFactor: maxLoadFactor,
		windowSize:    defaultWindowSize,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (b *BoundedLoadHashRing) AddNode(id, address string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	node := &Node{
		ID:           id,
		Address:      address,
		Load:         0,
		Healthy:      true,
		requestCount: 0,
		windowStart:  time.Now(),
	}

	b.nodes = append(b.nodes, node)

	// Add virtual nodes to the ring (Cassandra/DynamoDB approach)
	for i := 0; i < b.virtualNodes; i++ {
		virtualKey := fmt.Sprintf("%s-vnode-%d", id, i)
		hash := hashString(virtualKey)
		b.ring = append(b.ring, VirtualNode{
			hash: hash,
			node: node,
		})
	}

	// Keep ring sorted by hash value for binary search
	sort.Slice(b.ring, func(i, j int) bool {
		return b.ring[i].hash < b.ring[j].hash
	})

	log.Printf("Added node: %s (%s) with %d virtual nodes, ring size: %d", 
		id, address, b.virtualNodes, len(b.ring))
}

func (b *BoundedLoadHashRing) getNodeRequestRate(node *Node) float64 {
	// Calculate requests per second over the current window
	now := time.Now()
	elapsed := now.Sub(node.windowStart).Seconds()

	if elapsed < 0.1 {
		// Too soon, return current rate estimate
		return float64(node.requestCount) / 0.1
	}

	// If window expired, treat as zero (will be reset on next IncrementLoad)
	if elapsed >= b.windowSize.Seconds() {
		return 0
	}

	return float64(node.requestCount) / elapsed
}

func (b *BoundedLoadHashRing) GetNode(key string) *Node {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.ring) == 0 {
		return nil
	}

	// Calculate average request rate across healthy nodes
	avgRate := 0.0
	healthyCount := 0
	nodeRates := make(map[string]float64)
	for _, node := range b.nodes {
		if node.Healthy {
			rate := b.getNodeRequestRate(node)
			nodeRates[node.ID] = rate
			avgRate += rate
			healthyCount++
		}
	}

	if healthyCount > 0 {
		avgRate /= float64(healthyCount)
	}

	// Maximum allowed rate per node (bounded-load constraint)
	maxRate := avgRate * b.maxLoadFactor
	if maxRate < 0.1 {
		maxRate = 0.1 // Minimum threshold
	}

	hash := hashString(key)

	// Find the first virtual node >= hash using binary search (O(log N))
	idx := sort.Search(len(b.ring), func(i int) bool {
		return b.ring[i].hash >= hash
	})

	// Wrap around if we went past the end
	if idx >= len(b.ring) {
		idx = 0
	}

	firstChoice := b.ring[idx].node
	
	// Try virtual nodes in order (consistent hashing with bounded load)
	tried := make(map[string]bool)
	for i := 0; i < len(b.ring); i++ {
		vIdx := (idx + i) % len(b.ring)
		vnode := b.ring[vIdx]
		node := vnode.node

		// Skip if already tried this physical node
		if tried[node.ID] {
			continue
		}
		tried[node.ID] = true

		if node.Healthy {
			currentRate := b.getNodeRequestRate(node)

			if currentRate < maxRate {
				// Log when bounded-load causes rerouting
				if node.ID != firstChoice.ID {
					log.Printf("BOUNDED-LOAD REROUTE: %s (%.2f req/s) → %s (%.2f req/s) | avgRate=%.2f, maxRate=%.2f",
						firstChoice.ID, nodeRates[firstChoice.ID], node.ID, currentRate, avgRate, maxRate)
				}
				return node
			}
		}
	}

	// Fallback: return node with lowest request rate
	var leastLoaded *Node
	minRate := float64(1e9)

	for _, node := range b.nodes {
		if node.Healthy {
			rate := b.getNodeRequestRate(node)
			if rate < minRate {
				minRate = rate
				leastLoaded = node
			}
		}
	}

	return leastLoaded
}

func (b *BoundedLoadHashRing) IncrementLoad(nodeID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	for _, node := range b.nodes {
		if node.ID == nodeID {
			// Check if we need to reset the window
			elapsed := now.Sub(node.windowStart)
			if elapsed >= b.windowSize {
				node.requestCount = 0
				node.windowStart = now
			}

			node.Load++         // Keep for backward compatibility
			node.requestCount++ // Increment request count in current window
			break
		}
	}
}

func (b *BoundedLoadHashRing) DecrementLoad(nodeID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, node := range b.nodes {
		if node.ID == nodeID {
			if node.Load > 0 {
				node.Load--
			}
			break
		}
	}
}

func (b *BoundedLoadHashRing) HealthCheck() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		b.mu.Lock()
		for _, node := range b.nodes {
			resp, err := b.httpClient.Get("http://" + node.Address + "/health")

			healthy := false
			if err == nil && resp != nil && resp.StatusCode == 200 {
				healthy = true
			}

			if resp != nil {
				resp.Body.Close()
			}

			if !healthy {
				if node.Healthy {
					log.Printf("Node %s is DOWN (err: %v)", node.ID, err)
				}
				node.Healthy = false
			} else {
				if !node.Healthy {
					log.Printf("Node %s is UP", node.ID)
				}
				node.Healthy = true
			}
		}
		b.mu.Unlock()
	}
}

func (b *BoundedLoadHashRing) DumpStatus() []NodeStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()

	statuses := make([]NodeStatus, 0, len(b.nodes))
	for _, n := range b.nodes {
		statuses = append(statuses, NodeStatus{
			ID:      n.ID,
			Address: n.Address,
			Load:    n.Load,
			Healthy: n.Healthy,
		})
	}

	return statuses
}

func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
