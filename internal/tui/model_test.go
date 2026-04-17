package tui

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"buddyapy/internal/app"
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

func TestPaneCycling(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), fakeAPI{}, func() time.Time { return time.Unix(0, 0).UTC() })

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := updated.(model)
	if got.focus != paneResults {
		t.Fatalf("focus after Tab = %v, want %v", got.focus, paneResults)
	}

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	got = updated.(model)
	if got.focus != paneFilters {
		t.Fatalf("focus after Shift+Tab = %v, want %v", got.focus, paneFilters)
	}
}

func TestSlashJumpsToSymbolEditor(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), fakeAPI{}, func() time.Time { return time.Unix(0, 0).UTC() })

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	got := updated.(model)
	if !got.editing {
		t.Fatal("editing = false, want true")
	}
	if got.editingField != app.FilterSymbol {
		t.Fatalf("editingField = %q, want %q", got.editingField, app.FilterSymbol)
	}
	if cmd == nil {
		t.Fatal("focus command = nil, want text input focus command")
	}
}

func TestEditingUpdatesFilterState(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), fakeAPI{
		pools: []llama.Pool{
			makePool("pool-a", "lido", "Ethereum", "STETH", false, 20_000_000, 5, 8),
		},
	}, func() time.Time { return time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC) })
	m.cachedPools = []llama.Pool{
		makePool("pool-a", "lido", "Ethereum", "STETH", false, 20_000_000, 5, 8),
	}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	got := updated.(model)

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	got = updated.(model)
	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}))
	got = updated.(model)
	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	got = updated.(model)

	if got.state.Symbol != "eth" {
		t.Fatalf("Symbol = %q, want eth", got.state.Symbol)
	}
	if got.searchSeq == 0 {
		t.Fatal("searchSeq = 0, want debounced search scheduled")
	}
}

func TestChartMeanMarksSearchDirty(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), fakeAPI{}, func() time.Time { return time.Unix(0, 0).UTC() })
	m.cachedPools = []llama.Pool{
		makePool("pool-a", "maple", "Ethereum", "USDC", true, 20_000_000, 4, 7),
	}
	m.filterList.Select(filterIndex(app.FilterRankBy))

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got := updated.(model)

	if currentRankBy(got.state) != string(app.RankByChartMean) {
		t.Fatalf("RankBy = %q, want %q", currentRankBy(got.state), app.RankByChartMean)
	}
	if !got.searchDirty {
		t.Fatal("searchDirty = false, want true")
	}
}

func TestApplyResultsTriggersLazyChartLoadingOnce(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), fakeAPI{}, func() time.Time { return time.Unix(0, 0).UTC() })
	m.applyResults([]app.PoolResult{
		{Rank: 1, PoolID: "pool-a", URL: "https://defillama.com/yields/pool/pool-a", Project: "lido", Chain: "Ethereum", Symbol: "STETH", TVLUsd: 20_000_000, Metric: string(app.RankBySnapshot30dMean), MetricValue: 8},
		{Rank: 2, PoolID: "pool-b", URL: "https://defillama.com/yields/pool/pool-b", Project: "rocket-pool", Chain: "Ethereum", Symbol: "RETH", TVLUsd: 19_000_000, Metric: string(app.RankBySnapshot30dMean), MetricValue: 7},
	})

	cmd := m.ensureSelectedChart()
	if cmd == nil {
		t.Fatal("ensureSelectedChart() cmd = nil, want chart load command")
	}
	if !m.chartBusy["pool-a"] {
		t.Fatal("chartBusy[pool-a] = false, want true")
	}

	cmd = m.ensureSelectedChart()
	if cmd != nil {
		t.Fatal("ensureSelectedChart() with busy chart returned non-nil command, want nil")
	}

	updated, _ := m.Update(chartLoadedMsg{
		poolID: "pool-a",
		points: []llama.ChartPoint{{Timestamp: time.Now().UTC(), TVLUsd: 20_000_000, APY: floatPtr(8)}},
	})
	got := updated.(model)

	if len(got.chartCache["pool-a"]) != 1 {
		t.Fatalf("chartCache[pool-a] len = %d, want 1", len(got.chartCache["pool-a"]))
	}
	if got.chartBusy["pool-a"] {
		t.Fatal("chartBusy[pool-a] = true, want false")
	}
}

func makePool(poolID, project, chain, symbol string, stablecoin bool, tvl, apy, apyMean30d float64) llama.Pool {
	return llama.Pool{
		Pool:       poolID,
		Project:    project,
		Chain:      chain,
		Symbol:     symbol,
		Stablecoin: stablecoin,
		TVLUsd:     tvl,
		APY:        floatPtr(apy),
		APYMean30d: floatPtr(apyMean30d),
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
