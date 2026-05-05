package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	SnapshotSuccessTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "snapshot_success_total",
		Help: "Total successful snapshots, by trigger reason.",
	}, []string{"trigger"})

	SnapshotFailureTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "snapshot_failure_total",
		Help: "Total failed snapshots, by trigger reason.",
	}, []string{"trigger"})

	SnapshotDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "snapshot_duration_seconds",
		Help:    "Duration of snapshot HTTP call in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"trigger"})
)
