package controller_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/controller"
	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/runner"
)

type fakeRunner struct {
	mu     sync.Mutex
	calls  []string
	retErr error
}

func (f *fakeRunner) Run(_ context.Context, trigger string) error {
	f.mu.Lock()
	f.calls = append(f.calls, trigger)
	f.mu.Unlock()
	return f.retErr
}

func (f *fakeRunner) calledWith() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

func TestPodReconciler_TriggersEvictionSnapshot(t *testing.T) {
	fr := &fakeRunner{}
	log := zaptest.NewLogger(t)
	r := controller.NewPodReconciler(fr, log)

	result, err := r.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	assert.Eventually(t, func() bool {
		return len(fr.calledWith()) == 1
	}, 200*time.Millisecond, 10*time.Millisecond)
	assert.Equal(t, runner.TriggerEviction, fr.calledWith()[0])
}

func TestPodReconciler_FailOpenOnRunnerError(t *testing.T) {
	fr := &fakeRunner{retErr: errors.New("snapshot failed")}
	log := zaptest.NewLogger(t)
	r := controller.NewPodReconciler(fr, log)

	result, err := r.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestNodeReconciler_TriggersNodeDisruptionSnapshot(t *testing.T) {
	fr := &fakeRunner{}
	log := zaptest.NewLogger(t)
	r := controller.NewNodeReconciler(fr, log)

	result, err := r.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	assert.Eventually(t, func() bool {
		return len(fr.calledWith()) == 1
	}, 200*time.Millisecond, 10*time.Millisecond)
	assert.Equal(t, runner.TriggerNodeDisruption, fr.calledWith()[0])
}
