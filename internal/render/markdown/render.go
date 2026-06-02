package markdownrender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"github.com/sttts/kubectl-doc/internal/crd"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

type Dialect string

const (
	DialectGitHub Dialect = "markdown-github"
	DialectFern   Dialect = "markdown-fern"
)

type Renderer struct {
	Dialect      Dialect
	ExpandDepth  int
	Descriptions yamlrender.DescriptionMode
	Columns      int
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
		if _, err := fmt.Fprintf(out, "---\ntitle: %s\n---\n\n", docs[0].Kind); err != nil {
			return err
		}
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
		if err := r.renderFieldDetails(out, doc, multiple); err != nil {
			return err
		}
	}
	return nil
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

	switch r.dialect() {
	case DialectFern:
		codeTitle := apiVersion(doc.Group, doc.Version) + " " + doc.Kind
		_, err := fmt.Fprintf(out, "<Accordion title=%q defaultOpen={true}>\n\n```yaml title=%q wordWrap showLineNumbers={false}\n%s```\n\n</Accordion>\n", title, codeTitle, yaml.String())
		return err
	default:
		if !multiple {
			if _, err := fmt.Fprint(out, "\n## YAML\n\n"); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(out, "<details open>\n<summary>%s</summary>\n\n```yaml\n%s```\n</details>\n", title, yaml.String())
		return err
	}
}

func (r Renderer) renderFieldDetails(out io.Writer, doc *crd.Document, multiple bool) error {
	fields := collectFieldDetails(doc)
	if len(fields) == 0 {
		return nil
	}

	title := "Field Details"
	if multiple {
		title = "Field details: " + apiVersion(doc.Group, doc.Version)
	}

	switch r.dialect() {
	case DialectFern:
		if _, err := fmt.Fprintf(out, "\n<Accordion title=%q>\n\n", title); err != nil {
			return err
		}
		if err := renderFieldDetailItems(out, fields, 4, r.Columns); err != nil {
			return err
		}
		_, err := fmt.Fprintln(out, "</Accordion>")
		return err
	default:
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
}

func renderFieldDetailItems(out io.Writer, fields []fieldDetail, headingLevel, columns int) error {
	heading := strings.Repeat("#", headingLevel)
	for _, field := range fields {
		if _, err := fmt.Fprintf(out, "<a id=%q></a>\n\n%s `%s`\n\n", field.Anchor, heading, field.Path); err != nil {
			return err
		}
		for _, line := range field.SummaryLines(columns) {
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
	return "`" + value + "`"
}

type fieldDetail struct {
	Path        string
	Anchor      string
	Type        string
	Required    bool
	Description string
	Metadata    []string
}

func (f fieldDetail) SummaryLines(columns int) []string {
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

func collectFieldDetails(doc *crd.Document) []fieldDetail {
	if doc == nil || doc.Schema == nil {
		return nil
	}

	var fields []fieldDetail
	required := requiredSet(doc.Schema)
	for _, name := range sortedProperties(doc.Schema) {
		if name == "apiVersion" || name == "kind" || name == "metadata" {
			continue
		}
		field := doc.Schema.Properties[name]
		collectFieldDetail(doc, &fields, name, &field, required[name])
	}
	return fields
}

func collectFieldDetail(doc *crd.Document, fields *[]fieldDetail, path string, field *docschema.Structural, required bool) {
	*fields = append(*fields, fieldDetail{
		Path:        path,
		Anchor:      fieldAnchor(doc, path),
		Type:        fieldType(field),
		Required:    required,
		Description: strings.TrimSpace(field.Description),
		Metadata:    fieldMetadata(field),
	})

	switch effectiveType(field) {
	case "object":
		childRequired := requiredSet(field)
		for _, name := range sortedProperties(field) {
			child := field.Properties[name]
			collectFieldDetail(doc, fields, path+"."+name, &child, childRequired[name])
		}
		if field.AdditionalProperties != nil && field.AdditionalProperties.Structural != nil {
			collectFieldDetail(doc, fields, path+".<key>", field.AdditionalProperties.Structural, false)
		}
	case "array":
		if field.Items == nil {
			return
		}
		itemPath := path + "[]"
		if effectiveType(field.Items) != "object" || len(field.Items.Properties) == 0 {
			collectFieldDetail(doc, fields, itemPath, field.Items, true)
			return
		}
		itemRequired := requiredSet(field.Items)
		for _, name := range sortedProperties(field.Items) {
			child := field.Items.Properties[name]
			collectFieldDetail(doc, fields, itemPath+"."+name, &child, itemRequired[name])
		}
	}
}

func fieldAnchor(doc *crd.Document, path string) string {
	return "field-" + slug(apiVersion(doc.Group, doc.Version)+"-"+path)
}

func slug(value string) string {
	var out strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func fieldType(field *docschema.Structural) string {
	if field == nil {
		return "object"
	}
	if field.XIntOrString {
		return "int-or-string"
	}
	if field.Type == "array" && field.Items != nil {
		return "array<" + fieldType(field.Items) + ">"
	}
	if field.Type != "" {
		if field.ValueValidation != nil && field.ValueValidation.Format != "" {
			return field.Type + "/" + field.ValueValidation.Format
		}
		return field.Type
	}
	if len(field.Properties) > 0 || field.AdditionalProperties != nil {
		return "object"
	}
	if field.Items != nil {
		return "array<" + fieldType(field.Items) + ">"
	}
	return "object"
}

func fieldMetadata(field *docschema.Structural) []string {
	if field == nil {
		return nil
	}
	var metadata []string
	if field.Default.Object != nil {
		metadata = append(metadata, "default: "+jsonValue(field.Default.Object))
	} else if len(field.Examples) > 0 && field.Examples[0].Value.Object != nil {
		metadata = append(metadata, "example: "+jsonValue(field.Examples[0].Value.Object))
	}
	if field.ValueValidation != nil {
		if len(field.ValueValidation.Enum) > 0 {
			values := make([]string, 0, len(field.ValueValidation.Enum))
			for _, value := range field.ValueValidation.Enum {
				values = append(values, jsonValue(value.Object))
			}
			metadata = append(metadata, "enum: "+strings.Join(values, " | "))
		}
		if field.ValueValidation.MinLength != nil {
			metadata = append(metadata, fmt.Sprintf("minLength: %d", *field.ValueValidation.MinLength))
		}
		if field.ValueValidation.MaxLength != nil {
			metadata = append(metadata, fmt.Sprintf("maxLength: %d", *field.ValueValidation.MaxLength))
		}
		if field.ValueValidation.Minimum != nil {
			metadata = append(metadata, "minimum: "+trimFloat(*field.ValueValidation.Minimum))
		}
		if field.ValueValidation.Maximum != nil {
			metadata = append(metadata, "maximum: "+trimFloat(*field.ValueValidation.Maximum))
		}
		if field.ValueValidation.Pattern != "" {
			metadata = append(metadata, "pattern: "+field.ValueValidation.Pattern)
		}
	}
	if field.Nullable {
		metadata = append(metadata, "nullable")
	}
	if field.XPreserveUnknownFields {
		metadata = append(metadata, "preserveUnknownFields")
	}
	if field.XEmbeddedResource {
		metadata = append(metadata, "embeddedResource")
	}
	if field.XIntOrString {
		metadata = append(metadata, "intOrString")
	}
	if field.XListType != nil {
		metadata = append(metadata, "listType: "+*field.XListType)
	}
	if len(field.XListMapKeys) > 0 {
		metadata = append(metadata, "listMapKeys: "+strings.Join(field.XListMapKeys, ", "))
	}
	if field.XMapType != nil {
		metadata = append(metadata, "mapType: "+*field.XMapType)
	}
	return metadata
}

func jsonValue(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
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

func effectiveType(field *docschema.Structural) string {
	if field == nil {
		return "object"
	}
	if field.XIntOrString {
		return "string"
	}
	if field.Type != "" {
		return field.Type
	}
	if len(field.Properties) > 0 || field.AdditionalProperties != nil {
		return "object"
	}
	if field.Items != nil {
		return "array"
	}
	return "object"
}

func sortedProperties(field *docschema.Structural) []string {
	if field == nil {
		return nil
	}
	names := make([]string, 0, len(field.Properties))
	for name := range field.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func requiredSet(field *docschema.Structural) map[string]bool {
	required := map[string]bool{}
	if field == nil || field.ValueValidation == nil {
		return required
	}
	for _, name := range field.ValueValidation.Required {
		required[name] = true
	}
	return required
}

func trimFloat(value float64) string {
	return fmt.Sprintf("%g", value)
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
