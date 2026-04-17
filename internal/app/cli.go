package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"buddyapy/internal/llama"
)

type Config struct {
	API        API
	BaseURL    string
	HTTPClient *http.Client
	Now        func() time.Time
}

func DefaultConfig() Config {
	return Config{
		BaseURL:    DefaultBaseURL,
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
		Now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer, cfg Config) error {
	if len(args) == 0 {
		printRootUsage(stderr)
		return errors.New("missing subcommand")
	}

	switch args[0] {
	case "help", "-h", "--help":
		printRootUsage(stdout)
		return nil
	case "pools":
		return runPools(ctx, args[1:], stdout, stderr, cfg)
	case "chart":
		return runChart(ctx, args[1:], stdout, stderr, cfg)
	default:
		printRootUsage(stderr)
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func runPools(ctx context.Context, args []string, stdout, stderr io.Writer, cfg Config) error {
	fs := flag.NewFlagSet("pools", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: buddyapy pools [flags]")
		fs.PrintDefaults()
	}

	state := DefaultFilterState()
	jsonOutput := false

	for _, definition := range PoolFilterDefinitions() {
		switch definition.ID {
		case FilterStablecoin, FilterFuzzy:
			current := state.BoolValue(definition.ID)
			switch definition.ID {
			case FilterStablecoin:
				fs.BoolVar(&state.Stablecoin, definition.FlagName, current, definition.Help)
			case FilterFuzzy:
				fs.BoolVar(&state.Fuzzy, definition.FlagName, current, definition.Help)
			}
		case FilterMinTVL:
			fs.StringVar(&state.MinTVL, definition.FlagName, state.MinTVL, definition.Help)
		case FilterLookback:
			fs.StringVar(&state.Lookback, definition.FlagName, state.Lookback, definition.Help)
		case FilterRankBy:
			fs.StringVar(&state.RankBy, definition.FlagName, state.RankBy, definition.Help)
		case FilterMinYield:
			fs.StringVar(&state.MinYield, definition.FlagName, state.MinYield, definition.Help)
		case FilterMaxYield:
			fs.StringVar(&state.MaxYield, definition.FlagName, state.MaxYield, definition.Help)
		case FilterSymbol:
			fs.StringVar(&state.Symbol, definition.FlagName, state.Symbol, definition.Help)
		case FilterChain:
			fs.StringVar(&state.Chain, definition.FlagName, state.Chain, definition.Help)
		case FilterProject:
			fs.StringVar(&state.Project, definition.FlagName, state.Project, definition.Help)
		case FilterLimit:
			fs.StringVar(&state.Limit, definition.FlagName, state.Limit, definition.Help)
		}
	}
	fs.BoolVar(&jsonOutput, "json", false, "output results as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	options, err := state.ToPoolsOptions()
	if err != nil {
		return err
	}

	results, err := SearchPools(ctx, resolveAPI(cfg), options, resolveNow(cfg))
	if err != nil {
		return err
	}

	if jsonOutput {
		return writeJSON(stdout, results)
	}

	return RenderPoolsTable(stdout, results)
}

func runChart(ctx context.Context, args []string, stdout, stderr io.Writer, cfg Config) error {
	fs := flag.NewFlagSet("chart", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: buddyapy chart --pool <pool-id> [flags]")
		fs.PrintDefaults()
	}

	var (
		poolID      string
		lookbackRaw string
		jsonOutput  bool
	)

	fs.StringVar(&poolID, "pool", "", "pool identifier from /pools")
	fs.StringVar(&lookbackRaw, "lookback", "", "optional lookback window (for example 30d, 7d, 48h)")
	fs.BoolVar(&jsonOutput, "json", false, "output chart rows as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	var lookback time.Duration
	if strings.TrimSpace(lookbackRaw) != "" {
		var err error
		lookback, err = parseLookback(lookbackRaw)
		if err != nil {
			return fmt.Errorf("parse --lookback: %w", err)
		}
	}

	points, err := LoadChart(ctx, resolveAPI(cfg), ChartOptions{
		PoolID:   poolID,
		Lookback: lookback,
	}, resolveNow(cfg))
	if err != nil {
		return err
	}

	if jsonOutput {
		return writeJSON(stdout, points)
	}

	return RenderChartTable(stdout, points)
}

func resolveAPI(cfg Config) API {
	if cfg.API != nil {
		return cfg.API
	}
	baseURL := cfg.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	return llama.NewClient(baseURL, cfg.HTTPClient)
}

func ResolveAPI(cfg Config) API {
	return resolveAPI(cfg)
}

func resolveNow(cfg Config) func() time.Time {
	if cfg.Now != nil {
		return cfg.Now
	}
	return func() time.Time {
		return time.Now().UTC()
	}
}

func ResolveNow(cfg Config) func() time.Time {
	return resolveNow(cfg)
}

func parseUSDValue(raw string) (float64, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return 0, nil
	}

	multiplier := 1.0
	switch suffix := raw[len(raw)-1]; suffix {
	case 'k':
		multiplier = 1_000
		raw = raw[:len(raw)-1]
	case 'm':
		multiplier = 1_000_000
		raw = raw[:len(raw)-1]
	case 'b':
		multiplier = 1_000_000_000
		raw = raw[:len(raw)-1]
	}

	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, errors.New("value cannot be negative")
	}

	return value * multiplier, nil
}

func parseLookback(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return 0, errors.New("value is required")
	}
	if len(raw) < 2 {
		return 0, fmt.Errorf("invalid duration %q", raw)
	}

	unit := raw[len(raw)-1]
	numberPart := strings.TrimSpace(raw[:len(raw)-1])
	value, err := strconv.ParseFloat(numberPart, 64)
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, errors.New("duration must be greater than 0")
	}

	switch unit {
	case 'm':
		return time.Duration(value * float64(time.Minute)), nil
	case 'h':
		return time.Duration(value * float64(time.Hour)), nil
	case 'd':
		return time.Duration(value * float64(24*time.Hour)), nil
	case 'w':
		return time.Duration(value * float64(7*24*time.Hour)), nil
	default:
		return 0, fmt.Errorf("unsupported duration suffix %q", string(unit))
	}
}

func parseOptionalFloat(raw string) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, err
	}

	return &value, nil
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  buddyapy pools [flags]")
	fmt.Fprintln(w, "  buddyapy chart --pool <pool-id> [flags]")
	fmt.Fprintln(w, "  buddyapy tui")
}
