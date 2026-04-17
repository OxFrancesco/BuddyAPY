package llama

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

func (c *Client) Pools(ctx context.Context) ([]Pool, error) {
	var response PoolsResponse
	if err := c.getJSON(ctx, "/pools", &response); err != nil {
		return nil, err
	}
	if response.Status != "" && response.Status != "success" {
		return nil, fmt.Errorf("unexpected pools status %q", response.Status)
	}
	return response.Data, nil
}

func (c *Client) Chart(ctx context.Context, pool string) ([]ChartPoint, error) {
	var response ChartResponse
	if err := c.getJSON(ctx, "/chart/"+pool, &response); err != nil {
		return nil, err
	}
	if response.Status != "" && response.Status != "success" {
		return nil, fmt.Errorf("unexpected chart status %q", response.Status)
	}
	return response.Data, nil
}

func (c *Client) getJSON(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request %s: %w", path, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("request %s: unexpected status %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}

	return nil
}
