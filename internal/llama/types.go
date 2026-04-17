package llama

import "time"

type PoolsResponse struct {
	Status string `json:"status"`
	Data   []Pool `json:"data"`
}

type Pool struct {
	Chain            string   `json:"chain"`
	Project          string   `json:"project"`
	Symbol           string   `json:"symbol"`
	TVLUsd           float64  `json:"tvlUsd"`
	APYBase          *float64 `json:"apyBase"`
	APYReward        *float64 `json:"apyReward"`
	APY              *float64 `json:"apy"`
	RewardTokens     []string `json:"rewardTokens"`
	Pool             string   `json:"pool"`
	APYPct1D         *float64 `json:"apyPct1D"`
	APYPct7D         *float64 `json:"apyPct7D"`
	APYPct30D        *float64 `json:"apyPct30D"`
	Stablecoin       bool     `json:"stablecoin"`
	ILRisk           string   `json:"ilRisk"`
	Exposure         string   `json:"exposure"`
	PoolMeta         *string  `json:"poolMeta"`
	Mu               *float64 `json:"mu"`
	Sigma            *float64 `json:"sigma"`
	Count            int      `json:"count"`
	Outlier          bool     `json:"outlier"`
	UnderlyingTokens []string `json:"underlyingTokens"`
	IL7d             *float64 `json:"il7d"`
	APYBase7d        *float64 `json:"apyBase7d"`
	APYMean30d       *float64 `json:"apyMean30d"`
	VolumeUsd1d      *float64 `json:"volumeUsd1d"`
	VolumeUsd7d      *float64 `json:"volumeUsd7d"`
	APYBaseInception *float64 `json:"apyBaseInception"`
}

type ChartResponse struct {
	Status string       `json:"status"`
	Data   []ChartPoint `json:"data"`
}

type ChartPoint struct {
	Timestamp time.Time `json:"timestamp"`
	TVLUsd    float64   `json:"tvlUsd"`
	APY       *float64  `json:"apy"`
	APYBase   *float64  `json:"apyBase"`
	APYReward *float64  `json:"apyReward"`
	IL7d      *float64  `json:"il7d"`
	APYBase7d *float64  `json:"apyBase7d"`
}
