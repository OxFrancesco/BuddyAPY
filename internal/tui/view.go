package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"buddyapy/internal/app"
	"buddyapy/internal/llama"
)

type styles struct {
	page        lipgloss.Style
	title       lipgloss.Style
	subtitle    lipgloss.Style
	status      lipgloss.Style
	error       lipgloss.Style
	helpBar     lipgloss.Style
	chip        lipgloss.Style
	chipMuted   lipgloss.Style
	panel       lipgloss.Style
	panelFocus  lipgloss.Style
	panelTitle  lipgloss.Style
	panelMuted  lipgloss.Style
	inputBox    lipgloss.Style
	empty       lipgloss.Style
	spinner     lipgloss.Style
	metricLabel lipgloss.Style
}

func newStyles() styles {
	return styles{
		page: lipgloss.NewStyle().Padding(1, 1),
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F8FAFC")),
		subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")),
		status: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0F172A")).
			Background(lipgloss.Color("#99F6E4")).
			Padding(0, 1),
		error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8FAFC")).
			Background(lipgloss.Color("#B91C1C")).
			Padding(0, 1),
		helpBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CBD5E1")).
			PaddingTop(1),
		chip: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0F172A")).
			Background(lipgloss.Color("#CCFBF1")).
			Padding(0, 1),
		chipMuted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0")).
			Background(lipgloss.Color("#334155")).
			Padding(0, 1),
		panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#334155")).
			Padding(0, 1),
		panelFocus: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2DD4BF")).
			Padding(0, 1),
		panelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F8FAFC")),
		panelMuted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")),
		inputBox: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#22C55E")).
			Padding(0, 1),
		empty: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")).
			Padding(1, 0),
		spinner: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2DD4BF")),
		metricLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FDE68A")).
			Bold(true),
	}
}

func (s styles) panelFrameSize() (int, int) {
	return s.panel.GetFrameSize()
}

func (s styles) panelStyle(focused bool) lipgloss.Style {
	if focused {
		return s.panelFocus
	}
	return s.panel
}

func (s styles) filterDelegateStyles() list.DefaultItemStyles {
	styles := list.NewDefaultItemStyles(true)
	styles.NormalTitle = styles.NormalTitle.Foreground(lipgloss.Color("#E2E8F0"))
	styles.NormalDesc = styles.NormalDesc.Foreground(lipgloss.Color("#94A3B8"))
	styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#2DD4BF")).
		Foreground(lipgloss.Color("#CCFBF1")).
		Padding(0, 0, 0, 1)
	styles.SelectedDesc = styles.SelectedTitle.Foreground(lipgloss.Color("#99F6E4"))
	styles.FilterMatch = lipgloss.NewStyle().Foreground(lipgloss.Color("#FDE68A")).Bold(true)
	return styles
}

func (s styles) tableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		Foreground(lipgloss.Color("#0F172A")).
		Background(lipgloss.Color("#99F6E4")).
		Bold(true)
	styles.Cell = styles.Cell.Foreground(lipgloss.Color("#E2E8F0"))
	styles.Selected = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8FAFC")).
		Background(lipgloss.Color("#0F766E")).
		Bold(true)
	return styles
}

func (m model) renderHeader() string {
	title := m.styles.title.Render("BuddyAPY TUI")
	subtitleParts := []string{
		"DefiLlama yields explorer",
	}
	if !m.lastRefresh.IsZero() {
		subtitleParts = append(subtitleParts, "snapshot "+m.lastRefresh.Format(time.RFC3339))
	}
	subtitle := m.styles.subtitle.Render(strings.Join(subtitleParts, "  •  "))

	chips := m.state.ActiveSummaries()
	if len(chips) == 0 {
		chips = []string{"No active filters"}
	}
	renderedChips := make([]string, 0, len(chips))
	for index, chip := range chips {
		if len(m.state.ActiveSummaries()) == 0 || chip == "No active filters" {
			renderedChips = append(renderedChips, m.styles.chipMuted.Render(chip))
			continue
		}
		if index < 4 {
			renderedChips = append(renderedChips, m.styles.chip.Render(chip))
			continue
		}
		renderedChips = append(renderedChips, m.styles.chipMuted.Render(chip))
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		subtitle,
		strings.Join(renderedChips, " "),
	)
}

func (m model) renderStatus() string {
	var base string
	if m.busy() {
		base = m.spinner.View() + " " + m.status
	} else {
		base = m.status
	}
	if base == "" {
		base = m.resultStatus()
	}

	if m.lastErr != "" {
		return m.styles.error.Render(base)
	}
	return m.styles.status.Render(base)
}

func (m model) renderWide(height int) string {
	filterWidth := maxInt(28, m.width*28/100)
	resultsWidth := maxInt(52, m.width*42/100)
	detailWidth := maxInt(34, m.width-filterWidth-resultsWidth-6)

	filterPane := m.renderPane("Filters", m.renderFilterPane(), m.focus == paneFilters, filterWidth, height)
	resultsPane := m.renderPane("Results", m.renderResultsPane(), m.focus == paneResults, resultsWidth, height)
	detailPane := m.renderPane("Details", m.renderDetailsPane(), m.focus == paneDetails, detailWidth, height)

	return lipgloss.JoinHorizontal(lipgloss.Top, filterPane, resultsPane, detailPane)
}

func (m model) renderCompact(height int) string {
	tabs := []string{
		m.renderTab("Filters", m.focus == paneFilters),
		m.renderTab("Results", m.focus == paneResults),
		m.renderTab("Details", m.focus == paneDetails),
	}

	var body string
	switch m.focus {
	case paneFilters:
		body = m.renderPane("Filters", m.renderFilterPane(), true, m.width-2, height)
	case paneResults:
		body = m.renderPane("Results", m.renderResultsPane(), true, m.width-2, height)
	default:
		body = m.renderPane("Details", m.renderDetailsPane(), true, m.width-2, height)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		strings.Join(tabs, " "),
		body,
	)
}

func (m model) renderTab(label string, active bool) string {
	if active {
		return m.styles.chip.Render(label)
	}
	return m.styles.chipMuted.Render(label)
}

func (m model) renderPane(title, body string, focused bool, width, height int) string {
	style := m.styles.panelStyle(focused)
	frameX, frameY := style.GetFrameSize()
	innerWidth := maxInt(12, width-frameX)
	innerHeight := maxInt(4, height-frameY)
	titleLine := m.styles.panelTitle.Render(title)
	content := lipgloss.JoinVertical(lipgloss.Left, titleLine, body)
	return style.Width(width).Height(height).Render(lipgloss.NewStyle().Width(innerWidth).Height(innerHeight).Render(content))
}

func (m model) renderFilterPane() string {
	active := m.styles.panelMuted.Render("Use Enter to edit, Space to toggle, / for symbol")
	content := lipgloss.JoinVertical(lipgloss.Left, active, m.filterList.View())

	if m.editing {
		editor := lipgloss.JoinVertical(
			lipgloss.Left,
			m.styles.panelMuted.Render("Live editing"),
			m.styles.inputBox.Render(m.textInput.View()),
		)
		content = lipgloss.JoinVertical(lipgloss.Left, content, editor)
	}

	return content
}

func (m model) renderResultsPane() string {
	if len(m.results) == 0 && !m.searching && !m.loadingPools {
		return m.styles.empty.Render("No pools match the current filters.")
	}

	summary := m.styles.panelMuted.Render(m.resultStatus())
	if m.searchDirty {
		summary = m.styles.metricLabel.Render("Search is stale. Press r to rerun.")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		summary,
		m.resultsTable.View(),
	)
}

func (m model) renderDetailsPane() string {
	return m.details.View()
}

func (m model) renderDetailsContent(selected *app.PoolResult) string {
	if selected == nil {
		return m.styles.empty.Render("Select a pool to inspect live APY, TVL, and the DefiLlama page link.")
	}

	lines := []string{
		m.styles.metricLabel.Render(strings.ToUpper(selected.Metric)) + "  " + shortMetric(selected.MetricValue),
		fmt.Sprintf("%s / %s / %s", selected.Project, selected.Chain, selected.Symbol),
		"",
		fmt.Sprintf("Pool ID: %s", selected.PoolID),
		fmt.Sprintf("URL: %s", selected.URL),
		fmt.Sprintf("TVL: %s", compactUSD(selected.TVLUsd)),
		fmt.Sprintf("Current APY: %s", shortNullable(selected.CurrentAPY)),
		fmt.Sprintf("30d mean: %s", shortNullable(selected.APYMean30d)),
	}

	points, ok := m.chartCache[selected.PoolID]
	if !ok {
		if m.chartBusy[selected.PoolID] {
			lines = append(lines, "", "Loading chart history…")
		} else {
			lines = append(lines, "", "Chart history not loaded yet.")
		}
		return strings.Join(lines, "\n")
	}

	trimmed := trimForDisplay(points, m.state)
	if len(trimmed) == 0 {
		lines = append(lines, "", "No chart points available for the current lookback.")
		return strings.Join(lines, "\n")
	}

	lastPoint := trimmed[len(trimmed)-1]
	lines = append(lines,
		"",
		fmt.Sprintf("Last chart point: %s", lastPoint.Timestamp.UTC().Format(time.RFC3339)),
		"APY history: "+sparkline(apysFromPoints(trimmed)),
		"TVL history: "+sparkline(tvlsFromPoints(trimmed)),
	)

	return strings.Join(lines, "\n")
}

func (m model) resultStatus() string {
	if len(m.cachedPools) == 0 {
		return "No cached pools yet."
	}

	mode := currentRankBy(m.state)
	return fmt.Sprintf("%d results from %d pools • rank-by=%s", len(m.results), len(m.cachedPools), mode)
}

func resultColumns(width int) []table.Column {
	switch {
	case width >= 88:
		return []table.Column{
			{Title: "#", Width: 3},
			{Title: "metric", Width: 8},
			{Title: "apy", Width: 8},
			{Title: "tvl", Width: 9},
			{Title: "project", Width: 14},
			{Title: "chain", Width: 14},
			{Title: "symbol", Width: 12},
		}
	case width >= 64:
		return []table.Column{
			{Title: "#", Width: 3},
			{Title: "metric", Width: 8},
			{Title: "apy", Width: 8},
			{Title: "tvl", Width: 9},
			{Title: "project", Width: 14},
			{Title: "symbol", Width: 10},
		}
	default:
		return []table.Column{
			{Title: "#", Width: 3},
			{Title: "metric", Width: 8},
			{Title: "apy", Width: 8},
			{Title: "tvl", Width: 9},
			{Title: "symbol", Width: 10},
		}
	}
}

func resultRow(result app.PoolResult, columns []table.Column) table.Row {
	switch len(columns) {
	case 7:
		return table.Row{
			fmt.Sprintf("%d", result.Rank),
			shortMetric(result.MetricValue),
			shortNullable(result.CurrentAPY),
			compactUSD(result.TVLUsd),
			result.Project,
			result.Chain,
			result.Symbol,
		}
	case 6:
		return table.Row{
			fmt.Sprintf("%d", result.Rank),
			shortMetric(result.MetricValue),
			shortNullable(result.CurrentAPY),
			compactUSD(result.TVLUsd),
			result.Project,
			result.Symbol,
		}
	default:
		return table.Row{
			fmt.Sprintf("%d", result.Rank),
			shortMetric(result.MetricValue),
			shortNullable(result.CurrentAPY),
			compactUSD(result.TVLUsd),
			result.Symbol,
		}
	}
}

func trimForDisplay(points []llama.ChartPoint, state app.FilterState) []llama.ChartPoint {
	options, err := state.ToPoolsOptions()
	if err != nil || options.Lookback <= 0 {
		return points
	}
	cutoff := time.Now().UTC().Add(-options.Lookback)
	trimmed := make([]llama.ChartPoint, 0, len(points))
	for _, point := range points {
		if point.Timestamp.UTC().Before(cutoff) {
			continue
		}
		trimmed = append(trimmed, point)
	}
	if len(trimmed) == 0 {
		return points
	}
	return trimmed
}

func sparkline(values []float64) string {
	if len(values) == 0 {
		return "n/a"
	}

	const ticks = "▁▂▃▄▅▆▇█"
	minValue, maxValue := values[0], values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	if maxValue == minValue {
		return strings.Repeat(string([]rune(ticks)[3]), len(values))
	}

	runes := []rune(ticks)
	var builder strings.Builder
	for _, value := range values {
		index := int((value - minValue) / (maxValue - minValue) * float64(len(runes)-1))
		if index < 0 {
			index = 0
		}
		if index >= len(runes) {
			index = len(runes) - 1
		}
		builder.WriteRune(runes[index])
	}
	return builder.String()
}

func apysFromPoints(points []llama.ChartPoint) []float64 {
	values := make([]float64, 0, len(points))
	for _, point := range points {
		if point.APY == nil {
			continue
		}
		values = append(values, *point.APY)
	}
	return values
}

func tvlsFromPoints(points []llama.ChartPoint) []float64 {
	values := make([]float64, 0, len(points))
	for _, point := range points {
		values = append(values, point.TVLUsd)
	}
	return values
}

func shortNullable(value *float64) string {
	if value == nil {
		return "n/a"
	}
	return shortMetric(*value)
}

func shortMetric(value float64) string {
	return fmt.Sprintf("%.2f%%", value)
}

func compactUSD(value float64) string {
	switch {
	case value >= 1_000_000_000:
		return fmt.Sprintf("$%.2fB", value/1_000_000_000)
	case value >= 1_000_000:
		return fmt.Sprintf("$%.2fM", value/1_000_000)
	case value >= 1_000:
		return fmt.Sprintf("$%.2fK", value/1_000)
	default:
		return fmt.Sprintf("$%.0f", value)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
