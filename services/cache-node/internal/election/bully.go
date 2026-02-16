package election

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	ElectionTimeout    = 5 * time.Second
	HeartbeatTimeout   = 3 * time.Second
	ResponseTimeout    = 2 * time.Second
	ElectionPortOffset = 1000
)

type BullyElection struct {
	mu                 sync.RWMutex
	nodeID             int
	address            string
	port               int
	nodes              map[int]NodeInfo // all nodes in cluster
	leaderID           int
	state              ElectionState
	term               int
	lastHeartbeat      time.Time
	electionInProgress bool
	stopCh             chan struct{}
	client             *http.Client
	leaderCallbacks    []func(newLeaderID int)
	server             *http.Server
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
	go b.startElectionServer()
	go b.monitorLeader()

	go func() {
		time.Sleep(1 * time.Second)
		b.startElection()
	}()

	log.Printf("[bully] node %d started", b.nodeID)

	return nil
}

func (b *BullyElection) Stop() {
	close(b.stopCh)

	// shutdown election server gracefully
	if b.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		b.server.Shutdown(ctx)
	}

	// wait a bit for goroutines to finish
	time.Sleep(100 * time.Millisecond)

	log.Printf("[bully] node %d stopped", b.nodeID)
}

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

// monitorLeader checks if we're still receiving heartbeats from the leader.
// if heartbeat timeout occurs, we start a new election
func (b *BullyElection) monitorLeader() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.mu.RLock()
			currentState := b.state
			lastHeartbeat := b.lastHeartbeat
			b.mu.RUnlock()

			// if we're the leader, no need to monitor ourselves
			if currentState == StateLeader {
				continue
			}

			// detect leader failure - check if too much time passed since last heartbeat
			if time.Since(lastHeartbeat) > HeartbeatTimeout {
				log.Printf("[bully] Leader heartbeat timeout. Starting election...")
				go b.startElection()
			}

		case <-b.stopCh:
			return
		}
	}
}

func (b *BullyElection) startElection() {
	b.mu.Lock()

	if b.electionInProgress {
		b.mu.Unlock()
		return
	}

	log.Printf("[bully] node %d starting election", b.nodeID)

	b.state = StateCandidate
	b.electionInProgress = true
	b.lastHeartbeat = time.Now()
	b.term++
	term := b.term
	b.mu.Unlock()

	higherNodes := b.getNodesWithHigherID()

	// if no higher nodes, become leader immediately
	if len(higherNodes) == 0 {
		b.mu.Lock()
		b.electionInProgress = false
		b.becomeLeader()
		b.mu.Unlock()
		return
	}

	// send election messages to all higher nodes
	responseCh := make(chan bool, len(higherNodes))

	for _, node := range higherNodes {
		go func(n NodeInfo) {
			ok := b.sendElectionMsg(n, term)
			responseCh <- ok
		}(node)
	}

	// wait for all responses or timeout
	responsesReceived := 0
	anyHigherNodeAlive := false
	timeout := time.After(ElectionTimeout)

	for responsesReceived < len(higherNodes) {
		select {
		case response := <-responseCh:
			responsesReceived++
			if response {
				anyHigherNodeAlive = true
			}

		case <-timeout:
			log.Printf("[bully] node %d election timeout after %d/%d responses",
				b.nodeID, responsesReceived, len(higherNodes))
			goto ElectionComplete
		}
	}

ElectionComplete:
	b.mu.Lock()
	b.electionInProgress = false

	if !anyHigherNodeAlive {
		// no higher node responded, i win
		b.becomeLeader()
	} else {
		// some higher node is alive, they will become leader
		b.state = StateFollower
		log.Printf("[bully] node %d waiting for victory announcement", b.nodeID)
	}

	b.mu.Unlock()
}

func (b *BullyElection) becomeLeader() {
	log.Printf("[bully] Node %d became leader", b.nodeID)

	b.state = StateLeader
	b.leaderID = b.nodeID
	b.lastHeartbeat = time.Now()

	// notify listeners
	for _, cb := range b.leaderCallbacks {
		go cb(b.nodeID)
	}

	// broadcast victory
	go b.broadcastVictory()

	// start sending heartbeats
	go b.sendHeartbeats()
}

func (b *BullyElection) broadcastVictory() {
	for _, node := range b.nodes {
		if node.ID == b.nodeID {
			continue
		}

		msg := ElectionMessage{
			Type:      MsgVictory,
			SenderID:  b.nodeID,
			Term:      b.term,
			Timestamp: time.Now(),
		}

		// fire and forget, if they don't get it they'll timeout and start election
		go b.sendMessage(node, "/election/victory", msg)
	}

	log.Printf("[bully] victory message broadcasted to %d nodes", len(b.nodes)-1)
}

func (b *BullyElection) sendHeartbeats() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.mu.RLock()
			if b.state != StateLeader {
				b.mu.RUnlock()
				return
			}
			b.mu.RUnlock()

			for _, node := range b.nodes {
				if node.ID == b.nodeID {
					continue
				}

				msg := ElectionMessage{
					Type:      MsgHeartbeat,
					SenderID:  b.nodeID,
					Term:      b.term,
					Timestamp: time.Now(),
				}

				go b.sendMessage(node, "/election/heartbeat", msg)
			}

		case <-b.stopCh:
			log.Printf("[bully] stopping heartbeat sender")
			return
		}
	}
}

func (b *BullyElection) getNodesWithHigherID() []NodeInfo {
	higherNodes := make([]NodeInfo, 0)
	for id, node := range b.nodes {
		if id > b.nodeID {
			higherNodes = append(higherNodes, node)
		}
	}

	return higherNodes
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
	url := fmt.Sprintf("http://%s:%d%s", node.Address, node.Port+ElectionPortOffset, endpoint)

	reqBody, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[bully] failed to marshal message: %v", err)
		return false
	}

	resp, err := b.client.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		// only log if it's not expected (not during election when nodes might be down)
		if msg.Type != MsgElection {
			log.Printf("[bully] failed to send %v to node %d: %v", msg.Type, node.ID, err)
		}
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (b *BullyElection) startElectionServer() {
	mux := http.NewServeMux()

	mux.HandleFunc("/election/election", b.handleElectionMsg)
	mux.HandleFunc("/election/victory", b.handleVictoryMsg)
	mux.HandleFunc("/election/heartbeat", b.handleHeartbeatMsg)

	addr := fmt.Sprintf(":%d", b.port+ElectionPortOffset)

	b.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("[bully] election server listening on %s", addr)

	if err := b.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("[bully] election server error: %v", err)
	}
}

func (b *BullyElection) handleElectionMsg(w http.ResponseWriter, r *http.Request) {
	var msg ElectionMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		log.Printf("[bully] bad election message: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("[bully] node %d got election message from node %d", b.nodeID, msg.SenderID)

	// if we're already leader, just respond without starting new election
	b.mu.RLock()
	isLeader := b.state == StateLeader
	b.mu.RUnlock()

	if isLeader {
		// already leader, just ack to tell them we're alive
		w.WriteHeader(http.StatusOK)
		return
	}

	if b.nodeID > msg.SenderID {
		// i have higher id, tell them and start my own election
		response := ElectionMessage{
			Type:      MsgAnswer,
			SenderID:  b.nodeID,
			Timestamp: time.Now(),
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

		// Only start election if we don't have a recent leader
		// avoid unnecessary election storms when we have a stable leader
		if time.Since(b.lastHeartbeat) > HeartbeatTimeout {
			go b.startElection()
		}
		return
	}

	// my id is lower, just ack
	w.WriteHeader(http.StatusOK)
}

func (b *BullyElection) handleVictoryMsg(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var msg ElectionMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("[bully] node %d acknowledges leader %d", b.nodeID, msg.SenderID)

	b.leaderID = msg.SenderID
	b.state = StateFollower
	b.lastHeartbeat = time.Now()

	w.WriteHeader(http.StatusOK)
}

func (b *BullyElection) handleHeartbeatMsg(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var msg ElectionMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	b.leaderID = msg.SenderID
	b.state = StateFollower
	b.lastHeartbeat = time.Now()

	w.WriteHeader(http.StatusOK)
}
