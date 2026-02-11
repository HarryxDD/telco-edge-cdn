package gossip

import "time"

// GossipMessageType represents the type of gossip message
type GossipMessageType int

const (
	MsgDigest          GossipMessageType = iota // State digest
	MsgDiff                                     // State difference
	MsgCacheAdd                                 // New segment cached
	MsgCacheInvalidate                          // Cache invalidation
	MsgPreFetch                                 // AI pre-fetch command
	MsgPing                                     // Heartbeat
)

// GossipMessage represents a gossip protocol message
type GossipMessage struct {
	Type        GossipMessageType
	SenderID    string
	Data        interface{}
	VectorClock map[string]int // For versioning
	Timestamp   time.Time
}

// CacheInventory represents what segments a node has cached
type CacheInventory struct {
	NodeID   string
	Segments map[string]bool // segmentID -> exists
	Load     float64         // 0.0 to 1.0 representing current load
	Version  int
}

// CacheAddNotification represents a new cached segment
type CacheAddNotification struct {
	NodeID    string
	SegmentID string
	Size      int64
}

// CacheInvalidateNotification represents a cache invalidation
type CacheInvalidateNotification struct {
	VideoID   string
	Timestamp time.Time
}

// PreFetchCommand represents AI-driven pre-fetch
type PreFetchCommand struct {
	VideoID   string
	SegmentID string
	Priority  int // Higher means more urgent
	ExpiresAt time.Time
}

type GossipProtocol interface {
	Start() error
	Stop()
	Broadcast(msg GossipMessage)
	SendTo(nodeID string, msg GossipMessage) error
	FindPeerWithSegment(segmentID string) (string, bool)
	RegisterHandler(msgType GossipMessageType, handler func(msg GossipMessage))
	GetInventory(nodeID string) (*CacheInventory, bool)
}
