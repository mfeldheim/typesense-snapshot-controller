package controller

import (
	"context"
	"time"

	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/runner"
)

// SnapshotRunner is satisfied by *runner.Runner and can be mocked in tests.
type SnapshotRunner interface {
	Run(ctx context.Context, trigger string) error
}

// PodReconciler triggers a snapshot when a watched pod enters Terminating state.
type PodReconciler struct {
	runner SnapshotRunner
	log    *zap.Logger
}

func NewPodReconciler(r SnapshotRunner, log *zap.Logger) *PodReconciler {
	return &PodReconciler{runner: r, log: log}
}

func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info("pod eviction detected, triggering snapshot", zap.String("pod", req.String()))
	go func() {
		snapCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := r.runner.Run(snapCtx, runner.TriggerEviction); err != nil {
			r.log.Error("eviction-triggered snapshot failed", zap.Error(err))
		}
	}()
	return ctrl.Result{}, nil
}
