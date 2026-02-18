package ring

import (
	"hash/fnv"
	"log"
	"net/http"
	"sync"
	"time"
)

type Node struct {
	ID      string
	Address string
	Load    int
	Healthy bool
}

type NodeStatus struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Load    int    `json:"load"`
	Healthy bool   `json:"healthy"`
}

type BoundedLoadHashRing struct {
	mu            sync.RWMutex
	nodes         []*Node
	virtualNodes  int
	maxLoadFactor float64
	httpClient    *http.Client
}

func NewBoundedLoadHashRing(virtualNodes int, maxLoadFactor float64) *BoundedLoadHashRing {
	return &BoundedLoadHashRing{
		nodes:         make([]*Node, 0),
		virtualNodes:  virtualNodes,
		maxLoadFactor: maxLoadFactor,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (b *BoundedLoadHashRing) AddNode(id, address string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	node := &Node{
		ID:      id,
		Address: address,
		Load:    0,
		Healthy: true,
	}

	b.nodes = append(b.nodes, node)
	log.Printf("Added node: %s (%s)", id, address)
}

func (b *BoundedLoadHashRing) GetNode(key string) *Node {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.nodes) == 0 {
		return nil
	}

	avgLoad := 0.0
	healthyCount := 0
	for _, node := range b.nodes {
		if node.Healthy {
			avgLoad += float64(node.Load)
			healthyCount++
		}
	}

	if healthyCount > 0 {
		avgLoad /= float64(healthyCount)
	}

	maxLoad := avgLoad * b.maxLoadFactor
	if maxLoad < 1 {
		maxLoad = 1
	}

	hash := hashString(key)

	// Try to find healthy node under load limit
	for i := 0; i < len(b.nodes); i++ {
		idx := (int(hash) + i) % len(b.nodes)
		node := b.nodes[idx]

		if node.Healthy && float64(node.Load) < maxLoad {
			return node
		}
	}

	// Fallback: return least loaded healthy node
	var leastLoaded *Node
	for _, node := range b.nodes {
		if node.Healthy {
			if leastLoaded == nil || node.Load < leastLoaded.Load {
				leastLoaded = node
			}
		}
	}

	return leastLoaded
}

func (b *BoundedLoadHashRing) IncrementLoad(nodeID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, node := range b.nodes {
		if node.ID == nodeID {
			node.Load++
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
