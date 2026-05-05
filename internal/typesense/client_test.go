package typesense_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/typesense"
)

func TestIsReady_ReturnsTrue_When200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/readyz", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("X-TYPESENSE-API-KEY"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := typesense.NewClientFromURL(srv.URL, "test-key", 5*time.Second)
	ok, err := c.IsReady(context.Background())
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestIsReady_ReturnsFalse_When503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := typesense.NewClientFromURL(srv.URL, "test-key", 5*time.Second)
	ok, err := c.IsReady(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestIsReady_ReturnsError_OnNetworkFailure(t *testing.T) {
	c := typesense.NewClientFromURL("http://127.0.0.1:1", "test-key", 100*time.Millisecond)
	_, err := c.IsReady(context.Background())
	require.Error(t, err)
}

func TestTakeSnapshot_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/operations/snapshot", r.URL.Path)
		assert.Equal(t, "/snapshots", r.URL.Query().Get("snapshot_path"))
		assert.Equal(t, "test-key", r.Header.Get("X-TYPESENSE-API-KEY"))
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer srv.Close()

	c := typesense.NewClientFromURL(srv.URL, "test-key", 5*time.Second)
	err := c.TakeSnapshot(context.Background(), "/snapshots")
	require.NoError(t, err)
}

func TestTakeSnapshot_ReturnsError_On401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Forbidden"}`))
	}))
	defer srv.Close()

	c := typesense.NewClientFromURL(srv.URL, "bad-key", 5*time.Second)
	err := c.TakeSnapshot(context.Background(), "/snapshots")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestTakeSnapshot_ReturnsError_WhenSuccessFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"success": false})
	}))
	defer srv.Close()

	c := typesense.NewClientFromURL(srv.URL, "test-key", 5*time.Second)
	err := c.TakeSnapshot(context.Background(), "/snapshots")
	require.Error(t, err)
}
