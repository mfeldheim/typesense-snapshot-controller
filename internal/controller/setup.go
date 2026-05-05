package controller

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/config"
)

const karpenterDisruptionTaint = "karpenter.sh/disruption"

// Setup registers the pod and node reconcilers with the Manager.
func Setup(mgr ctrl.Manager, cfg config.Config, r SnapshotRunner, log *zap.Logger) error {
	podRec := NewPodReconciler(r, log.With(zap.String("reconciler", "pod")))
	labelMap := parseLabelSelector(cfg.PodLabelSelector)

	if err := ctrl.NewControllerManagedBy(mgr).
		Named("pod-eviction-watcher").
		For(&corev1.Pod{}, builder.WithPredicates(
			namespacePredicate(cfg.Namespace),
			labelPredicate(labelMap),
			deletionPredicate(),
		)).
		Complete(podRec); err != nil {
		return fmt.Errorf("setup pod reconciler: %w", err)
	}

	nodeRec := NewNodeReconciler(r, log.With(zap.String("reconciler", "node")))

	if err := ctrl.NewControllerManagedBy(mgr).
		Named("node-disruption-watcher").
		For(&corev1.Node{}, builder.WithPredicates(karpenterTaintPredicate())).
		Complete(nodeRec); err != nil {
		return fmt.Errorf("setup node reconciler: %w", err)
	}

	return nil
}

func namespacePredicate(ns string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == ns
	})
}

func labelPredicate(labels map[string]string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		objLabels := obj.GetLabels()
		for k, v := range labels {
			if objLabels[k] != v {
				return false
			}
		}
		return true
	})
}

func deletionPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(_ event.CreateEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return e.ObjectNew.GetDeletionTimestamp() != nil },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func karpenterTaintPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(_ event.CreateEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return hasKarpenterDisruptionTaint(e.ObjectNew) && !hasKarpenterDisruptionTaint(e.ObjectOld) },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func hasKarpenterDisruptionTaint(obj client.Object) bool {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return false
	}
	for _, t := range node.Spec.Taints {
		if t.Key == karpenterDisruptionTaint {
			return true
		}
	}
	return false
}

func parseLabelSelector(selector string) map[string]string {
	m := make(map[string]string)
	for _, pair := range strings.Split(selector, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}
