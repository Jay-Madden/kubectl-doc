package htmlrender

import (
	"bytes"
	"encoding/json"
	"fmt"
	htmlpkg "html"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/sttts/kubectl-doc/internal/crd"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

const fullExpandDepth = 1000

type Renderer struct {
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

	if _, err := fmt.Fprintf(out, "<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<title>%s</title>\n%s\n</head>\n<body>\n", escape(docs[0].Kind), styleElement()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "<main class=\"kubectl-doc\" data-kubectl-doc>\n<header class=\"kdoc-header\">\n<h1>%s</h1>\n", escape(docs[0].Kind)); err != nil {
		return err
	}
	if err := renderMetadata(out, docs); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "<div class=\"kdoc-search\"><input type=\"search\" aria-label=\"Search\" placeholder=\"Search\" data-kdoc-search></div>\n</header>"); err != nil {
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

	if _, err := fmt.Fprintln(out, "</section><aside class=\"kdoc-details\" data-kdoc-details aria-live=\"polite\"><h2>Details</h2><div class=\"kdoc-detail-body\" data-kdoc-detail-body><p class=\"kdoc-detail-empty\">Select a field.</p></div></aside></div>"); err != nil {
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

	var yaml bytes.Buffer
	if err := (yamlrender.Renderer{
		ExpandDepth:  fullExpandDepth,
		Descriptions: r.Descriptions,
		Columns:      r.Columns,
		RenderStatus: true,
	}).Render(&yaml, doc); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(out, "<div class=\"kdoc-tree\" role=\"tree\" aria-label=\"%s\">\n", escape(title)); err != nil {
		return err
	}
	details := collectFieldDetails(doc)
	for _, line := range buildLines(yaml.String(), r.initialExpandDepth(), details) {
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

func renderLine(out io.Writer, line yamlLine) error {
	classes := "kdoc-line"
	if strings.TrimSpace(line.Text) == "" {
		classes += " kdoc-blank"
	}
	detailID := line.DetailID
	if detailID == "" {
		detailID = "line-" + strconv.Itoa(line.Index)
	}

	if _, err := fmt.Fprintf(out, "<div class=\"%s\" role=\"treeitem\" data-kdoc-line data-index=\"%d\" data-depth=\"%d\" data-search=\"%s\" data-field=\"%s\" data-path=\"%s\" data-detail-id=\"%s\" data-detail=\"%s\" data-detail-html=\"%s\">",
		classes,
		line.Index,
		line.Depth,
		escapeAttr(strings.ToLower(line.SearchText)),
		escapeAttr(strings.ToLower(line.Field)),
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
	if _, err := fmt.Fprintf(out, "<span class=\"kdoc-yaml-text\">%s</span></div>\n", renderYAMLText(line.Text)); err != nil {
		return err
	}
	return nil
}

type yamlLine struct {
	Index      int
	Text       string
	Depth      int
	Foldable   bool
	Collapsed  bool
	Field      string
	Path       string
	Required   bool
	DetailID   string
	Detail     string
	DetailHTML string
	SearchText string
}

func buildLines(rendered string, expandDepth int, details map[string]fieldDetail) []yamlLine {
	rawLines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	lines := make([]yamlLine, 0, len(rawLines))
	paths := map[int]string{}
	for i, raw := range rawLines {
		depth := lineDepth(raw)
		field := fieldName(raw)
		path := ""
		if field != "" {
			paths[depth] = field
			for existingDepth := range paths {
				if existingDepth > depth {
					delete(paths, existingDepth)
				}
			}
			path = joinPath(paths, depth)
		}
		lines = append(lines, yamlLine{
			Index:      i,
			Text:       raw,
			Depth:      depth,
			Field:      field,
			Path:       path,
			SearchText: raw,
		})
	}

	for i := range lines {
		if lines[i].Path == "" {
			continue
		}
		detail, ok := lookupFieldDetail(details, lines[i].Path)
		if !ok {
			continue
		}
		applyFieldDetail(&lines[i], detail)
		for j := i - 1; j >= 0; j-- {
			if !isDescriptionForField(lines[j], lines[i]) {
				break
			}
			applyFieldDetail(&lines[j], detail)
		}
	}

	for i := range lines {
		if strings.TrimSpace(lines[i].Text) == "" {
			continue
		}
		nextDepth, ok := nextContentDepth(lines, i)
		lines[i].Foldable = ok && nextDepth > lines[i].Depth
		lines[i].Collapsed = lines[i].Foldable && lines[i].Depth >= expandDepth
		if lines[i].Foldable && lines[i].Path == "status" {
			lines[i].Collapsed = true
		}
	}
	return lines
}

func applyFieldDetail(line *yamlLine, detail fieldDetail) {
	line.Path = detail.Path
	line.Required = detail.Required
	line.DetailID = detail.ID
	line.Detail = detail.Text()
	line.DetailHTML = detail.HTML()
	line.SearchText = strings.Join([]string{line.Text, detail.SearchText()}, " ")
}

func lookupFieldDetail(details map[string]fieldDetail, path string) (fieldDetail, bool) {
	if detail, ok := details[path]; ok {
		return detail, true
	}
	return fieldDetail{}, false
}

func isDescriptionForField(comment, field yamlLine) bool {
	if comment.Depth == field.Depth && isPlainDescriptionComment(comment.Text) {
		return true
	}
	if comment.Depth == field.Depth-1 && (isListDescriptionComment(comment.Text) || isCommentedListDescriptionComment(comment.Text)) {
		return true
	}
	return false
}

func isPlainDescriptionComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "# ") {
		return false
	}
	return fieldName(trimmed) == ""
}

func isListDescriptionComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "- # ") {
		return false
	}
	return fieldName(trimmed) == ""
}

func isCommentedListDescriptionComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "# - # ") {
		return false
	}
	return fieldName(trimmed) == ""
}

func lineDepth(line string) int {
	spaces := len(line) - len(strings.TrimLeft(line, " "))
	return spaces / 2
}

func nextContentDepth(lines []yamlLine, index int) (int, bool) {
	for i := index + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i].Text) == "" {
			continue
		}
		return lines[i].Depth, true
	}
	return 0, false
}

func fieldName(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "- ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	}
	if strings.HasPrefix(trimmed, "# ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
	}
	if strings.HasPrefix(trimmed, "- ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	}
	commentIndex := strings.Index(trimmed, " # ")
	if commentIndex >= 0 {
		trimmed = trimmed[:commentIndex]
	}
	colon := strings.Index(trimmed, ":")
	if colon <= 0 {
		return ""
	}
	key := strings.TrimSpace(trimmed[:colon])
	if key == "" || strings.ContainsAny(key, " \t{}[]") {
		return ""
	}
	return strings.Trim(key, `"'`)
}

func joinPath(paths map[int]string, depth int) string {
	parts := make([]string, 0, depth+1)
	for i := 0; i <= depth; i++ {
		if part := paths[i]; part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ".")
}

type fieldDetail struct {
	ID          string
	Path        string
	Type        string
	Required    bool
	Description string
	Metadata    []string
}

func (f fieldDetail) Text() string {
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

func (f fieldDetail) HTML() string {
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

func (f fieldDetail) SearchText() string {
	return f.Description
}

func collectFieldDetails(doc *crd.Document) map[string]fieldDetail {
	if doc == nil || doc.Schema == nil {
		return nil
	}

	fields := map[string]fieldDetail{}
	required := requiredSet(doc.Schema)
	for _, name := range sortedProperties(doc.Schema) {
		if name == "apiVersion" || name == "kind" || name == "metadata" {
			continue
		}
		field := doc.Schema.Properties[name]
		collectFieldDetail(doc, fields, name, &field, required[name])
	}
	return fields
}

func collectFieldDetail(doc *crd.Document, fields map[string]fieldDetail, path string, field *docschema.Structural, required bool) {
	detail := fieldDetail{
		ID:          fieldID(doc, path),
		Path:        path,
		Type:        fieldType(field),
		Required:    required,
		Description: strings.TrimSpace(field.Description),
		Metadata:    fieldMetadata(field),
	}
	addFieldDetail(fields, detail)

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

func addFieldDetail(fields map[string]fieldDetail, detail fieldDetail) {
	fields[detail.Path] = detail
	if strings.Contains(detail.Path, "[]") {
		fields[strings.ReplaceAll(detail.Path, "[]", "")] = detail
	}
}

func fieldID(doc *crd.Document, path string) string {
	return "field-" + slug(apiVersion(doc.Group, doc.Version)+"-"+path)
}

func slug(value string) string {
	var out strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
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
	} else {
		for _, example := range field.Examples {
			if example.Value.Object == nil {
				continue
			}
			label := "example"
			if example.Name != "" {
				label += " " + example.Name
			}
			metadata = append(metadata, label+": "+jsonValue(example.Value.Object))
		}
	}
	if field.ValueValidation != nil {
		metadata = append(metadata, validationMetadata(field.ValueValidation)...)
	}
	if field.Nullable {
		metadata = append(metadata, "nullable")
	}
	if field.XPreserveUnknownFields {
		metadata = append(metadata, "x-kubernetes-preserve-unknown-fields")
	}
	if field.XEmbeddedResource {
		metadata = append(metadata, "x-kubernetes-embedded-resource")
	}
	if field.XIntOrString {
		metadata = append(metadata, "x-kubernetes-int-or-string")
	}
	if field.XListType != nil {
		metadata = append(metadata, "x-kubernetes-list-type: "+*field.XListType)
	}
	if len(field.XListMapKeys) > 0 {
		metadata = append(metadata, "x-kubernetes-list-map-keys: "+strings.Join(field.XListMapKeys, ", "))
	}
	if field.XMapType != nil {
		metadata = append(metadata, "x-kubernetes-map-type: "+*field.XMapType)
	}
	for i, rule := range field.XValidations {
		prefix := fmt.Sprintf("x-kubernetes-validations[%d]", i)
		if rule.Rule != "" {
			metadata = append(metadata, prefix+".rule: "+rule.Rule)
		}
		if rule.Message != "" {
			metadata = append(metadata, prefix+".message: "+rule.Message)
		}
		if rule.MessageExpression != "" {
			metadata = append(metadata, prefix+".messageExpression: "+rule.MessageExpression)
		}
		if rule.Reason != nil {
			metadata = append(metadata, prefix+".reason: "+*rule.Reason)
		}
		if rule.FieldPath != "" {
			metadata = append(metadata, prefix+".fieldPath: "+rule.FieldPath)
		}
		if rule.OptionalOldSelf != nil {
			metadata = append(metadata, fmt.Sprintf("%s.optionalOldSelf: %t", prefix, *rule.OptionalOldSelf))
		}
	}
	return metadata
}

func validationMetadata(validation *docschema.ValueValidation) []string {
	var metadata []string
	if validation.Format != "" {
		metadata = append(metadata, "format: "+validation.Format)
	}
	if len(validation.Enum) > 0 {
		values := make([]string, 0, len(validation.Enum))
		for _, value := range validation.Enum {
			values = append(values, jsonValue(value.Object))
		}
		metadata = append(metadata, "enum: "+strings.Join(values, " | "))
	}
	if validation.MinLength != nil {
		metadata = append(metadata, fmt.Sprintf("minLength: %d", *validation.MinLength))
	}
	if validation.MaxLength != nil {
		metadata = append(metadata, fmt.Sprintf("maxLength: %d", *validation.MaxLength))
	}
	if validation.Minimum != nil {
		metadata = append(metadata, "minimum: "+trimFloat(*validation.Minimum))
	}
	if validation.ExclusiveMinimum {
		metadata = append(metadata, "exclusiveMinimum")
	}
	if validation.Maximum != nil {
		metadata = append(metadata, "maximum: "+trimFloat(*validation.Maximum))
	}
	if validation.ExclusiveMaximum {
		metadata = append(metadata, "exclusiveMaximum")
	}
	if validation.Pattern != "" {
		metadata = append(metadata, "pattern: "+validation.Pattern)
	}
	if validation.MinItems != nil {
		metadata = append(metadata, fmt.Sprintf("minItems: %d", *validation.MinItems))
	}
	if validation.MaxItems != nil {
		metadata = append(metadata, fmt.Sprintf("maxItems: %d", *validation.MaxItems))
	}
	if validation.UniqueItems {
		metadata = append(metadata, "uniqueItems")
	}
	if validation.MultipleOf != nil {
		metadata = append(metadata, "multipleOf: "+trimFloat(*validation.MultipleOf))
	}
	if validation.MinProperties != nil {
		metadata = append(metadata, fmt.Sprintf("minProperties: %d", *validation.MinProperties))
	}
	if validation.MaxProperties != nil {
		metadata = append(metadata, fmt.Sprintf("maxProperties: %d", *validation.MaxProperties))
	}
	if len(validation.Required) > 0 {
		metadata = append(metadata, "requiredFields: "+strings.Join(validation.Required, ", "))
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
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func renderMetadata(out io.Writer, docs []*crd.Document) error {
	doc := docs[0]
	if _, err := fmt.Fprintln(out, "<table class=\"kdoc-metadata\"><tbody>"); err != nil {
		return err
	}
	if len(docs) == 1 {
		if err := metadataRow(out, "API Version", apiVersion(doc.Group, doc.Version)); err != nil {
			return err
		}
	} else if err := metadataRow(out, "Versions", versionList(docs)); err != nil {
		return err
	}
	if err := metadataRow(out, "Kind", doc.Kind); err != nil {
		return err
	}
	if doc.Plural != "" {
		if err := metadataRow(out, "Resource", doc.Plural); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, "</tbody></table>")
	return err
}

func metadataRow(out io.Writer, label, value string) error {
	_, err := fmt.Fprintf(out, "<tr><th>%s</th><td><code>%s</code></td></tr>\n", escape(label), escape(value))
	return err
}

func versionList(docs []*crd.Document) string {
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

func renderYAMLText(line string) string {
	indentLength := len(line) - len(strings.TrimLeft(line, " "))
	indent := line[:indentLength]
	rest := line[indentLength:]
	if rest == "" {
		return escape(indent)
	}
	if strings.HasPrefix(rest, "# ") {
		content := strings.TrimPrefix(rest, "# ")
		if fieldName(rest) != "" {
			return escape(indent) + span("kdoc-yaml-comment", "# ") + renderYAMLCode(content)
		}
		return escape(indent) + span("kdoc-yaml-comment", rest)
	}
	return escape(indent) + renderYAMLCode(rest)
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
	const requiredLabel = "# Required"
	index := strings.Index(comment, requiredLabel)
	if index < 0 {
		return span("kdoc-yaml-comment", comment)
	}
	var out strings.Builder
	if prefix := comment[:index]; prefix != "" {
		out.WriteString(span("kdoc-yaml-comment", prefix))
	}
	out.WriteString(span("kdoc-required-label", requiredLabel))
	if suffix := comment[index+len(requiredLabel):]; suffix != "" {
		out.WriteString(span("kdoc-yaml-comment", suffix))
	}
	return out.String()
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
		return span("kdoc-yaml-placeholder", token)
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
.kubectl-doc{--kdoc-fg:#1f2933;--kdoc-muted:#57606a;--kdoc-border:#d8dee4;--kdoc-panel:#f6f8fa;--kdoc-selected:#fff7cc;--kdoc-current:#111;--kdoc-match-bg:#ff8c00;--kdoc-match-fg:#111;--kdoc-required:#cf222e;--kdoc-ok:#116329;--kdoc-yaml-key:#0550ae;--kdoc-yaml-string:#0a7f42;--kdoc-yaml-comment:#6e7781;--kdoc-yaml-punct:#8c959f;--kdoc-yaml-number:#953800;--kdoc-yaml-bool:#8250df;--kdoc-yaml-null:#8250df;--kdoc-yaml-placeholder:#cf222e;box-sizing:border-box;color:var(--kdoc-fg);background:#fff;font:14px/1.45 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;max-width:100%;padding:24px}
.kubectl-doc *{box-sizing:border-box}
.kdoc-header{border-bottom:1px solid var(--kdoc-border);margin-bottom:16px;padding-bottom:16px}
.kdoc-header h1{font-size:24px;line-height:1.2;margin:0 0 12px}
.kdoc-metadata{border-collapse:collapse;margin:0 0 12px}
.kdoc-metadata th{color:var(--kdoc-muted);font-weight:600;padding:2px 16px 2px 0;text-align:left}
.kdoc-metadata td{padding:2px 0}
.kdoc-search input{border:1px solid #afb8c1;border-radius:6px;font:inherit;max-width:360px;padding:6px 8px;width:100%}
.kdoc-layout{display:grid;gap:16px;grid-template-columns:minmax(0,1fr) minmax(240px,320px)}
.kdoc-docs{min-width:0}
.kdoc-version h2{font-size:18px;margin:16px 0 8px}
.kdoc-tree{background:var(--kdoc-panel);border:1px solid var(--kdoc-border);border-radius:8px;overflow:auto;padding:10px 0}
.kdoc-line{align-items:baseline;display:flex;font:13px/1.32 ui-monospace,SFMono-Regular,SFMono,Consolas,"Liberation Mono",Menlo,monospace;margin:0;min-height:17px;padding:0 12px;white-space:pre}
.kdoc-line[hidden]{display:none}
.kdoc-fold,.kdoc-gutter{background:transparent;border:0;color:var(--kdoc-muted);flex:0 0 24px;font:inherit;height:17px;margin:0;padding:0;text-align:left;user-select:none}
.kdoc-fold{cursor:pointer}
.kdoc-fold::before{content:"▶"}
.kdoc-fold[aria-expanded="true"]::before{content:"▼"}
.kdoc-yaml-text{white-space:pre}
.kdoc-yaml-key{color:var(--kdoc-yaml-key);font-weight:600}
.kdoc-yaml-string{color:var(--kdoc-yaml-string)}
.kdoc-yaml-comment{color:var(--kdoc-yaml-comment)}
.kdoc-yaml-punct{color:var(--kdoc-yaml-punct)}
.kdoc-yaml-number{color:var(--kdoc-yaml-number)}
.kdoc-yaml-bool,.kdoc-yaml-null{color:var(--kdoc-yaml-bool)}
.kdoc-yaml-placeholder{color:var(--kdoc-yaml-placeholder)}
.kdoc-required-label{color:var(--kdoc-required);font-weight:700}
.kdoc-search-hit{background:var(--kdoc-match-bg);border-radius:2px;color:var(--kdoc-match-fg);padding:1px 0}
.kdoc-current .kdoc-yaml-text{box-shadow:inset 3px 0 0 var(--kdoc-current);padding-left:4px}
.kdoc-selected .kdoc-yaml-text{background:var(--kdoc-selected)}
.kdoc-details{border:1px solid var(--kdoc-border);border-radius:8px;font:13px/1.45 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;min-width:0;padding:12px;position:sticky;top:12px}
.kdoc-details h2{font-size:16px;line-height:1.25;margin:0 0 10px}
.kdoc-detail-body{display:grid;gap:12px}
.kdoc-detail-empty{color:var(--kdoc-muted);margin:0}
.kdoc-detail-grid{display:grid;gap:7px;margin:0}
.kdoc-detail-row{align-items:center;display:grid;gap:8px;grid-template-columns:72px minmax(0,1fr)}
.kdoc-detail-row dt{color:var(--kdoc-muted);font-size:11px;font-weight:700;letter-spacing:.02em;line-height:1.35;text-transform:uppercase}
.kdoc-detail-row dd{line-height:1.35;margin:0;min-width:0}
.kdoc-detail-code,.kdoc-detail-list code{font:12px/1.35 ui-monospace,SFMono-Regular,SFMono,Consolas,"Liberation Mono",Menlo,monospace;overflow-wrap:anywhere}
.kdoc-detail-badge{background:#eaeef2;border:1px solid var(--kdoc-border);border-radius:999px;color:#24292f;display:inline-block;font-size:12px;font-weight:600;line-height:1;padding:3px 7px}
.kdoc-detail-badge-required{background:#ffebe9;border-color:#ff8182;color:var(--kdoc-required)}
.kdoc-detail-badge-optional{background:#dafbe1;border-color:#aceebb;color:var(--kdoc-ok)}
.kdoc-detail-section{border-top:1px solid var(--kdoc-border);padding-top:10px}
.kdoc-detail-section h3{color:var(--kdoc-muted);font-size:11px;letter-spacing:.02em;margin:0 0 6px;text-transform:uppercase}
.kdoc-detail-description{margin:0}
.kdoc-detail-list{display:grid;gap:4px;margin:0;padding-left:18px}
@media(max-width:900px){.kubectl-doc{padding:16px}.kdoc-layout{grid-template-columns:1fr}.kdoc-details{position:static}}
</style>`
}

func scriptElement() string {
	return `<script>
(function(){
  function ready(fn){ if(document.readyState !== "loading"){ fn(); } else { document.addEventListener("DOMContentLoaded", fn); } }
  ready(function(){
    document.querySelectorAll("[data-kubectl-doc]").forEach(function(root){
      var lines = Array.prototype.slice.call(root.querySelectorAll("[data-kdoc-line]"));
      var search = root.querySelector("[data-kdoc-search]");
      var details = root.querySelector("[data-kdoc-detail-body]");
      var results = [];
      var current = -1;

      function button(line){ return line.querySelector("[data-kdoc-toggle]"); }
      function depth(line){ return Number(line.getAttribute("data-depth") || "0"); }
      function expanded(line){ var b = button(line); return !b || b.getAttribute("aria-expanded") !== "false"; }
      function setExpanded(line, value){
        var b = button(line);
        if(!b){ return; }
        b.setAttribute("aria-expanded", value ? "true" : "false");
      }
      function applyFolds(){
        lines.forEach(function(line){ line.hidden = false; });
        lines.forEach(function(line, index){
          if(line.hidden || expanded(line)){ return; }
          var parentDepth = depth(line);
          for(var i = index + 1; i < lines.length; i++){
            if(depth(lines[i]) <= parentDepth && lines[i].textContent.trim() !== ""){ break; }
            lines[i].hidden = true;
          }
        });
      }
      function reveal(line){
        var index = lines.indexOf(line);
        var targetDepth = depth(line);
        for(var i = index - 1; i >= 0; i--){
          var candidateDepth = depth(lines[i]);
          if(candidateDepth < targetDepth){
            setExpanded(lines[i], true);
            targetDepth = candidateDepth;
          }
        }
        applyFolds();
      }
      function groupedLines(line){
        var id = line.getAttribute("data-detail-id");
        if(!id){ return [line]; }
        return lines.filter(function(item){ return item.getAttribute("data-detail-id") === id; });
      }
      function cleanLineText(line){
        var text = line.querySelector(".kdoc-yaml-text").textContent.trim();
        if(text.indexOf("# ") === 0){ text = text.slice(2).trim(); }
        return text;
      }
      function escapeHTML(value){
        return String(value || "").replace(/[&<>"']/g, function(ch){
          return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[ch];
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
      function select(line){
        lines.forEach(function(item){ item.classList.remove("kdoc-selected"); });
        groupedLines(line).forEach(function(item){ item.classList.add("kdoc-selected"); });
        showDetails(line);
      }
      function clearSearchHighlights(){
        root.querySelectorAll(".kdoc-search-hit").forEach(function(hit){
          hit.replaceWith(document.createTextNode(hit.textContent));
        });
      }
      function highlightLine(line, query){
        if(!query){ return; }
        var text = line.querySelector(".kdoc-yaml-text");
        if(!text){ return; }
        var nodes = [];
        var walker = document.createTreeWalker(text, NodeFilter.SHOW_TEXT);
        while(walker.nextNode()){ nodes.push(walker.currentNode); }
        nodes.forEach(function(node){
          var source = node.nodeValue;
          var lower = source.toLowerCase();
          var start = 0;
          var index = lower.indexOf(query, start);
          if(index < 0){ return; }
          var fragment = document.createDocumentFragment();
          while(index >= 0){
            if(index > start){ fragment.appendChild(document.createTextNode(source.slice(start, index))); }
            var hit = document.createElement("mark");
            hit.className = "kdoc-search-hit";
            hit.textContent = source.slice(index, index + query.length);
            fragment.appendChild(hit);
            start = index + query.length;
            index = lower.indexOf(query, start);
          }
          if(start < source.length){ fragment.appendChild(document.createTextNode(source.slice(start))); }
          node.parentNode.replaceChild(fragment, node);
        });
      }
      function focusResult(next){
        if(results.length === 0){ return; }
        current = (next + results.length) % results.length;
        lines.forEach(function(line){ line.classList.remove("kdoc-current"); });
        var line = results[current];
        reveal(line);
        line.classList.add("kdoc-current");
        showDetails(line);
        line.scrollIntoView({block:"center"});
      }
      function applySearch(){
        var query = (search && search.value || "").toLowerCase();
        var fieldOnly = query.indexOf("/") === 0;
        if(fieldOnly){ query = query.slice(1); }
        results = [];
        current = -1;
        clearSearchHighlights();
        lines.forEach(function(line){
          line.classList.remove("kdoc-match", "kdoc-current", "kdoc-selected");
          if(query === ""){ return; }
          var haystack = fieldOnly ? line.getAttribute("data-field") : line.getAttribute("data-search");
          if((haystack || "").indexOf(query) >= 0){
            line.classList.add("kdoc-match");
            highlightLine(line, query);
            results.push(line);
          }
        });
        if(results.length > 0){ focusResult(0); }
      }

      root.addEventListener("click", function(event){
        var toggle = event.target.closest("[data-kdoc-toggle]");
        if(toggle){
          var line = toggle.closest("[data-kdoc-line]");
          setExpanded(line, !expanded(line));
          applyFolds();
          select(line);
          return;
        }
        var line = event.target.closest("[data-kdoc-line]");
        if(line){ select(line); }
      });
      if(search){
        search.addEventListener("input", applySearch);
        search.addEventListener("keydown", function(event){
          if(event.key === "Escape"){ search.value = ""; applySearch(); search.blur(); event.preventDefault(); }
          if(event.key === "ArrowDown"){ focusResult(current + 1); event.preventDefault(); }
          if(event.key === "ArrowUp"){ focusResult(current - 1); event.preventDefault(); }
        });
      }
      document.addEventListener("keydown", function(event){
        var tag = event.target && event.target.tagName;
        if(event.key === "/" && tag !== "INPUT" && tag !== "TEXTAREA" && search){
          search.focus();
          event.preventDefault();
        }
        if(tag !== "INPUT" && tag !== "TEXTAREA" && (event.key === "n" || event.key === "ArrowDown")){
          focusResult(current + 1);
          event.preventDefault();
        }
        if(tag !== "INPUT" && tag !== "TEXTAREA" && (event.key === "p" || event.key === "ArrowUp")){
          focusResult(current - 1);
          event.preventDefault();
        }
      });
      applyFolds();
    });
  });
})();
</script>`
}
