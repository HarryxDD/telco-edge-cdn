package coordination

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// RequestLockFromLeader requests a fetch lock from the leader node.
// Returns (granted, fetchingNode) where fetchingNode is who currently holds the lock.
func (c *Coordinator) requestFetchLockFromLeader(segmentID string) (bool, string) {
	// If we are the leader, grant locally
	if c.election.IsLeader() {
		return c.leaderLocks.RequestFetchLock(segmentID, c.nodeID, 30*time.Second)
	}

	// Get leader address
	leaderID := c.election.GetLeaderID()
	if leaderID == -1 {
		log.Printf("[LOCK_CLIENT] No leader available")
		return false, ""
	}

	leaderURL := GetLeaderAddress(leaderID) + "/coordination/request-lock"

	// Prepare request
	type LockRequest struct {
		SegmentID string `json:"segment_id"`
		NodeID    string `json:"node_id"`
	}

	reqBody, _ := json.Marshal(LockRequest{
		SegmentID: segmentID,
		NodeID:    c.nodeID,
	})

	// Send HTTP request to leader
	resp, err := http.Post(leaderURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("[LOCK_CLIENT] Failed to request lock from leader: %v", err)
		return false, ""
	}
	defer resp.Body.Close()

	// Parse response
	type LockResponse struct {
		Granted      bool   `json:"granted"`
		FetchingNode string `json:"fetching_node"`
	}

	var lockResp LockResponse
	if err := json.NewDecoder(resp.Body).Decode(&lockResp); err != nil {
		log.Printf("[LOCK_CLIENT] Failed to decode lock response: %v", err)
		return false, ""
	}

	return lockResp.Granted, lockResp.FetchingNode
}

// ReleaseLockToLeader releases a fetch lock through the leader node
func (c *Coordinator) releaseFetchLockToLeader(segmentID string) {
	// If we are the leader, release locally
	if c.election.IsLeader() {
		c.leaderLocks.ReleaseFetchLock(segmentID, c.nodeID)
		return
	}

	// Get leader address
	leaderID := c.election.GetLeaderID()
	if leaderID == -1 {
		log.Printf("[LOCK_CLIENT] No leader available to release lock")
		return
	}

	leaderURL := GetLeaderAddress(leaderID) + "/coordination/release-lock"

	// Prepare request
	type ReleaseLockRequest struct {
		SegmentID string `json:"segment_id"`
		NodeID    string `json:"node_id"`
	}

	reqBody, _ := json.Marshal(ReleaseLockRequest{
		SegmentID: segmentID,
		NodeID:    c.nodeID,
	})

	// Send HTTP request to leader
	_, err := http.Post(leaderURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("[LOCK_CLIENT] Failed to release lock to leader: %v", err)
	}
}
