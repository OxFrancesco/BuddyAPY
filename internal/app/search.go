package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"buddyapy/internal/llama"
)

const (
	DefaultBaseURL       = "https://yields.llama.fi"
	DefaultPoolPageURL   = "https://defillama.com/yields/pool"
	defaultWorkerCount   = 6
	defaultPoolsLimit    = 10
	defaultPoolsLookback = 30 * 24 * time.Hour
)

type RankBy string

const (
	RankBySnapshot30dMean RankBy = "snapshot-30d-mean"
	RankByChartMean       RankBy = "chart-mean"
	RankByCurrentAPY      RankBy = "current-apy"
)

type API interface {
	Pools(ctx context.Context) ([]llama.Pool, error)
	Chart(ctx context.Context, pool string) ([]llama.ChartPoint, error)
}

type PoolsOptions struct {
	Stablecoin bool
	MinTVL     float64
	Lookback   time.Duration
	RankBy     RankBy
	MinYield   *float64
	MaxYield   *float64
	Chain      string
	Project    string
	Limit      int
}

func (o PoolsOptions) Validate() error {
	if o.Limit <= 0 {
		return errors.New("limit must be greater than 0")
	}
	if o.Lookback <= 0 {
		return errors.New("lookback must be greater than 0")
	}

	switch o.RankBy {
	case RankBySnapshot30dMean:
		if o.Lookback != defaultPoolsLookback {
			return fmt.Errorf("rank-by %q requires a 30d lookback; use --rank-by=%s for other windows", o.RankBy, RankByChartMean)
		}
	case RankByChartMean, RankByCurrentAPY:
	default:
		return fmt.Errorf("unsupported rank-by %q", o.RankBy)
	}
	if o.MinYield != nil && o.MaxYield != nil && *o.MinYield > *o.MaxYield {
		return errors.New("min-yield cannot be greater than max-yield")
	}

	return nil
}

type ChartOptions struct {
	PoolID   string
	Lookback time.Duration
}

func (o ChartOptions) Validate() error {
	if strings.TrimSpace(o.PoolID) == "" {
		return errors.New("pool is required")
	}
	if o.Lookback < 0 {
		return errors.New("lookback cannot be negative")
	}
	return nil
}

type PoolResult struct {
	Rank        int      `json:"rank"`
	Metric      string   `json:"metric"`
	MetricValue float64  `json:"metricValue"`
	PoolID      string   `json:"pool"`
	URL         string   `json:"url"`
	Project     string   `json:"project"`
	Chain       string   `json:"chain"`
	Symbol      string   `json:"symbol"`
	TVLUsd      float64  `json:"tvlUsd"`
	Stablecoin  bool     `json:"stablecoin"`
	CurrentAPY  *float64 `json:"apy,omitempty"`
	APYMean30d  *float64 `json:"apyMean30d,omitempty"`
	APYPct30D   *float64 `json:"apyPct30D,omitempty"`
}

func SearchPools(ctx context.Context, api API, opts PoolsOptions, now func() time.Time) ([]PoolResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}

	pools, err := api.Pools(ctx)
	if err != nil {
		return nil, err
	}

	filtered := filterPools(pools, opts)
	if len(filtered) == 0 {
		return []PoolResult{}, nil
	}

	var results []PoolResult
	switch opts.RankBy {
	case RankBySnapshot30dMean:
		results = snapshotMeanResults(filtered)
	case RankByCurrentAPY:
		results = currentAPYResults(filtered)
	case RankByChartMean:
		results, err = chartMeanResults(ctx, api, filtered, opts.Lookback, now().UTC())
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported rank-by %q", opts.RankBy)
	}

	results = filterResultsByYield(results, opts.MinYield, opts.MaxYield)
	sortPoolResults(results)
	if len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	for idx := range results {
		results[idx].Rank = idx + 1
	}

	return results, nil
}

func LoadChart(ctx context.Context, api API, opts ChartOptions, now func() time.Time) ([]llama.ChartPoint, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}

	points, err := api.Chart(ctx, opts.PoolID)
	if err != nil {
		return nil, err
	}
	if opts.Lookback == 0 {
		return points, nil
	}

	return trimChart(points, now().UTC().Add(-opts.Lookback)), nil
}

func filterPools(pools []llama.Pool, opts PoolsOptions) []llama.Pool {
	filtered := make([]llama.Pool, 0, len(pools))
	chain := strings.ToLower(strings.TrimSpace(opts.Chain))
	project := strings.ToLower(strings.TrimSpace(opts.Project))

	for _, pool := range pools {
		if opts.Stablecoin && !pool.Stablecoin {
			continue
		}
		if pool.TVLUsd < opts.MinTVL {
			continue
		}
		if chain != "" && strings.ToLower(pool.Chain) != chain {
			continue
		}
		if project != "" && strings.ToLower(pool.Project) != project {
			continue
		}
		filtered = append(filtered, pool)
	}

	return filtered
}

func snapshotMeanResults(pools []llama.Pool) []PoolResult {
	results := make([]PoolResult, 0, len(pools))
	for _, pool := range pools {
		if pool.APYMean30d == nil {
			continue
		}
		results = append(results, newPoolResult(pool, string(RankBySnapshot30dMean), *pool.APYMean30d))
	}
	return results
}

func currentAPYResults(pools []llama.Pool) []PoolResult {
	results := make([]PoolResult, 0, len(pools))
	for _, pool := range pools {
		if pool.APY == nil {
			continue
		}
		results = append(results, newPoolResult(pool, string(RankByCurrentAPY), *pool.APY))
	}
	return results
}

func chartMeanResults(ctx context.Context, api API, pools []llama.Pool, lookback time.Duration, now time.Time) ([]PoolResult, error) {
	type chartResult struct {
		pool   llama.Pool
		metric float64
		ok     bool
		err    error
	}

	jobs := make(chan llama.Pool)
	resultsCh := make(chan chartResult, len(pools))

	workerCount := defaultWorkerCount
	if len(pools) < workerCount {
		workerCount = len(pools)
	}

	var wg sync.WaitGroup
	cutoff := now.Add(-lookback)

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pool := range jobs {
				points, err := api.Chart(ctx, pool.Pool)
				if err != nil {
					resultsCh <- chartResult{pool: pool, err: err}
					continue
				}
				mean, ok := meanAPYSince(points, cutoff)
				resultsCh <- chartResult{pool: pool, metric: mean, ok: ok}
			}
		}()
	}

	go func() {
		for _, pool := range pools {
			jobs <- pool
		}
		close(jobs)
		wg.Wait()
		close(resultsCh)
	}()

	results := make([]PoolResult, 0, len(pools))
	var firstErr error
	for result := range resultsCh {
		if result.err != nil && firstErr == nil {
			firstErr = result.err
			continue
		}
		if !result.ok {
			continue
		}
		results = append(results, newPoolResult(result.pool, string(RankByChartMean), result.metric))
	}

	if firstErr != nil {
		return nil, firstErr
	}

	return results, nil
}

func meanAPYSince(points []llama.ChartPoint, cutoff time.Time) (float64, bool) {
	var (
		sum   float64
		count int
	)

	for _, point := range points {
		if point.Timestamp.Before(cutoff) || point.APY == nil {
			continue
		}
		sum += *point.APY
		count++
	}

	if count == 0 {
		return 0, false
	}

	return sum / float64(count), true
}

func trimChart(points []llama.ChartPoint, cutoff time.Time) []llama.ChartPoint {
	trimmed := make([]llama.ChartPoint, 0, len(points))
	for _, point := range points {
		if point.Timestamp.Before(cutoff) {
			continue
		}
		trimmed = append(trimmed, point)
	}
	return trimmed
}

func newPoolResult(pool llama.Pool, metric string, metricValue float64) PoolResult {
	return PoolResult{
		Metric:      metric,
		MetricValue: metricValue,
		PoolID:      pool.Pool,
		URL:         poolPageURL(pool.Pool),
		Project:     pool.Project,
		Chain:       pool.Chain,
		Symbol:      pool.Symbol,
		TVLUsd:      pool.TVLUsd,
		Stablecoin:  pool.Stablecoin,
		CurrentAPY:  pool.APY,
		APYMean30d:  pool.APYMean30d,
		APYPct30D:   pool.APYPct30D,
	}
}

func poolPageURL(poolID string) string {
	return DefaultPoolPageURL + "/" + strings.TrimSpace(poolID)
}

func filterResultsByYield(results []PoolResult, minYield, maxYield *float64) []PoolResult {
	if minYield == nil && maxYield == nil {
		return results
	}

	filtered := make([]PoolResult, 0, len(results))
	for _, result := range results {
		if minYield != nil && result.MetricValue < *minYield {
			continue
		}
		if maxYield != nil && result.MetricValue > *maxYield {
			continue
		}
		filtered = append(filtered, result)
	}

	return filtered
}

func sortPoolResults(results []PoolResult) {
	sort.SliceStable(results, func(i, j int) bool {
		left, right := results[i], results[j]
		if !nearlyEqual(left.MetricValue, right.MetricValue) {
			return left.MetricValue > right.MetricValue
		}
		if !nearlyEqual(left.TVLUsd, right.TVLUsd) {
			return left.TVLUsd > right.TVLUsd
		}
		if left.Project != right.Project {
			return left.Project < right.Project
		}
		if left.Chain != right.Chain {
			return left.Chain < right.Chain
		}
		if left.Symbol != right.Symbol {
			return left.Symbol < right.Symbol
		}
		return left.PoolID < right.PoolID
	})
}

func nearlyEqual(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}
