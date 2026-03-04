package logging

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"time"
)

// NDJSON format
type AccessLog struct {
	LogVersion     string  `json:"log_version"`
	Timestamp      string  `json:"timestamp"`
	EdgeNodeID     string  `json:"edge_node_id"`
	ClientID       string  `json:"client_id"`
	SessionID      string  `json:"session_id"`
	VideoID        string  `json:"video_id"`
	VideoCategory  string  `json:"video_category"`
	SegmentNumber  int     `json:"segment_number"`
	RequestType    string  `json:"request_type"`
	CacheHit       bool    `json:"cache_hit"`
	ResponseTimeMs float64 `json:"response_time_ms"`
	BytesSent      int64   `json:"bytes_sent"`
	ClientRegion   string  `json:"client_region"`
	Protocol       string  `json:"protocol"`
	BitrateKbps    int64   `json:"bitrate_requested_kbps"`
	RebufferEvent  bool    `json:"rebuffer_event"`
	StatusCode     int     `json:"status_code"`
}

type AccessLogger struct {
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
	nodeID  string
}

// creates a 30-minute session window ID based on client IP, user agent, and time bucket
func GenerateSessionID(clientIP, userAgent string, t time.Time) string {
	bucket := math.Floor(float64(t.Unix()) / 1800)
	raw := fmt.Sprintf("%s%s%d", clientIP, userAgent, int64(bucket))
	return fmt.Sprintf("%x", md5.Sum([]byte(raw)))
}

// creates a logger writing to /app/logs/access_${NODE_ID}.ndjson
func NewAccessLogger(logPath string, nodeID string) (*AccessLogger, error) {
	if err := os.MkdirAll(getDir(logPath), 0755); err != nil {
		return nil, err
	}

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

// writes one NDJSON line, injecting node ID and log version
func (al *AccessLogger) Log(entry AccessLog) {
	al.mu.Lock()
	defer al.mu.Unlock()

	entry.LogVersion = "v1"
	entry.EdgeNodeID = al.nodeID

	// ensures timestamp with ms
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	}

	al.encoder.Encode(entry)
}

func (al *AccessLogger) Close() error {
	return al.file.Close()
}

// extracts directory from a full file path
func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
