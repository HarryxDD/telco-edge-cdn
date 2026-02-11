package gossip

import (
	"net/http"
	"sync"
	"time"
)

const (
	GossipInterval    = 500 * time.Millisecond
	CleanupInterval   = 30 * time.Second
	StaleEntryTimeout = 60 * time.Second
	RequestTimeout    = 2 * time.Second
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
	return nil
}

func (eg *EpidemicGossip) Stop() {
	close(eg.stopCh)
}

func (eg *EpidemicGossip) Broadcast(msg GossipMessage) {}

func (eg *EpidemicGossip) SendTo(nodeID string, msg GossipMessage) error {
	return nil
}

func (eg *EpidemicGossip) FindPeerWithSegment(segmentID string) (string, bool) {
	return "", false
}

func (eg *EpidemicGossip) RegisterHandler(msgType GossipMessageType, handler func(msg GossipMessage)) {
}

func (eg *EpidemicGossip) GetInventory(nodeID string) (*CacheInventory, bool) {
	return nil, false
}

func (eg *EpidemicGossip) gossipLoop() {}

func (eg *EpidemicGossip) performGossipRound() {}

func (eg *EpidemicGossip) cleanupLoop() {}

func (eg *EpidemicGossip) sendMessage(peer NodeInfo, endpoint string, msg GossipMessage) error {
	return nil
}

func (eg *EpidemicGossip) startGossipServer() {}

func (eg *EpidemicGossip) handleGossipMsg(w http.ResponseWriter, r *http.Request) {}

func (eg *EpidemicGossip) handleDigest(msg GossipMessage, w http.ResponseWriter) {}

func (eg *EpidemicGossip) handleCacheAdd(msg GossipMessage) {}

func (eg *EpidemicGossip) handleCacheInvalidate(msg GossipMessage) {}

func (eg *EpidemicGossip) handlePreFetch(msg GossipMessage) {}

func (eg *EpidemicGossip) NotifyCacheAdd(segmentID string, size int64) {}
