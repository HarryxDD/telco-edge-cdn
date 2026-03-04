package gossip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

const (
	GossipInterval    = 500 * time.Millisecond
	CleanupInterval   = 30 * time.Second
	StaleEntryTimeout = 60 * time.Second
	RequestTimeout    = 2 * time.Second
	GossipPortOffset  = 2000
)

type EpidemicGossip struct {
	mu       sync.RWMutex
	nodeID   string
	address  string
	port     int
	peers    map[string]NodeInfo // nodeID -> address:port
	state    *GossipState
	handlers map[GossipMessageType][]func(msg GossipMessage)
	client   *http.Client
	stopCh   chan struct{}
}

type NodeInfo struct {
	ID      string
	Address string
	Port    int
}

func NewEpidemicGossip(nodeID, address string, port int, peers map[string]NodeInfo) *EpidemicGossip {
	return &EpidemicGossip{
		nodeID:   nodeID,
		address:  address,
		port:     port,
		peers:    peers,
		state:    NewGossipState(nodeID),
		handlers: make(map[GossipMessageType][]func(msg GossipMessage)),
		stopCh:   make(chan struct{}),
		client: &http.Client{
			Timeout: RequestTimeout,
		},
	}
}

func (eg *EpidemicGossip) Start() error {
	go eg.startGossipServer()
	go eg.gossipLoop()
	go eg.cleanupLoop()

	log.Printf("[GOSSIP] Node %s started gossip protocol", eg.nodeID)
	return nil
}

func (eg *EpidemicGossip) Stop() {
	close(eg.stopCh)
}

func (eg *EpidemicGossip) Broadcast(msg GossipMessage) {
	msg.SenderID = eg.nodeID
	msg.Timestamp = time.Now()
	msg.VectorClock = eg.state.GetDigest()

	eg.mu.RLock()
	peers := make([]NodeInfo, 0, len(eg.peers))
	for _, peer := range eg.peers {
		peers = append(peers, peer)
	}
	eg.mu.RUnlock()

	for _, peer := range peers {
		go eg.SendTo(peer.ID, msg)
	}
}

func (eg *EpidemicGossip) SendTo(nodeID string, msg GossipMessage) error {
	eg.mu.RLock()
	peer, exists := eg.peers[nodeID]
	eg.mu.RUnlock()

	if !exists {
		return fmt.Errorf("peer %s not found", nodeID)
	}

	msg.SenderID = eg.nodeID
	msg.Timestamp = time.Now()
	msg.VectorClock = eg.state.GetDigest()

	return eg.sendMessage(peer, "/gossip/message", msg)
}

func (eg *EpidemicGossip) FindPeerWithSegment(segmentID string) (string, bool) {
	nodes := eg.state.FindNodesWithSegment(segmentID)
	if len(nodes) == 0 {
		return "", false
	}

	return nodes[rand.Intn(len(nodes))], true
}

func (eg *EpidemicGossip) RegisterHandler(msgType GossipMessageType, handler func(msg GossipMessage)) {
	eg.mu.Lock()
	defer eg.mu.Unlock()

	if _, exists := eg.handlers[msgType]; !exists {
		eg.handlers[msgType] = make([]func(msg GossipMessage), 0)
	}
	eg.handlers[msgType] = append(eg.handlers[msgType], handler)
}

func (eg *EpidemicGossip) GetInventory(nodeID string) (*CacheInventory, bool) {
	return eg.state.GetInventory(nodeID)
}

func (eg *EpidemicGossip) gossipLoop() {
	ticker := time.NewTicker(GossipInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			eg.performGossipRound()
		case <-eg.stopCh:
			return
		}
	}
}

func (eg *EpidemicGossip) performGossipRound() {
	eg.mu.RLock()
	if len(eg.peers) == 0 {
		eg.mu.RUnlock()
		return
	}

	// Pick random peer
	peers := make([]NodeInfo, 0, len(eg.peers))
	for _, peer := range eg.peers {
		peers = append(peers, peer)
	}
	eg.mu.RUnlock()

	peer := peers[rand.Intn(len(peers))]

	// Send digest
	digest := eg.state.GetDigest()
	msg := GossipMessage{
		Type:        MsgDigest,
		SenderID:    eg.nodeID,
		Data:        digest,
		VectorClock: digest,
		Timestamp:   time.Now(),
	}

	eg.sendMessage(peer, "/gossip/message", msg)
}

func (eg *EpidemicGossip) cleanupLoop() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			eg.state.CleanupStaleEntries(StaleEntryTimeout)
		case <-eg.stopCh:
			return
		}
	}
}

func (eg *EpidemicGossip) sendMessage(peer NodeInfo, endpoint string, msg GossipMessage) error {
	url := fmt.Sprintf("http://%s:%d%s", peer.Address, peer.Port+GossipPortOffset, endpoint)

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	resp, err := eg.client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gossip message failed: %d", resp.StatusCode)
	}

	return nil
}

func (eg *EpidemicGossip) startGossipServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/gossip/message", eg.handleGossipMsg)

	addr := fmt.Sprintf(":%d", eg.port+GossipPortOffset)
	log.Printf("[GOSSIP] Node %s starting gossip server on %s", eg.nodeID, addr)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[GOSSIP] Server error: %v", err)
	}
}

func (eg *EpidemicGossip) handleGossipMsg(w http.ResponseWriter, r *http.Request) {
	var msg GossipMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Process based on message type
	switch msg.Type {
	case MsgDigest:
		eg.handleDigest(msg, w)
	case MsgCacheAdd:
		eg.handleCacheAdd(msg)
	case MsgCacheInvalidate:
		eg.handleCacheInvalidate(msg)
	case MsgPreFetch:
		eg.handlePreFetch(msg)
	default:
		// Call registered handlers
		eg.mu.RLock()
		handlers := eg.handlers[msg.Type]
		eg.mu.RUnlock()

		for _, handler := range handlers {
			go handler(msg)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (eg *EpidemicGossip) handleDigest(msg GossipMessage, w http.ResponseWriter) {
	remoteDigest, ok := msg.Data.(map[string]int)
	if !ok {
		return
	}

	// Check if we need updates
	diff := eg.state.CompareTo(remoteDigest)
	if len(diff) > 0 {
		// Request missing data
		response := GossipMessage{
			Type:     MsgDiff,
			SenderID: eg.nodeID,
			Data:     diff,
		}
		json.NewEncoder(w).Encode(response)
	}
}

func (eg *EpidemicGossip) handleCacheAdd(msg GossipMessage) {
	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		return
	}

	nodeID, _ := data["node_id"].(string)
	segmentID, _ := data["segment_id"].(string)

	if nodeID != "" && segmentID != "" {
		eg.state.AddSegmentToInventory(nodeID, segmentID)
		log.Printf("[GOSSIP] Learned: Node %s cached %s", nodeID, segmentID)
	}
}

func (eg *EpidemicGossip) handleCacheInvalidate(msg GossipMessage) {
	eg.mu.RLock()
	handlers := eg.handlers[MsgCacheInvalidate]
	eg.mu.RUnlock()

	for _, handler := range handlers {
		go handler(msg)
	}
}

func (eg *EpidemicGossip) handlePreFetch(msg GossipMessage) {
	eg.mu.RLock()
	handlers := eg.handlers[MsgPreFetch]
	eg.mu.RUnlock()

	for _, handler := range handlers {
		go handler(msg)
	}
}

func (eg *EpidemicGossip) NotifyCacheAdd(segmentID string, size int64) {
	eg.state.AddSegmentToInventory(eg.nodeID, segmentID)

	msg := GossipMessage{
		Type: MsgCacheAdd,
		Data: map[string]interface{}{
			"node_id":    eg.nodeID,
			"segment_id": segmentID,
			"size":       size,
		},
	}

	eg.Broadcast(msg)
}
