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
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
)

const fullExpandDepth = 1000

type Renderer struct {
	ExpandDepth  int
	Descriptions yamlrender.DescriptionMode
	Columns      int
	BackURL      string
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
	if _, err := fmt.Fprintf(out, "<main class=\"kubectl-doc\" data-kubectl-doc%s>\n<header class=\"kdoc-header\">\n<h1>%s <small>%s</small></h1>\n<div class=\"kdoc-filter-overlay\" data-kdoc-filter-overlay hidden></div>\n", backAttr, escape(docs[0].Kind), escape(headerVersion(docs))); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "</header>"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "<div class=\"kdoc-layout\"><section class=\"kdoc-docs\">"); err != nil {
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
	return `<style>
.kubectl-doc{--kdoc-fg:#1f2933;--kdoc-muted:#57606a;--kdoc-border:#d8dee4;--kdoc-panel:#f6f8fa;--kdoc-selected:#fff7cc;--kdoc-filter:#fb8500;--kdoc-required:#cf222e;--kdoc-ok:#116329;--kdoc-yaml-key:#0550ae;--kdoc-yaml-string:#0a7f42;--kdoc-yaml-comment:#6e7781;--kdoc-yaml-punct:#8c959f;--kdoc-yaml-number:#953800;--kdoc-yaml-type-number:#007c89;--kdoc-yaml-bool:#8250df;--kdoc-yaml-null:#8250df;box-sizing:border-box;color:var(--kdoc-fg);background:#fff;font:14px/1.45 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;max-width:100%;padding:24px}
.kubectl-doc *{box-sizing:border-box}
.kdoc-filter-overlay{background:#fff7cc;border:1px solid #f0d35b;border-radius:6px;color:#7a4b00;display:inline-block;font:12px/1.25 ui-monospace,SFMono-Regular,SFMono,Consolas,"Liberation Mono",Menlo,monospace;margin:0;padding:4px 7px}
.kdoc-filter-overlay[hidden]{display:none}
.kdoc-view-controls{bottom:calc(12px + 2.5em);display:flex;height:0;justify-content:flex-end;pointer-events:none;position:sticky;z-index:4}
.kdoc-wrap-toggle{align-items:center;background:transparent;border:0;color:var(--kdoc-muted);cursor:pointer;display:flex;font-size:12px;font-weight:600;gap:.65em;line-height:1;padding:0;pointer-events:auto}
.kdoc-wrap-toggle input{block-size:1px;clip:rect(0 0 0 0);clip-path:inset(50%);inline-size:1px;margin:0;overflow:hidden;position:absolute;white-space:nowrap}
.kdoc-switch{background:#d0d7de;border-radius:999px;box-shadow:inset 0 0 0 1px rgba(31,41,51,.08);display:inline-block;flex:0 0 auto;inline-size:2.65em;block-size:1.5em;position:relative;transition:background-color .16s ease,box-shadow .16s ease}
.kdoc-switch::after{background:#fff;border-radius:50%;box-shadow:0 .08em .24em rgba(31,41,51,.28);content:"";display:block;inline-size:1.18em;block-size:1.18em;position:absolute;inset-block-start:50%;inset-inline-start:.16em;transform:translateY(-50%);transition:inset-inline-start .16s ease}
.kdoc-wrap-toggle input:checked + .kdoc-switch{background:#34c759;box-shadow:inset 0 0 0 1px rgba(17,99,41,.12)}
.kdoc-wrap-toggle input:checked + .kdoc-switch::after{inset-inline-start:1.31em}
.kdoc-wrap-toggle input:focus-visible + .kdoc-switch{box-shadow:0 0 0 2px rgba(9,105,218,.25),inset 0 0 0 1px rgba(31,41,51,.08)}
.kdoc-header{align-items:center;border-bottom:1px solid var(--kdoc-border);display:flex;flex-wrap:wrap;gap:12px;margin-bottom:16px;padding-bottom:16px;padding-right:150px}
.kdoc-header h1{font-size:24px;line-height:1.2;margin:0}
.kdoc-header small{color:var(--kdoc-muted);font-size:.6em;font-weight:500}
.kdoc-layout{align-items:start;display:grid;gap:16px;grid-template-columns:minmax(0,1fr) minmax(240px,320px)}
.kdoc-docs{min-width:0}
.kdoc-version h2{font-size:18px;margin:16px 0 8px}
.kdoc-tree{background:var(--kdoc-panel);border:1px solid var(--kdoc-border);border-radius:8px;overflow:auto;padding:10px 0}
.kdoc-line{align-items:flex-start;display:flex;font:13px/1.3 ui-monospace,SFMono-Regular,SFMono,Consolas,"Liberation Mono",Menlo,monospace;margin:0;min-height:1.3em;padding:0 12px;white-space:pre}
.kdoc-line[hidden]{display:none}
.kdoc-fold,.kdoc-gutter{background:transparent;border:0;color:var(--kdoc-muted);display:block;flex:0 0 24px;font:inherit;height:1.3em;line-height:inherit;margin:0;padding:0;text-align:left;user-select:none}
.kdoc-fold{cursor:pointer}
.kdoc-fold:focus{outline:0}
.kdoc-fold:focus-visible::before{color:var(--kdoc-yaml-key)}
.kdoc-fold::before{content:"▶";display:block;line-height:inherit}
.kdoc-fold[aria-expanded="true"]::before{content:"▼"}
.kdoc-yaml-text{min-width:0;white-space:pre}
.kdoc-yaml-comment-text,.kdoc-comment,.kdoc-comment-line{color:var(--kdoc-yaml-comment)}
.kdoc-comment-prefix{white-space:pre}
.kubectl-doc.kdoc-wrap-comments .kdoc-yaml-comment-text{display:block;flex:1 1 auto;white-space:normal}
.kubectl-doc.kdoc-wrap-comments .kdoc-comment{display:block}
.kubectl-doc.kdoc-wrap-comments .kdoc-comment-line{display:block;white-space:pre}
.kdoc-yaml-key{color:var(--kdoc-yaml-key);font-weight:600}
.kdoc-yaml-string{color:var(--kdoc-yaml-string)}
.kdoc-yaml-comment{color:var(--kdoc-yaml-comment)}
.kdoc-yaml-punct{color:var(--kdoc-yaml-punct)}
.kdoc-yaml-number{color:var(--kdoc-yaml-number)}
.kdoc-yaml-type-number{color:var(--kdoc-yaml-type-number)}
.kdoc-yaml-bool,.kdoc-yaml-null{color:var(--kdoc-yaml-bool)}
.kdoc-yaml-placeholder{color:var(--kdoc-muted)}
.kdoc-required-label{background:#ffebe9;border:1px solid #ff8182;border-radius:999px;color:var(--kdoc-required);display:inline-block;font-weight:700;line-height:1.1;padding:0 .35em;vertical-align:baseline}
.kdoc-filter-hit{background:var(--kdoc-filter);border-radius:2px;color:#111;font-weight:700;padding:0 .08em}
.kdoc-selected .kdoc-yaml-text{background:var(--kdoc-selected)}
.kdoc-selected .kdoc-required-label{color:var(--kdoc-required)}
.kdoc-details{align-self:start;background:#fff;border:1px solid var(--kdoc-border);border-radius:8px;font:13px/1.45 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;max-height:calc(100vh - 24px);min-width:0;overflow:auto;padding:12px;position:sticky;scrollbar-gutter:stable;top:12px;z-index:2}
.kdoc-details h2{font-size:16px;line-height:1.25;margin:0 0 10px}
.kdoc-detail-body{display:grid;gap:12px}
.kdoc-detail-empty{color:var(--kdoc-muted);margin:0}
.kdoc-detail-grid{display:grid;gap:7px;margin:0}
.kdoc-detail-row{align-items:baseline;display:grid;gap:8px;grid-template-columns:72px minmax(0,1fr)}
.kdoc-detail-row dt{color:var(--kdoc-muted);font-size:11px;font-weight:700;letter-spacing:.02em;line-height:inherit;text-transform:uppercase}
.kdoc-detail-row dd{line-height:inherit;margin:0;min-width:0}
.kdoc-detail-code,.kdoc-detail-list code{font:12px/1.45 ui-monospace,SFMono-Regular,SFMono,Consolas,"Liberation Mono",Menlo,monospace;overflow-wrap:anywhere}
.kdoc-detail-badge{background:#eaeef2;border:1px solid var(--kdoc-border);border-radius:999px;color:#24292f;display:inline-block;font-size:12px;font-weight:600;line-height:1;padding:.2em .55em;vertical-align:baseline}
.kdoc-detail-badge-required{background:#ffebe9;border-color:#ff8182;color:var(--kdoc-required)}
.kdoc-detail-badge-optional{background:#dafbe1;border-color:#aceebb;color:var(--kdoc-ok)}
.kdoc-detail-section{border-top:1px solid var(--kdoc-border);min-width:0;padding-top:10px}
.kdoc-detail-section h3{color:var(--kdoc-muted);font-size:11px;letter-spacing:.02em;margin:0 0 6px;text-transform:uppercase}
.kdoc-detail-description{margin:0;overflow-wrap:anywhere}
.kdoc-detail-list{display:grid;gap:4px;margin:0;padding-left:18px}
@media(max-width:900px){.kubectl-doc{padding:16px}.kdoc-layout{grid-template-columns:1fr}.kdoc-details{max-height:calc(100vh - 16px);top:8px}}
@media(max-width:640px){.kdoc-view-controls{bottom:calc(8px + 2.5em)}.kdoc-header{padding-right:0}}
</style>`
}

func scriptElement() string {
	return `<script>
(function(){
  function ready(fn){ if(document.readyState !== "loading"){ fn(); } else { document.addEventListener("DOMContentLoaded", fn); } }
  ready(function(){
    document.querySelectorAll("[data-kubectl-doc]").forEach(function(root){
      var lines = Array.prototype.slice.call(root.querySelectorAll("[data-kdoc-line]"));
      var comments = Array.prototype.slice.call(root.querySelectorAll("[data-kdoc-comment]"));
      var details = root.querySelector("[data-kdoc-detail-body]");
      var wrapComments = root.querySelector("[data-kdoc-wrap-comments]");
      var filterOverlay = root.querySelector("[data-kdoc-filter-overlay]");
      var backURL = root.getAttribute("data-kdoc-back-url");
      var resizeFrame = 0;
      var charWidthCache = 0;
      var commentColumnCache = 0;
      var currentLine = null;
      var filterQuery = "";
      var activeFilterState = null;
      var lineStates = [];
      var stateByLine = new Map();
      var fieldStates = [];
      var detailFieldByID = new Map();
      var detailLineGroups = new Map();
      var allLineSet = new Set(lines);
      var commentStates = [];

      lines.forEach(function(line, index){
        var detailID = line.getAttribute("data-detail-id") || "";
        var path = (line.getAttribute("data-path") || "").toLowerCase();
        var state = {
          line: line,
          index: index,
          depth: Number(line.getAttribute("data-depth") || "0"),
          field: line.hasAttribute("data-kdoc-field"),
          filterText: (line.getAttribute("data-kdoc-filter-text") || "").toLowerCase(),
          path: path,
          pathParts: path ? path.split(".") : [],
          detailID: detailID,
          textTrim: line.textContent.trim(),
          toggle: line.querySelector("[data-kdoc-toggle]"),
          fieldState: null,
          ancestors: []
        };
        lineStates.push(state);
        stateByLine.set(line, state);
        if(detailID){
          if(!detailLineGroups.has(detailID)){ detailLineGroups.set(detailID, []); }
          detailLineGroups.get(detailID).push(line);
        }
        if(state.field){
          state.fieldState = state;
          fieldStates.push(state);
          if(detailID && !detailFieldByID.has(detailID)){ detailFieldByID.set(detailID, state); }
        }
      });
      lineStates.forEach(function(state){
        if(!state.field && state.detailID && detailFieldByID.has(state.detailID)){
          state.fieldState = detailFieldByID.get(state.detailID);
        }
      });
      var ancestorStack = [];
      fieldStates.forEach(function(state){
        while(ancestorStack.length && ancestorStack[ancestorStack.length - 1].depth >= state.depth){ ancestorStack.pop(); }
        state.ancestors = ancestorStack.slice();
        ancestorStack.push(state);
      });

      function lineState(line){ return stateByLine.get(line) || null; }
      function button(line){ var state = lineState(line); return state ? state.toggle : line.querySelector("[data-kdoc-toggle]"); }
      function depth(line){ var state = lineState(line); return state ? state.depth : Number(line.getAttribute("data-depth") || "0"); }
      comments.forEach(function(comment){
        var line = comment.closest("[data-kdoc-line]");
        commentStates.push({
          comment: comment,
          line: line,
          firstPrefix: comment.getAttribute("data-kdoc-comment-prefix") || "",
          nextPrefix: comment.getAttribute("data-kdoc-comment-wrap-prefix") || comment.getAttribute("data-kdoc-comment-prefix") || "",
          text: comment.getAttribute("data-kdoc-comment-text") || "",
          wrapState: ""
        });
      });
      function expanded(line){ var b = button(line); return !b || b.getAttribute("aria-expanded") !== "false"; }
      function setExpanded(line, value){
        var b = button(line);
        if(!b){ return; }
        b.setAttribute("aria-expanded", value ? "true" : "false");
      }
      function nextContentDepth(index){
        for(var i = index; i < lineStates.length; i++){
          if(lineStates[i].textTrim !== ""){ return lineStates[i].depth; }
        }
        return null;
      }
      function cleanPathComponent(component){
        return String(component || "").replace(/\[\]$/, "");
      }
      function cleanPathComponents(parts){
        return parts.map(cleanPathComponent);
      }
      function pathComponentEqual(component, token){
        return component === token || cleanPathComponent(component) === token;
      }
      function pathComponentContains(component, token){
        return component.indexOf(token) >= 0 || cleanPathComponent(component).indexOf(token) >= 0;
      }
      function parsePathFilter(query){
        query = String(query || "").toLowerCase();
        var anchored = query.indexOf(".") === 0 && query.indexOf("...") !== 0;
        if(anchored){ query = query.slice(1); }
        if(!query || (!anchored && query.indexOf(".") < 0)){ return null; }

        var filter = {anchored: anchored, tokens: [], suffix: ""};
        for(var i = 0; i < query.length; ){
          if(query.slice(i, i + 3) === "..."){
            filter.tokens.push("...");
            i += 3;
            continue;
          }
          if(query[i] === "."){
            i++;
            continue;
          }

          var start = i;
          while(i < query.length && query[i] !== "."){ i++; }
          var token = query.slice(start, i);
          if(!token){ continue; }
          if(/\s/.test(token)){
            filter.suffix = query.slice(start);
            break;
          }
          filter.tokens.push(token);
        }
        if(!filter.tokens.length && !filter.suffix){ return null; }
        return filter;
      }
      function pathSuffixOverlapsFinalComponent(parts, suffix){
        var text = parts.join(".");
        var finalStart = text.length - parts[parts.length - 1].length;
        var offset = 0;
        while(offset <= text.length){
          var index = text.indexOf(suffix, offset);
          if(index < 0){ return false; }
          if(index + suffix.length > finalStart){ return true; }
          offset = index + 1;
        }
        return false;
      }
      function pathSuffixHighlight(parts, suffix){
        if(!parts.length){ return ""; }
        if(pathSuffixOverlapsFinalComponent(parts, suffix) || pathSuffixOverlapsFinalComponent(cleanPathComponents(parts), suffix)){
          var index = suffix.lastIndexOf(".");
          return index >= 0 ? suffix.slice(index + 1) : suffix;
        }
        return "";
      }
      function matchPathFilter(parts, partIndex, tokens, tokenIndex, suffix){
        if(tokenIndex === tokens.length){
          if(suffix){ return pathSuffixHighlight(parts.slice(partIndex), suffix); }
          return partIndex === parts.length ? "__match__" : "";
        }
        if(tokens[tokenIndex] === "..."){
          if(tokenIndex === tokens.length - 1 && !suffix){
            return cleanPathComponent(parts[parts.length - 1] || "");
          }
          for(var skip = partIndex; skip <= parts.length; skip++){
            var wildcardHit = matchPathFilter(parts, skip, tokens, tokenIndex + 1, suffix);
            if(wildcardHit){ return wildcardHit; }
          }
          return "";
        }
        if(partIndex >= parts.length){ return ""; }

        var token = tokens[tokenIndex];
        if(tokenIndex === tokens.length - 1 && !suffix){
          return partIndex === parts.length - 1 && pathComponentContains(parts[partIndex], token) ? token : "";
        }
        if(!pathComponentEqual(parts[partIndex], token)){ return ""; }
        return matchPathFilter(parts, partIndex + 1, tokens, tokenIndex + 1, suffix);
      }
      function pathFilterHighlightForState(state, filter){
        if(!filter || !state || !state.pathParts.length){ return ""; }
        var parts = state.pathParts;
        if(filter.anchored){
          var anchoredHit = matchPathFilter(parts, 0, filter.tokens, 0, filter.suffix);
          return anchoredHit === "__match__" ? "" : anchoredHit;
        }
        for(var start = 0; start < parts.length; start++){
          var hit = matchPathFilter(parts, start, filter.tokens, 0, filter.suffix);
          if(hit){ return hit === "__match__" ? "" : hit; }
        }
        return "";
      }
      function pathFilterHighlight(line, query){
        return pathFilterHighlightForState(lineState(line), parsePathFilter(query));
      }
      function ancestorFieldLines(line){
        var state = lineState(line);
        if(!state || !state.fieldState){ return []; }
        return state.fieldState.ancestors.map(function(ancestor){ return ancestor.line; }).reverse();
      }
      function currentFilterState(){
        var query = filterQuery.toLowerCase();
        if(!query){ return null; }
        if(activeFilterState && activeFilterState.query === query){ return activeFilterState; }

        var pathFilter = parsePathFilter(query);
        var directFields = new Set();
        fieldStates.forEach(function(state){
          if(state.filterText.indexOf(query) >= 0 || pathFilterHighlightForState(state, pathFilter)){
            directFields.add(state);
          }
        });

        var includedFields = new Set();
        fieldStates.forEach(function(state){
          if(directFields.has(state)){
            includedFields.add(state);
            state.ancestors.forEach(function(ancestor){ includedFields.add(ancestor); });
            return;
          }
          for(var i = 0; i < state.ancestors.length; i++){
            if(directFields.has(state.ancestors[i])){
              includedFields.add(state);
              return;
            }
          }
        });

        var allowedLines = new Set();
        lineStates.forEach(function(state){
          if(state.fieldState && includedFields.has(state.fieldState)){ allowedLines.add(state.line); }
        });

        var directLines = new Set();
        directFields.forEach(function(state){ directLines.add(state.line); });
        activeFilterState = {
          query: query,
          pathFilter: pathFilter,
          directFields: directFields,
          directLines: directLines,
          includedFields: includedFields,
          allowedLines: allowedLines
        };
        return activeFilterState;
      }
      function directFilterMatches(){
        var state = currentFilterState();
        return state ? state.directLines : new Set();
      }
      function directFilterMatchLines(){
        var direct = directFilterMatches();
        return visibleFieldLines().filter(function(line){ return direct.has(line); });
      }
      function filterAllowedLines(){
        var state = currentFilterState();
        return state ? state.allowedLines : allLineSet;
      }
      function applyFolds(){
        var allowed = filterAllowedLines();
        lines.forEach(function(line){ line.hidden = !allowed.has(line); });
        if(filterQuery){
          applyFilterHighlights();
          return;
        }
        lineStates.forEach(function(state, index){
          var line = state.line;
          if(line.hidden || expanded(line)){ return; }
          var parentDepth = state.depth;
          for(var i = index + 1; i < lines.length; i++){
            var blank = lineStates[i].textTrim === "";
            var followingDepth = blank ? nextContentDepth(i + 1) : null;
            if(blank && followingDepth !== null && followingDepth <= parentDepth){ break; }
            if(!blank && lineStates[i].depth <= parentDepth){ break; }
            lines[i].hidden = true;
          }
        });
        applyFilterHighlights();
      }
      function groupedLines(line){
        var id = line.getAttribute("data-detail-id");
        if(!id){ return [line]; }
        return detailLineGroups.get(id) || [line];
      }
      function fieldLineFor(line){
        var state = lineState(line);
        return state && state.fieldState ? state.fieldState.line : null;
      }
      function visibleFieldLines(){
        return fieldStates.filter(function(state){ return !state.line.hidden; }).map(function(state){ return state.line; });
      }
      function visibleFoldableLines(){
        return fieldStates.filter(function(state){ return !state.line.hidden && !!state.toggle; }).map(function(state){ return state.line; });
      }
      function currentFieldLine(){
        if(currentLine && !currentLine.hidden){ return currentLine; }
        return visibleFieldLines()[0] || null;
      }
      function lineIndex(collection, line){
        for(var i = 0; i < collection.length; i++){
          if(collection[i] === line){ return i; }
        }
        return -1;
      }
      function selectFieldByOffset(delta){
        var fields = visibleFieldLines();
        if(!fields.length){ return false; }
        var current = currentFieldLine();
        var index = lineIndex(fields, current);
        if(index < 0){ index = 0; }
        index = Math.max(0, Math.min(fields.length - 1, index + delta));
        select(fields[index], {scroll:true});
        return true;
      }
      function selectFirstField(){
        var fields = visibleFieldLines();
        if(!fields.length){ return false; }
        select(fields[0], {scroll:true});
        return true;
      }
      function selectLastField(){
        var fields = visibleFieldLines();
        if(!fields.length){ return false; }
        select(fields[fields.length - 1], {scroll:true});
        return true;
      }
      function pageFieldDistance(){
        var line = currentFieldLine();
        var height = 18;
        if(line){
          height = Math.max(line.getBoundingClientRect().height, height);
        }
        return Math.max(1, Math.floor(window.innerHeight / height / 2));
      }
      function parentField(line){
        if(!line){ return null; }
        var state = lineState(line);
        if(!state || !state.fieldState){ return null; }
        var ancestors = state.fieldState.ancestors;
        for(var i = ancestors.length - 1; i >= 0; i--){
          if(!ancestors[i].line.hidden){ return ancestors[i].line; }
        }
        return null;
      }
      function firstChildField(line){
        if(!line){ return null; }
        var currentDepth = depth(line);
        var fields = visibleFieldLines();
        var index = lineIndex(fields, line);
        for(var i = index + 1; i < fields.length; i++){
          if(depth(fields[i]) <= currentDepth){ return null; }
          return fields[i];
        }
        return null;
      }
      function toggleField(line){
        var toggle = button(line);
        if(!toggle){ return false; }
        setExpanded(line, !expanded(line));
        applyFolds();
        scheduleCommentWrap();
        select(line, {scroll:true});
        return true;
      }
      function collapseOrParent(){
        var line = currentFieldLine();
        if(!line){ return false; }
        if(button(line) && expanded(line)){
          setExpanded(line, false);
          applyFolds();
          scheduleCommentWrap();
          select(line, {scroll:true});
          return true;
        }
        var parent = parentField(line);
        if(!parent){ return false; }
        select(parent, {scroll:true});
        return true;
      }
      function expandOrChild(){
        var line = currentFieldLine();
        if(!line){ return false; }
        if(!button(line)){ return false; }
        if(!expanded(line)){
          setExpanded(line, true);
          applyFolds();
          scheduleCommentWrap();
          select(line, {scroll:true});
          return true;
        }
        var child = firstChildField(line);
        if(!child){ return false; }
        select(child, {scroll:true});
        return true;
      }
      function selectFoldable(delta){
        var foldable = visibleFoldableLines();
        if(!foldable.length){ return false; }
        var current = currentFieldLine();
        var index = lineIndex(foldable, current);
        if(index < 0){ index = delta > 0 ? -1 : 0; }
        index = (index + delta + foldable.length) % foldable.length;
        select(foldable[index], {scroll:true});
        return true;
      }
      function selectFilterMatch(delta){
        var matches = directFilterMatchLines();
        if(!matches.length){ return false; }
        var current = currentFieldLine();
        var index = lineIndex(matches, current);
        if(index < 0){
          index = delta > 0 ? 0 : matches.length - 1;
        } else {
          index = (index + delta + matches.length) % matches.length;
        }
        select(matches[index], {scroll:true});
        return true;
      }
      function cleanLineText(line){
        var comment = line.querySelector("[data-kdoc-comment]");
        if(comment){ return (comment.getAttribute("data-kdoc-comment-text") || "").trim(); }
        var text = line.querySelector(".kdoc-yaml-text").textContent.trim();
        if(text.indexOf("# ") === 0){ text = text.slice(2).trim(); }
        return text;
      }
      function escapeHTML(value){
        return String(value || "").replace(/[&<>"']/g, function(ch){
          return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[ch];
        });
      }
      function charWidth(){
        if(charWidthCache){ return charWidthCache; }
        var sample = document.createElement("span");
        sample.textContent = "0000000000";
        sample.style.position = "absolute";
        sample.style.visibility = "hidden";
        sample.style.whiteSpace = "pre";
        if(lines[0]){
          sample.style.font = window.getComputedStyle(lines[0]).font;
        }
        root.appendChild(sample);
        charWidthCache = Math.max(sample.getBoundingClientRect().width / 10, 1);
        sample.remove();
        return charWidthCache;
      }
      function visibleMeasureLine(){
        for(var i = 0; i < lines.length; i++){
          if(!lines[i].hidden){ return lines[i]; }
        }
        return lines[0] || null;
      }
      function commentLineChars(){
        if(commentColumnCache){ return commentColumnCache; }
        var line = visibleMeasureLine();
        if(!line){ return 8; }
        var gutter = line.querySelector(".kdoc-fold,.kdoc-gutter");
        var style = window.getComputedStyle(line);
        var width = line.clientWidth - parseFloat(style.paddingLeft || "0") - parseFloat(style.paddingRight || "0");
        if(gutter){ width -= gutter.getBoundingClientRect().width; }
        commentColumnCache = Math.max(Math.floor(Math.max(width, 0) / charWidth()), 8);
        return commentColumnCache;
      }
      function splitLongWord(out, word, limit){
        while(word.length > limit){
          out.push(word.slice(0, limit));
          word = word.slice(limit);
        }
        return word;
      }
      function wrapCommentText(text, firstLimit, nextLimit){
        var words = String(text || "").trim().split(/\s+/).filter(Boolean);
        var out = [];
        var current = "";
        function limit(){ return out.length === 0 ? firstLimit : nextLimit; }
        words.forEach(function(word){
          var currentLimit = Math.max(limit(), 1);
          if(word.length > currentLimit){
            if(current){
              out.push(current);
              current = "";
              currentLimit = Math.max(limit(), 1);
            }
            word = splitLongWord(out, word, currentLimit);
            if(!word){ return; }
          }
          if(!current){
            current = word;
            return;
          }
          if(current.length + 1 + word.length <= Math.max(limit(), 1)){
            current += " " + word;
            return;
          }
          out.push(current);
          current = word;
        });
        if(current){ out.push(current); }
        return out.length ? out : [""];
      }
      function renderCommentLine(prefix, text){
        return "<span class=\"kdoc-comment-line\"><span class=\"kdoc-yaml-comment kdoc-comment-prefix\">" + escapeHTML(prefix) + "</span><span class=\"kdoc-yaml-comment kdoc-comment-body\">" + escapeHTML(text) + "</span></span>";
      }
      function renderComment(state, wrapped, lineChars){
        if(wrapped && state.line && state.line.hidden){ return false; }
        var wrapState = wrapped ? "wrap:" + lineChars : "nowrap";
        if(state.wrapState === wrapState){ return false; }
        if(!wrapped){
          state.comment.innerHTML = "<span class=\"kdoc-yaml-comment kdoc-comment-prefix\">" + escapeHTML(state.firstPrefix) + "</span><span class=\"kdoc-yaml-comment kdoc-comment-body\">" + escapeHTML(state.text) + "</span>";
          state.wrapState = wrapState;
          return true;
        }
        var firstLimit = Math.max(lineChars - state.firstPrefix.length, 8);
        var nextLimit = Math.max(lineChars - state.nextPrefix.length, 8);
        var chunks = wrapCommentText(state.text, firstLimit, nextLimit);
        state.comment.innerHTML = chunks.map(function(chunk, index){
          return renderCommentLine(index === 0 ? state.firstPrefix : state.nextPrefix, chunk);
        }).join("\n");
        state.wrapState = wrapState;
        return true;
      }
      function applyCommentWrap(){
        if(!wrapComments){ return; }
        var wrapped = wrapComments.checked;
        var lineChars = wrapped ? commentLineChars() : 0;
        var changed = false;
        root.classList.toggle("kdoc-wrap-comments", wrapped);
        commentStates.forEach(function(state){
          if(renderComment(state, wrapped, lineChars)){ changed = true; }
        });
        if(changed){ applyFilterHighlights(); }
      }
      function scheduleCommentWrap(){
        if(!wrapComments || !wrapComments.checked || resizeFrame){ return; }
        resizeFrame = window.requestAnimationFrame(function(){
          resizeFrame = 0;
          applyCommentWrap();
        });
      }
      function fallbackDetail(line){
        var path = line.getAttribute("data-path");
        var text = cleanLineText(line);
        var html = "";
        if(path){
          html += "<dl class=\"kdoc-detail-grid\"><div class=\"kdoc-detail-row\"><dt>Path</dt><dd><code class=\"kdoc-detail-code\">" + escapeHTML(path) + "</code></dd></div></dl>";
        }
        if(text){
          html += "<section class=\"kdoc-detail-section\"><p class=\"kdoc-detail-description\">" + escapeHTML(text) + "</p></section>";
        }
        return html || "<p class=\"kdoc-detail-empty\">No field details.</p>";
      }
      function showDetails(line){
        if(details){
          var detailHTML = line.getAttribute("data-detail-html");
          if(detailHTML){
            details.innerHTML = detailHTML;
          } else {
            details.innerHTML = fallbackDetail(line);
          }
        }
      }
      function updateFilterOverlay(){
        if(!filterOverlay){ return; }
        if(!filterQuery){
          filterOverlay.hidden = true;
          filterOverlay.textContent = "";
          return;
        }
        filterOverlay.hidden = false;
        filterOverlay.textContent = "filter: " + filterQuery;
      }
      function expandAncestors(line){
        ancestorFieldLines(line).forEach(function(ancestor){ setExpanded(ancestor, true); });
      }
      function clearFilter(){
        var line = currentLine;
        filterQuery = "";
        activeFilterState = null;
        updateFilterOverlay();
        if(line){ expandAncestors(line); }
        applyFolds();
        scheduleCommentWrap();
        select(line || visibleFieldLines()[0] || lines[0], {scroll:true});
      }
      function acceptFilter(){
        var line = currentLine;
        visibleFieldLines().forEach(function(field){ expandAncestors(field); });
        filterQuery = "";
        activeFilterState = null;
        updateFilterOverlay();
        applyFolds();
        scheduleCommentWrap();
        select(line || visibleFieldLines()[0] || lines[0], {scroll:true});
      }
      function ensureFilteredFocus(){
        if(currentLine && !currentLine.hidden){
          select(currentLine, {scroll:true});
          return;
        }
        select(visibleFieldLines()[0] || lines[0], {scroll:true});
      }
      function setFilter(value){
        filterQuery = value;
        activeFilterState = null;
        updateFilterOverlay();
        applyFolds();
        scheduleCommentWrap();
        ensureFilteredFocus();
      }
      function filterKey(event){
        if(event.key === "/" || event.key.length !== 1){ return ""; }
        if(event.key < " " || event.key === "\x7f"){ return ""; }
        return event.key;
      }
      function clearFilterHighlights(){
        root.querySelectorAll("mark.kdoc-filter-hit").forEach(function(mark){
          mark.replaceWith(document.createTextNode(mark.textContent || ""));
        });
      }
      function highlightTextNode(node, query, needle){
        var value = node.nodeValue || "";
        var lower = value.toLowerCase();
        var index = lower.indexOf(needle);
        if(index < 0){ return; }
        var fragment = document.createDocumentFragment();
        var remaining = value;
        var remainingLower = lower;
        while(index >= 0){
          if(index > 0){ fragment.appendChild(document.createTextNode(remaining.slice(0, index))); }
          var hit = document.createElement("mark");
          hit.className = "kdoc-filter-hit";
          hit.textContent = remaining.slice(index, index + query.length);
          fragment.appendChild(hit);
          remaining = remaining.slice(index + query.length);
          remainingLower = remainingLower.slice(index + query.length);
          index = remainingLower.indexOf(needle);
        }
        if(remaining){ fragment.appendChild(document.createTextNode(remaining)); }
        node.replaceWith(fragment);
      }
      function highlightElement(element, query){
        var needle = query.toLowerCase();
        var walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT, {
          acceptNode: function(node){
            if(!node.nodeValue || node.parentElement.closest("mark.kdoc-filter-hit")){ return NodeFilter.FILTER_REJECT; }
            return node.nodeValue.toLowerCase().indexOf(needle) >= 0 ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT;
          }
        });
        var nodes = [];
        while(walker.nextNode()){ nodes.push(walker.currentNode); }
        nodes.forEach(function(node){ highlightTextNode(node, query, needle); });
      }
      function applyFilterHighlights(){
        clearFilterHighlights();
        if(!filterQuery){ return; }
        var filterState = currentFilterState();
        lineStates.forEach(function(state){
          var line = state.line;
          if(line.hidden){ return; }
          var text = line.querySelector(".kdoc-yaml-text");
          if(!text){ return; }
          highlightElement(text, filterQuery);
          var pathHit = pathFilterHighlightForState(state.fieldState || state, filterState ? filterState.pathFilter : null);
          if(pathHit){ highlightElement(text, pathHit); }
        });
      }
      function select(line, options){
        if(!line){ return; }
        options = options || {};
        var fieldLine = fieldLineFor(line);
        if(fieldLine){
          line = fieldLine;
          currentLine = fieldLine;
        }
        lines.forEach(function(item){ item.classList.remove("kdoc-selected"); });
        groupedLines(line).forEach(function(item){ item.classList.add("kdoc-selected"); });
        showDetails(line);
        if(options.scroll && line.scrollIntoView){
          line.scrollIntoView({block:"nearest", inline:"nearest"});
        }
      }
      function typingTarget(target){
        return !!(target && (target.closest("input,textarea,select") || target.isContentEditable));
      }
      function handleCursorKey(event){
        if(event.defaultPrevented || typingTarget(event.target)){ return false; }
        if(event.altKey || event.ctrlKey || event.metaKey){ return false; }
        var handled = false;
        if(event.key === "Escape" && filterQuery){
          clearFilter();
          handled = true;
        } else if(event.key === "Enter" && filterQuery){
          acceptFilter();
          handled = true;
        } else if(event.key === "Backspace" && filterQuery){
          setFilter(filterQuery.slice(0, -1));
          handled = true;
        } else {
          var typed = filterKey(event);
          if(typed){
            setFilter(filterQuery + typed);
            handled = true;
          }
        }
        if(!handled){ switch(event.key){
        case "ArrowUp":
          handled = selectFieldByOffset(-1);
          break;
        case "ArrowDown":
          handled = selectFieldByOffset(1);
          break;
        case "ArrowLeft":
          handled = collapseOrParent();
          break;
        case "ArrowRight":
          handled = expandOrChild();
          break;
        case "Enter":
          handled = toggleField(currentFieldLine());
          break;
        case "Tab":
          handled = filterQuery ? selectFilterMatch(event.shiftKey ? -1 : 1) : selectFoldable(event.shiftKey ? -1 : 1);
          break;
        case "Home":
          handled = selectFirstField();
          break;
        case "End":
          handled = selectLastField();
          break;
        case "PageUp":
          handled = selectFieldByOffset(-pageFieldDistance());
          break;
        case "PageDown":
          handled = selectFieldByOffset(pageFieldDistance());
          break;
        case "Escape":
          if(backURL){
            window.location.href = backURL;
            handled = true;
          }
          break;
        } }
        if(handled){
          event.preventDefault();
          event.stopPropagation();
        }
        return handled;
      }

      root.addEventListener("click", function(event){
        var toggle = event.target.closest("[data-kdoc-toggle]");
        if(toggle){
          var line = toggle.closest("[data-kdoc-line]");
          setExpanded(line, !expanded(line));
          applyFolds();
          scheduleCommentWrap();
          select(line);
          return;
        }
        var line = event.target.closest("[data-kdoc-line]");
        if(line){ select(line); }
      });
      document.addEventListener("keydown", handleCursorKey);
      if(wrapComments){
        wrapComments.addEventListener("change", function(){
          applyCommentWrap();
        });
      }
      window.addEventListener("resize", function(){
        commentColumnCache = 0;
        scheduleCommentWrap();
      });
      applyCommentWrap();
      applyFolds();
      select(visibleFieldLines()[0] || lines[0]);
    });
  });
})();
</script>`
}
