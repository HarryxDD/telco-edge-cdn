package coordination

import (
"fmt"
"io"
"log"
"net/http"
"time"
)

// FetchFromPeer fetches a segment from another cache node
func FetchFromPeer(peerID, segmentID string) ([]byte, error) {
	// Get peer address using helper
	peerURL := GetPeerAddress(peerID)

	// segmentID format: "/videos/wolf-1770292891/segment_0000.m4s"
	// Convert to /hls/ path for peer API: "/hls/wolf-1770292891/segment_0000.m4s"
	path := segmentID
	if len(segmentID) > 7 && segmentID[:7] == "/videos" {
		path = "/hls" + segmentID[7:] // replace /videos with /hls
	}

	url := peerURL + path
	log.Printf("[PEER_CLIENT] Fetching %s from peer %s: %s", segmentID, peerID, url)

	client := &http.Client{Timeout: 30 * time.Second} // Increased for high-latency scenarios
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("peer fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("peer returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}
