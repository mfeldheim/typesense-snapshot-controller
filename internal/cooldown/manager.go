package cooldown

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	keyLastSnapshot = "last-snapshot-time"
	keyTrigger      = "trigger"
)

type Manager struct {
	k8s      kubernetes.Interface
	ns       string
	cmName   string
	cooldown time.Duration
}

func New(k8s kubernetes.Interface, ns, cmName string, cooldown time.Duration) *Manager {
	return &Manager{k8s: k8s, ns: ns, cmName: cmName, cooldown: cooldown}
}

// InCooldown returns true when the last recorded snapshot is still within the cooldown window.
func (m *Manager) InCooldown(ctx context.Context) (bool, error) {
	cm, err := m.k8s.CoreV1().ConfigMaps(m.ns).Get(ctx, m.cmName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get cooldown configmap: %w", err)
	}
	lastStr, ok := cm.Data[keyLastSnapshot]
	if !ok {
		return false, nil
	}
	last, err := time.Parse(time.RFC3339, lastStr)
	if err != nil {
		return false, nil
	}
	return time.Since(last) < m.cooldown, nil
}

// Record writes the current UTC time as the last snapshot time.
func (m *Manager) Record(ctx context.Context, trigger string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	cm, err := m.k8s.CoreV1().ConfigMaps(m.ns).Get(ctx, m.cmName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = m.k8s.CoreV1().ConfigMaps(m.ns).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: m.cmName, Namespace: m.ns},
			Data:       map[string]string{keyLastSnapshot: now, keyTrigger: trigger},
		}, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return fmt.Errorf("get cooldown configmap: %w", err)
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[keyLastSnapshot] = now
	cm.Data[keyTrigger] = trigger
	_, err = m.k8s.CoreV1().ConfigMaps(m.ns).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}
