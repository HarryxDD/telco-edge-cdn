package election

import (
	"net/http"
	"sync"
	"time"
)

const (
	ElectionTimeout  = 5 * time.Second
	HeartbeatTimeout = 3 * time.Second
	ResponseTimeout  = 2 * time.Second
)

type BullyElection struct {
	mu                 sync.RWMutex
	nodeID             int
	address            string
	port               int
	nodes              map[int]NodeInfo // All nodes in cluster
	leaderID           int
	state              ElectionState
	term               int
	lastHeartbeat      time.Time
	electionInProgress bool
	stopCh             chan struct{}
	client             *http.Client
	leaderCallbacks    []func(newLeaderID int)
}

func NewBullyElection(nodeID int, address string, port int, nodes map[int]NodeInfo) *BullyElection {
	return &BullyElection{
		nodeID:          nodeID,
		address:         address,
		port:            port,
		nodes:           nodes,
		leaderID:        -1,
		state:           StateFollower,
		term:            0,
		lastHeartbeat:   time.Now(),
		stopCh:          make(chan struct{}),
		leaderCallbacks: make([]func(newLeaderID int), 0),
		client: &http.Client{
			Timeout: ResponseTimeout,
		},
	}
}

func (b *BullyElection) Start() error {
	return nil
}

func (b *BullyElection) Stop() {}

func (b *BullyElection) GetLeaderID() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.leaderID
}

func (b *BullyElection) IsLeader() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state == StateLeader
}

func (b *BullyElection) GetState() ElectionState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

func (b *BullyElection) RegisterLeaderChangeCallback(callback func(newLeaderID int)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.leaderCallbacks = append(b.leaderCallbacks, callback)
}

func (b *BullyElection) monitorLeader() {
	// Used for monitoring leader heartbeats and triggering elections if needed
}

func (b *BullyElection) handleElectionTimeout() {}

func (b *BullyElection) startElection() {}

func (b *BullyElection) becomeLeader() {}

func (b *BullyElection) broadcastVictory() {}

func (b *BullyElection) sendHeartbeats() {}

func (b *BullyElection) getNodesWithHigherID() []NodeInfo {
	higerNodes := make([]NodeInfo, 0)
	for id, node := range b.nodes {
		if id > b.nodeID {
			higerNodes = append(higerNodes, node)
		}
	}

	return higerNodes
}

func (b *BullyElection) sendElectionMsg(node NodeInfo, term int) bool {
	msg := ElectionMessage{
		Type:      MsgElection,
		SenderID:  b.nodeID,
		Term:      term,
		Timestamp: time.Now(),
	}

	return b.sendMessage(node, "/election/election", msg)
}

func (b *BullyElection) sendMessage(node NodeInfo, endpoint string, msg ElectionMessage) bool {
	return true
}

func (b *BullyElection) startElectionServer() {}

func (b *BullyElection) handleElectionMsg(w http.ResponseWriter, r *http.Request) {}

func (b *BullyElection) handleVictoryMsg(w http.ResponseWriter, r *http.Request) {}

func (b *BullyElection) handleHeartbeatMsg(w http.ResponseWriter, r *http.Request) {}
