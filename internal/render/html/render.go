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
	"github.com/sttts/kubectl-doc/internal/render/yamltokens"
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
	for _, line := range attachFieldDetails(doc, lines, details) {
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
	commentGroupAttr := ""
	if line.CommentGroup != "" {
		commentGroupAttr = ` data-kdoc-comment-group="` + escapeAttr(line.CommentGroup) + `"`
	}

	if _, err := fmt.Fprintf(out, "<div class=\"%s\" role=\"treeitem\" data-kdoc-line%s%s data-index=\"%d\" data-depth=\"%d\" data-path=\"%s\" data-detail-id=\"%s\" data-detail=\"%s\" data-detail-html=\"%s\">",
		classes,
		fieldAttr,
		commentGroupAttr,
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

func attachFieldDetails(doc *crd.Document, lines []tree.Line, details map[string]fielddetail.Field) []htmlLine {
	out := make([]htmlLine, 0, len(lines))
	rootID := rootDescriptionDetailID(doc)
	rootHTML := rootDescriptionDetailHTML(doc)
	for _, line := range lines {
		html := htmlLine{Line: line}
		if detail, ok := lookupFieldDetail(details, line.Path); ok {
			applyFieldDetail(&html, detail)
		} else if line.RootDescription {
			html.DetailID = rootID
			html.DetailHTML = rootHTML
		}
		out = append(out, html)
	}
	return out
}

func rootDescriptionDetailID(doc *crd.Document) string {
	if doc == nil {
		return "root-description"
	}
	return "root-description-" + fielddetail.Slug(apiVersion(doc.Group, doc.Version))
}

func rootDescriptionDetailHTML(doc *crd.Document) string {
	if doc == nil || doc.Schema == nil || strings.TrimSpace(doc.Schema.Description) == "" {
		return ""
	}
	return `<section class="kdoc-detail-section"><h3>Description</h3><p class="kdoc-detail-description">` +
		escape(strings.TrimSpace(doc.Schema.Description)) +
		`</p></section>`
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
	return yamltokens.RenderHTML(line.Text, line.Field != "")
}

func yamlTextClass(line htmlLine) string {
	if yamltokens.Render(line.Text, line.Field != "").Comment == nil {
		return ""
	}
	return " kdoc-yaml-comment-text"
}

func styleElement() string {
	return web.StyleElement()
}

func scriptElement() string {
	return web.ScriptElement()
}
