package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/tree"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
)

const (
	fullSchemaDepth      = 1000
	cursorBackgroundANSI = "\x1b[48;5;236m"
	ansiReset            = "\x1b[m"
)

var cursorStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("236"))

var detailTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Underline(true).
	Foreground(lipgloss.Color("15"))

var (
	overviewGroupStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	filterHitStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("214"))
	filterStatusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	detailLabelStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
	detailValueStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	detailRequiredStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	detailOptionalStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	detailFooterStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

var separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

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
	filter textFilterState
}

type searchState struct {
	active    bool
	fieldOnly bool
	query     string
	matches   []int
	position  int
}

type textFilterState struct {
	query string
}

func (f textFilterState) active() bool {
	return f.query != ""
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
	if errors.Is(err, tea.ErrInterrupted) {
		return nil
	}
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

func (m Model) FilterQuery() string {
	return m.filter.query
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	}
	if msg.Key().Code == tea.KeyF10 {
		return m, tea.Quit
	}

	if m.search.active {
		return m.handleSearchKey(msg), nil
	}

	if m.search.query != "" {
		switch msg.String() {
		case "n":
			m.focusNextSearchMatch()
			m.ensureFocusVisible()
			return m, nil
		case "p":
			m.focusPreviousSearchMatch()
			m.ensureFocusVisible()
			return m, nil
		}
	}

	if m.handleFilterKey(msg) {
		m.ensureFocusVisible()
		return m, nil
	}

	key := msg.Key()
	if key.Code == tea.KeyTab {
		if m.filter.active() {
			if key.Mod.Contains(tea.ModShift) {
				m.focusPreviousFilterMatch()
			} else {
				m.focusNextFilterMatch()
			}
			m.ensureFocusVisible()
			return m, nil
		}
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
		m.collapseOrFocusParent()
	case tea.KeyRight:
		m.expandOrFocusChild()
	case tea.KeyPgUp:
		m.focusByHalfPage(-1)
	case tea.KeyPgDown:
		m.focusByHalfPage(1)
	case tea.KeyEnter:
		m.toggleFocused()
	case tea.KeyHome:
		m.focusFirstField()
	case tea.KeyEnd:
		m.focusLastField()
	default:
		switch msg.String() {
		case "/":
			m.search = searchState{active: true}
		}
	}
	m.ensureFocusVisible()
	return m, nil
}

func (m *Model) handleFilterKey(msg tea.KeyPressMsg) bool {
	switch msg.Key().Code {
	case tea.KeyEsc:
		if !m.filter.active() {
			return false
		}
		m.clearFilter()
		return true
	case tea.KeyEnter:
		if !m.filter.active() {
			return false
		}
		m.acceptFilter()
		return true
	case tea.KeyBackspace:
		if !m.filter.active() {
			return false
		}
		m.filter.query = m.filter.query[:len(m.filter.query)-1]
		m.ensureFilterFocus()
		return true
	}
	text, ok := filterTextFromKey(msg)
	if !ok {
		return false
	}
	m.filter.query += text
	m.ensureFilterFocus()
	return true
}

func (m *Model) acceptFilter() {
	for _, index := range m.visibleFieldIndexes() {
		m.expandAncestors(m.lines[index].Path)
	}
	m.filter.query = ""
}

func (m *Model) clearFilter() {
	path := m.FocusPath()
	m.filter.query = ""
	if path != "" {
		m.expandAncestors(path)
	}
}

func (m *Model) ensureFilterFocus() {
	if !m.filter.active() {
		return
	}
	direct := m.filterDirectFieldIndexes()
	if indexOf(direct, m.focus) >= 0 {
		return
	}
	if len(direct) > 0 {
		m.focus = direct[0]
		return
	}
	visible := m.visibleFieldIndexes()
	if len(visible) > 0 {
		m.focus = visible[0]
	}
}

func (m Model) filterDirectFieldIndexes() []int {
	query := strings.ToLower(m.filter.query)
	var direct []int
	seen := map[string]bool{}
	for i, line := range m.lines {
		if line.Field == "" || seen[line.Path] {
			continue
		}
		seen[line.Path] = true
		if m.filterDirectMatch(line, query) {
			direct = append(direct, i)
		}
	}
	return direct
}

func (m *Model) focusNextFilterMatch() {
	m.focusFilterMatch(1)
}

func (m *Model) focusPreviousFilterMatch() {
	m.focusFilterMatch(-1)
}

func (m *Model) focusFilterMatch(delta int) {
	matches := m.filterDirectFieldIndexes()
	if len(matches) == 0 {
		return
	}
	position := indexOf(matches, m.focus)
	if position < 0 {
		position = 0
	} else {
		position = (position + delta + len(matches)) % len(matches)
	}
	m.focus = matches[position]
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
	if m.filter.active() && len(visible) == 0 {
		m.top = 0
		return
	}
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

func (m *Model) collapseOrFocusParent() {
	if !m.hasFocus() {
		return
	}
	line := m.lines[m.focus]
	if line.Foldable && !line.Collapsed {
		m.lines[m.focus].Collapsed = true
		return
	}
	m.focusParent()
}

func (m *Model) expandOrFocusChild() {
	if !m.hasFocus() {
		return
	}
	line := m.lines[m.focus]
	if !line.Foldable {
		return
	}
	if line.Collapsed {
		m.lines[m.focus].Collapsed = false
		return
	}
	m.focusFirstChild()
}

func (m *Model) focusFirstChild() {
	if !m.hasFocus() {
		return
	}
	line := m.lines[m.focus]
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

func (m *Model) focusByHalfPage(direction int) {
	distance := max(1, m.schemaHeight()/2)
	m.focusByVisibleFieldOffset(direction * distance)
}

func (m *Model) focusByVisibleFieldOffset(delta int) {
	fields := m.visibleFieldIndexes()
	position := indexOf(fields, m.focus)
	if position < 0 {
		return
	}
	position += delta
	if position < 0 {
		position = 0
	}
	if position >= len(fields) {
		position = len(fields) - 1
	}
	m.focus = fields[position]
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
	if m.filter.active() {
		return m.filteredIndexes()
	}
	return m.collapsedVisibleIndexes()
}

func (m Model) collapsedVisibleIndexes() []int {
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

func (m Model) filteredIndexes() []int {
	included := m.filterIncludedPaths()
	var visible []int
	for i, line := range m.lines {
		if line.Path == "" {
			continue
		}
		if included[line.Path] {
			visible = append(visible, i)
		}
	}
	return visible
}

func (m Model) filterIncludedPaths() map[string]bool {
	query := strings.ToLower(m.filter.query)
	direct := map[string]bool{}
	var fields []string
	seen := map[string]bool{}
	for _, line := range m.lines {
		if line.Field == "" || seen[line.Path] {
			continue
		}
		seen[line.Path] = true
		fields = append(fields, line.Path)
		if m.filterDirectMatch(line, query) {
			direct[line.Path] = true
		}
	}

	included := map[string]bool{}
	for _, path := range fields {
		if direct[path] {
			included[path] = true
			for _, ancestor := range ancestorPaths(path) {
				included[ancestor] = true
			}
		}
	}
	for _, path := range fields {
		if direct[path] || hasDirectAncestor(path, direct) {
			included[path] = true
		}
	}
	return included
}

func (m Model) filterDirectMatch(line tree.Line, query string) bool {
	if query == "" {
		return true
	}
	if strings.Contains(strings.ToLower(line.Field), query) {
		return true
	}
	if _, ok := pathFilterHighlight(line.Path, query); ok {
		return true
	}
	for _, description := range m.descriptionLines(line.Path) {
		if strings.Contains(strings.ToLower(description), query) {
			return true
		}
	}
	return false
}

func hasDirectAncestor(path string, direct map[string]bool) bool {
	for _, ancestor := range ancestorPaths(path) {
		if direct[ancestor] {
			return true
		}
	}
	return false
}

func ancestorPaths(path string) []string {
	var ancestors []string
	for {
		switch {
		case strings.HasSuffix(path, "[]"):
			path = strings.TrimSuffix(path, "[]")
		default:
			index := strings.LastIndex(path, ".")
			if index < 0 {
				return ancestors
			}
			path = path[:index]
		}
		if path == "" {
			return ancestors
		}
		ancestors = append(ancestors, path)
	}
}

func (m *Model) expandAncestors(path string) {
	ancestors := map[string]bool{}
	for _, ancestor := range ancestorPaths(path) {
		ancestors[ancestor] = true
	}
	for i := range m.lines {
		if ancestors[m.lines[i].Path] {
			m.lines[i].Collapsed = false
		}
	}
}

func (m Model) visibleFieldIndexes() []int {
	var fields []int
	for _, index := range m.visibleIndexes() {
		if m.lines[index].Field != "" {
			fields = append(fields, index)
		}
	}
	return fields
}

func (m *Model) ensureFocusVisible() {
	visible := m.visibleIndexes()
	if len(visible) == 0 {
		m.focus = -1
		m.top = 0
		return
	}
	if m.top < 0 {
		m.top = 0
	}
	if m.top >= len(visible) {
		m.top = len(visible) - 1
	}
	if !m.hasFocus() {
		m.focus = m.firstVisibleField()
	}
	position := indexOf(visible, m.focus)
	if position < 0 {
		m.focus = m.firstVisibleField()
		position = indexOf(visible, m.focus)
	}
	if position < 0 {
		return
	}
	height := m.schemaHeight()
	width := m.schemaPaneWidth()
	if height <= 0 {
		return
	}

	rows := m.schemaRowCounts(visible, width)
	prefix := prefixSums(rows)
	if rows[position] >= height {
		m.top = position
		return
	}

	focusEnd := prefix[position+1] - prefix[m.top]
	if m.top > position || focusEnd > height {
		target := max(0, prefix[position+1]-height)
		m.top = firstPrefixAtLeast(prefix, target, position)
		return
	}

	focusStart := prefix[position] - prefix[m.top]
	if focusStart == 0 && m.top > 0 {
		target := max(0, prefix[position+1]-height)
		m.top = firstPrefixAtLeast(prefix, target, position)
	}
}

func (m Model) schemaPaneWidth() int {
	width := m.width
	if width <= 0 {
		width = 120
	}
	if width < 100 {
		return width
	}
	schemaWidth, _ := m.widePaneWidths(width)
	return schemaWidth
}

func (m Model) renderedSchemaRows(index, width int) int {
	line := m.lines[index]
	return len(wrapSchemaLine(line, m.schemaLineText(line), width))
}

func (m Model) schemaRowCounts(visible []int, width int) []int {
	rows := make([]int, len(visible))
	for i, index := range visible {
		rows[i] = m.renderedSchemaRows(index, width)
	}
	return rows
}

func prefixSums(values []int) []int {
	prefix := make([]int, len(values)+1)
	for i, value := range values {
		prefix[i+1] = prefix[i] + value
	}
	return prefix
}

func firstPrefixAtLeast(prefix []int, target, maxIndex int) int {
	low := 0
	high := maxIndex
	for low < high {
		mid := (low + high) / 2
		if prefix[mid] >= target {
			high = mid
		} else {
			low = mid + 1
		}
	}
	return low
}

func (m Model) view() string {
	width := m.width
	if width <= 0 {
		width = 120
	}

	statusLine := m.statusLine()
	statusPrefix := ""
	if statusLine != "" {
		statusPrefix = statusLine + "\n"
	}
	if width >= 100 {
		schemaWidth, detailsWidth := m.widePaneWidths(width)
		schema := m.schemaView(schemaWidth, m.schemaHeight())
		details := m.detailsView(detailsWidth, m.contentHeight())
		return statusPrefix + lipgloss.JoinHorizontal(lipgloss.Top, schema, wideSeparator(m.contentHeight()), details)
	}

	schema := m.schemaView(width, m.schemaHeight())
	detailsHeight := max(1, m.contentHeight()-m.schemaHeight()-2)
	return statusPrefix + schema + "\n\n" + m.detailsView(width, detailsHeight)
}

const wideSeparatorWidth = 3

func (m Model) widePaneWidths(width int) (schemaWidth, detailsWidth int) {
	detailsWidth = max(25, width/4)
	schemaWidth = max(40, width-detailsWidth-wideSeparatorWidth)
	return schemaWidth, detailsWidth
}

func wideSeparator(height int) string {
	if height <= 0 {
		height = 1
	}
	lines := make([]string, height)
	for i := range lines {
		lines[i] = " " + separatorStyle.Render("│") + " "
	}
	return strings.Join(lines, "\n")
}

func (m Model) statusLine() string {
	if m.search.active {
		prefix := "/"
		if m.search.fieldOnly {
			prefix = "//"
		}
		return fmt.Sprintf("search: %s%s", prefix, m.search.query)
	}
	if m.search.query != "" {
		return fmt.Sprintf("matches: %d", len(m.search.matches))
	}
	if m.filter.active() {
		return fmt.Sprintf("filter: %s", m.filter.query)
	}
	return ""
}

func (m Model) schemaView(width, height int) string {
	visible := m.visibleIndexes()
	if height <= 0 {
		height = len(visible)
	}
	var out []string
	if len(visible) == 0 {
		for len(out) < height {
			out = append(out, strings.Repeat(" ", max(0, width)))
		}
		return strings.Join(out, "\n")
	}
	top := m.top
	if top < 0 {
		top = 0
	}
	if top >= len(visible) {
		top = len(visible) - 1
	}
	end := min(len(visible), top+height)
	for _, index := range visible[top:end] {
		line := m.lines[index]
		text := m.schemaLineText(line)
		wrapped := wrapSchemaLine(line, text, width)
		if index == m.focus {
			for i := range wrapped {
				wrapped[i].Text = colorFocusedSchemaLine(wrapped[i].Text, wrapped[i].Code, width)
				wrapped[i].Text = m.highlightFilterLine(line, wrapped[i].Text)
			}
		} else {
			for i := range wrapped {
				wrapped[i].Text = colorSchemaLine(wrapped[i].Text, wrapped[i].Code)
				wrapped[i].Text = m.highlightFilterLine(line, wrapped[i].Text)
			}
		}
		for _, wrappedLine := range wrapped {
			if len(out) >= height {
				return strings.Join(out, "\n")
			}
			out = append(out, wrappedLine.Text)
		}
	}
	for len(out) < height {
		out = append(out, strings.Repeat(" ", max(0, width)))
	}
	return strings.Join(out, "\n")
}

type schemaVisualLine struct {
	Text string
	Code bool
}

func wrapSchemaLine(line tree.Line, text string, width int) []schemaVisualLine {
	if line.Code {
		wrapped := tree.WrapInlineCommentText(text, true, width)
		if len(wrapped) > 1 {
			out := make([]schemaVisualLine, 0, len(wrapped))
			for _, wrappedLine := range wrapped {
				out = append(out, schemaVisualLine{Text: wrappedLine.Text, Code: wrappedLine.Code})
			}
			return out
		}
	}

	plain := wrapRenderedLine(text, width)
	out := make([]schemaVisualLine, 0, len(plain))
	for _, wrappedLine := range plain {
		out = append(out, schemaVisualLine{Text: wrappedLine, Code: line.Code})
	}
	return out
}

func colorFocusedSchemaLine(line string, code bool, width int) string {
	colored := padVisible(colorSchemaLine(line, code), width)
	return persistentBackground(colored)
}

func persistentBackground(line string) string {
	return cursorBackgroundANSI + strings.ReplaceAll(line, ansiReset, ansiReset+cursorBackgroundANSI) + ansiReset
}

func (m Model) schemaLineText(line tree.Line) string {
	prefix := "  "
	if line.Foldable {
		if line.Collapsed {
			prefix = "▶ "
		} else {
			prefix = "▼ "
		}
	}
	return prefix + line.Text
}

func padVisible(line string, width int) string {
	if width <= 0 {
		return line
	}
	if padding := width - lipgloss.Width(line); padding > 0 {
		return line + strings.Repeat(" ", padding)
	}
	return line
}

func colorSchemaLine(line string, code bool) string {
	for _, prefix := range []string{"▶ ", "▼ ", "  "} {
		if strings.HasPrefix(line, prefix) {
			return prefix + yamlrender.ColorLineWithMetadata(strings.TrimPrefix(line, prefix), code)
		}
	}
	return yamlrender.ColorLineWithMetadata(line, code)
}

func (m Model) highlightFilterLine(line tree.Line, text string) string {
	text = highlightFilterMatches(text, m.filter.query)
	if highlight, ok := pathFilterHighlight(line.Path, m.filter.query); ok {
		text = highlightFilterMatches(text, highlight)
	}
	return text
}

func highlightFilterMatches(text, query string) string {
	if query == "" {
		return text
	}
	var out strings.Builder
	activeSGR := ""
	for text != "" {
		escape := strings.IndexByte(text, '\x1b')
		if escape < 0 {
			out.WriteString(highlightFilterSegment(text, query, activeSGR))
			return out.String()
		}
		out.WriteString(highlightFilterSegment(text[:escape], query, activeSGR))
		text = text[escape:]
		end := strings.IndexByte(text, 'm')
		if end < 0 {
			out.WriteString(text)
			return out.String()
		}
		sequence := text[:end+1]
		out.WriteString(sequence)
		activeSGR = updateActiveSGR(activeSGR, sequence)
		text = text[end+1:]
	}
	return out.String()
}

func highlightFilterSegment(text, query, activeSGR string) string {
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	var out strings.Builder
	for {
		index := strings.Index(lowerText, lowerQuery)
		if index < 0 {
			out.WriteString(text)
			return out.String()
		}
		out.WriteString(text[:index])
		end := index + len(query)
		out.WriteString(filterHitStyle.Render(text[index:end]))
		out.WriteString(activeSGR)
		text = text[end:]
		lowerText = lowerText[end:]
	}
}

func updateActiveSGR(active, sequence string) string {
	if !strings.HasPrefix(sequence, "\x1b[") || !strings.HasSuffix(sequence, "m") {
		return active
	}
	params := strings.TrimSuffix(strings.TrimPrefix(sequence, "\x1b["), "m")
	if params == "" {
		return ""
	}

	hasReset := false
	hasStyle := false
	for _, param := range strings.FieldsFunc(params, func(r rune) bool {
		return r == ';' || r == ':'
	}) {
		switch param {
		case "", "0":
			hasReset = true
		default:
			hasStyle = true
		}
	}
	if hasReset && !hasStyle {
		return ""
	}
	if hasReset {
		return sequence
	}
	return active + sequence
}

func filterTextFromKey(msg tea.KeyPressMsg) (string, bool) {
	text := msg.Key().Text
	if text == "" {
		text = msg.String()
	}
	if text == "/" || text == "" || text == "\x7f" {
		return "", false
	}
	runes := []rune(text)
	if len(runes) != 1 || runes[0] < ' ' {
		return "", false
	}
	return text, true
}

func (m Model) detailsView(width, height int) string {
	if !m.hasFocus() {
		return stickFooter([]string{detailTitleStyle.Render("Details"), "Select a field."}, detailFooterLines(width), height)
	}

	line := m.lines[m.focus]
	rows := []string{
		detailTitleStyle.Render("Details"),
		"",
	}
	rows = append(rows, detailRowLines("PATH", valueOr(line.Path, "<root>"), detailValueStyle, width)...)
	rows = append(rows, detailRowLines("TYPE", fieldType(line), detailValueStyle, width)...)
	rows = append(rows, detailRowLines("REQUIRED", yesNo(line.Required), requiredStyleFor(line.Required), width)...)

	descriptions := m.descriptionLines(line.Path)
	if len(descriptions) > 0 {
		rows = append(rows, "", detailLabelStyle.Render("DESCRIPTION"))
		for _, description := range descriptions {
			rows = append(rows, wrapPlain(description, width)...)
		}
	}

	metadata := m.validationMetadata(line)
	if len(metadata) > 0 {
		rows = append(rows, "", detailLabelStyle.Render("VALIDATION AND METADATA"))
		for _, item := range metadata {
			rows = append(rows, wrapPlain("- "+item, width)...)
		}
	}

	return stickFooter(rows, detailFooterLines(width), height)
}

func requiredStyleFor(required bool) lipgloss.Style {
	if required {
		return detailRequiredStyle
	}
	return detailOptionalStyle
}

func detailRowLines(label, value string, valueStyle lipgloss.Style, width int) []string {
	const gap = "  "
	prefixWidth := len(label) + len(gap)
	valueWidth := max(1, width-prefixWidth)
	wrapped := wrapPlain(value, valueWidth)
	if len(wrapped) == 0 {
		wrapped = []string{""}
	}

	lines := make([]string, 0, len(wrapped))
	lines = append(lines, detailLabelStyle.Render(label)+gap+valueStyle.Render(wrapped[0]))
	continuationPrefix := strings.Repeat(" ", prefixWidth)
	for _, line := range wrapped[1:] {
		lines = append(lines, continuationPrefix+valueStyle.Render(line))
	}
	return lines
}

func detailFooterLines(width int) []string {
	const footer = "up/down focus  left parent/collapse  right expand/child  enter toggle  tab folds  q/F10/Ctrl-C quit"
	lines := wrapPlain(footer, width)
	for i := range lines {
		lines[i] = detailFooterStyle.Render(lines[i])
	}
	return lines
}

func stickFooter(rows []string, footer []string, height int) string {
	if height <= 0 {
		rows = append(rows, "")
		rows = append(rows, footer...)
		return strings.Join(rows, "\n")
	}
	if len(footer) > height {
		footer = footer[len(footer)-height:]
	}
	availableRows := height - len(footer)
	if len(rows) > availableRows {
		rows = rows[:availableRows]
	}
	spacing := height - len(rows) - len(footer)
	for i := 0; i < spacing; i++ {
		rows = append(rows, "")
	}
	rows = append(rows, footer...)
	return strings.Join(rows, "\n")
}

func (m Model) validationMetadata(line tree.Line) []string {
	comment := inlineComment(line.Text)
	parts := splitCommentMetadata(comment)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "optional" {
			continue
		}
		out = append(out, part)
	}
	for _, candidate := range m.lines {
		if candidate.Path != line.Path || !candidate.Metadata {
			continue
		}
		text := strings.TrimSpace(candidate.Text)
		text = strings.TrimSpace(strings.TrimPrefix(text, "#"))
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func inlineComment(text string) string {
	index := strings.Index(text, " # ")
	if index < 0 {
		return ""
	}
	return strings.TrimSpace(text[index+3:])
}

func splitCommentMetadata(comment string) []string {
	raw := strings.Split(comment, ", ")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		parts = append(parts, strings.TrimSpace(part))
	}
	return parts
}

func fieldType(line tree.Line) string {
	if line.Path == "apiVersion" || line.Path == "kind" {
		return "string"
	}
	text := strings.TrimSpace(line.Text)
	if strings.HasPrefix(text, "# ") {
		text = strings.TrimSpace(strings.TrimPrefix(text, "# "))
	}
	if strings.HasPrefix(text, "- ") {
		text = strings.TrimSpace(strings.TrimPrefix(text, "- "))
	}
	if strings.HasPrefix(text, "# ") {
		text = strings.TrimSpace(strings.TrimPrefix(text, "# "))
	}
	colon := strings.Index(text, ":")
	if colon < 0 {
		return "object"
	}
	value := strings.TrimSpace(text[colon+1:])
	if index := strings.Index(value, " # "); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}
	switch {
	case value == "":
		return "object"
	case strings.HasPrefix(value, "#"):
		return "object"
	case value == "{}":
		return "object"
	case value == "[]":
		return "array"
	case strings.HasPrefix(value, "["):
		return "array"
	case strings.HasPrefix(value, `"`):
		return "string"
	case strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">"):
		return strings.Trim(value, "<>")
	case value == "true" || value == "false":
		return "boolean"
	default:
		return "scalar"
	}
}

func (m Model) descriptionLines(path string) []string {
	for _, line := range m.lines {
		if line.Path != path || line.Field == "" || line.Metadata {
			continue
		}
		if description := strings.TrimSpace(line.Description); description != "" {
			return []string{description}
		}
	}
	var descriptions []string
	for _, line := range m.lines {
		if line.Path != path || line.Field != "" || line.Metadata {
			continue
		}
		text := strings.TrimSpace(line.Description)
		if text == "" {
			continue
		}
		descriptions = append(descriptions, text)
	}
	return descriptions
}

func (m Model) schemaHeight() int {
	if m.height <= 0 {
		return 24
	}
	if m.width >= 100 {
		return m.contentHeight()
	}
	return max(1, m.contentHeight()/2)
}

func (m Model) contentHeight() int {
	if m.height <= 0 {
		return 24
	}
	height := m.height
	if m.statusLine() != "" {
		height--
	}
	return max(1, height)
}

func wrapRenderedLine(line string, width int) []string {
	if width <= 0 || lipgloss.Width(line) <= width {
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
	if width <= 0 || lipgloss.Width(line) <= width {
		return []string{line}
	}
	return wrapWithPrefix("", line, width)
}

func wrapWithPrefix(prefix, text string, width int) []string {
	prefixWidth := lipgloss.Width(prefix)
	if width <= prefixWidth+1 {
		return hardWrap(prefix+text, width)
	}
	limit := width - prefixWidth
	var lines []string
	var current strings.Builder
	for _, word := range strings.Fields(text) {
		if lipgloss.Width(word) > limit {
			if current.Len() > 0 {
				lines = append(lines, prefix+current.String())
				current.Reset()
			}
			for _, line := range hardWrap(word, limit) {
				lines = append(lines, prefix+line)
			}
			continue
		}
		if current.Len() == 0 {
			current.WriteString(word)
			continue
		}
		if lipgloss.Width(current.String())+1+lipgloss.Width(word) > limit {
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

func hardWrap(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	var lines []string
	var current strings.Builder
	currentWidth := 0
	for _, r := range text {
		rWidth := lipgloss.Width(string(r))
		if currentWidth > 0 && currentWidth+rWidth > width {
			lines = append(lines, current.String())
			current.Reset()
			currentWidth = 0
		}
		current.WriteRune(r)
		currentWidth += rWidth
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func descriptionMode(mode tree.DescriptionMode) tree.DescriptionMode {
	if mode == "" {
		return tree.DescriptionTrue
	}
	return mode
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
