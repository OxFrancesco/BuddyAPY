package llama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientPoolsAndChart(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pools":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":[
					{
						"chain":"Ethereum",
						"project":"maple",
						"symbol":"USDC",
						"tvlUsd":12345678,
						"apy":4.25,
						"apyMean30d":4.5,
						"apyPct30D":-0.1,
						"stablecoin":true,
						"pool":"pool-1"
					}
				]
			}`))
		case "/chart/pool-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":[
					{
						"timestamp":"2026-04-10T00:00:00Z",
						"tvlUsd":12000000,
						"apy":4.1,
						"apyBase":3.8,
						"apyReward":0.3
					}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())

	pools, err := client.Pools(context.Background())
	if err != nil {
		t.Fatalf("Pools() error = %v", err)
	}
	if len(pools) != 1 {
		t.Fatalf("Pools() len = %d, want 1", len(pools))
	}
	if pools[0].APY == nil || *pools[0].APY != 4.25 {
		t.Fatalf("Pools() APY = %v, want 4.25", pools[0].APY)
	}
	if pools[0].APYMean30d == nil || *pools[0].APYMean30d != 4.5 {
		t.Fatalf("Pools() APYMean30d = %v, want 4.5", pools[0].APYMean30d)
	}

	chart, err := client.Chart(context.Background(), "pool-1")
	if err != nil {
		t.Fatalf("Chart() error = %v", err)
	}
	if len(chart) != 1 {
		t.Fatalf("Chart() len = %d, want 1", len(chart))
	}
	if got, want := chart[0].Timestamp, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("Chart() timestamp = %s, want %s", got, want)
	}
	if chart[0].APYReward == nil || *chart[0].APYReward != 0.3 {
		t.Fatalf("Chart() APYReward = %v, want 0.3", chart[0].APYReward)
	}
}
