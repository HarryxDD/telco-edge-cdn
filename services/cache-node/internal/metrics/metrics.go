package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// cache hit/miss counters per node and video
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cdn_cache_hits_total",
			Help: "Total cache hits",
		},
		[]string{"node_id", "video_id"},
	)

	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cdn_cache_misses_total",
			Help: "Total cache misses",
		},
		[]string{"node_id", "video_id"},
	)

	// how long each request takes, split by node and whether it was a hit
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cdn_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5},
		},
		[]string{"node_id", "cache_hit"},
	)

	// total bytes served per node, useful for bandwidth tracking
	BytesServed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cdn_bytes_served_total",
			Help: "Total bytes served per edge node",
		},
		[]string{"node_id"},
	)

	// counts non-200 responses so we can track error rate per node
	ErrorResponses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cdn_error_responses_total",
			Help: "Total non-200 responses per node",
		},
		[]string{"node_id", "status_code"},
	)

	// rebuffer events broken down by bitrate level
	RebufferEvents = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cdn_rebuffer_events_total",
			Help: "Total rebuffer events per bitrate level",
		},
		[]string{"node_id", "bitrate_kbps"},
	)

	// tracks unique active sessions per node in a time window
	ActiveSessions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "cdn_active_sessions",
			Help: "Active user sessions across all edge nodes",
		},
	)

	// distribution of requested bitrates, helps spot quality trends
	BitrateRequested = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cdn_bitrate_requested_kbps",
			Help:    "Distribution of requested bitrates in kbps",
			Buckets: []float64{360, 720, 1080, 2000, 4500, 8000},
		},
		[]string{"node_id"},
	)

	// total requests per node and video, drives the popular content panel
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cdn_requests_total",
			Help: "Total requests per node and video",
		},
		[]string{"node_id", "video_id"},
	)
)
