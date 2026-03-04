package coordination

import "fmt"

// GetPeerAddress returns the HTTP address for a cache node
func GetPeerAddress(nodeID string) string {
	// Node IDs: "cache-1", "cache-2", "cache-3"
	// In containerlab: oulu-cache-1/2/3 all listen on 8080
	
	switch nodeID {
	case "cache-1":
		return "http://oulu-cache-1:8080"
	case "cache-2":
		return "http://oulu-cache-2:8080"
	case "cache-3":
		return "http://oulu-cache-3:8080"
	default:
		return fmt.Sprintf("http://%s:8080", nodeID)
	}
}

// GetLeaderAddress returns the HTTP address for the leader node
func GetLeaderAddress(leaderID int) string {
	// Leader ID format: 1, 2, 3
	// In containerlab all caches listen on 8080
	return fmt.Sprintf("http://oulu-cache-%d:8080", leaderID)
}
