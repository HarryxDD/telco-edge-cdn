package coordination

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/election"
	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/gossip"
)

// inflightRequest tracks a single in-flight fetch request
type inflightRequest struct {
	data []byte
	peer string
	err  error
	done chan struct{}
}

// Coordinator integrates leader election, gossip, and lock management
// for distributed cache coordination
type Coordinator struct {
	nodeID      string
	election    election.LeaderElection
	gossip      gossip.GossipProtocol
	leaderLocks *LockManager

	// Local in-flight request deduplication (prevents local stampede)
	inflightMu sync.Mutex
	inflight   map[string]*inflightRequest

	// Callback for fetching from origin
	onCacheMiss func(segmentID string) ([]byte, error)
}

func NewCoordinator(
	nodeID string,
	elec election.LeaderElection,
	gosp gossip.GossipProtocol,
) *Coordinator {
	coord := &Coordinator{
		nodeID:      nodeID,
		election:    elec,
		gossip:      gosp,
		leaderLocks: NewLockManager(),
		inflight:    make(map[string]*inflightRequest),
	}

	// Register leader change callback
	elec.RegisterLeaderChangeCallback(coord.onLeaderChange)

	// Start cleanup goroutine for expired locks
	go coord.cleanupLoop()

	return coord
}

func (c *Coordinator) Start() error {
	if err := c.election.Start(); err != nil {
		return fmt.Errorf("failed to start election: %w", err)
	}

	if err := c.gossip.Start(); err != nil {
		return fmt.Errorf("failed to start gossip: %w", err)
	}

	log.Printf("[COORDINATOR] Started for node %s", c.nodeID)
	return nil
}

func (c *Coordinator) Stop() {
	c.election.Stop()
	c.gossip.Stop()
}

// RegisterCallbacks registers callback for fetching from origin
func (c *Coordinator) RegisterCallbacks(
	onCacheMiss func(segmentID string) ([]byte, error),
) {
	c.onCacheMiss = onCacheMiss
}

func (c *Coordinator) HandleCacheMiss(segmentID string) ([]byte, string, error) {
	// LOCAL DEDUPLICATION: Check if another goroutine in this node is already fetching
	c.inflightMu.Lock()
	if req, exists := c.inflight[segmentID]; exists {
		// Another goroutine is fetching, wait for it
		c.inflightMu.Unlock()
		log.Printf("[COORDINATOR] Waiting for local in-flight fetch of %s", segmentID)
		<-req.done
		return req.data, req.peer, req.err
	}

	// We're the first - create in-flight tracker
	req := &inflightRequest{
		done: make(chan struct{}),
	}
	c.inflight[segmentID] = req
	c.inflightMu.Unlock()

	// Ensure cleanup
	defer func() {
		c.inflightMu.Lock()
		delete(c.inflight, segmentID)
		c.inflightMu.Unlock()
		close(req.done)
	}()

	// Check if any peer has it via gossip
	if peerID, found := c.gossip.FindPeerWithSegment(segmentID); found {
		log.Printf("[COORDINATOR] Segment %s found at peer %s", segmentID, peerID)
		req.peer = peerID
		return nil, peerID, nil
	}

	// No peer has it - request lock to fetch from origin
	granted, fetchingNode := c.requestFetchLockFromLeader(segmentID)

	// TODO: Future improvement - implement request queueing during leader election
	// If fetchingNode is empty, it means leader is unavailable. Currently this will
	// timeout after 6s. A production system would queue these requests and process
	// them after election completes (similar to Kafka/etcd approach).

	if granted {
		log.Printf("[COORDINATOR] Got lock for %s, fetching from origin", segmentID)

		// Fetch from origin
		data, err := c.onCacheMiss(segmentID)
		if err != nil {
			c.releaseFetchLockToLeader(segmentID)
			req.err = err
			return nil, "", err
		}

		// Release lock and broadcast to cluster
		c.releaseFetchLockToLeader(segmentID)
		c.NotifyCacheAdd(segmentID, int64(len(data)))

		req.data = data
		return data, "", nil
	}

	// Lock denied - another node is fetching
	// Wait with retries for the fetching node to broadcast
	log.Printf("[COORDINATOR] Node %s is fetching %s, waiting for broadcast...", fetchingNode, segmentID)

	// Retry up to 6 times (1s intervals = 6s max wait, enough for origin fetch)
	for i := 0; i < 6; i++ {
		time.Sleep(1 * time.Second)

		if peerID, found := c.gossip.FindPeerWithSegment(segmentID); found {
			log.Printf("[COORDINATOR] Segment %s now available at peer %s (after %ds)", segmentID, peerID, i+1)
			req.peer = peerID
			return nil, peerID, nil
		}
	}

	req.err = fmt.Errorf("timeout waiting for segment %s after 6s", segmentID)
	return nil, "", req.err
}

// NotifyCacheAdd broadcasts to cluster that this node has a segment
func (c *Coordinator) NotifyCacheAdd(segmentID string, size int64) {
	if eg, ok := c.gossip.(*gossip.EpidemicGossip); ok {
		eg.NotifyCacheAdd(segmentID, size)
	}
}

// Internal Methods
func (c *Coordinator) onLeaderChange(newLeaderID int) {
	log.Printf("[COORDINATOR] Leader changed to %d", newLeaderID)
	if c.election.IsLeader() {
		log.Printf("[COORDINATOR] I am now the leader")
	}
}

func (c *Coordinator) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if c.election.IsLeader() {
			c.leaderLocks.CleanupExpiredLocks()
		}
	}
}

// Public API Methods (for HTTP handlers)
func (c *Coordinator) IsLeader() bool {
	return c.election.IsLeader()
}

func (c *Coordinator) GetLeaderID() int {
	return c.election.GetLeaderID()
}

func (c *Coordinator) GetState() string {
	state := c.election.GetState()
	switch state {
	case election.StateLeader:
		return "leader"
	case election.StateFollower:
		return "follower"
	case election.StateCandidate:
		return "candidate"
	default:
		return "unknown"
	}
}

// RequestLeaderLock handles lock requests from followers (leader only)
func (c *Coordinator) RequestLeaderLock(segmentID, nodeID string) (bool, string) {
	return c.leaderLocks.RequestFetchLock(segmentID, nodeID, 30*time.Second)
}

// ReleaseLeaderLock handles lock release from followers (leader only)
func (c *Coordinator) ReleaseLeaderLock(segmentID, nodeID string) error {
	return c.leaderLocks.ReleaseFetchLock(segmentID, nodeID)
}
