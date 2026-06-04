package markdownrender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/fielddetail"
	"github.com/sttts/kubectl-doc/internal/render/tree"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
)

type Dialect string

const (
	DialectGitHub Dialect = "markdown-github"
	DialectFern   Dialect = "markdown-fern"
)

const fullExpandDepth = 1000

type Renderer struct {
	Dialect           Dialect
	ExpandDepth       int
	Descriptions      yamlrender.DescriptionMode
	Columns           int
	HideFieldDetails  bool
	DisableFiltering  bool
	FernComponentPath string
}

func (r Renderer) Render(out io.Writer, doc *crd.Document) error {
	return r.RenderAll(out, []*crd.Document{doc})
}

func (r Renderer) RenderAll(out io.Writer, docs []*crd.Document) error {
	docs = compactDocuments(docs)
	if len(docs) == 0 {
		return fmt.Errorf("at least one document is required")
	}

	dialect := r.dialect()
	if dialect == DialectFern {
		return r.renderFernAll(out, docs)
	}

	if _, err := fmt.Fprintf(out, "# %s\n\n", docs[0].Kind); err != nil {
		return err
	}
	if err := renderMetadata(out, docs); err != nil {
		return err
	}

	multiple := len(docs) > 1
	for i, doc := range docs {
		if i > 0 {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		if multiple {
			if _, err := fmt.Fprintf(out, "\n## %s\n\n", apiVersion(doc.Group, doc.Version)); err != nil {
				return err
			}
		}
		if err := r.renderYAML(out, doc, multiple); err != nil {
			return err
		}
		if !r.HideFieldDetails {
			if err := r.renderFieldDetails(out, doc, multiple); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r Renderer) renderFernAll(out io.Writer, docs []*crd.Document) error {
	if _, err := fmt.Fprintf(out, "---\ntitle: %s\n---\n\n", yamlString(docs[0].Kind)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "import { KubeSchemaDoc } from %s;\n\n", jsonString(r.fernComponentPath())); err != nil {
		return err
	}
	payloads := make([]fernDocumentPayload, 0, len(docs))
	for _, doc := range docs {
		payloads = append(payloads, r.fernPayload(doc))
	}
	if _, err := fmt.Fprintf(out, "export const kubectlDocSchemas = %s;\n\n", jsonBlock(payloads)); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(out, "# %s\n\n", escapeMDXText(docs[0].Kind)); err != nil {
		return err
	}
	if err := renderFernMetadata(out, docs); err != nil {
		return err
	}

	if len(docs) == 1 {
		return r.renderFernDocument(out, docs[0], 0, false)
	}
	if _, err := fmt.Fprintln(out, "\n<Tabs>"); err != nil {
		return err
	}
	for i, doc := range docs {
		if _, err := fmt.Fprintf(out, "  <Tab title={%s}>\n\n", jsonString(apiVersion(doc.Group, doc.Version))); err != nil {
			return err
		}
		if err := r.renderFernDocument(out, doc, i, true); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "  </Tab>"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, "</Tabs>")
	return err
}

func (r Renderer) renderFernDocument(out io.Writer, doc *crd.Document, payloadIndex int, multiple bool) error {
	if multiple {
		if _, err := fmt.Fprintf(out, "  ## %s\n\n", escapeMDXText(apiVersion(doc.Group, doc.Version))); err != nil {
			return err
		}
	}
	indent := ""
	if multiple {
		indent = "  "
	}
	if _, err := fmt.Fprintf(out, "%s<Accordion title={%s} defaultOpen={true}>\n\n", indent, jsonString("YAML")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "%s  <KubeSchemaDoc data={kubectlDocSchemas[%d]} filtering={%t} />\n\n", indent, payloadIndex, !r.DisableFiltering); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "%s</Accordion>\n", indent); err != nil {
		return err
	}
	if r.HideFieldDetails {
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
		return nil
	}
	return r.renderFernFieldDetails(out, doc, multiple)
}

func (r Renderer) renderFernFieldDetails(out io.Writer, doc *crd.Document, multiple bool) error {
	fields := fielddetail.Collect(doc)
	if len(fields) == 0 {
		return nil
	}

	title := "Field Details"
	if multiple {
		title = "Field details: " + apiVersion(doc.Group, doc.Version)
	}
	indent := ""
	if multiple {
		indent = "  "
	}
	if _, err := fmt.Fprintf(out, "\n%s<Accordion title={%s}>\n\n", indent, jsonString(title)); err != nil {
		return err
	}
	for _, field := range fields {
		if err := renderFernParamField(out, field, r.Columns, indent+"  "); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(out, "%s</Accordion>\n\n", indent)
	return err
}

func (r Renderer) renderYAML(out io.Writer, doc *crd.Document, multiple bool) error {
	var yaml bytes.Buffer
	if err := (yamlrender.Renderer{
		ExpandDepth:  r.ExpandDepth,
		Descriptions: r.Descriptions,
		Columns:      r.Columns,
	}).Render(&yaml, doc); err != nil {
		return err
	}

	title := "YAML"
	if multiple {
		title = "YAML: " + apiVersion(doc.Group, doc.Version)
	}

	if !multiple {
		if _, err := fmt.Fprint(out, "\n## YAML\n\n"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(out, "<details open>\n<summary>%s</summary>\n\n```yaml\n%s```\n</details>\n", title, yaml.String())
	return err
}

func (r Renderer) renderFieldDetails(out io.Writer, doc *crd.Document, multiple bool) error {
	fields := fielddetail.Collect(doc)
	if len(fields) == 0 {
		return nil
	}

	title := "Field Details"
	if multiple {
		title = "Field details: " + apiVersion(doc.Group, doc.Version)
	}

	heading := "##"
	fieldHeadingLevel := 3
	if multiple {
		heading = "###"
		fieldHeadingLevel = 4
	}
	if _, err := fmt.Fprintf(out, "\n%s %s\n\n", heading, title); err != nil {
		return err
	}
	return renderFieldDetailItems(out, fields, fieldHeadingLevel, r.Columns)
}

func renderFieldDetailItems(out io.Writer, fields []fielddetail.Field, headingLevel, columns int) error {
	heading := strings.Repeat("#", headingLevel)
	for _, field := range fields {
		if _, err := fmt.Fprintf(out, "<a id=%q></a>\n\n%s `%s`\n\n", field.ID, heading, field.Path); err != nil {
			return err
		}
		for _, line := range fieldSummaryLines(field, columns) {
			if _, err := fmt.Fprintln(out, line); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}
	return nil
}

func renderMetadata(out io.Writer, docs []*crd.Document) error {
	doc := docs[0]
	rows := []metadataRow{
		{"Kind", codeSpan(doc.Kind)},
	}
	if len(docs) == 1 {
		rows = append([]metadataRow{{"API Version", codeSpan(apiVersion(doc.Group, doc.Version))}}, rows...)
	} else {
		rows = append(rows, metadataRow{"Versions", formatVersionList(docs)})
	}
	if doc.Plural != "" {
		rows = append(rows, metadataRow{"Resource", codeSpan(doc.Plural)})
	}

	if _, err := fmt.Fprintln(out, "| Field | Value |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "| --- | --- |"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(out, "| %s | %s |\n", row.Field, row.Value); err != nil {
			return err
		}
	}
	return nil
}

type metadataRow struct {
	Field string
	Value string
}

func renderFernMetadata(out io.Writer, docs []*crd.Document) error {
	doc := docs[0]
	rows := []metadataRow{
		{"Kind", codeSpan(doc.Kind)},
	}
	if len(docs) == 1 {
		rows = append([]metadataRow{{"API Version", codeSpan(apiVersion(doc.Group, doc.Version))}}, rows...)
	} else {
		rows = append(rows, metadataRow{"Versions", formatVersionList(docs)})
	}
	if doc.Plural != "" {
		rows = append(rows, metadataRow{"Resource", codeSpan(doc.Plural)})
	}

	if _, err := fmt.Fprintln(out, "| Field | Value |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "| --- | --- |"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(out, "| %s | %s |\n", escapeMDXText(row.Field), row.Value); err != nil {
			return err
		}
	}
	return nil
}

func (r Renderer) dialect() Dialect {
	if r.Dialect == "" {
		return DialectGitHub
	}
	return r.Dialect
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

func formatVersionList(docs []*crd.Document) string {
	versions := make([]string, 0, len(docs))
	for _, doc := range docs {
		versions = append(versions, codeSpan(apiVersion(doc.Group, doc.Version)))
	}
	return strings.Join(versions, ", ")
}

func codeSpan(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "\\`") + "`"
}

func fieldSummaryLines(f fielddetail.Field, columns int) []string {
	var lines []string
	lines = append(lines, "- Type: `"+f.Type+"`")
	lines = append(lines, "- Required: `"+yesNo(f.Required)+"`")
	if f.Description != "" {
		lines = append(lines, wrapMarkdownParagraph("- Description: ", f.Description, columns)...)
	}
	if len(f.Metadata) > 0 {
		lines = append(lines, "- Metadata: `"+strings.Join(f.Metadata, "`, `")+"`")
	}
	return lines
}

func wrapMarkdownParagraph(prefix, text string, columns int) []string {
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) == 0 {
		return nil
	}
	if columns <= 0 || len(prefix) >= columns {
		return []string{prefix + strings.Join(words, " ")}
	}

	width := columns - len(prefix)
	var lines []string
	var line strings.Builder
	for _, word := range words {
		if line.Len() == 0 {
			line.WriteString(word)
			continue
		}
		if line.Len()+1+len(word) > width {
			lines = append(lines, prefix+line.String())
			prefix = "  "
			width = columns - len(prefix)
			line.Reset()
			line.WriteString(word)
			continue
		}
		line.WriteByte(' ')
		line.WriteString(word)
	}
	if line.Len() > 0 {
		lines = append(lines, prefix+line.String())
	}
	return lines
}

type fernDocumentPayload struct {
	APIVersion string             `json:"apiVersion"`
	Group      string             `json:"group"`
	Version    string             `json:"version"`
	Kind       string             `json:"kind"`
	Resource   string             `json:"resource,omitempty"`
	Lines      []fernLinePayload  `json:"lines"`
	Fields     []fernFieldPayload `json:"fields"`
}

type fernLinePayload struct {
	Index       int    `json:"index"`
	Text        string `json:"text"`
	Description string `json:"description,omitempty"`
	Depth       int    `json:"depth"`
	Field       string `json:"field,omitempty"`
	Path        string `json:"path,omitempty"`
	Code        bool   `json:"code,omitempty"`
	Metadata    bool   `json:"metadata,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Foldable    bool   `json:"foldable,omitempty"`
	Collapsed   bool   `json:"collapsed,omitempty"`
	DetailID    string `json:"detailId,omitempty"`
	FilterText  string `json:"filterText,omitempty"`
}

type fernFieldPayload struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Description string   `json:"description,omitempty"`
	Metadata    []string `json:"metadata,omitempty"`
}

func (r Renderer) fernPayload(doc *crd.Document) fernDocumentPayload {
	lines := tree.WithCollapsed(tree.Build(doc, tree.Options{
		ExpandDepth:    fullExpandDepth,
		Descriptions:   tree.DescriptionMode(r.Descriptions),
		Columns:        r.Columns,
		RenderStatus:   true,
		RenderMetadata: true,
	}), r.initialExpandDepth())
	details := fielddetail.ByPath(doc)

	payload := fernDocumentPayload{
		APIVersion: apiVersion(doc.Group, doc.Version),
		Group:      doc.Group,
		Version:    doc.Version,
		Kind:       doc.Kind,
		Resource:   doc.Plural,
		Fields:     fernFieldPayloads(fielddetail.Collect(doc)),
	}
	for _, line := range lines {
		payload.Lines = append(payload.Lines, r.fernLinePayload(line, details))
	}
	return payload
}

func (r Renderer) fernLinePayload(line tree.Line, details map[string]fielddetail.Field) fernLinePayload {
	payload := fernLinePayload{
		Index:       line.Index,
		Text:        line.Text,
		Description: line.Description,
		Depth:       line.Depth,
		Field:       line.Field,
		Path:        line.Path,
		Code:        line.Code,
		Metadata:    line.Metadata,
		Required:    line.Required,
		Foldable:    line.Foldable,
		Collapsed:   line.Collapsed,
	}
	if detail, ok := details[line.Path]; ok {
		payload.DetailID = detail.ID
		if !r.DisableFiltering && line.Field != "" {
			payload.FilterText = line.Field + "\n" + detail.Description
		}
	} else {
		payload.DetailID = fmt.Sprintf("line-%d", line.Index)
	}
	return payload
}

func fernFieldPayloads(fields []fielddetail.Field) []fernFieldPayload {
	payloads := make([]fernFieldPayload, 0, len(fields))
	for _, field := range fields {
		payloads = append(payloads, fernFieldPayload{
			ID:          field.ID,
			Path:        field.Path,
			Type:        field.Type,
			Required:    field.Required,
			Description: field.Description,
			Metadata:    field.Metadata,
		})
	}
	return payloads
}

func (r Renderer) initialExpandDepth() int {
	if r.ExpandDepth < 0 {
		return 0
	}
	return r.ExpandDepth
}

func (r Renderer) fernComponentPath() string {
	if r.FernComponentPath != "" {
		return r.FernComponentPath
	}
	return "@/components/kubectl-doc/KubeSchemaDoc"
}

func renderFernParamField(out io.Writer, field fielddetail.Field, columns int, indent string) error {
	attrs := []string{
		"path={" + jsonString(field.Path) + "}",
		"type={" + jsonString(field.Type) + "}",
	}
	if field.Required {
		attrs = append(attrs, "required={true}")
	}
	if value, ok := firstMetadataValue(field.Metadata, "default: "); ok {
		attrs = append(attrs, "default={"+jsonString(value)+"}")
	}
	if _, err := fmt.Fprintf(out, "%s<ParamField %s>\n", indent, strings.Join(attrs, " ")); err != nil {
		return err
	}
	for _, line := range wrapMDXBody(field.Description, columns) {
		if _, err := fmt.Fprintf(out, "%s  %s\n", indent, line); err != nil {
			return err
		}
	}
	if len(field.Metadata) > 0 {
		if field.Description != "" {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		for _, item := range field.Metadata {
			if _, err := fmt.Fprintf(out, "%s  - %s\n", indent, escapeMDXText(item)); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(out, "%s</ParamField>\n\n", indent)
	return err
}

func firstMetadataValue(metadata []string, prefix string) (string, bool) {
	for _, item := range metadata {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix), true
		}
	}
	return "", false
}

func wrapMDXBody(text string, columns int) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lines := wrapMarkdownParagraph("", text, columns)
	for i := range lines {
		lines[i] = escapeMDXText(lines[i])
	}
	return lines
}

func jsonBlock(value interface{}) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "null"
	}
	return string(data)
}

func jsonString(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(data)
}

func yamlString(value string) string {
	return jsonString(value)
}

func escapeMDXText(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `{`, `\{`)
	value = strings.ReplaceAll(value, `}`, `\}`)
	value = strings.ReplaceAll(value, `<`, `&lt;`)
	value = strings.ReplaceAll(value, `>`, `&gt;`)
	return value
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
