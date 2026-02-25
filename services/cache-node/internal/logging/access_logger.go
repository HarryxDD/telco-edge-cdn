package logging

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type AccessLog struct {
	Timestamp      time.Time `json:"timestamp"`
	EdgeNodeID     string    `json:"edge_node_id"`
	VideoID        string    `json:"video_id"`
	ClientID       string    `json:"client_id"`
	SegmentPath    string    `json:"segment_path"`
	CacheHit       bool      `json:"cache_hit"`
	ResponseTimeMs float64   `json:"response_time_ms"`
	BytesSent      int64     `json:"bytes_sent"`
	StatusCode     int       `json:"status_code"`
	Protocol       string    `json:"protocol"`
	BitrateKbps    int64     `json:"bitrate_kbps"`
	RebufferEvent  bool      `json:"rebuffer_event"`
}

type AccessLogger struct {
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
	nodeID  string
}

// NewAccessLogger creates logger writing to shared volume
func NewAccessLogger(logPath string, nodeID string) (*AccessLogger, error) {
	// Open in append mode (multiple cache nodes write to same file)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &AccessLogger{
		file:    f,
		encoder: json.NewEncoder(f),
		nodeID:  nodeID,
	}, nil
}

// Log writes an access log entry
func (al *AccessLogger) Log(entry AccessLog) {
	al.mu.Lock()
	defer al.mu.Unlock()

	// Ensure EdgeNodeID is set
	entry.EdgeNodeID = al.nodeID
	al.encoder.Encode(entry)
}

// Close the log file
func (al *AccessLogger) Close() error {
	return al.file.Close()
}
