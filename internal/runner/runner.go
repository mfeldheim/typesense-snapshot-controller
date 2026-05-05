package runner

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/metrics"
	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/typesense"
)

const (
	TriggerScheduled      = "scheduled"
	TriggerEviction       = "eviction"
	TriggerNodeDisruption = "node-disruption"
)

// Discoverer finds the current leader pod IP.
type Discoverer interface {
	Discover(ctx context.Context) (string, error)
}

// CooldownManager checks and records cooldown state.
type CooldownManager interface {
	InCooldown(ctx context.Context) (bool, error)
	Record(ctx context.Context, trigger string) error
}

// Runner orchestrates cooldown → discovery → snapshot.
type Runner struct {
	discovery           Discoverer
	cooldown            CooldownManager
	port                int
	apiKey              string
	snapshotPath        string
	timeout             time.Duration
	log                 *zap.Logger
	snapshotURLOverride string // non-empty in tests only
}

func New(disc Discoverer, cd CooldownManager, port int, apiKey, snapshotPath string, timeout time.Duration, log *zap.Logger) *Runner {
	return &Runner{
		discovery:    disc,
		cooldown:     cd,
		port:         port,
		apiKey:       apiKey,
		snapshotPath: snapshotPath,
		timeout:      timeout,
		log:          log,
	}
}

// NewWithSnapshotURL is for tests only — points the snapshot call at a test server URL.
func NewWithSnapshotURL(disc Discoverer, cd CooldownManager, snapshotBaseURL, apiKey, snapshotPath string, timeout time.Duration, log *zap.Logger) *Runner {
	r := New(disc, cd, 0, apiKey, snapshotPath, timeout, log)
	r.snapshotURLOverride = snapshotBaseURL
	return r
}

// Run executes one snapshot attempt for the given trigger reason.
// Returns nil when skipped due to cooldown — that is not an error.
func (r *Runner) Run(ctx context.Context, trigger string) error {
	inCooldown, err := r.cooldown.InCooldown(ctx)
	if err != nil {
		return fmt.Errorf("cooldown check: %w", err)
	}
	if inCooldown {
		r.log.Info("skipping: in cooldown", zap.String("trigger", trigger))
		return nil
	}

	leaderIP, err := r.discovery.Discover(ctx)
	if err != nil {
		metrics.SnapshotFailureTotal.WithLabelValues(trigger).Inc()
		return fmt.Errorf("discover leader: %w", err)
	}

	var client *typesense.Client
	if r.snapshotURLOverride != "" {
		client = typesense.NewClientFromURL(r.snapshotURLOverride, r.apiKey, r.timeout)
	} else {
		client = typesense.NewClient(leaderIP, r.port, r.apiKey, r.timeout)
	}

	start := time.Now()
	if err := client.TakeSnapshot(ctx, r.snapshotPath); err != nil {
		metrics.SnapshotFailureTotal.WithLabelValues(trigger).Inc()
		r.log.Error("snapshot failed",
			zap.String("trigger", trigger),
			zap.String("leader", leaderIP),
			zap.Error(err),
		)
		return fmt.Errorf("take snapshot: %w", err)
	}

	dur := time.Since(start).Seconds()
	metrics.SnapshotSuccessTotal.WithLabelValues(trigger).Inc()
	metrics.SnapshotDurationSeconds.WithLabelValues(trigger).Observe(dur)

	if err := r.cooldown.Record(ctx, trigger); err != nil {
		r.log.Warn("failed to record cooldown state", zap.Error(err))
	}

	r.log.Info("snapshot complete",
		zap.String("trigger", trigger),
		zap.String("leader", leaderIP),
		zap.Float64("duration_s", dur),
	)
	return nil
}
