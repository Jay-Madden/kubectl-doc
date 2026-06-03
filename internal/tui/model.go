package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/tree"
)

const fullSchemaDepth = 1000

type Config struct {
	ExpandDepth  int
	Descriptions tree.DescriptionMode
	Columns      int
	Input        io.Reader
	Output       io.Writer
}

type Model struct {
	doc   *crd.Document
	lines []tree.Line

	focus  int
	top    int
	width  int
	height int

	search searchState
}

type searchState struct {
	active    bool
	fieldOnly bool
	query     string
	matches   []int
	position  int
}

func Run(ctx context.Context, out io.Writer, doc *crd.Document, config Config) error {
	if config.Input == nil {
		config.Input = os.Stdin
	}
	if config.Output == nil {
		config.Output = out
	}

	model := NewModel(doc, config)
	program := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(config.Input),
		tea.WithOutput(config.Output),
	)
	_, err := program.Run()
	return err
}

func NewModel(doc *crd.Document, config Config) Model {
	columns := config.Columns
	if columns <= 0 {
		columns = 80
	}
	lines := tree.WithCollapsed(tree.Build(doc, tree.Options{
		ExpandDepth:    fullSchemaDepth,
		Descriptions:   descriptionMode(config.Descriptions),
		Columns:        columns,
		RenderStatus:   true,
		RenderMetadata: true,
	}), config.ExpandDepth)

	model := Model{
		doc:    doc,
		lines:  lines,
		width:  columns,
		height: 24,
	}
	model.focus = model.firstVisibleField()
	model.ensureFocusVisible()
	return model
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureFocusVisible()
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) View() tea.View {
	return tea.NewView(m.view())
}

func (m Model) FocusPath() string {
	if !m.hasFocus() {
		return ""
	}
	return m.lines[m.focus].Path
}

func (m Model) FocusLine() tree.Line {
	if !m.hasFocus() {
		return tree.Line{}
	}
	return m.lines[m.focus]
}

func (m Model) IsCollapsed(path string) bool {
	for _, line := range m.lines {
		if line.Path == path && line.Field != "" {
			return line.Collapsed
		}
	}
	return false
}

func (m Model) SearchQuery() string {
	return m.search.query
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.search.active {
		return m.handleSearchKey(msg), nil
	}

	key := msg.Key()
	if key.Code == tea.KeyTab {
		if key.Mod.Contains(tea.ModShift) {
			m.focusPreviousFoldable()
		} else {
			m.focusNextFoldable()
		}
		m.ensureFocusVisible()
		return m, nil
	}

	switch key.Code {
	case tea.KeyUp:
		m.focusPreviousField()
	case tea.KeyDown:
		m.focusNextField()
	case tea.KeyLeft:
		m.focusParent()
	case tea.KeyRight:
		m.expandAndFocusChild()
	case tea.KeyEnter:
		m.toggleFocused()
	case tea.KeyHome:
		m.focusFirstField()
	case tea.KeyEnd:
		m.focusLastField()
	case tea.KeyF10:
		return m, tea.Quit
	default:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "/":
			m.search = searchState{active: true}
		case "n":
			m.focusNextSearchMatch()
		case "p":
			m.focusPreviousSearchMatch()
		}
	}
	m.ensureFocusVisible()
	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyPressMsg) Model {
	key := msg.Key()
	switch key.Code {
	case tea.KeyEsc:
		m.search.active = false
	case tea.KeyEnter:
		m.search.active = false
	case tea.KeyBackspace:
		if m.search.query != "" {
			m.search.query = m.search.query[:len(m.search.query)-1]
			m.updateSearchMatches()
		} else if m.search.fieldOnly {
			m.search.fieldOnly = false
			m.updateSearchMatches()
		}
	default:
		text := key.Text
		if text == "" {
			text = msg.String()
		}
		if text == "/" && m.search.query == "" {
			m.search.fieldOnly = true
			m.updateSearchMatches()
			break
		}
		if len(text) == 1 && text >= " " && text != "\x7f" {
			m.search.query += text
			m.updateSearchMatches()
		}
	}
	m.ensureFocusVisible()
	return m
}

func (m *Model) updateSearchMatches() {
	m.search.matches = nil
	m.search.position = -1
	if m.search.query == "" {
		return
	}
	query := strings.ToLower(m.search.query)
	for _, index := range m.visibleIndexes() {
		line := m.lines[index]
		if line.Field == "" {
			continue
		}
		if m.lineMatches(line, query) {
			m.search.matches = append(m.search.matches, index)
		}
	}
	if len(m.search.matches) == 0 {
		return
	}
	m.search.position = 0
	if m.hasFocus() {
		for i, match := range m.search.matches {
			if match >= m.focus {
				m.search.position = i
				break
			}
		}
	}
	m.focus = m.search.matches[m.search.position]
}

func (m Model) lineMatches(line tree.Line, query string) bool {
	if strings.Contains(strings.ToLower(line.Field), query) {
		return true
	}
	if m.search.fieldOnly {
		return false
	}
	if strings.Contains(strings.ToLower(line.Text), query) {
		return true
	}
	for _, description := range m.descriptionLines(line.Path) {
		if strings.Contains(strings.ToLower(description), query) {
			return true
		}
	}
	return false
}

func (m *Model) focusNextSearchMatch() {
	if len(m.search.matches) == 0 {
		return
	}
	m.search.position = (m.search.position + 1) % len(m.search.matches)
	m.focus = m.search.matches[m.search.position]
}

func (m *Model) focusPreviousSearchMatch() {
	if len(m.search.matches) == 0 {
		return
	}
	m.search.position--
	if m.search.position < 0 {
		m.search.position = len(m.search.matches) - 1
	}
	m.focus = m.search.matches[m.search.position]
}

func (m *Model) focusPreviousField() {
	visible := m.visibleIndexes()
	position := indexOf(visible, m.focus)
	for i := position - 1; i >= 0; i-- {
		if m.lines[visible[i]].Field != "" {
			m.focus = visible[i]
			return
		}
	}
}

func (m *Model) focusNextField() {
	visible := m.visibleIndexes()
	position := indexOf(visible, m.focus)
	for i := position + 1; i < len(visible); i++ {
		if m.lines[visible[i]].Field != "" {
			m.focus = visible[i]
			return
		}
	}
}

func (m *Model) focusParent() {
	if !m.hasFocus() {
		return
	}
	currentDepth := m.lines[m.focus].Depth
	visible := m.visibleIndexes()
	position := indexOf(visible, m.focus)
	for i := position - 1; i >= 0; i-- {
		line := m.lines[visible[i]]
		if line.Field != "" && line.Depth < currentDepth {
			m.focus = visible[i]
			return
		}
	}
}

func (m *Model) expandAndFocusChild() {
	if !m.hasFocus() {
		return
	}
	line := m.lines[m.focus]
	if !line.Foldable || !line.Collapsed {
		return
	}
	m.lines[m.focus].Collapsed = false
	visible := m.visibleIndexes()
	position := indexOf(visible, m.focus)
	for i := position + 1; i < len(visible); i++ {
		child := m.lines[visible[i]]
		if child.Depth <= line.Depth {
			return
		}
		if child.Field != "" {
			m.focus = visible[i]
			return
		}
	}
}

func (m *Model) toggleFocused() {
	if !m.hasFocus() || !m.lines[m.focus].Foldable {
		return
	}
	m.lines[m.focus].Collapsed = !m.lines[m.focus].Collapsed
}

func (m *Model) focusNextFoldable() {
	visible := m.visibleIndexes()
	position := indexOf(visible, m.focus)
	for i := position + 1; i < len(visible); i++ {
		if m.lines[visible[i]].Foldable {
			m.focus = visible[i]
			return
		}
	}
	for i := 0; i <= position && i < len(visible); i++ {
		if m.lines[visible[i]].Foldable {
			m.focus = visible[i]
			return
		}
	}
}

func (m *Model) focusPreviousFoldable() {
	visible := m.visibleIndexes()
	position := indexOf(visible, m.focus)
	for i := position - 1; i >= 0; i-- {
		if m.lines[visible[i]].Foldable {
			m.focus = visible[i]
			return
		}
	}
	for i := len(visible) - 1; i >= position && i >= 0; i-- {
		if m.lines[visible[i]].Foldable {
			m.focus = visible[i]
			return
		}
	}
}

func (m *Model) focusFirstField() {
	m.focus = m.firstVisibleField()
}

func (m *Model) focusLastField() {
	visible := m.visibleIndexes()
	for i := len(visible) - 1; i >= 0; i-- {
		if m.lines[visible[i]].Field != "" {
			m.focus = visible[i]
			return
		}
	}
}

func (m Model) firstVisibleField() int {
	for _, index := range m.visibleIndexes() {
		if m.lines[index].Field != "" {
			return index
		}
	}
	return -1
}

func (m Model) hasFocus() bool {
	return m.focus >= 0 && m.focus < len(m.lines)
}

func (m Model) visibleIndexes() []int {
	hiddenDepth := -1
	var visible []int
	for i, line := range m.lines {
		if hiddenDepth >= 0 {
			if strings.TrimSpace(line.Text) == "" || line.Depth > hiddenDepth {
				continue
			}
			hiddenDepth = -1
		}
		visible = append(visible, i)
		if line.Foldable && line.Collapsed {
			hiddenDepth = line.Depth
		}
	}
	return visible
}

func (m *Model) ensureFocusVisible() {
	if !m.hasFocus() {
		return
	}
	visible := m.visibleIndexes()
	position := indexOf(visible, m.focus)
	if position < 0 {
		m.focus = m.firstVisibleField()
		position = indexOf(visible, m.focus)
	}
	if position < 0 {
		return
	}
	height := m.schemaHeight()
	if position < m.top {
		m.top = position
	}
	if height > 0 && position >= m.top+height {
		m.top = position - height + 1
	}
	if m.top < 0 {
		m.top = 0
	}
}

func (m Model) view() string {
	width := m.width
	if width <= 0 {
		width = 120
	}

	header := m.header()
	if width >= 100 {
		detailsWidth := max(34, width/3)
		schemaWidth := max(40, width-detailsWidth-2)
		schema := m.schemaView(schemaWidth, m.schemaHeight())
		details := m.detailsView(detailsWidth)
		return header + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, schema, "  ", details)
	}

	schema := m.schemaView(width, m.schemaHeight())
	return header + "\n" + schema + "\n\n" + m.detailsView(width)
}

func (m Model) header() string {
	version := apiVersion(m.doc.Group, m.doc.Version)
	search := ""
	if m.search.active {
		prefix := "/"
		if m.search.fieldOnly {
			prefix = "//"
		}
		search = fmt.Sprintf("  search: %s%s", prefix, m.search.query)
	} else if m.search.query != "" {
		search = fmt.Sprintf("  matches: %d", len(m.search.matches))
	}
	return fmt.Sprintf("%s %s%s", m.doc.Kind, version, search)
}

func (m Model) schemaView(width, height int) string {
	visible := m.visibleIndexes()
	if height <= 0 || height > len(visible) {
		height = len(visible)
	}
	end := min(len(visible), m.top+height)
	var out []string
	for _, index := range visible[m.top:end] {
		line := m.lines[index]
		prefix := "  "
		if line.Foldable {
			if line.Collapsed {
				prefix = "▶ "
			} else {
				prefix = "▼ "
			}
		}
		text := prefix + line.Text
		wrapped := wrapRenderedLine(text, width)
		if index == m.focus {
			for i := range wrapped {
				wrapped[i] = "> " + strings.TrimPrefix(wrapped[i], "  ")
			}
		}
		out = append(out, wrapped...)
	}
	return strings.Join(out, "\n")
}

func (m Model) detailsView(width int) string {
	if !m.hasFocus() {
		return "Details\nSelect a field."
	}

	line := m.lines[m.focus]
	rows := []string{
		"Details",
		"",
		"Path: " + valueOr(line.Path, "<root>"),
		"Required: " + yesNo(line.Required),
		"Foldable: " + yesNo(line.Foldable),
	}
	if line.Foldable {
		rows = append(rows, "Collapsed: "+yesNo(line.Collapsed))
	}
	rows = append(rows, "", "YAML:", line.Text)

	descriptions := m.descriptionLines(line.Path)
	if len(descriptions) > 0 {
		rows = append(rows, "", "Description:")
		for _, description := range descriptions {
			rows = append(rows, wrapPlain(description, width)...)
		}
	}

	rows = append(rows, "", "Keys: up/down focus, left parent, right expand, enter toggle, tab folds, q quit")
	return strings.Join(rows, "\n")
}

func (m Model) descriptionLines(path string) []string {
	var descriptions []string
	for _, line := range m.lines {
		if line.Path != path || line.Field != "" {
			continue
		}
		text := strings.TrimSpace(line.Text)
		if text == "" {
			continue
		}
		text = strings.TrimSpace(strings.TrimPrefix(text, "#"))
		if text != "" {
			descriptions = append(descriptions, text)
		}
	}
	return descriptions
}

func (m Model) schemaHeight() int {
	if m.height <= 0 {
		return 24
	}
	if m.width >= 100 {
		return max(5, m.height-2)
	}
	return max(5, m.height/2)
}

func wrapRenderedLine(line string, width int) []string {
	if width <= 0 || len(line) <= width {
		return []string{line}
	}
	trimmed := strings.TrimLeft(line, " ")
	commentIndex := strings.Index(trimmed, "# ")
	if commentIndex < 0 {
		return wrapPlain(line, width)
	}
	indent := line[:len(line)-len(trimmed)] + trimmed[:commentIndex]
	prefix := indent + "# "
	body := strings.TrimSpace(trimmed[commentIndex+2:])
	if body == "" {
		return []string{line}
	}
	return wrapWithPrefix(prefix, body, width)
}

func wrapPlain(line string, width int) []string {
	if width <= 0 || len(line) <= width {
		return []string{line}
	}
	return wrapWithPrefix("", line, width)
}

func wrapWithPrefix(prefix, text string, width int) []string {
	if width <= len(prefix)+1 {
		return []string{prefix + text}
	}
	limit := width - len(prefix)
	var lines []string
	var current strings.Builder
	for _, word := range strings.Fields(text) {
		if current.Len() == 0 {
			current.WriteString(word)
			continue
		}
		if current.Len()+1+len(word) > limit {
			lines = append(lines, prefix+current.String())
			current.Reset()
			current.WriteString(word)
			continue
		}
		current.WriteByte(' ')
		current.WriteString(word)
	}
	if current.Len() > 0 {
		lines = append(lines, prefix+current.String())
	}
	return lines
}

func descriptionMode(mode tree.DescriptionMode) tree.DescriptionMode {
	if mode == "" {
		return tree.DescriptionTrue
	}
	return mode
}

func apiVersion(group, version string) string {
	if group == "" {
		return version
	}
	return group + "/" + version
}

func indexOf(indexes []int, wanted int) int {
	for i, index := range indexes {
		if index == wanted {
			return i
		}
	}
	return -1
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
