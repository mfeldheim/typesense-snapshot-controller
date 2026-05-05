package discovery

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/typesense"
)

// ReadyChecker can probe whether a pod is the leader.
type ReadyChecker interface {
	IsReady(ctx context.Context) (bool, error)
}

// ClientFactory produces a ReadyChecker for a given pod IP.
type ClientFactory func(ip string, port int, apiKey string, timeout time.Duration) ReadyChecker

// HTTPReadyChecker returns a ReadyChecker backed by an explicit base URL (for tests).
func HTTPReadyChecker(baseURL, apiKey string, timeout time.Duration) ReadyChecker {
	return typesense.NewClientFromURL(baseURL, apiKey, timeout)
}

type LeaderDiscovery struct {
	k8s           kubernetes.Interface
	namespace     string
	labelSelector string
	port          int
	apiKey        string
	timeout       time.Duration
	log           *zap.Logger
	factory       ClientFactory
}

// New creates a LeaderDiscovery using the production client factory.
func New(k8s kubernetes.Interface, namespace, labelSelector string, port int, apiKey string, timeout time.Duration, log *zap.Logger) *LeaderDiscovery {
	return NewWithClientFactory(k8s, namespace, labelSelector, port, apiKey, timeout, log,
		func(ip string, port int, apiKey string, timeout time.Duration) ReadyChecker {
			return typesense.NewClient(ip, port, apiKey, timeout)
		},
	)
}

// NewWithClientFactory creates a LeaderDiscovery with an injectable factory (for tests).
func NewWithClientFactory(k8s kubernetes.Interface, namespace, labelSelector string, port int, apiKey string, timeout time.Duration, log *zap.Logger, factory ClientFactory) *LeaderDiscovery {
	return &LeaderDiscovery{
		k8s:           k8s,
		namespace:     namespace,
		labelSelector: labelSelector,
		port:          port,
		apiKey:        apiKey,
		timeout:       timeout,
		log:           log,
		factory:       factory,
	}
}

// Discover returns the PodIP of the current Typesense leader.
func (d *LeaderDiscovery) Discover(ctx context.Context) (string, error) {
	pods, err := d.k8s.CoreV1().Pods(d.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: d.labelSelector,
	})
	if err != nil {
		return "", fmt.Errorf("list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found with selector %q in namespace %q", d.labelSelector, d.namespace)
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" {
			continue
		}
		checker := d.factory(pod.Status.PodIP, d.port, d.apiKey, d.timeout)
		ready, err := checker.IsReady(ctx)
		if err != nil {
			d.log.Warn("readyz probe failed", zap.String("pod", pod.Name), zap.Error(err))
			continue
		}
		if ready {
			d.log.Info("found leader", zap.String("pod", pod.Name), zap.String("ip", pod.Status.PodIP))
			return pod.Status.PodIP, nil
		}
	}
	return "", fmt.Errorf("no leader found among %d pods", len(pods.Items))
}

// compile-time check: *typesense.Client satisfies ReadyChecker
var _ ReadyChecker = (*typesense.Client)(nil)
