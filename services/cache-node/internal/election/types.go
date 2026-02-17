package election

import "time"

// ElectionState represents the current election status
type ElectionState int

const (
	StateFollower ElectionState = iota
	StateCandidate
	StateLeader
)

// ElectionMessage represents messages exchanged during the election
type ElectionMessage struct {
	Type      MessageType
	SenderID  int
	Term      int
	Timestamp time.Time
}

type MessageType int

const (
	MsgElection  MessageType = iota // Start election
	MsgAnswer                       // Response to election
	MsgVictory                      // Leader announcement
	MsgHeartbeat                    // Leader is alive
)

// LeaderElection defines the interface for leader election
type LeaderElection interface {
	Start() error
	Stop()
	GetLeaderID() int
	IsLeader() bool
	GetState() ElectionState
	RegisterLeaderChangeCallback(callback func(newLeaderID int))
}

// NodeInfo represents information about a node in the cluster
type NodeInfo struct {
	ID      int
	Address string
	Port    int
}
