package tui

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/kube"
	"github.com/sttts/kubectl-doc/internal/render/termstyle"
)

type OverviewDocumentLoader func(context.Context, string, string, string) (*crd.Document, error)

type OverviewConfig struct {
	Config
	LoadDocument OverviewDocumentLoader
}

type OverviewModel struct {
	ctx          context.Context
	config       Config
	loadDocument OverviewDocumentLoader

	rows   []overviewRow
	focus  int
	top    int
	width  int
	height int

	schema  *Model
	loading bool
	err     string
	filter  textFilterState
}

type overviewRowKind int

const (
	overviewGroupRow overviewRowKind = iota
	overviewVersionRow
)

type overviewRow struct {
	kind       overviewRowKind
	label      string
	item       overviewItem
	selectable bool
}

type overviewItem struct {
	group      string
	resource   string
	version    string
	shortNames []string
}

type overviewLoadedMsg struct {
	doc *crd.Document
	err error
}

func RunOverview(ctx context.Context, out io.Writer, overview *kube.Overview, config OverviewConfig) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if config.Input == nil {
		config.Input = os.Stdin
	}
	if config.Output == nil {
		config.Output = out
	}

	model := NewOverviewModel(overview, config.Config)
	model.ctx = ctx
	model.loadDocument = config.LoadDocument
	program := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(config.Input),
		tea.WithOutput(config.Output),
	)
	_, err := program.Run()
	if errors.Is(err, tea.ErrInterrupted) {
		return nil
	}
	return err
}

func NewOverviewModel(overview *kube.Overview, config Config) OverviewModel {
	width := config.Columns
	if width <= 0 {
		width = 80
	}
	model := OverviewModel{
		ctx:    context.Background(),
		config: config,
		rows:   overviewRows(overview),
		width:  width,
		height: 24,
	}
	model.focusFirstVersion()
	model.ensureOverviewFocusVisible()
	return model
}

func (m OverviewModel) Init() tea.Cmd {
	return nil
}

func (m OverviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.schema != nil {
			updated, cmd := m.schema.Update(msg)
			schema := updated.(Model)
			m.schema = &schema
			return m, cmd
		}
		m.ensureOverviewFocusVisible()
	case overviewLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		schema := NewModel(msg.doc, m.config)
		updated, cmd := schema.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		schema = updated.(Model)
		m.schema = &schema
		m.err = ""
		return m, cmd
	case tea.KeyPressMsg:
		if m.schema != nil {
			return m.handleSchemaKey(msg)
		}
		return m.handleOverviewKey(msg)
	}
	return m, nil
}

func (m OverviewModel) View() tea.View {
	view := tea.NewView(m.view())
	view.AltScreen = true
	return view
}

func (m OverviewModel) FocusedItem() overviewItem {
	if m.focus < 0 || m.focus >= len(m.rows) || !m.rows[m.focus].selectable {
		return overviewItem{}
	}
	return m.rows[m.focus].item
}

func (m OverviewModel) handleSchemaKey(msg tea.KeyPressMsg) (OverviewModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	}
	key := msg.Key()
	if key.Code == tea.KeyF10 {
		return m, tea.Quit
	}
	if key.Code == tea.KeyEsc && !m.schema.search.active && !m.schema.filter.active() {
		m.schema = nil
		m.ensureOverviewFocusVisible()
		return m, nil
	}

	updated, cmd := m.schema.Update(msg)
	schema := updated.(Model)
	m.schema = &schema
	return m, cmd
}

func (m OverviewModel) handleOverviewKey(msg tea.KeyPressMsg) (OverviewModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	}

	if m.handleOverviewFilterKey(msg) {
		m.ensureOverviewFocusVisible()
		return m, nil
	}

	switch msg.Key().Code {
	case tea.KeyUp:
		m.moveOverviewFocus(-1)
	case tea.KeyDown:
		m.moveOverviewFocus(1)
	case tea.KeyLeft:
		m.focusGroup(-1)
	case tea.KeyRight:
		m.focusGroup(1)
	case tea.KeyTab:
		if msg.Key().Mod.Contains(tea.ModShift) {
			m.focusGroup(-1)
		} else {
			m.focusGroup(1)
		}
	case tea.KeyHome:
		m.focusFirstVersion()
	case tea.KeyEnd:
		m.focusLastVersion()
	case tea.KeyPgUp:
		m.moveOverviewFocus(-max(1, m.overviewBodyHeight()/2))
	case tea.KeyPgDown:
		m.moveOverviewFocus(max(1, m.overviewBodyHeight()/2))
	case tea.KeyEnter:
		return m.openFocusedVersion()
	case tea.KeyEsc:
		return m, tea.Quit
	case tea.KeyF10:
		return m, tea.Quit
	}
	m.ensureOverviewFocusVisible()
	return m, nil
}

func (m *OverviewModel) handleOverviewFilterKey(msg tea.KeyPressMsg) bool {
	switch msg.Key().Code {
	case tea.KeyEsc:
		if !m.filter.active() {
			return false
		}
		m.filter.query = ""
		return true
	case tea.KeyBackspace:
		if !m.filter.active() {
			return false
		}
		m.filter.query = m.filter.query[:len(m.filter.query)-1]
		m.ensureOverviewFilterFocus()
		return true
	}
	text, ok := filterTextFromKey(msg)
	if !ok {
		return false
	}
	m.filter.query += text
	m.ensureOverviewFilterFocus()
	return true
}

func (m *OverviewModel) ensureOverviewFilterFocus() {
	if !m.filter.active() {
		return
	}
	selectable := m.selectableRows()
	if indexOf(selectable, m.focus) >= 0 {
		return
	}
	if len(selectable) > 0 {
		m.focus = selectable[0]
	}
}

func (m OverviewModel) openFocusedVersion() (OverviewModel, tea.Cmd) {
	item := m.FocusedItem()
	if item.resource == "" || item.version == "" {
		return m, nil
	}
	if m.loadDocument == nil {
		m.err = "schema loading is not configured"
		return m, nil
	}
	m.loading = true
	m.err = ""
	return m, loadOverviewDocument(m.ctx, m.loadDocument, item)
}

func loadOverviewDocument(ctx context.Context, loader OverviewDocumentLoader, item overviewItem) tea.Cmd {
	return func() tea.Msg {
		doc, err := loader(ctx, item.group, item.version, item.resource)
		return overviewLoadedMsg{doc: doc, err: err}
	}
}

func (m OverviewModel) view() string {
	if m.schema != nil {
		return m.schema.view()
	}

	width := m.width
	if width <= 0 {
		width = 80
	}
	var rows []string
	appendRow := func(line string) {
		rows = append(rows, padVisible(line, width))
	}
	appendRow(detailTitleStyle.Render("Kubernetes resources"))
	if m.filter.active() {
		appendRow(filterStatusStyle.Render("filter: " + m.filter.query))
	}
	appendRow("")
	if m.loading {
		appendRow(detailFooterStyle.Render("loading schema..."))
	}
	if m.err != "" {
		appendRow(detailRequiredStyle.Render(m.err))
	}

	height := m.overviewBodyHeight()
	visible := m.visibleOverviewIndexes()
	end := min(len(visible), m.top+height)
	for _, index := range visible[m.top:end] {
		line := renderOverviewRow(m.rows[index], m.filter.query)
		if index == m.focus && m.rows[index].selectable {
			line = cursorStyle.Render(padVisible(line, width))
		}
		appendRow(line)
	}

	return stickFooter(rows, overviewFooterLines(width), m.height)
}

func overviewFooterLines(width int) []string {
	const footer = "enter open  up/down focus  left/right/tab group  page move  esc/q/F10/Ctrl-C quit"
	lines := wrapPlain(footer, width)
	for i := range lines {
		lines[i] = detailFooterStyle.Render(lines[i])
	}
	return lines
}

func (m OverviewModel) overviewBodyHeight() int {
	height := m.height
	if height <= 0 {
		height = 24
	}
	headerRows := 2
	if m.filter.active() {
		headerRows++
	}
	if m.loading {
		headerRows++
	}
	if m.err != "" {
		headerRows++
	}
	return max(1, height-headerRows-len(overviewFooterLines(m.width)))
}

func (m *OverviewModel) moveOverviewFocus(delta int) {
	selectable := m.selectableRows()
	position := indexOf(selectable, m.focus)
	if position < 0 {
		m.focusFirstVersion()
		return
	}
	position += delta
	if position < 0 {
		position = 0
	}
	if position >= len(selectable) {
		position = len(selectable) - 1
	}
	if position >= 0 && position < len(selectable) {
		m.focus = selectable[position]
	}
}

func (m *OverviewModel) focusGroup(delta int) {
	group := m.focusGroupRow()
	if group < 0 {
		return
	}
	groups := m.visibleGroupRows()
	position := indexOf(groups, group)
	if position < 0 {
		return
	}
	position += delta
	for position >= 0 && position < len(groups) {
		if version := m.firstVersionInGroup(groups[position]); version >= 0 {
			m.focus = version
			if version == m.firstSelectableRow() {
				m.top = 0
			}
			return
		}
		position += delta
	}
}

func (m OverviewModel) focusGroupRow() int {
	if m.focus < 0 || m.focus >= len(m.rows) {
		return -1
	}
	for i := m.focus; i >= 0; i-- {
		if m.rows[i].kind == overviewGroupRow {
			return i
		}
	}
	return -1
}

func (m OverviewModel) firstVersionInGroup(group int) int {
	visible := m.visibleOverviewIndexes()
	position := indexOf(visible, group)
	if position < 0 {
		return -1
	}
	for _, i := range visible[position+1:] {
		if m.rows[i].kind == overviewGroupRow {
			return -1
		}
		if m.rows[i].selectable {
			return i
		}
	}
	return -1
}

func (m *OverviewModel) focusFirstVersion() {
	selectable := m.selectableRows()
	if len(selectable) == 0 {
		m.focus = -1
		return
	}
	m.focus = selectable[0]
}

func (m *OverviewModel) focusLastVersion() {
	selectable := m.selectableRows()
	if len(selectable) == 0 {
		m.focus = -1
		return
	}
	m.focus = selectable[len(selectable)-1]
}

func (m OverviewModel) selectableRows() []int {
	var selectable []int
	for _, i := range m.visibleOverviewIndexes() {
		row := m.rows[i]
		if row.selectable {
			selectable = append(selectable, i)
		}
	}
	return selectable
}

func (m OverviewModel) firstSelectableRow() int {
	selectable := m.selectableRows()
	if len(selectable) == 0 {
		return -1
	}
	return selectable[0]
}

func (m OverviewModel) visibleGroupRows() []int {
	var groups []int
	for _, i := range m.visibleOverviewIndexes() {
		if m.rows[i].kind == overviewGroupRow {
			groups = append(groups, i)
		}
	}
	return groups
}

func (m OverviewModel) visibleOverviewIndexes() []int {
	if !m.filter.active() {
		indexes := make([]int, len(m.rows))
		for i := range m.rows {
			indexes[i] = i
		}
		return indexes
	}
	query := strings.ToLower(m.filter.query)
	groupMatches := map[int]bool{}
	rowMatches := map[int]bool{}
	for i, row := range m.rows {
		if row.kind != overviewGroupRow {
			continue
		}
		if strings.Contains(strings.ToLower(row.label), query) {
			groupMatches[i] = true
		}
	}
	currentGroup := -1
	for i, row := range m.rows {
		if row.kind == overviewGroupRow {
			currentGroup = i
			continue
		}
		if currentGroup < 0 {
			continue
		}
		if groupMatches[currentGroup] || overviewRowMatches(row, query) {
			rowMatches[i] = true
			groupMatches[currentGroup] = true
		}
	}
	var visible []int
	for i, row := range m.rows {
		switch row.kind {
		case overviewGroupRow:
			if groupMatches[i] {
				visible = append(visible, i)
			}
		case overviewVersionRow:
			if rowMatches[i] {
				visible = append(visible, i)
			}
		}
	}
	return visible
}

func overviewRowMatches(row overviewRow, query string) bool {
	if strings.Contains(strings.ToLower(row.item.resource), query) {
		return true
	}
	if strings.Contains(strings.ToLower(row.item.version), query) {
		return true
	}
	for _, shortName := range row.item.shortNames {
		if strings.Contains(strings.ToLower(shortName), query) {
			return true
		}
	}
	return false
}

func (m *OverviewModel) ensureOverviewFocusVisible() {
	if len(m.rows) == 0 {
		m.focus = -1
		m.top = 0
		return
	}
	if m.focus < 0 || m.focus >= len(m.rows) || !m.rows[m.focus].selectable {
		m.focusFirstVersion()
	}
	height := m.overviewBodyHeight()
	visible := m.visibleOverviewIndexes()
	if m.filter.active() && len(visible) == 0 {
		m.top = 0
		return
	}
	if m.top < 0 {
		m.top = 0
	}
	if m.top >= len(visible) {
		m.top = max(0, len(visible)-1)
	}
	if height <= 0 || m.focus < 0 {
		return
	}
	position := indexOf(visible, m.focus)
	if position < 0 {
		m.focusFirstVersion()
		position = indexOf(visible, m.focus)
	}
	if position < 0 {
		return
	}
	if m.focus == m.firstSelectableRow() {
		m.top = 0
		return
	}
	if position < m.top {
		m.top = position
	}
	if position >= m.top+height {
		m.top = position - height + 1
	}
}

func overviewRows(overview *kube.Overview) []overviewRow {
	if overview == nil {
		return nil
	}
	var rows []overviewRow
	for _, group := range overview.Groups {
		rows = append(rows, overviewRow{kind: overviewGroupRow, label: group.Name})
		itemGroup := group.Name
		if itemGroup == kube.CoreGroup {
			itemGroup = ""
		}
		for _, resource := range group.Resources {
			for _, version := range resource.Versions {
				rows = append(rows, overviewRow{
					kind:       overviewVersionRow,
					label:      resource.Name,
					selectable: true,
					item: overviewItem{
						group:      itemGroup,
						resource:   resource.Name,
						version:    version,
						shortNames: append([]string(nil), resource.ShortNames...),
					},
				})
			}
		}
	}
	return rows
}

func renderOverviewRow(row overviewRow, query string) string {
	switch row.kind {
	case overviewGroupRow:
		return highlightFilterMatches(termstyle.KeyStyle.Render(row.label), query)
	default:
		resource := highlightFilterMatches(row.label, query)
		if query != "" && overviewAliasMatches(row.item.shortNames, strings.ToLower(query)) && !strings.Contains(strings.ToLower(row.label), strings.ToLower(query)) {
			resource = filterHitStyle.Render(row.label)
		}
		version := highlightFilterMatches(termstyle.ScalarStyle.Render(row.item.version), query)
		return "  " + resource + "  " + version
	}
}

func overviewAliasMatches(shortNames []string, query string) bool {
	for _, shortName := range shortNames {
		if strings.Contains(strings.ToLower(shortName), query) {
			return true
		}
	}
	return false
}
