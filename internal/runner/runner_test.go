package runner_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/runner"
)

type fakeDiscovery struct {
	ip  string
	err error
}

func (f *fakeDiscovery) Discover(_ context.Context) (string, error) { return f.ip, f.err }

type fakeCooldown struct {
	in       bool
	recorded string
	err      error
}

func (f *fakeCooldown) InCooldown(_ context.Context) (bool, error) { return f.in, f.err }
func (f *fakeCooldown) Record(_ context.Context, trigger string) error {
	f.recorded = trigger
	return f.err
}

func TestRun_SkipsWhenInCooldown(t *testing.T) {
	log := zaptest.NewLogger(t)
	cd := &fakeCooldown{in: true}
	disc := &fakeDiscovery{}
	r := runner.New(disc, cd, 8108, "key", "/snap", 5*time.Second, log)

	err := r.Run(context.Background(), "scheduled")
	require.NoError(t, err)
	assert.Empty(t, cd.recorded, "cooldown should not be updated when skipping")
}

func TestRun_ReturnsErrorWhenDiscoveryFails(t *testing.T) {
	log := zaptest.NewLogger(t)
	cd := &fakeCooldown{in: false}
	disc := &fakeDiscovery{err: errors.New("no leader")}
	r := runner.New(disc, cd, 8108, "key", "/snap", 5*time.Second, log)

	err := r.Run(context.Background(), "eviction")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discover leader")
}

func TestRun_RecordsCooldownOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	log := zaptest.NewLogger(t)
	cd := &fakeCooldown{in: false}
	disc := &fakeDiscovery{ip: "127.0.0.1"}
	r := runner.NewWithSnapshotURL(disc, cd, srv.URL, "key", "/snap", 5*time.Second, log)

	err := r.Run(context.Background(), "scheduled")
	require.NoError(t, err)
	assert.Equal(t, "scheduled", cd.recorded)
}

func TestRun_PropagatesSnapshotError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	log := zaptest.NewLogger(t)
	cd := &fakeCooldown{in: false}
	disc := &fakeDiscovery{ip: "127.0.0.1"}
	r := runner.NewWithSnapshotURL(disc, cd, srv.URL, "key", "/snap", 5*time.Second, log)

	err := r.Run(context.Background(), "node-disruption")
	require.Error(t, err)
	assert.Empty(t, cd.recorded, "cooldown must not be recorded on failure")
}
