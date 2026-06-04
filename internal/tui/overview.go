package tui

import (
	"context"
	"errors"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/kube"
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
	group    string
	resource string
	version  string
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
	return tea.NewView(m.view())
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
	if key.Code == tea.KeyEsc && !m.schema.search.active {
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

	switch msg.Key().Code {
	case tea.KeyUp:
		m.moveOverviewFocus(-1)
	case tea.KeyDown:
		m.moveOverviewFocus(1)
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
	case tea.KeyF10:
		return m, tea.Quit
	}
	m.ensureOverviewFocusVisible()
	return m, nil
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
	rows := []string{detailTitleStyle.Render("Kubernetes resources"), ""}
	if m.loading {
		rows = append(rows, detailFooterStyle.Render("loading schema..."))
	}
	if m.err != "" {
		rows = append(rows, detailRequiredStyle.Render(m.err))
	}

	height := m.overviewBodyHeight()
	end := min(len(m.rows), m.top+height)
	for i := m.top; i < end; i++ {
		line := renderOverviewRow(m.rows[i])
		if i == m.focus && m.rows[i].selectable {
			line = cursorStyle.Render(padVisible(line, width))
		}
		rows = append(rows, line)
	}

	return stickFooter(rows, overviewFooterLines(width), m.height)
}

func overviewFooterLines(width int) []string {
	const footer = "enter open  up/down focus  page move  esc back from schema  q/F10/Ctrl-C quit"
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
	for i, row := range m.rows {
		if row.selectable {
			selectable = append(selectable, i)
		}
	}
	return selectable
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
	if m.top < 0 {
		m.top = 0
	}
	if m.top >= len(m.rows) {
		m.top = len(m.rows) - 1
	}
	if height <= 0 || m.focus < 0 {
		return
	}
	if m.focus < m.top {
		m.top = m.focus
	}
	if m.focus >= m.top+height {
		m.top = m.focus - height + 1
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
						group:    itemGroup,
						resource: resource.Name,
						version:  version,
					},
				})
			}
		}
	}
	return rows
}

func renderOverviewRow(row overviewRow) string {
	switch row.kind {
	case overviewGroupRow:
		return overviewGroupStyle.Render(row.label)
	default:
		return "  " + row.label + "  " + detailOptionalStyle.Render(row.item.version)
	}
}
