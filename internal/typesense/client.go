package typesense

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient builds a client from host + port (production use).
func NewClient(host string, port int, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    fmt.Sprintf("http://%s:%d", host, port),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// NewClientFromURL builds a client from a raw base URL (test use).
func NewClientFromURL(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// IsReady returns true when the pod's /readyz responds 200 (leader probe).
func (c *Client) IsReady(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/readyz", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-TYPESENSE-API-KEY", c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

type snapshotResponse struct {
	Success bool `json:"success"`
}

// TakeSnapshot calls POST /operations/snapshot on this node.
func (c *Client) TakeSnapshot(ctx context.Context, snapshotPath string) error {
	endpoint := fmt.Sprintf("%s/operations/snapshot?snapshot_path=%s",
		c.baseURL, url.QueryEscape(snapshotPath))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-TYPESENSE-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("snapshot failed with HTTP %d: %s", resp.StatusCode, string(body))
	}

	var sr snapshotResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return fmt.Errorf("decode snapshot response: %w", err)
	}
	if !sr.Success {
		return fmt.Errorf("snapshot returned success=false")
	}
	return nil
}
