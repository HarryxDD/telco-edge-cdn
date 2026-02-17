package coordination

import "fmt"

// GetPeerAddress returns the HTTP address for a cache node
func GetPeerAddress(nodeID string) string {
	// Node IDs: "cache-1", "cache-2", "cache-3"
	// Port mapping: cache-1 -> 8081, cache-2 -> 8082, cache-3 -> 8083
	
	switch nodeID {
	case "cache-1":
		return "http://cache-1:8081"
	case "cache-2":
		return "http://cache-2:8082"
	case "cache-3":
		return "http://cache-3:8083"
	default:
		return fmt.Sprintf("http://%s:8081", nodeID)
	}
}

// GetLeaderAddress returns the HTTP address for the leader node
func GetLeaderAddress(leaderID int) string {
	// Leader ID format: 1, 2, 3
	// Port mapping: 1 -> 8081, 2 -> 8082, 3 -> 8083
	port := 8080 + leaderID
	return fmt.Sprintf("http://cache-%d:%d", leaderID, port)
}
