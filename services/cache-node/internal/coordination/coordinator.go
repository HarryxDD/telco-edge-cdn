package coordination

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/election"
	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/gossip"
	"github.com/gin-gonic/gin"
)

// Coordinator integrates leader election and gossip for cache coordination
type Coordinator struct {
	nodeID      string
	election    election.LeaderElection
	gossip      gossip.GossipProtocol
	lockManager *LockManager
	leaderLocks *LockManager // Only leader uses this

	// Callbacks
	onCacheMiss  func(segmentID string) ([]byte, error)
	onPreFetch   func(cmd gossip.PreFetchCommand)
	onInvalidate func(videoID string)
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
		lockManager: NewLockManager(),
		leaderLocks: NewLockManager(),
	}

	// Register leader change callback
	elec.RegisterLeaderChangeCallback(coord.onLeaderChange)

	// Register gossip handlers
	gosp.RegisterHandler(gossip.MsgPreFetch, coord.handlePreFetchGossip)
	gosp.RegisterHandler(gossip.MsgCacheInvalidate, coord.handleInvalidateGossip)

	// Start cleanup goroutine
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

// HandleCacheMiss coordinates cache miss handling with leader
func (c *Coordinator) HandleCacheMiss(segmentID string) ([]byte, string, error) {
	// Check if any peer has it via gossip
	if peerID, found := c.gossip.FindPeerWithSegment(segmentID); found {
		log.Printf("[COORDINATOR] Segment %s found at peer %s", segmentID, peerID)
		return nil, peerID, nil
	}

	// If no peer has it, we need to fetch from origin
	// Request lock from leader
	granted, fetchingNode := c.requestFetchLockFromLeader(segmentID)
	if granted {
		// got the lock, fetch from origin
		log.Printf("[COORDINATOR] Got lock for %s, fetching from origin", segmentID)

		data, err := c.onCacheMiss(segmentID)
		if err != nil {
			c.releaseFetchLockToLeader(segmentID)
			return nil, "", err
		}

		// Release lock
		c.releaseFetchLockToLeader(segmentID)

		// Gossip that the node has it
		c.NotifyCacheAdd(segmentID, int64(len(data)))

		return data, "", nil
	}

	// Someone else is fetching, wait for gossip
	log.Printf("[COORDINATOR] Node %s is fetching %s, waiting...", fetchingNode, segmentID)

	if c.lockManager.WaitForSegment(segmentID, 10*time.Second) {
		// Check gossip again
		if peerID, found := c.gossip.FindPeerWithSegment(segmentID); found {
			log.Printf("[COORDINATOR] Segment %s now available at peer %s", segmentID, peerID)
			return nil, peerID, nil
		}
	}

	return nil, "", fmt.Errorf("timeout waiting for segment %s", segmentID)
}

func (c *Coordinator) NotifyCacheAdd(segmentID string, size int64) {
	if eg, ok := c.gossip.(*gossip.EpidemicGossip); ok {
		eg.NotifyCacheAdd(segmentID, size)
	}
}

// BroadcastPreFetch broadcasts AI pre-fetch command (leader only)
func (c *Coordinator) BroadcastPreFetch(cmd gossip.PreFetchCommand) {
	if !c.election.IsLeader() {
		log.Printf("[COORDINATOR] Not leader, cannot broadcast pre-fetch")
		return
	}

	msg := gossip.GossipMessage{
		Type: gossip.MsgPreFetch,
		Data: cmd,
	}

	c.gossip.Broadcast(msg)
	log.Printf("[COORDINATOR] Leader broadcast pre-fetch for video %s", cmd.VideoID)
}

// BroadcastInvalidate broadcasts cache invalidation (leader only)
func (c *Coordinator) BroadcastInvalidate(videoID string) {
	if !c.election.IsLeader() {
		log.Printf("[COORDINATOR] Not leader, cannot broadcast invalidate")
		return
	}

	msg := gossip.GossipMessage{
		Type: gossip.MsgCacheInvalidate,
		Data: gossip.CacheInvalidateNotification{
			VideoID:   videoID,
			Timestamp: time.Now(),
		},
	}

	c.gossip.Broadcast(msg)
	log.Printf("[COORDINATOR] Leader broadcast invalidate for video %s", videoID)
}

// RegisterCallbacks registers callbacks for cache operations
func (c *Coordinator) RegisterCallbacks(
	onCacheMiss func(segmentID string) ([]byte, error),
	onPreFetch func(cmd gossip.PreFetchCommand),
	onInvalidate func(videoID string),
) {
	c.onCacheMiss = onCacheMiss
	c.onPreFetch = onPreFetch
	c.onInvalidate = onInvalidate
}

// Internal methods
func (c *Coordinator) onLeaderChange(newLeaderID int) {
	log.Printf("[COORDINATOR] Leader changed to %d", newLeaderID)

	if c.election.IsLeader() {
		log.Printf("[COORDINATOR] I'm the leader")
		// Start leader-specific tasks if needed
	}
}

func (c *Coordinator) requestFetchLockFromLeader(segmentID string) (bool, string) {
	if c.election.IsLeader() {
		return c.leaderLocks.RequestFetchLock(segmentID, c.nodeID, 30*time.Second)
	}

	// TODO: Send RPC to leader to request lock
	// For now, simplified: each node manages its own locks
	return c.lockManager.RequestFetchLock(segmentID, c.nodeID, 30*time.Second)
}

func (c *Coordinator) releaseFetchLockToLeader(segmentID string) {
	if c.election.IsLeader() {
		c.leaderLocks.ReleaseFetchLock(segmentID, c.nodeID)
	} else {
		c.lockManager.ReleaseFetchLock(segmentID, c.nodeID)
	}
}

func (c *Coordinator) handlePreFetchGossip(msg gossip.GossipMessage) {
	cmd, ok := msg.Data.(gossip.PreFetchCommand)
	if !ok {
		return
	}

	log.Printf("[COORDINATOR] Received pre-fetch command for %s", cmd.VideoID)

	if c.onPreFetch != nil {
		c.onPreFetch(cmd)
	}
}

func (c *Coordinator) handleInvalidateGossip(msg gossip.GossipMessage) {
	notif, ok := msg.Data.(gossip.CacheInvalidateNotification)
	if !ok {
		return
	}

	log.Printf("[COORDINATOR] Received invalidate for video %s", notif.VideoID)

	if c.onInvalidate != nil {
		c.onInvalidate(notif.VideoID)
	}
}

func (c *Coordinator) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.lockManager.CleanupExpiredLocks()
		if c.election.IsLeader() {
			c.leaderLocks.CleanupExpiredLocks()
		}
	}
}

// StartCoordinationAPI starts HTTP API for coordination (leader endpoints)
func (c *Coordinator) StartCoordinationAPI(port int) {
	router := gin.Default()

	// Leader endpoints
	router.POST("/coord/lock/request", c.handleLockRequest)
	router.POST("/coord/lock/release", c.handleLockRelease)
	router.GET("/coord/status", c.handleStatus)

	addr := fmt.Sprintf(":%d", port+3000) // Coordination port = node port + 3000
	log.Printf("[COORDINATOR] API listening on %s", addr)

	go router.Run(addr)
}

func (c *Coordinator) handleLockRequest(ctx *gin.Context) {
	var req struct {
		SegmentID string `json:"segment_id"`
		NodeID    string `json:"node_id"`
	}

	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !c.election.IsLeader() {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error":     "not leader",
			"leader_id": c.election.GetLeaderID(),
		})

		return
	}

	granted, fetchingNode := c.leaderLocks.RequestFetchLock(req.SegmentID, req.NodeID, 30*time.Second)

	ctx.JSON(http.StatusOK, gin.H{
		"granted":       granted,
		"fetching_node": fetchingNode,
	})
}

func (c *Coordinator) handleLockRelease(ctx *gin.Context) {
	var req struct {
		SegmentID string `json:"segment_id"`
		NodeID    string `json:"node_id"`
	}

	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !c.election.IsLeader() {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "not leader",
		})
		return
	}

	err := c.leaderLocks.ReleaseFetchLock(req.SegmentID, req.NodeID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "released"})
}

func (c *Coordinator) handleStatus(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"node_id":   c.nodeID,
		"is_leader": c.election.IsLeader(),
		"leader_id": c.election.GetLeaderID(),
		"state":     c.election.GetState(),
	})
}
