package controller

import (
	"context"
	"time"

	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/runner"
)

// NodeReconciler triggers a snapshot when Karpenter disruption is detected on any node.
type NodeReconciler struct {
	runner SnapshotRunner
	log    *zap.Logger
}

func NewNodeReconciler(r SnapshotRunner, log *zap.Logger) *NodeReconciler {
	return &NodeReconciler{runner: r, log: log}
}

func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info("node disruption detected, triggering snapshot", zap.String("node", req.String()))
	go func() {
		snapCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := r.runner.Run(snapCtx, runner.TriggerNodeDisruption); err != nil {
			r.log.Error("node-disruption snapshot failed", zap.Error(err))
		}
	}()
	return ctrl.Result{}, nil
}
