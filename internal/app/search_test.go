package app

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"buddyapy/internal/llama"
)

type fakeAPI struct {
	pools  []llama.Pool
	charts map[string][]llama.ChartPoint
}

func (f fakeAPI) Pools(context.Context) ([]llama.Pool, error) {
	return f.pools, nil
}

func (f fakeAPI) Chart(_ context.Context, pool string) ([]llama.ChartPoint, error) {
	return f.charts[pool], nil
}

func TestParseUSDValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		raw   string
		want  float64
		isErr bool
	}{
		{name: "empty", raw: "", want: 0},
		{name: "plain", raw: "10000000", want: 10_000_000},
		{name: "millions", raw: "10m", want: 10_000_000},
		{name: "billions", raw: "1.5b", want: 1_500_000_000},
		{name: "thousands", raw: "12k", want: 12_000},
		{name: "negative", raw: "-1", isErr: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseUSDValue(test.raw)
			if test.isErr {
				if err == nil {
					t.Fatalf("parseUSDValue(%q) error = nil, want error", test.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseUSDValue(%q) error = %v", test.raw, err)
			}
			if got != test.want {
				t.Fatalf("parseUSDValue(%q) = %f, want %f", test.raw, got, test.want)
			}
		})
	}
}

func TestParseLookback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw   string
		want  time.Duration
		isErr bool
	}{
		{raw: "30d", want: 30 * 24 * time.Hour},
		{raw: "48h", want: 48 * time.Hour},
		{raw: "2w", want: 14 * 24 * time.Hour},
		{raw: "0d", isErr: true},
		{raw: "30x", isErr: true},
	}

	for _, test := range tests {
		got, err := parseLookback(test.raw)
		if test.isErr {
			if err == nil {
				t.Fatalf("parseLookback(%q) error = nil, want error", test.raw)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseLookback(%q) error = %v", test.raw, err)
		}
		if got != test.want {
			t.Fatalf("parseLookback(%q) = %s, want %s", test.raw, got, test.want)
		}
	}
}

func TestPoolsOptionsValidate(t *testing.T) {
	t.Parallel()

	err := (PoolsOptions{
		Lookback: 7 * 24 * time.Hour,
		RankBy:   RankBySnapshot30dMean,
		Limit:    10,
	}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want validation error for snapshot-30d-mean with non-30d lookback")
	}

	minYield := 10.0
	maxYield := 5.0
	err = (PoolsOptions{
		Lookback: 30 * 24 * time.Hour,
		RankBy:   RankBySnapshot30dMean,
		MinYield: &minYield,
		MaxYield: &maxYield,
		Limit:    10,
	}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want validation error for min-yield > max-yield")
	}
}

func TestSearchPoolsSnapshotFiltersAndRanks(t *testing.T) {
	t.Parallel()

	api := fakeAPI{
		pools: []llama.Pool{
			makePool("pool-a", "maple", "Ethereum", "USDC", true, 15_000_000, floatPtr(4), floatPtr(6), nil),
			makePool("pool-b", "goldfinch", "Ethereum", "USDC", true, 12_000_000, floatPtr(5), floatPtr(9), nil),
			makePool("pool-c", "volatile", "Ethereum", "ETH", false, 20_000_000, floatPtr(12), floatPtr(20), nil),
			makePool("pool-d", "small", "Ethereum", "USDC", true, 9_000_000, floatPtr(8), floatPtr(30), nil),
		},
	}

	results, err := SearchPools(context.Background(), api, PoolsOptions{
		Stablecoin: true,
		MinTVL:     10_000_000,
		Lookback:   30 * 24 * time.Hour,
		RankBy:     RankBySnapshot30dMean,
		Chain:      "ethereum",
		Limit:      10,
	}, func() time.Time { return time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("SearchPools() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("SearchPools() len = %d, want 2", len(results))
	}
	if results[0].PoolID != "pool-b" || results[1].PoolID != "pool-a" {
		t.Fatalf("SearchPools() order = [%s, %s], want [pool-b, pool-a]", results[0].PoolID, results[1].PoolID)
	}
	if results[0].Rank != 1 || results[1].Rank != 2 {
		t.Fatalf("SearchPools() ranks = [%d, %d], want [1,2]", results[0].Rank, results[1].Rank)
	}
	if results[0].URL != "https://defillama.com/yields/pool/pool-b" {
		t.Fatalf("SearchPools() url = %q, want pool URL", results[0].URL)
	}
}

func TestSearchPoolsChartMeanRanksByWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	api := fakeAPI{
		pools: []llama.Pool{
			makePool("pool-a", "maple", "Ethereum", "USDC", true, 15_000_000, floatPtr(4), floatPtr(6), nil),
			makePool("pool-b", "goldfinch", "Ethereum", "USDC", true, 12_000_000, floatPtr(5), floatPtr(9), nil),
		},
		charts: map[string][]llama.ChartPoint{
			"pool-a": {
				makePoint(now.Add(-2*24*time.Hour), 10),
				makePoint(now.Add(-4*24*time.Hour), 12),
				makePoint(now.Add(-10*24*time.Hour), 30),
			},
			"pool-b": {
				makePoint(now.Add(-1*24*time.Hour), 7),
				makePoint(now.Add(-3*24*time.Hour), 8),
			},
		},
	}

	results, err := SearchPools(context.Background(), api, PoolsOptions{
		Stablecoin: true,
		MinTVL:     10_000_000,
		Lookback:   7 * 24 * time.Hour,
		RankBy:     RankByChartMean,
		Limit:      10,
	}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("SearchPools() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("SearchPools() len = %d, want 2", len(results))
	}
	if results[0].PoolID != "pool-a" {
		t.Fatalf("top result = %s, want pool-a", results[0].PoolID)
	}
	if results[0].MetricValue != 11 {
		t.Fatalf("pool-a metric = %f, want 11", results[0].MetricValue)
	}
}

func TestSearchPoolsFiltersByYieldRange(t *testing.T) {
	t.Parallel()

	minYield := 7.0
	maxYield := 10.0
	api := fakeAPI{
		pools: []llama.Pool{
			makePool("pool-a", "maple", "Ethereum", "USDC", true, 15_000_000, floatPtr(4), floatPtr(6), nil),
			makePool("pool-b", "goldfinch", "Ethereum", "USDC", true, 12_000_000, floatPtr(5), floatPtr(9), nil),
			makePool("pool-c", "mainstreet", "Ethereum", "MSUSD", true, 20_000_000, floatPtr(12), floatPtr(12), nil),
		},
	}

	results, err := SearchPools(context.Background(), api, PoolsOptions{
		Stablecoin: true,
		MinTVL:     10_000_000,
		Lookback:   30 * 24 * time.Hour,
		RankBy:     RankBySnapshot30dMean,
		MinYield:   &minYield,
		MaxYield:   &maxYield,
		Limit:      10,
	}, func() time.Time { return time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("SearchPools() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("SearchPools() len = %d, want 1", len(results))
	}
	if results[0].PoolID != "pool-b" {
		t.Fatalf("SearchPools() pool = %s, want pool-b", results[0].PoolID)
	}
}

func TestLoadChartTrimsLookback(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	api := fakeAPI{
		charts: map[string][]llama.ChartPoint{
			"pool-a": {
				makePoint(now.Add(-40*24*time.Hour), 4),
				makePoint(now.Add(-10*24*time.Hour), 5),
				makePoint(now.Add(-1*24*time.Hour), 6),
			},
		},
	}

	points, err := LoadChart(context.Background(), api, ChartOptions{
		PoolID:   "pool-a",
		Lookback: 30 * 24 * time.Hour,
	}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("LoadChart() error = %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("LoadChart() len = %d, want 2", len(points))
	}
}

func TestRenderPoolsTable(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	err := RenderPoolsTable(&buffer, []PoolResult{
		{
			Rank:        1,
			MetricValue: 4.25,
			CurrentAPY:  floatPtr(4.10),
			TVLUsd:      12_500_000,
			Project:     "maple",
			Chain:       "Ethereum",
			Symbol:      "USDC",
			PoolID:      "pool-a",
			URL:         "https://defillama.com/yields/pool/pool-a",
		},
	})
	if err != nil {
		t.Fatalf("RenderPoolsTable() error = %v", err)
	}

	output := buffer.String()
	for _, fragment := range []string{"rank", "maple", "pool-a", "$12.50M", "4.25000", "https://defillama.com/yields/pool/pool-a"} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("RenderPoolsTable() output missing %q:\n%s", fragment, output)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	err := writeJSON(&buffer, []PoolResult{
		{
			Rank:        1,
			Metric:      string(RankBySnapshot30dMean),
			MetricValue: 4.25,
			URL:         "https://defillama.com/yields/pool/pool-a",
			Project:     "maple",
		},
	})
	if err != nil {
		t.Fatalf("writeJSON() error = %v", err)
	}

	var decoded []map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &decoded); err != nil {
		t.Fatalf("writeJSON() invalid JSON: %v", err)
	}
	if decoded[0]["metric"] != string(RankBySnapshot30dMean) {
		t.Fatalf("metric = %v, want %q", decoded[0]["metric"], RankBySnapshot30dMean)
	}
	if decoded[0]["url"] != "https://defillama.com/yields/pool/pool-a" {
		t.Fatalf("url = %v, want pool page URL", decoded[0]["url"])
	}
}

func TestParseOptionalFloat(t *testing.T) {
	t.Parallel()

	value, err := parseOptionalFloat("12.5")
	if err != nil {
		t.Fatalf("parseOptionalFloat() error = %v", err)
	}
	if value == nil || *value != 12.5 {
		t.Fatalf("parseOptionalFloat() = %v, want 12.5", value)
	}

	value, err = parseOptionalFloat("")
	if err != nil {
		t.Fatalf("parseOptionalFloat(empty) error = %v", err)
	}
	if value != nil {
		t.Fatalf("parseOptionalFloat(empty) = %v, want nil", value)
	}
}

func makePool(poolID, project, chain, symbol string, stablecoin bool, tvl float64, apy, apyMean30d, apyPct30d *float64) llama.Pool {
	return llama.Pool{
		Pool:       poolID,
		Project:    project,
		Chain:      chain,
		Symbol:     symbol,
		Stablecoin: stablecoin,
		TVLUsd:     tvl,
		APY:        apy,
		APYMean30d: apyMean30d,
		APYPct30D:  apyPct30d,
	}
}

func makePoint(ts time.Time, apy float64) llama.ChartPoint {
	return llama.ChartPoint{
		Timestamp: ts,
		TVLUsd:    10_000_000,
		APY:       floatPtr(apy),
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
