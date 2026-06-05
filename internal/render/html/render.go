package htmlrender

import (
	"fmt"
	htmlpkg "html"
	"io"
	"strconv"
	"strings"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/fielddetail"
	"github.com/sttts/kubectl-doc/internal/render/tree"
	"github.com/sttts/kubectl-doc/internal/render/web"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
)

const fullExpandDepth = 1000

type Renderer struct {
	ExpandDepth  int
	Descriptions yamlrender.DescriptionMode
	Columns      int
	BackURL      string
	QuitURL      string
}

func (r Renderer) Render(out io.Writer, doc *crd.Document) error {
	return r.RenderAll(out, []*crd.Document{doc})
}

func (r Renderer) RenderAll(out io.Writer, docs []*crd.Document) error {
	docs = compactDocuments(docs)
	if len(docs) == 0 {
		return fmt.Errorf("at least one document is required")
	}

	if _, err := fmt.Fprintf(out, "<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<title>%s</title>\n%s\n</head>\n<body>\n", escape(docs[0].Kind), styleElement()); err != nil {
		return err
	}
	backAttr := ""
	if r.BackURL != "" {
		backAttr = ` data-kdoc-back-url="` + escapeAttr(r.BackURL) + `"`
	}
	quitAttr := ""
	if r.QuitURL != "" {
		quitAttr = ` data-kdoc-quit-url="` + escapeAttr(r.QuitURL) + `"`
	}
	if _, err := fmt.Fprintf(out, "<main class=\"kubectl-doc\" data-kubectl-doc%s%s>\n<header class=\"kdoc-header\">\n<h1>%s <small>%s</small></h1>\n", backAttr, quitAttr, escape(docs[0].Kind), escape(headerVersion(docs))); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "</header>"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "<div class=\"kdoc-layout\"><section class=\"kdoc-docs\"><div class=\"kdoc-filter-overlay\" data-kdoc-filter-overlay hidden></div>"); err != nil {
		return err
	}

	multiple := len(docs) > 1
	for i, doc := range docs {
		if i > 0 {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		if err := r.renderDocument(out, doc, multiple); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(out, "</section><aside class=\"kdoc-details\" data-kdoc-details aria-live=\"polite\"><h2>Details</h2><div class=\"kdoc-detail-body\" data-kdoc-detail-body><p class=\"kdoc-detail-empty\">Select a field.</p></div></aside></div><div class=\"kdoc-view-controls\" aria-label=\"View options\"><label class=\"kdoc-wrap-toggle\"><input type=\"checkbox\" data-kdoc-wrap-comments checked><span class=\"kdoc-switch\" aria-hidden=\"true\"></span><span class=\"kdoc-wrap-label\">wrap</span></label></div>"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "%s\n</main>\n</body>\n</html>\n", scriptElement()); err != nil {
		return err
	}
	return nil
}

func (r Renderer) renderDocument(out io.Writer, doc *crd.Document, multiple bool) error {
	title := "YAML"
	if multiple {
		title = apiVersion(doc.Group, doc.Version)
		if _, err := fmt.Fprintf(out, "<section class=\"kdoc-version\"><h2>%s</h2>\n", escape(title)); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintln(out, "<section class=\"kdoc-version\">"); err != nil {
		return err
	}

	lines := tree.WithCollapsed(tree.Build(doc, tree.Options{
		ExpandDepth:    fullExpandDepth,
		Descriptions:   tree.DescriptionMode(r.Descriptions),
		Columns:        r.Columns,
		RenderStatus:   true,
		RenderMetadata: true,
	}), r.initialExpandDepth())

	if _, err := fmt.Fprintf(out, "<div class=\"kdoc-tree\" role=\"tree\" aria-label=\"%s\">\n", escape(title)); err != nil {
		return err
	}
	details := collectFieldDetails(doc)
	for _, line := range attachFieldDetails(lines, details) {
		if err := renderLine(out, line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, "</div>\n</section>")
	return err
}

func (r Renderer) initialExpandDepth() int {
	if r.ExpandDepth < 0 {
		return 0
	}
	return r.ExpandDepth
}

func renderLine(out io.Writer, line htmlLine) error {
	classes := "kdoc-line"
	if strings.TrimSpace(line.Text) == "" {
		classes += " kdoc-blank"
	}
	detailID := line.DetailID
	if detailID == "" {
		detailID = "line-" + strconv.Itoa(line.Index)
	}
	fieldAttr := ""
	if line.Field != "" {
		fieldAttr = ` data-kdoc-field data-kdoc-field-name="` + escapeAttr(line.Field) + `" data-kdoc-filter-text="` + escapeAttr(line.Field+"\n"+line.Description) + `"`
	}

	if _, err := fmt.Fprintf(out, "<div class=\"%s\" role=\"treeitem\" data-kdoc-line%s data-index=\"%d\" data-depth=\"%d\" data-path=\"%s\" data-detail-id=\"%s\" data-detail=\"%s\" data-detail-html=\"%s\">",
		classes,
		fieldAttr,
		line.Index,
		line.Depth,
		escapeAttr(line.Path),
		escapeAttr(detailID),
		escapeAttr(line.Detail),
		escapeAttr(line.DetailHTML),
	); err != nil {
		return err
	}
	if line.Foldable {
		expanded := "true"
		if line.Collapsed {
			expanded = "false"
		}
		if _, err := fmt.Fprintf(out, "<button class=\"kdoc-fold\" type=\"button\" aria-label=\"Toggle\" aria-expanded=\"%s\" data-kdoc-toggle></button>", expanded); err != nil {
			return err
		}
	} else if _, err := fmt.Fprint(out, "<span class=\"kdoc-gutter\"></span>"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "<span class=\"kdoc-yaml-text%s\">%s</span></div>\n", yamlTextClass(line), renderYAMLText(line)); err != nil {
		return err
	}
	return nil
}

type htmlLine struct {
	tree.Line

	DetailID   string
	Detail     string
	DetailHTML string
}

func attachFieldDetails(lines []tree.Line, details map[string]fielddetail.Field) []htmlLine {
	out := make([]htmlLine, 0, len(lines))
	for _, line := range lines {
		html := htmlLine{Line: line}
		if detail, ok := lookupFieldDetail(details, line.Path); ok {
			applyFieldDetail(&html, detail)
		}
		out = append(out, html)
	}
	return out
}

func applyFieldDetail(line *htmlLine, detail fielddetail.Field) {
	line.Path = detail.Path
	line.Required = detail.Required
	line.DetailID = detail.ID
	line.Detail = fieldDetailText(detail)
	line.DetailHTML = fieldDetailHTML(detail)
}

func lookupFieldDetail(details map[string]fielddetail.Field, path string) (fielddetail.Field, bool) {
	if detail, ok := details[path]; ok {
		return detail, true
	}
	return fielddetail.Field{}, false
}

func fieldDetailText(f fielddetail.Field) string {
	var lines []string
	lines = append(lines, "Path: "+f.Path)
	lines = append(lines, "Type: "+f.Type)
	lines = append(lines, "Required: "+yesNo(f.Required))
	if f.Description != "" {
		lines = append(lines, "", "Description:", f.Description)
	}
	if len(f.Metadata) > 0 {
		lines = append(lines, "", "Validation and metadata:")
		for _, item := range f.Metadata {
			lines = append(lines, "- "+item)
		}
	}
	return strings.Join(lines, "\n")
}

func fieldDetailHTML(f fielddetail.Field) string {
	var out strings.Builder
	out.WriteString(`<dl class="kdoc-detail-grid">`)
	detailRow(&out, "Path", `<code class="kdoc-detail-code">`+escape(f.Path)+`</code>`)
	detailRow(&out, "Type", `<code class="kdoc-detail-code">`+escape(f.Type)+`</code>`)
	requiredClass := "kdoc-detail-badge"
	if f.Required {
		requiredClass += " kdoc-detail-badge-required"
	} else {
		requiredClass += " kdoc-detail-badge-optional"
	}
	detailRow(&out, "Required", `<span class="`+requiredClass+`">`+yesNo(f.Required)+`</span>`)
	out.WriteString(`</dl>`)
	if f.Description != "" {
		out.WriteString(`<section class="kdoc-detail-section"><h3>Description</h3><p class="kdoc-detail-description">`)
		out.WriteString(escape(f.Description))
		out.WriteString(`</p></section>`)
	}
	if len(f.Metadata) > 0 {
		out.WriteString(`<section class="kdoc-detail-section"><h3>Validation and metadata</h3><ul class="kdoc-detail-list">`)
		for _, item := range f.Metadata {
			out.WriteString(`<li><code>`)
			out.WriteString(escape(item))
			out.WriteString(`</code></li>`)
		}
		out.WriteString(`</ul></section>`)
	}
	return out.String()
}

func detailRow(out *strings.Builder, label, valueHTML string) {
	out.WriteString(`<div class="kdoc-detail-row"><dt>`)
	out.WriteString(escape(label))
	out.WriteString(`</dt><dd>`)
	out.WriteString(valueHTML)
	out.WriteString(`</dd></div>`)
}

func collectFieldDetails(doc *crd.Document) map[string]fielddetail.Field {
	return fielddetail.ByPath(doc)
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func headerVersion(docs []*crd.Document) string {
	if len(docs) == 1 {
		return apiVersion(docs[0].Group, docs[0].Version)
	}
	versions := make([]string, 0, len(docs))
	for _, doc := range docs {
		versions = append(versions, apiVersion(doc.Group, doc.Version))
	}
	return strings.Join(versions, ", ")
}

func compactDocuments(docs []*crd.Document) []*crd.Document {
	var out []*crd.Document
	for _, doc := range docs {
		if doc != nil {
			out = append(out, doc)
		}
	}
	return out
}

func apiVersion(group, version string) string {
	if group == "" {
		return version
	}
	return group + "/" + version
}

func escape(value string) string {
	return htmlpkg.EscapeString(value)
}

func escapeAttr(value string) string {
	return htmlpkg.EscapeString(value)
}

func renderYAMLText(line htmlLine) string {
	indentLength := len(line.Text) - len(strings.TrimLeft(line.Text, " "))
	indent := line.Text[:indentLength]
	rest := line.Text[indentLength:]
	if rest == "" {
		return escape(indent)
	}
	if _, _, ok := standaloneCommentPrefixes(rest, line.Field != ""); ok {
		return renderStandaloneComment(indent, rest)
	}
	if strings.HasPrefix(rest, "# ") {
		content := strings.TrimPrefix(rest, "# ")
		if line.Field != "" {
			return escape(indent) + span("kdoc-yaml-comment", "# ") + renderYAMLCode(content)
		}
		return escape(indent) + span("kdoc-yaml-comment", rest)
	}
	return escape(indent) + renderYAMLCode(rest)
}

func yamlTextClass(line htmlLine) string {
	rest := strings.TrimLeft(line.Text, " ")
	if _, _, ok := standaloneCommentPrefixes(rest, line.Field != ""); !ok {
		return ""
	}
	return " kdoc-yaml-comment-text"
}

func standaloneCommentPrefixes(rest string, fieldLine bool) (string, string, bool) {
	if fieldLine {
		return "", "", false
	}
	switch {
	case strings.HasPrefix(rest, "# - # "):
		return "# - # ", "#   # ", true
	case strings.HasPrefix(rest, "- # "):
		return "- # ", "  # ", true
	case strings.HasPrefix(rest, "# "):
		return "# ", "# ", true
	default:
		return "", "", false
	}
}

func renderStandaloneComment(indent, rest string) string {
	prefix, wrapPrefix, ok := standaloneCommentPrefixes(rest, false)
	if !ok {
		return escape(indent) + span("kdoc-yaml-comment", rest)
	}

	fullPrefix := indent + prefix
	fullWrapPrefix := indent + wrapPrefix
	text := strings.TrimPrefix(rest, prefix)
	var out strings.Builder
	out.WriteString(`<span class="kdoc-comment" data-kdoc-comment data-kdoc-comment-prefix="`)
	out.WriteString(escapeAttr(fullPrefix))
	out.WriteString(`" data-kdoc-comment-wrap-prefix="`)
	out.WriteString(escapeAttr(fullWrapPrefix))
	out.WriteString(`" data-kdoc-comment-text="`)
	out.WriteString(escapeAttr(text))
	out.WriteString(`"><span class="kdoc-yaml-comment kdoc-comment-prefix">`)
	out.WriteString(escape(fullPrefix))
	out.WriteString(`</span><span class="kdoc-yaml-comment kdoc-comment-body">`)
	out.WriteString(escape(text))
	out.WriteString(`</span></span>`)
	return out.String()
}

func renderYAMLCode(code string) string {
	inlineComment := ""
	if index := strings.Index(code, " # "); index >= 0 {
		inlineComment = code[index:]
		code = code[:index]
	}

	var out strings.Builder
	if strings.HasPrefix(code, "- ") {
		out.WriteString(span("kdoc-yaml-punct", "-"))
		out.WriteByte(' ')
		code = strings.TrimPrefix(code, "- ")
	} else if code == "-" {
		out.WriteString(span("kdoc-yaml-punct", "-"))
		code = ""
	}

	if colon := strings.Index(code, ":"); colon > 0 {
		key := code[:colon]
		value := code[colon+1:]
		out.WriteString(span("kdoc-yaml-key", key))
		out.WriteString(span("kdoc-yaml-punct", ":"))
		out.WriteString(renderYAMLValue(value))
	} else {
		out.WriteString(renderYAMLValue(code))
	}
	if inlineComment != "" {
		out.WriteString(renderYAMLComment(inlineComment))
	}
	return out.String()
}

func renderYAMLComment(comment string) string {
	index, end := requiredCommentToken(comment)
	if index < 0 {
		return span("kdoc-yaml-comment", comment)
	}
	var out strings.Builder
	if prefix := comment[:index]; prefix != "" {
		out.WriteString(span("kdoc-yaml-comment", prefix))
	}
	out.WriteString(span("kdoc-required-label", "required"))
	if suffix := comment[end:]; suffix != "" {
		out.WriteString(span("kdoc-yaml-comment", suffix))
	}
	return out.String()
}

func requiredCommentToken(comment string) (int, int) {
	const token = "required"
	for start := 0; start < len(comment); {
		index := strings.Index(comment[start:], token)
		if index < 0 {
			return -1, -1
		}
		index += start
		end := index + len(token)
		if commentTokenBoundary(comment, index-1) && commentTokenBoundary(comment, end) {
			return index, end
		}
		start = end
	}
	return -1, -1
}

func commentTokenBoundary(comment string, index int) bool {
	if index < 0 || index >= len(comment) {
		return true
	}
	switch comment[index] {
	case ' ', '\t', ',', ';', '#':
		return true
	default:
		return false
	}
}

func renderYAMLValue(value string) string {
	leadingLength := len(value) - len(strings.TrimLeft(value, " "))
	if leadingLength == len(value) {
		return escape(value)
	}
	return escape(value[:leadingLength]) + renderYAMLScalar(value[leadingLength:])
}

func renderYAMLScalar(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		switch value[i] {
		case '[', ']', '{', '}', ',', ':':
			out.WriteString(span("kdoc-yaml-punct", value[i:i+1]))
			i++
		case ' ', '\t':
			out.WriteByte(value[i])
			i++
		case '"', '\'':
			end := quotedEnd(value, i)
			out.WriteString(span("kdoc-yaml-string", value[i:end]))
			i = end
		default:
			end := tokenEnd(value, i)
			token := value[i:end]
			out.WriteString(renderScalarToken(token))
			i = end
		}
	}
	return out.String()
}

func quotedEnd(value string, start int) int {
	quote := value[start]
	for i := start + 1; i < len(value); i++ {
		if value[i] == '\\' && quote == '"' {
			i++
			continue
		}
		if value[i] == quote {
			return i + 1
		}
	}
	return len(value)
}

func tokenEnd(value string, start int) int {
	for i := start; i < len(value); i++ {
		switch value[i] {
		case '[', ']', '{', '}', ',', ':', ' ', '\t':
			return i
		}
	}
	return len(value)
}

func renderScalarToken(token string) string {
	switch {
	case strings.HasPrefix(token, "<") && strings.HasSuffix(token, ">"):
		return renderPlaceholderToken(token)
	case token == "true" || token == "false":
		return span("kdoc-yaml-bool", token)
	case token == "null":
		return span("kdoc-yaml-null", token)
	case isNumber(token):
		return span("kdoc-yaml-number", token)
	default:
		return span("kdoc-yaml-scalar", token)
	}
}

func renderPlaceholderToken(token string) string {
	switch inner := strings.TrimSuffix(strings.TrimPrefix(token, "<"), ">"); {
	case inner == "int-or-string":
		return span("kdoc-yaml-placeholder", token)
	case inner == "string" || inner == "name":
		return span("kdoc-yaml-string", token)
	case inner == "boolean":
		return span("kdoc-yaml-bool", token)
	case isNumberPlaceholder(inner):
		return span("kdoc-yaml-type-number", token)
	default:
		return span("kdoc-yaml-placeholder", token)
	}
}

func isNumberPlaceholder(inner string) bool {
	switch inner {
	case "integer", "number", "int", "int32", "int64", "float", "float32", "float64", "double":
		return true
	default:
		return false
	}
}

func isNumber(token string) bool {
	if token == "" {
		return false
	}
	_, err := strconv.ParseFloat(token, 64)
	return err == nil
}

func span(className, value string) string {
	return `<span class="` + className + `">` + escape(value) + `</span>`
}

func styleElement() string {
	return web.StyleElement()
}

func scriptElement() string {
	return web.ScriptElement()
}
