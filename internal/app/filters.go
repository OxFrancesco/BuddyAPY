package app

import (
	"fmt"
	"strconv"
	"strings"
)

type FilterID string

const (
	FilterStablecoin FilterID = "stablecoin"
	FilterMinTVL     FilterID = "min-tvl"
	FilterLookback   FilterID = "lookback"
	FilterRankBy     FilterID = "rank-by"
	FilterMinYield   FilterID = "min-yield"
	FilterMaxYield   FilterID = "max-yield"
	FilterSymbol     FilterID = "symbol"
	FilterFuzzy      FilterID = "fuzzy"
	FilterChain      FilterID = "chain"
	FilterProject    FilterID = "project"
	FilterLimit      FilterID = "limit"
)

type FilterKind string

const (
	FilterKindBool   FilterKind = "bool"
	FilterKindText   FilterKind = "text"
	FilterKindSelect FilterKind = "select"
)

type FilterOption struct {
	Value string
	Label string
}

type FilterDefinition struct {
	ID          FilterID
	FlagName    string
	Label       string
	Kind        FilterKind
	Help        string
	Placeholder string
	DefaultText string
	DefaultBool bool
	Options     []FilterOption
}

var poolFilterDefinitions = []FilterDefinition{
	{
		ID:          FilterStablecoin,
		FlagName:    "stablecoin",
		Label:       "Stablecoin",
		Kind:        FilterKindBool,
		Help:        "only include pools marked as stablecoin",
		DefaultBool: false,
	},
	{
		ID:          FilterMinTVL,
		FlagName:    "min-tvl",
		Label:       "Min TVL",
		Kind:        FilterKindText,
		Help:        "minimum TVL in USD (supports k, m, b suffixes)",
		Placeholder: "10m",
	},
	{
		ID:          FilterLookback,
		FlagName:    "lookback",
		Label:       "Lookback",
		Kind:        FilterKindText,
		Help:        "lookback window (for example 30d, 7d, 48h)",
		Placeholder: "30d",
		DefaultText: "30d",
	},
	{
		ID:          FilterRankBy,
		FlagName:    "rank-by",
		Label:       "Rank By",
		Kind:        FilterKindSelect,
		Help:        "ranking metric: snapshot-30d-mean, chart-mean, current-apy",
		DefaultText: string(RankBySnapshot30dMean),
		Options: []FilterOption{
			{Value: string(RankBySnapshot30dMean), Label: "30d mean"},
			{Value: string(RankByChartMean), Label: "Chart mean"},
			{Value: string(RankByCurrentAPY), Label: "Current APY"},
		},
	},
	{
		ID:          FilterMinYield,
		FlagName:    "min-yield",
		Label:       "Min Yield",
		Kind:        FilterKindText,
		Help:        "minimum yield for the selected ranking metric",
		Placeholder: "8",
	},
	{
		ID:          FilterMaxYield,
		FlagName:    "max-yield",
		Label:       "Max Yield",
		Kind:        FilterKindText,
		Help:        "maximum yield for the selected ranking metric",
		Placeholder: "10",
	},
	{
		ID:          FilterSymbol,
		FlagName:    "symbol",
		Label:       "Symbol",
		Kind:        FilterKindText,
		Help:        "token or pool symbol filter",
		Placeholder: "eth",
	},
	{
		ID:          FilterFuzzy,
		FlagName:    "fuzzy",
		Label:       "Fuzzy",
		Kind:        FilterKindBool,
		Help:        "use substring matching for --symbol",
		DefaultBool: false,
	},
	{
		ID:          FilterChain,
		FlagName:    "chain",
		Label:       "Chain",
		Kind:        FilterKindText,
		Help:        "exact chain filter (case-insensitive)",
		Placeholder: "Ethereum",
	},
	{
		ID:          FilterProject,
		FlagName:    "project",
		Label:       "Project",
		Kind:        FilterKindText,
		Help:        "exact project filter (case-insensitive)",
		Placeholder: "maple",
	},
	{
		ID:          FilterLimit,
		FlagName:    "limit",
		Label:       "Limit",
		Kind:        FilterKindText,
		Help:        "maximum number of results to return",
		Placeholder: strconv.Itoa(defaultPoolsLimit),
		DefaultText: strconv.Itoa(defaultPoolsLimit),
	},
}

type FilterState struct {
	Stablecoin bool
	MinTVL     string
	Lookback   string
	RankBy     string
	MinYield   string
	MaxYield   string
	Symbol     string
	Fuzzy      bool
	Chain      string
	Project    string
	Limit      string
}

func DefaultFilterState() FilterState {
	return FilterState{
		Lookback: "30d",
		RankBy:   string(RankBySnapshot30dMean),
		Limit:    strconv.Itoa(defaultPoolsLimit),
	}
}

func PoolFilterDefinitions() []FilterDefinition {
	defs := make([]FilterDefinition, len(poolFilterDefinitions))
	copy(defs, poolFilterDefinitions)
	return defs
}

func PoolFilterDefinition(id FilterID) FilterDefinition {
	for _, definition := range poolFilterDefinitions {
		if definition.ID == id {
			return definition
		}
	}
	panic(fmt.Sprintf("unknown filter definition %q", id))
}

func (s FilterState) ToPoolsOptions() (PoolsOptions, error) {
	minTVL, err := parseUSDValue(s.MinTVL)
	if err != nil {
		return PoolsOptions{}, fmt.Errorf("parse %s: %w", FilterMinTVL, err)
	}
	lookback, err := parseLookback(s.effectiveText(FilterLookback))
	if err != nil {
		return PoolsOptions{}, fmt.Errorf("parse %s: %w", FilterLookback, err)
	}
	minYield, err := parseOptionalFloat(s.MinYield)
	if err != nil {
		return PoolsOptions{}, fmt.Errorf("parse %s: %w", FilterMinYield, err)
	}
	maxYield, err := parseOptionalFloat(s.MaxYield)
	if err != nil {
		return PoolsOptions{}, fmt.Errorf("parse %s: %w", FilterMaxYield, err)
	}
	limit, err := parsePositiveInt(s.effectiveText(FilterLimit))
	if err != nil {
		return PoolsOptions{}, fmt.Errorf("parse %s: %w", FilterLimit, err)
	}

	options := PoolsOptions{
		Stablecoin: s.Stablecoin,
		MinTVL:     minTVL,
		Lookback:   lookback,
		RankBy:     RankBy(s.effectiveText(FilterRankBy)),
		MinYield:   minYield,
		MaxYield:   maxYield,
		Symbol:     strings.TrimSpace(s.Symbol),
		Fuzzy:      s.Fuzzy,
		Chain:      strings.TrimSpace(s.Chain),
		Project:    strings.TrimSpace(s.Project),
		Limit:      limit,
	}

	return options, options.Validate()
}

func (s FilterState) Summary(id FilterID) string {
	switch id {
	case FilterStablecoin:
		if s.Stablecoin {
			return "Only stablecoins"
		}
		return "All pools"
	case FilterMinTVL:
		if strings.TrimSpace(s.MinTVL) == "" {
			return "Any TVL"
		}
		return ">=" + strings.TrimSpace(s.MinTVL)
	case FilterLookback:
		return s.effectiveText(FilterLookback)
	case FilterRankBy:
		for _, option := range PoolFilterDefinition(FilterRankBy).Options {
			if option.Value == s.effectiveText(FilterRankBy) {
				return option.Label
			}
		}
		return s.effectiveText(FilterRankBy)
	case FilterMinYield:
		if strings.TrimSpace(s.MinYield) == "" {
			return "No floor"
		}
		return ">=" + strings.TrimSpace(s.MinYield)
	case FilterMaxYield:
		if strings.TrimSpace(s.MaxYield) == "" {
			return "No cap"
		}
		return "<=" + strings.TrimSpace(s.MaxYield)
	case FilterSymbol:
		if strings.TrimSpace(s.Symbol) == "" {
			return "Any symbol"
		}
		return strings.TrimSpace(s.Symbol)
	case FilterFuzzy:
		if s.Fuzzy {
			return "Substring"
		}
		return "Exact"
	case FilterChain:
		if strings.TrimSpace(s.Chain) == "" {
			return "Any chain"
		}
		return strings.TrimSpace(s.Chain)
	case FilterProject:
		if strings.TrimSpace(s.Project) == "" {
			return "Any project"
		}
		return strings.TrimSpace(s.Project)
	case FilterLimit:
		return s.effectiveText(FilterLimit)
	default:
		return ""
	}
}

func (s FilterState) ActiveSummaries() []string {
	summaries := make([]string, 0, len(poolFilterDefinitions))
	for _, definition := range poolFilterDefinitions {
		if s.isDefault(definition.ID) {
			continue
		}
		label := definition.Label
		if definition.ID == FilterStablecoin && s.Stablecoin {
			summaries = append(summaries, "Stablecoins")
			continue
		}
		if definition.ID == FilterFuzzy && s.Fuzzy {
			summaries = append(summaries, "Fuzzy symbol")
			continue
		}
		summaries = append(summaries, fmt.Sprintf("%s: %s", label, s.Summary(definition.ID)))
	}
	return summaries
}

func (s FilterState) StringValue(id FilterID) string {
	switch id {
	case FilterMinTVL:
		return s.MinTVL
	case FilterLookback:
		return s.Lookback
	case FilterRankBy:
		return s.RankBy
	case FilterMinYield:
		return s.MinYield
	case FilterMaxYield:
		return s.MaxYield
	case FilterSymbol:
		return s.Symbol
	case FilterChain:
		return s.Chain
	case FilterProject:
		return s.Project
	case FilterLimit:
		return s.Limit
	default:
		return ""
	}
}

func (s *FilterState) SetStringValue(id FilterID, value string) {
	switch id {
	case FilterMinTVL:
		s.MinTVL = value
	case FilterLookback:
		s.Lookback = value
	case FilterRankBy:
		s.RankBy = value
	case FilterMinYield:
		s.MinYield = value
	case FilterMaxYield:
		s.MaxYield = value
	case FilterSymbol:
		s.Symbol = value
	case FilterChain:
		s.Chain = value
	case FilterProject:
		s.Project = value
	case FilterLimit:
		s.Limit = value
	}
}

func (s FilterState) BoolValue(id FilterID) bool {
	switch id {
	case FilterStablecoin:
		return s.Stablecoin
	case FilterFuzzy:
		return s.Fuzzy
	default:
		return false
	}
}

func (s *FilterState) SetBoolValue(id FilterID, value bool) {
	switch id {
	case FilterStablecoin:
		s.Stablecoin = value
	case FilterFuzzy:
		s.Fuzzy = value
	}
}

func (s *FilterState) Toggle(id FilterID) {
	s.SetBoolValue(id, !s.BoolValue(id))
}

func (s *FilterState) Clear(id FilterID) {
	definition := PoolFilterDefinition(id)
	switch definition.Kind {
	case FilterKindBool:
		s.SetBoolValue(id, definition.DefaultBool)
	default:
		s.SetStringValue(id, definition.DefaultText)
	}
}

func (s *FilterState) Reset() {
	*s = DefaultFilterState()
}

func (s FilterState) effectiveText(id FilterID) string {
	value := strings.TrimSpace(s.StringValue(id))
	if value != "" {
		return value
	}
	return PoolFilterDefinition(id).DefaultText
}

func (s FilterState) isDefault(id FilterID) bool {
	definition := PoolFilterDefinition(id)
	switch definition.Kind {
	case FilterKindBool:
		return s.BoolValue(id) == definition.DefaultBool
	default:
		return strings.TrimSpace(s.StringValue(id)) == strings.TrimSpace(definition.DefaultText)
	}
}

func parsePositiveInt(raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, fmt.Errorf("value must be greater than 0")
	}
	return value, nil
}
