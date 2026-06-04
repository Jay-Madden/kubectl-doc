package htmlrender

import (
	"encoding/json"
	"fmt"
	htmlpkg "html"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/tree"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
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
	if _, err := fmt.Fprintf(out, "<main class=\"kubectl-doc\" data-kubectl-doc%s>\n<header class=\"kdoc-header\">\n<h1>%s <small>%s</small></h1>\n", backAttr, escape(docs[0].Kind), escape(headerVersion(docs))); err != nil {
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
		fieldAttr = ` data-kdoc-field`
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

func attachFieldDetails(lines []tree.Line, details map[string]fieldDetail) []htmlLine {
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

func applyFieldDetail(line *htmlLine, detail fieldDetail) {
	line.Path = detail.Path
	line.Required = detail.Required
	line.DetailID = detail.ID
	line.Detail = detail.Text()
	line.DetailHTML = detail.HTML()
}

func lookupFieldDetail(details map[string]fieldDetail, path string) (fieldDetail, bool) {
	if detail, ok := details[path]; ok {
		return detail, true
	}
	return fieldDetail{}, false
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

func collectFieldDetails(doc *crd.Document) map[string]fieldDetail {
	if doc == nil {
		return nil
	}

	fields := map[string]fieldDetail{}
	collectFieldDetail(doc, fields, "apiVersion", doc.APIVersionSchema(), true)
	collectFieldDetail(doc, fields, "kind", doc.KindSchema(), true)
	collectFieldDetail(doc, fields, "metadata", doc.MetadataSchema(), true)
	if doc.Schema == nil {
		return fields
	}

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
.kubectl-doc{--kdoc-fg:#1f2933;--kdoc-muted:#57606a;--kdoc-border:#d8dee4;--kdoc-panel:#f6f8fa;--kdoc-selected:#fff7cc;--kdoc-required:#cf222e;--kdoc-ok:#116329;--kdoc-yaml-key:#0550ae;--kdoc-yaml-string:#0a7f42;--kdoc-yaml-comment:#6e7781;--kdoc-yaml-punct:#8c959f;--kdoc-yaml-number:#953800;--kdoc-yaml-type-number:#007c89;--kdoc-yaml-bool:#8250df;--kdoc-yaml-null:#8250df;box-sizing:border-box;color:var(--kdoc-fg);background:#fff;font:14px/1.45 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;max-width:100%;padding:24px}
.kubectl-doc *{box-sizing:border-box}
.kdoc-view-controls{bottom:calc(12px + 2.5em);display:flex;height:0;justify-content:flex-end;pointer-events:none;position:sticky;z-index:4}
.kdoc-wrap-toggle{align-items:center;background:transparent;border:0;color:var(--kdoc-muted);cursor:pointer;display:flex;font-size:12px;font-weight:600;gap:.65em;line-height:1;padding:0;pointer-events:auto}
.kdoc-wrap-toggle input{block-size:1px;clip:rect(0 0 0 0);clip-path:inset(50%);inline-size:1px;margin:0;overflow:hidden;position:absolute;white-space:nowrap}
.kdoc-switch{background:#d0d7de;border-radius:999px;box-shadow:inset 0 0 0 1px rgba(31,41,51,.08);display:inline-block;flex:0 0 auto;inline-size:2.65em;block-size:1.5em;position:relative;transition:background-color .16s ease,box-shadow .16s ease}
.kdoc-switch::after{background:#fff;border-radius:50%;box-shadow:0 .08em .24em rgba(31,41,51,.28);content:"";display:block;inline-size:1.18em;block-size:1.18em;position:absolute;inset-block-start:50%;inset-inline-start:.16em;transform:translateY(-50%);transition:inset-inline-start .16s ease}
.kdoc-wrap-toggle input:checked + .kdoc-switch{background:#34c759;box-shadow:inset 0 0 0 1px rgba(17,99,41,.12)}
.kdoc-wrap-toggle input:checked + .kdoc-switch::after{inset-inline-start:1.31em}
.kdoc-wrap-toggle input:focus-visible + .kdoc-switch{box-shadow:0 0 0 2px rgba(9,105,218,.25),inset 0 0 0 1px rgba(31,41,51,.08)}
.kdoc-header{border-bottom:1px solid var(--kdoc-border);margin-bottom:16px;padding-bottom:16px;padding-right:150px}
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
      var backURL = root.getAttribute("data-kdoc-back-url");
      var resizeFrame = 0;
      var charWidthCache = 0;
      var currentLine = null;

      function button(line){ return line.querySelector("[data-kdoc-toggle]"); }
      function depth(line){ return Number(line.getAttribute("data-depth") || "0"); }
      function expanded(line){ var b = button(line); return !b || b.getAttribute("aria-expanded") !== "false"; }
      function setExpanded(line, value){
        var b = button(line);
        if(!b){ return; }
        b.setAttribute("aria-expanded", value ? "true" : "false");
      }
      function nextContentDepth(index){
        for(var i = index; i < lines.length; i++){
          if(lines[i].textContent.trim() !== ""){ return depth(lines[i]); }
        }
        return null;
      }
      function applyFolds(){
        lines.forEach(function(line){ line.hidden = false; });
        lines.forEach(function(line, index){
          if(line.hidden || expanded(line)){ return; }
          var parentDepth = depth(line);
          for(var i = index + 1; i < lines.length; i++){
            var blank = lines[i].textContent.trim() === "";
            var followingDepth = blank ? nextContentDepth(i + 1) : null;
            if(blank && followingDepth !== null && followingDepth <= parentDepth){ break; }
            if(!blank && depth(lines[i]) <= parentDepth){ break; }
            lines[i].hidden = true;
          }
        });
      }
      function groupedLines(line){
        var id = line.getAttribute("data-detail-id");
        if(!id){ return [line]; }
        return lines.filter(function(item){ return item.getAttribute("data-detail-id") === id; });
      }
      function isFieldLine(line){ return !!(line && line.hasAttribute("data-kdoc-field")); }
      function fieldLineFor(line){
        if(!line){ return null; }
        if(isFieldLine(line)){ return line; }
        var id = line.getAttribute("data-detail-id");
        if(!id){ return null; }
        for(var i = 0; i < lines.length; i++){
          if(isFieldLine(lines[i]) && lines[i].getAttribute("data-detail-id") === id){
            return lines[i];
          }
        }
        return null;
      }
      function visibleFieldLines(){
        return lines.filter(function(line){ return isFieldLine(line) && !line.hidden; });
      }
      function visibleFoldableLines(){
        return visibleFieldLines().filter(function(line){ return !!button(line); });
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
        var currentDepth = depth(line);
        var fields = visibleFieldLines();
        var index = lineIndex(fields, line);
        for(var i = index - 1; i >= 0; i--){
          if(depth(fields[i]) < currentDepth){ return fields[i]; }
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
      function availableTextWidth(comment){
        var text = comment.closest(".kdoc-yaml-text");
        var line = comment.closest("[data-kdoc-line]");
        if(!text || !line){ return 0; }
        var gutter = line.querySelector(".kdoc-fold,.kdoc-gutter");
        var style = window.getComputedStyle(line);
        var width = line.clientWidth - parseFloat(style.paddingLeft || "0") - parseFloat(style.paddingRight || "0");
        if(gutter){ width -= gutter.getBoundingClientRect().width; }
        return Math.max(width, 0);
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
      function renderComment(comment, wrapped){
        var firstPrefix = comment.getAttribute("data-kdoc-comment-prefix") || "";
        var nextPrefix = comment.getAttribute("data-kdoc-comment-wrap-prefix") || firstPrefix;
        var text = comment.getAttribute("data-kdoc-comment-text") || "";
        var line = comment.closest("[data-kdoc-line]");
        if(wrapped && line && line.hidden){ return; }
        if(!wrapped){
          comment.innerHTML = "<span class=\"kdoc-yaml-comment kdoc-comment-prefix\">" + escapeHTML(firstPrefix) + "</span><span class=\"kdoc-yaml-comment kdoc-comment-body\">" + escapeHTML(text) + "</span>";
          return;
        }
        var width = availableTextWidth(comment);
        var lineChars = Math.max(Math.floor(width / charWidth()), 8);
        var firstLimit = Math.max(lineChars - firstPrefix.length, 8);
        var nextLimit = Math.max(lineChars - nextPrefix.length, 8);
        var chunks = wrapCommentText(text, firstLimit, nextLimit);
        comment.innerHTML = chunks.map(function(chunk, index){
          return renderCommentLine(index === 0 ? firstPrefix : nextPrefix, chunk);
        }).join("\n");
      }
      function applyCommentWrap(){
        if(!wrapComments){ return; }
        var wrapped = wrapComments.checked;
        root.classList.toggle("kdoc-wrap-comments", wrapped);
        comments.forEach(function(comment){ renderComment(comment, wrapped); });
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
        switch(event.key){
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
          handled = selectFoldable(event.shiftKey ? -1 : 1);
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
        }
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
      window.addEventListener("resize", scheduleCommentWrap);
      applyCommentWrap();
      applyFolds();
      select(visibleFieldLines()[0] || lines[0]);
    });
  });
})();
</script>`
}
