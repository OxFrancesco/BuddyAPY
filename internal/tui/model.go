package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"buddyapy/internal/app"
	"buddyapy/internal/llama"
)

const (
	autoSearchDebounce   = 250 * time.Millisecond
	wideLayoutBreakpoint = 148
	tallLayoutBreakpoint = 28
)

type pane int

const (
	paneFilters pane = iota
	paneResults
	paneDetails
)

type filterItem struct {
	Definition app.FilterDefinition
	Summary    string
}

func (i filterItem) Title() string {
	return i.Definition.Label
}

func (i filterItem) Description() string {
	return fmt.Sprintf("%s  |  %s", i.Summary, i.Definition.Help)
}

func (i filterItem) FilterValue() string {
	return strings.Join([]string{
		i.Definition.Label,
		i.Definition.FlagName,
		i.Summary,
		i.Definition.Help,
	}, " ")
}

type keyMap struct {
	nextPane   key.Binding
	prevPane   key.Binding
	edit       key.Binding
	toggle     key.Binding
	jumpSymbol key.Binding
	clear      key.Binding
	clearAll   key.Binding
	refresh    key.Binding
	quit       key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		nextPane: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next pane"),
		),
		prevPane: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev pane"),
		),
		edit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "edit/open"),
		),
		toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		jumpSymbol: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "symbol"),
		),
		clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear filter"),
		),
		clearAll: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "clear all"),
		),
		refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.nextPane, k.edit, k.jumpSymbol, k.refresh, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.nextPane, k.prevPane, k.edit, k.toggle},
		{k.jumpSymbol, k.clear, k.clearAll, k.refresh, k.quit},
	}
}

type poolsLoadedMsg struct {
	seq   int
	pools []llama.Pool
	err   error
}

type searchCompletedMsg struct {
	seq     int
	results []app.PoolResult
	err     error
}

type chartLoadedMsg struct {
	poolID string
	points []llama.ChartPoint
	err    error
}

type debounceElapsedMsg struct {
	seq int
}

type model struct {
	ctx context.Context
	api app.API
	now func() time.Time

	keys    keyMap
	styles  styles
	helper  help.Model
	spinner spinner.Model

	filterList   list.Model
	resultsTable table.Model
	details      viewport.Model
	textInput    textinput.Model

	state app.FilterState

	cachedPools []llama.Pool
	results     []app.PoolResult
	chartCache  map[string][]llama.ChartPoint
	chartBusy   map[string]bool

	width  int
	height int
	focus  pane

	editing      bool
	editingField app.FilterID
	editOriginal string

	loadingPools bool
	searching    bool
	searchDirty  bool

	fetchSeq  int
	searchSeq int

	lastRefresh time.Time
	status      string
	lastErr     string
	detailPool  string
}

func newModel(ctx context.Context, api app.API, now func() time.Time) model {
	if ctx == nil {
		ctx = context.Background()
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	state := app.DefaultFilterState()
	styles := newStyles()

	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2)
	delegate.SetSpacing(0)
	delegate.Styles = styles.filterDelegateStyles()

	filterList := list.New(filterItemsForState(state), delegate, 0, 0)
	filterList.DisableQuitKeybindings()
	filterList.SetFilteringEnabled(false)
	filterList.SetShowFilter(false)
	filterList.SetShowHelp(false)
	filterList.SetShowPagination(false)
	filterList.SetShowStatusBar(false)
	filterList.SetShowTitle(false)

	tableStyles := styles.tableStyles()
	resultsTable := table.New(
		table.WithColumns(resultColumns(72)),
		table.WithRows(nil),
		table.WithFocused(false),
		table.WithStyles(tableStyles),
	)

	details := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	input := textinput.New()
	input.Prompt = "value > "
	input.Placeholder = "type to filter"
	input.SetWidth(24)

	helper := help.New()
	spin := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(styles.spinner),
	)

	return model{
		ctx:          ctx,
		api:          api,
		now:          now,
		keys:         defaultKeyMap(),
		styles:       styles,
		helper:       helper,
		spinner:      spin,
		filterList:   filterList,
		resultsTable: resultsTable,
		details:      details,
		textInput:    input,
		state:        state,
		chartCache:   make(map[string][]llama.ChartPoint),
		chartBusy:    make(map[string]bool),
		loadingPools: true,
		fetchSeq:     1,
		status:       "Loading the live DefiLlama /pools snapshot…",
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(fetchPoolsCmd(m.ctx, m.api, m.fetchSeq), m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		m.refreshDetails(false)
		return m, nil

	case spinner.TickMsg:
		if !m.busy() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case poolsLoadedMsg:
		if msg.seq != m.fetchSeq {
			return m, nil
		}
		m.loadingPools = false
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			m.status = "Pool refresh failed: " + msg.err.Error()
			return m, nil
		}

		m.cachedPools = msg.pools
		m.lastRefresh = m.now().UTC()
		m.lastErr = ""
		return m, m.startSearch()

	case debounceElapsedMsg:
		if msg.seq != m.searchSeq || len(m.cachedPools) == 0 {
			return m, nil
		}
		return m, m.runSearch(msg.seq)

	case searchCompletedMsg:
		if msg.seq != m.searchSeq {
			return m, nil
		}
		m.searching = false
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			m.status = "Search error: " + msg.err.Error()
			m.refreshDetails(false)
			return m, nil
		}

		m.lastErr = ""
		m.searchDirty = false
		m.applyResults(msg.results)
		m.status = m.resultStatus()
		return m, m.ensureSelectedChart()

	case chartLoadedMsg:
		delete(m.chartBusy, msg.poolID)
		if msg.err != nil {
			m.status = "Chart load failed for " + msg.poolID + ": " + msg.err.Error()
			m.refreshDetails(false)
			return m, nil
		}
		m.chartCache[msg.poolID] = msg.points
		m.refreshDetails(false)
		return m, nil
	}

	if m.editing {
		return m.updateWhileEditing(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.nextPane):
			m.focus = nextPane(m.focus)
			m.syncFocus()
			m.refreshDetails(false)
			return m, nil
		case key.Matches(msg, m.keys.prevPane):
			m.focus = previousPane(m.focus)
			m.syncFocus()
			m.refreshDetails(false)
			return m, nil
		case key.Matches(msg, m.keys.jumpSymbol):
			m.focus = paneFilters
			m.syncFocus()
			return m, m.jumpToFilter(app.FilterSymbol)
		case key.Matches(msg, m.keys.clearAll):
			m.state.Reset()
			m.syncFilterItems()
			m.refreshDetails(false)
			return m, m.afterFilterChange()
		case key.Matches(msg, m.keys.refresh):
			return m, m.refreshRemote()
		case key.Matches(msg, m.keys.clear):
			if m.focus == paneFilters {
				selected := m.selectedFilterID()
				if selected != "" {
					m.state.Clear(selected)
					m.syncFilterItems()
					m.refreshDetails(false)
					return m, m.afterFilterChange()
				}
			}
		case key.Matches(msg, m.keys.toggle):
			if m.focus == paneFilters {
				definition := m.selectedFilterDefinition()
				if definition.ID != "" && definition.Kind == app.FilterKindBool {
					m.state.Toggle(definition.ID)
					m.syncFilterItems()
					m.refreshDetails(false)
					return m, m.afterFilterChange()
				}
			}
		case key.Matches(msg, m.keys.edit):
			switch m.focus {
			case paneFilters:
				return m, m.activateFocusedFilter()
			case paneResults:
				m.focus = paneDetails
				m.syncFocus()
				m.refreshDetails(true)
				return m, nil
			}
		}
	}

	switch m.focus {
	case paneFilters:
		var cmd tea.Cmd
		m.filterList, cmd = m.filterList.Update(msg)
		m.refreshDetails(false)
		return m, cmd
	case paneResults:
		before := m.resultsTable.Cursor()
		var cmd tea.Cmd
		m.resultsTable, cmd = m.resultsTable.Update(msg)
		if before != m.resultsTable.Cursor() {
			m.refreshDetails(true)
			return m, tea.Batch(cmd, m.ensureSelectedChart())
		}
		return m, cmd
	case paneDetails:
		var cmd tea.Cmd
		m.details, cmd = m.details.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m model) updateWhileEditing(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.nextPane):
			cmd := m.stopEditing(false)
			m.focus = nextPane(m.focus)
			m.syncFocus()
			m.refreshDetails(false)
			return m, cmd
		case key.Matches(msg, m.keys.prevPane):
			cmd := m.stopEditing(false)
			m.focus = previousPane(m.focus)
			m.syncFocus()
			m.refreshDetails(false)
			return m, cmd
		case msg.String() == "esc":
			return m, m.stopEditing(true)
		case msg.String() == "enter":
			return m, m.stopEditing(false)
		}
	}

	previous := m.textInput.Value()
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	if current := m.textInput.Value(); current != previous {
		m.state.SetStringValue(m.editingField, current)
		m.syncFilterItems()
		m.refreshDetails(false)
		return m, tea.Batch(cmd, m.afterFilterChange())
	}

	return m, cmd
}

func (m model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		view := tea.NewView(m.styles.page.Render("Loading BuddyAPY TUI…"))
		view.AltScreen = true
		return view
	}

	header := m.renderHeader()
	status := m.renderStatus()
	helpView := m.styles.helpBar.Render(m.helper.View(m.keys))
	bodyHeight := maxInt(8, m.height-lipgloss.Height(header)-lipgloss.Height(status)-lipgloss.Height(helpView)-2)

	var body string
	if m.useWideLayout() {
		body = m.renderWide(bodyHeight)
	} else {
		body = m.renderCompact(bodyHeight)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, body, status, helpView)
	view := tea.NewView(m.styles.page.Render(content))
	view.AltScreen = true
	return view
}

func (m *model) layout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	headerHeight := 5
	bodyHeight := maxInt(8, m.height-headerHeight-4)

	if m.useWideLayout() {
		filterWidth := maxInt(28, m.width*28/100)
		resultsWidth := maxInt(52, m.width*42/100)
		detailWidth := maxInt(34, m.width-filterWidth-resultsWidth-4)

		m.sizeFilterPane(filterWidth, bodyHeight)
		m.sizeResultsPane(resultsWidth, bodyHeight)
		m.sizeDetailPane(detailWidth, bodyHeight)
		return
	}

	fullWidth := maxInt(24, m.width-2)
	m.sizeFilterPane(fullWidth, bodyHeight)
	m.sizeResultsPane(fullWidth, bodyHeight)
	m.sizeDetailPane(fullWidth, bodyHeight)
}

func (m *model) sizeFilterPane(width, height int) {
	frameX, frameY := m.styles.panelFrameSize()
	innerWidth := maxInt(10, width-frameX)
	innerHeight := maxInt(6, height-frameY)
	listHeight := innerHeight - 4
	if m.editing {
		listHeight -= 3
	}
	m.filterList.SetSize(innerWidth, maxInt(3, listHeight))
	m.textInput.SetWidth(maxInt(12, innerWidth-2))
}

func (m *model) sizeResultsPane(width, height int) {
	frameX, frameY := m.styles.panelFrameSize()
	innerWidth := maxInt(20, width-frameX)
	innerHeight := maxInt(6, height-frameY)
	m.resultsTable.SetColumns(resultColumns(innerWidth))
	m.resultsTable.SetWidth(innerWidth)
	m.resultsTable.SetHeight(innerHeight)
}

func (m *model) sizeDetailPane(width, height int) {
	frameX, frameY := m.styles.panelFrameSize()
	innerWidth := maxInt(18, width-frameX)
	innerHeight := maxInt(6, height-frameY)
	m.details.SetWidth(innerWidth)
	m.details.SetHeight(innerHeight)
}

func (m *model) refreshDetails(resetScroll bool) {
	selected := m.selectedResult()
	content := m.renderDetailsContent(selected)
	m.details.SetContent(content)
	if resetScroll || (selected != nil && selected.PoolID != m.detailPool) {
		m.details.GotoTop()
	}
	if selected == nil {
		m.detailPool = ""
		return
	}
	m.detailPool = selected.PoolID
}

func (m *model) syncFocus() {
	if m.focus == paneResults {
		m.resultsTable.Focus()
	} else {
		m.resultsTable.Blur()
	}
}

func (m *model) syncFilterItems() {
	selected := m.selectedFilterID()
	items := filterItemsForState(m.state)
	_ = m.filterList.SetItems(items)
	index := filterIndex(selected)
	if index < 0 {
		index = 0
	}
	m.filterList.Select(index)
}

func (m *model) activateFocusedFilter() tea.Cmd {
	definition := m.selectedFilterDefinition()
	switch definition.Kind {
	case app.FilterKindBool:
		m.state.Toggle(definition.ID)
		m.syncFilterItems()
		m.refreshDetails(false)
		return m.afterFilterChange()
	case app.FilterKindSelect:
		m.advanceOption(definition)
		m.syncFilterItems()
		m.refreshDetails(false)
		return m.afterFilterChange()
	case app.FilterKindText:
		return m.beginEditing(definition.ID)
	default:
		return nil
	}
}

func (m *model) beginEditing(id app.FilterID) tea.Cmd {
	definition := app.PoolFilterDefinition(id)
	m.editing = true
	m.editingField = id
	m.editOriginal = m.state.StringValue(id)
	m.textInput.Reset()
	m.textInput.Prompt = strings.ToLower(definition.Label) + " > "
	m.textInput.Placeholder = definition.Placeholder
	m.textInput.SetValue(m.state.StringValue(id))
	m.status = "Editing " + definition.Label
	return m.textInput.Focus()
}

func (m *model) stopEditing(restore bool) tea.Cmd {
	var commands []tea.Cmd

	if restore {
		m.state.SetStringValue(m.editingField, m.editOriginal)
		m.syncFilterItems()
		m.refreshDetails(false)
		commands = append(commands, m.afterFilterChange())
	}

	m.editing = false
	m.editingField = ""
	m.editOriginal = ""
	m.textInput.Blur()
	if !restore && m.lastErr == "" {
		m.status = m.resultStatus()
	}

	return tea.Batch(commands...)
}

func (m *model) afterFilterChange() tea.Cmd {
	if len(m.cachedPools) == 0 {
		return nil
	}
	if currentRankBy(m.state) == string(app.RankByChartMean) {
		m.searchDirty = true
		m.status = "Chart-mean mode is expensive. Press r to rerun with live charts."
		return nil
	}

	m.searchSeq++
	seq := m.searchSeq
	m.searchDirty = false
	m.status = "Re-ranking from the cached /pools snapshot…"
	return debounceCmd(seq)
}

func (m *model) refreshRemote() tea.Cmd {
	m.fetchSeq++
	m.loadingPools = true
	m.searchDirty = false
	m.lastErr = ""
	m.status = "Refreshing the live /pools snapshot…"
	return tea.Batch(fetchPoolsCmd(m.ctx, m.api, m.fetchSeq), m.spinner.Tick)
}

func (m *model) startSearch() tea.Cmd {
	m.searchSeq++
	return tea.Batch(m.runSearch(m.searchSeq), m.spinner.Tick)
}

func (m *model) runSearch(seq int) tea.Cmd {
	m.searching = true
	state := m.state
	pools := append([]llama.Pool(nil), m.cachedPools...)
	return runSearchCmd(m.ctx, m.api, pools, state, m.now, seq)
}

func (m *model) applyResults(results []app.PoolResult) {
	m.results = results
	rows := make([]table.Row, 0, len(results))
	columns := m.resultsTable.Columns()
	if len(columns) == 0 {
		columns = resultColumns(72)
	}
	for _, result := range results {
		rows = append(rows, resultRow(result, columns))
	}

	m.resultsTable.SetRows(rows)
	if len(rows) == 0 {
		m.resultsTable.SetCursor(0)
	} else if m.resultsTable.Cursor() >= len(rows) {
		m.resultsTable.SetCursor(len(rows) - 1)
	}
	m.refreshDetails(true)
}

func (m *model) ensureSelectedChart() tea.Cmd {
	selected := m.selectedResult()
	if selected == nil {
		return nil
	}
	if _, ok := m.chartCache[selected.PoolID]; ok {
		return nil
	}
	if m.chartBusy[selected.PoolID] {
		return nil
	}

	m.chartBusy[selected.PoolID] = true
	m.refreshDetails(false)
	return tea.Batch(loadChartCmd(m.ctx, m.api, selected.PoolID), m.spinner.Tick)
}

func (m model) selectedResult() *app.PoolResult {
	if len(m.results) == 0 {
		return nil
	}
	cursor := m.resultsTable.Cursor()
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(m.results) {
		cursor = len(m.results) - 1
	}
	return &m.results[cursor]
}

func (m model) selectedFilterDefinition() app.FilterDefinition {
	selected := m.filterList.SelectedItem()
	item, ok := selected.(filterItem)
	if !ok {
		return app.FilterDefinition{}
	}
	return item.Definition
}

func (m model) selectedFilterID() app.FilterID {
	return m.selectedFilterDefinition().ID
}

func (m *model) advanceOption(definition app.FilterDefinition) {
	if len(definition.Options) == 0 {
		return
	}

	current := effectiveText(m.state, definition.ID)
	for index, option := range definition.Options {
		if option.Value != current {
			continue
		}
		next := definition.Options[(index+1)%len(definition.Options)]
		m.state.SetStringValue(definition.ID, next.Value)
		return
	}

	m.state.SetStringValue(definition.ID, definition.Options[0].Value)
}

func (m *model) jumpToFilter(id app.FilterID) tea.Cmd {
	index := filterIndex(id)
	if index >= 0 {
		m.filterList.Select(index)
	}

	definition := app.PoolFilterDefinition(id)
	if definition.Kind == app.FilterKindText {
		return m.beginEditing(id)
	}
	return nil
}

func (m model) busy() bool {
	if m.loadingPools || m.searching {
		return true
	}
	for _, busy := range m.chartBusy {
		if busy {
			return true
		}
	}
	return false
}

func (m model) useWideLayout() bool {
	return m.width >= wideLayoutBreakpoint && m.height >= tallLayoutBreakpoint
}

func filterItemsForState(state app.FilterState) []list.Item {
	definitions := app.PoolFilterDefinitions()
	items := make([]list.Item, 0, len(definitions))
	for _, definition := range definitions {
		items = append(items, filterItem{
			Definition: definition,
			Summary:    state.Summary(definition.ID),
		})
	}
	return items
}

func filterIndex(id app.FilterID) int {
	for index, definition := range app.PoolFilterDefinitions() {
		if definition.ID == id {
			return index
		}
	}
	return -1
}

func nextPane(current pane) pane {
	return pane((int(current) + 1) % 3)
}

func previousPane(current pane) pane {
	return pane((int(current) + 2) % 3)
}

func effectiveText(state app.FilterState, id app.FilterID) string {
	value := strings.TrimSpace(state.StringValue(id))
	if value != "" {
		return value
	}
	return app.PoolFilterDefinition(id).DefaultText
}

func currentRankBy(state app.FilterState) string {
	return effectiveText(state, app.FilterRankBy)
}

func fetchPoolsCmd(ctx context.Context, api app.API, seq int) tea.Cmd {
	return func() tea.Msg {
		pools, err := api.Pools(ctx)
		return poolsLoadedMsg{
			seq:   seq,
			pools: pools,
			err:   err,
		}
	}
}

func runSearchCmd(ctx context.Context, api app.API, pools []llama.Pool, state app.FilterState, now func() time.Time, seq int) tea.Cmd {
	return func() tea.Msg {
		options, err := state.ToPoolsOptions()
		if err != nil {
			return searchCompletedMsg{seq: seq, err: err}
		}

		results, err := app.SearchPoolsFromPools(ctx, api, pools, options, now)
		return searchCompletedMsg{
			seq:     seq,
			results: results,
			err:     err,
		}
	}
}

func loadChartCmd(ctx context.Context, api app.API, poolID string) tea.Cmd {
	return func() tea.Msg {
		points, err := api.Chart(ctx, poolID)
		return chartLoadedMsg{
			poolID: poolID,
			points: points,
			err:    err,
		}
	}
}

func debounceCmd(seq int) tea.Cmd {
	return tea.Tick(autoSearchDebounce, func(time.Time) tea.Msg {
		return debounceElapsedMsg{seq: seq}
	})
}
