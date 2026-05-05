package cooldown_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/cooldown"
)

func TestInCooldown_FalseWhenConfigMapMissing(t *testing.T) {
	k8s := fake.NewSimpleClientset()
	m := cooldown.New(k8s, "ts", "snapshot-cooldown", 3*time.Minute)
	in, err := m.InCooldown(context.Background())
	require.NoError(t, err)
	assert.False(t, in)
}

func TestInCooldown_TrueWithinWindow(t *testing.T) {
	k8s := fake.NewSimpleClientset()
	m := cooldown.New(k8s, "ts", "snapshot-cooldown", 3*time.Minute)

	require.NoError(t, m.Record(context.Background(), "scheduled"))

	in, err := m.InCooldown(context.Background())
	require.NoError(t, err)
	assert.True(t, in)
}

func TestInCooldown_FalseAfterWindowExpires(t *testing.T) {
	k8s := fake.NewSimpleClientset()
	m := cooldown.New(k8s, "ts", "snapshot-cooldown", 1*time.Millisecond)

	require.NoError(t, m.Record(context.Background(), "scheduled"))
	time.Sleep(5 * time.Millisecond)

	in, err := m.InCooldown(context.Background())
	require.NoError(t, err)
	assert.False(t, in)
}

func TestRecord_CreatesConfigMapOnFirstCall(t *testing.T) {
	k8s := fake.NewSimpleClientset()
	m := cooldown.New(k8s, "ts", "snapshot-cooldown", 3*time.Minute)

	require.NoError(t, m.Record(context.Background(), "eviction"))

	cm, err := k8s.CoreV1().ConfigMaps("ts").Get(context.Background(), "snapshot-cooldown", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "eviction", cm.Data["trigger"])
	assert.NotEmpty(t, cm.Data["last-snapshot-time"])
}

func TestRecord_UpdatesExistingConfigMap(t *testing.T) {
	k8s := fake.NewSimpleClientset()
	m := cooldown.New(k8s, "ts", "snapshot-cooldown", 3*time.Minute)

	require.NoError(t, m.Record(context.Background(), "scheduled"))
	require.NoError(t, m.Record(context.Background(), "eviction"))

	cm, err := k8s.CoreV1().ConfigMaps("ts").Get(context.Background(), "snapshot-cooldown", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "eviction", cm.Data["trigger"])
}
