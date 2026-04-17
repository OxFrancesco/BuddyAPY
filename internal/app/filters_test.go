package app

import (
	"testing"
	"time"
)

func TestDefaultFilterState(t *testing.T) {
	t.Parallel()

	state := DefaultFilterState()

	if state.Lookback != "30d" {
		t.Fatalf("Lookback = %q, want 30d", state.Lookback)
	}
	if state.RankBy != string(RankBySnapshot30dMean) {
		t.Fatalf("RankBy = %q, want %q", state.RankBy, RankBySnapshot30dMean)
	}
	if state.Limit != "10" {
		t.Fatalf("Limit = %q, want 10", state.Limit)
	}
}

func TestFilterStateRoundTrip(t *testing.T) {
	t.Parallel()

	state := DefaultFilterState()
	state.SetBoolValue(FilterStablecoin, true)
	state.SetStringValue(FilterMinTVL, "10m")
	state.SetStringValue(FilterMinYield, "8")
	state.SetStringValue(FilterMaxYield, "10")
	state.SetStringValue(FilterSymbol, "eth")
	state.SetBoolValue(FilterFuzzy, true)
	state.SetStringValue(FilterChain, "Ethereum")
	state.SetStringValue(FilterProject, "lido")
	state.SetStringValue(FilterLimit, "25")

	options, err := state.ToPoolsOptions()
	if err != nil {
		t.Fatalf("ToPoolsOptions() error = %v", err)
	}

	if !options.Stablecoin {
		t.Fatal("Stablecoin = false, want true")
	}
	if options.MinTVL != 10_000_000 {
		t.Fatalf("MinTVL = %f, want 10000000", options.MinTVL)
	}
	if options.Lookback != 30*24*time.Hour {
		t.Fatalf("Lookback = %s, want 30d", options.Lookback)
	}
	if options.MinYield == nil || *options.MinYield != 8 {
		t.Fatalf("MinYield = %v, want 8", options.MinYield)
	}
	if options.MaxYield == nil || *options.MaxYield != 10 {
		t.Fatalf("MaxYield = %v, want 10", options.MaxYield)
	}
	if options.Symbol != "eth" || !options.Fuzzy {
		t.Fatalf("symbol/fuzzy = %q/%v, want eth/true", options.Symbol, options.Fuzzy)
	}
	if options.Chain != "Ethereum" || options.Project != "lido" {
		t.Fatalf("chain/project = %q/%q, want Ethereum/lido", options.Chain, options.Project)
	}
	if options.Limit != 25 {
		t.Fatalf("Limit = %d, want 25", options.Limit)
	}
}

func TestFilterStateValidation(t *testing.T) {
	t.Parallel()

	state := DefaultFilterState()
	state.Fuzzy = true

	if _, err := state.ToPoolsOptions(); err == nil {
		t.Fatal("ToPoolsOptions() error = nil, want fuzzy-without-symbol validation error")
	}

	state = DefaultFilterState()
	state.Lookback = "7d"
	if _, err := state.ToPoolsOptions(); err == nil {
		t.Fatal("ToPoolsOptions() error = nil, want snapshot-30d-mean validation error")
	}
}

func TestFilterStateClearAndReset(t *testing.T) {
	t.Parallel()

	state := DefaultFilterState()
	state.SetStringValue(FilterSymbol, "eth")
	state.SetBoolValue(FilterStablecoin, true)
	state.Clear(FilterSymbol)
	state.Clear(FilterStablecoin)

	if state.Symbol != "" {
		t.Fatalf("Symbol after Clear = %q, want empty", state.Symbol)
	}
	if state.Stablecoin {
		t.Fatal("Stablecoin after Clear = true, want false")
	}

	state.SetStringValue(FilterChain, "Base")
	state.Reset()
	if state.Chain != "" || state.Lookback != "30d" {
		t.Fatalf("Reset() = %+v, want defaults", state)
	}
}
