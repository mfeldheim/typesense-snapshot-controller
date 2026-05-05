package discovery_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/discovery"
)

func makePod(name, ip string, phase corev1.PodPhase) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "ts",
			Labels:    map[string]string{"app": "typesense"},
		},
		Status: corev1.PodStatus{Phase: phase, PodIP: ip},
	}
}

func makePodPtr(name, ip string, phase corev1.PodPhase) *corev1.Pod {
	p := makePod(name, ip, phase)
	return &p
}

func TestDiscover_FindsLeaderPod(t *testing.T) {
	followerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer followerSrv.Close()

	leaderSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer leaderSrv.Close()

	followerIP := "10.0.0.1"
	leaderIP := "10.0.0.2"

	k8s := fake.NewSimpleClientset(
		makePodPtr("follower", followerIP, corev1.PodRunning),
		makePodPtr("leader", leaderIP, corev1.PodRunning),
	)

	log := zaptest.NewLogger(t)
	d := discovery.NewWithClientFactory(k8s, "ts", "app=typesense", 8108, "key",
		5*time.Second, log,
		func(ip string, port int, apiKey string, timeout time.Duration) discovery.ReadyChecker {
			if ip == followerIP {
				return discovery.HTTPReadyChecker(followerSrv.URL, apiKey, timeout)
			}
			return discovery.HTTPReadyChecker(leaderSrv.URL, apiKey, timeout)
		},
	)

	ip, err := d.Discover(context.Background())
	require.NoError(t, err)
	assert.Equal(t, leaderIP, ip)
}

func TestDiscover_ErrorWhenNoPods(t *testing.T) {
	k8s := fake.NewSimpleClientset()
	log := zaptest.NewLogger(t)
	d := discovery.NewWithClientFactory(k8s, "ts", "app=typesense", 8108, "key",
		5*time.Second, log, nil)

	_, err := d.Discover(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pods found")
}

func TestDiscover_ErrorWhenNoLeader(t *testing.T) {
	followerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer followerSrv.Close()

	k8s := fake.NewSimpleClientset(makePodPtr("node0", "10.0.0.1", corev1.PodRunning))
	log := zaptest.NewLogger(t)
	d := discovery.NewWithClientFactory(k8s, "ts", "app=typesense", 8108, "key",
		5*time.Second, log,
		func(ip string, port int, apiKey string, timeout time.Duration) discovery.ReadyChecker {
			return discovery.HTTPReadyChecker(followerSrv.URL, apiKey, timeout)
		},
	)

	_, err := d.Discover(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no leader found")
}
