package app

import (
	"fmt"
	"io"
	"math"
	"text/tabwriter"

	"buddyapy/internal/llama"
)

func RenderPoolsTable(w io.Writer, results []PoolResult) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "rank\tmetric_value\tcurrent_apy\ttvl_usd\tproject\tchain\tsymbol\tpool\turl")
	for _, result := range results {
		fmt.Fprintf(
			tw,
			"%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			result.Rank,
			formatAPY(result.MetricValue),
			formatNullableAPY(result.CurrentAPY),
			formatUSDCompact(result.TVLUsd),
			result.Project,
			result.Chain,
			result.Symbol,
			result.PoolID,
			result.URL,
		)
	}
	return tw.Flush()
}

func RenderChartTable(w io.Writer, points []llama.ChartPoint) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "timestamp\tapy\tapy_base\tapy_reward\ttvl_usd")
	for _, point := range points {
		fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			point.Timestamp.UTC().Format(timeLayout),
			formatNullableAPY(point.APY),
			formatNullableAPY(point.APYBase),
			formatNullableAPY(point.APYReward),
			formatUSDCompact(point.TVLUsd),
		)
	}
	return tw.Flush()
}

const timeLayout = "2006-01-02T15:04:05Z07:00"

func formatNullableAPY(value *float64) string {
	if value == nil {
		return "n/a"
	}
	return formatAPY(*value)
}

func formatAPY(value float64) string {
	return fmt.Sprintf("%.5f", value)
}

func formatUSDCompact(value float64) string {
	absValue := math.Abs(value)
	switch {
	case absValue >= 1_000_000_000:
		return fmt.Sprintf("$%.2fB", value/1_000_000_000)
	case absValue >= 1_000_000:
		return fmt.Sprintf("$%.2fM", value/1_000_000)
	case absValue >= 1_000:
		return fmt.Sprintf("$%.2fK", value/1_000)
	default:
		return fmt.Sprintf("$%.2f", value)
	}
}
