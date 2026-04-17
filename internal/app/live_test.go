package app

import (
	"context"
	"net/http"
	"testing"
	"time"

	"buddyapy/internal/llama"
)

func TestLiveDefiLlamaExampleSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live DefiLlama test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := llama.NewClient(DefaultBaseURL, &http.Client{Timeout: 20 * time.Second})

	pools, err := client.Pools(ctx)
	if err != nil {
		t.Fatalf("Pools() error = %v", err)
	}
	if len(pools) == 0 {
		t.Fatal("Pools() returned no data")
	}

	results, err := SearchPools(ctx, client, PoolsOptions{
		Stablecoin: true,
		MinTVL:     10_000_000,
		Lookback:   30 * 24 * time.Hour,
		RankBy:     RankBySnapshot30dMean,
		Limit:      10,
	}, func() time.Time { return time.Now().UTC() })
	if err != nil {
		t.Fatalf("SearchPools() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchPools() returned no results for the example query")
	}

	for idx, result := range results {
		if !result.Stablecoin {
			t.Fatalf("result %d stablecoin = false, want true", idx)
		}
		if result.TVLUsd < 10_000_000 {
			t.Fatalf("result %d tvlUsd = %f, want >= 10000000", idx, result.TVLUsd)
		}
		if idx > 0 && result.MetricValue > results[idx-1].MetricValue {
			t.Fatalf("results not sorted descending at index %d: %f > %f", idx, result.MetricValue, results[idx-1].MetricValue)
		}
	}

	chart, err := LoadChart(ctx, client, ChartOptions{
		PoolID:   results[0].PoolID,
		Lookback: 30 * 24 * time.Hour,
	}, func() time.Time { return time.Now().UTC() })
	if err != nil {
		t.Fatalf("LoadChart() error = %v", err)
	}
	if len(chart) == 0 {
		t.Fatalf("LoadChart() returned no recent history for pool %s", results[0].PoolID)
	}
}
