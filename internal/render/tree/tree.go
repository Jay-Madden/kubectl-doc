package tree

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sttts/kubectl-doc/internal/crd"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

type DescriptionMode string

const (
	DescriptionFalse    DescriptionMode = "false"
	DescriptionRequired DescriptionMode = "required"
	DescriptionTrue     DescriptionMode = "true"
)

type Options struct {
	ExpandDepth    int
	Descriptions   DescriptionMode
	Columns        int
	RenderStatus   bool
	RenderMetadata bool
}

type Line struct {
	Index       int
	Text        string
	Description string
	Depth       int
	Field       string
	Path        string
	Code        bool
	Metadata    bool
	Required    bool
	Foldable    bool
	Collapsed   bool

	RootDescription bool
	CommentGroup    string
}

func Build(doc *crd.Document, options Options) []Line {
	options.Descriptions = descriptionMode(options.Descriptions)
	lines := renderRootDescription(doc, options)
	lines = append(lines, renderTypeMeta(doc, options)...)
	lines = append(lines, renderMetadata(doc, options)...)

	rootRequired := requiredSet(doc.Schema)
	rootFields := 0
	for _, name := range sortedProperties(doc.Schema) {
		if name == "apiVersion" || name == "kind" || name == "metadata" {
			continue
		}
		child := doc.Schema.Properties[name]
		required := rootRequired[name]
		var fieldLines []Line
		if name == "status" && !required && !options.RenderStatus {
			fieldLines = []Line{line(fmt.Sprintf("# status: {}%s", compactComment(&child, false)), 0, "status", "status", required)}
			fieldLines = withDescription(&child, 0, "status", required, options, fieldLines)
		} else if name == "status" && !required && options.RenderStatus {
			fieldLines = renderFieldUncommentedWithOptional(name, &child, 0, name, required, true, options)
		} else {
			fieldLines = renderField(name, &child, 0, name, required, options)
		}
		lines = appendBlock(lines, fieldLines, rootFields > 0)
		rootFields++
	}

	lines = wrapInlineComments(lines, options.Columns)
	lines = reindex(lines)
	markFoldable(lines)
	return lines
}

func renderRootDescription(doc *crd.Document, options Options) []Line {
	if !options.Descriptions.show(true) || doc == nil || doc.Schema == nil {
		return nil
	}
	lines := descriptionComments(doc.Schema, 0, "", true, options.Columns)
	for i := range lines {
		lines[i].RootDescription = true
	}
	return lines
}

func renderTypeMeta(doc *crd.Document, options Options) []Line {
	typeMetaOptions := options
	typeMetaOptions.Descriptions = DescriptionFalse
	var lines []Line
	lines = append(lines, renderScalarFieldValue("apiVersion", apiVersion(doc.Group, doc.Version), doc.APIVersionSchema(), typeMetaOptions)...)
	lines = append(lines, renderScalarFieldValue("kind", doc.Kind, doc.KindSchema(), typeMetaOptions)...)
	return lines
}

func renderScalarFieldValue(name, value string, field *docschema.Structural, options Options) []Line {
	return withDescription(field, 0, name, true, options, []Line{
		line(fmt.Sprintf("%s: %s", name, value), 0, name, name, true),
	})
}

func WithCollapsed(lines []Line, expandDepth int) []Line {
	if expandDepth < 0 {
		expandDepth = 0
	}
	out := append([]Line(nil), lines...)
	for i := range out {
		out[i].Collapsed = out[i].Foldable && out[i].Depth >= expandDepth
		if out[i].Foldable && (out[i].Path == "status" || out[i].Path == "metadata" || strings.HasPrefix(out[i].Path, "metadata.")) {
			out[i].Collapsed = true
		}
	}
	return out
}

func Texts(lines []Line) []string {
	texts := make([]string, 0, len(lines))
	for _, line := range lines {
		texts = append(texts, line.Text)
	}
	return texts
}

func renderMetadata(doc *crd.Document, options Options) []Line {
	if !options.RenderMetadata {
		lines := []Line{
			line("metadata:", 0, "metadata", "metadata", true),
			line(`  name: "<name>"`, 1, "metadata.name", "name", true),
		}
		if doc != nil && doc.Namespaced {
			lines = append(lines, line(`  namespace: "<namespace>"`, 1, "metadata.namespace", "namespace", true))
		}
		return lines
	}

	metadata := doc.MetadataSchema()
	description := metadata.Description
	metadata.Description = ""
	lines := renderFieldUncommentedWithOptional("metadata", metadata, 0, "metadata", false, false, options)
	for i := range lines {
		if lines[i].Path == "metadata" && lines[i].Field != "" {
			lines[i].Description = description
			break
		}
	}
	return lines
}

func apiVersion(group, version string) string {
	if group == "" {
		return version
	}
	return group + "/" + version
}

func descriptionMode(mode DescriptionMode) DescriptionMode {
	if mode == "" {
		return DescriptionTrue
	}
	return mode
}

func (m DescriptionMode) show(required bool) bool {
	switch m {
	case DescriptionFalse:
		return false
	case DescriptionRequired:
		return required
	default:
		return true
	}
}

func renderField(name string, field *docschema.Structural, depth int, path string, required bool, options Options) []Line {
	lines := renderFieldUncommented(name, field, depth, path, required, options)
	if required || hasRequiredDescendant(field) {
		return lines
	}
	return commentLines(lines)
}

func renderFieldUncommented(name string, field *docschema.Structural, depth int, path string, required bool, options Options) []Line {
	return renderFieldUncommentedWithOptional(name, field, depth, path, required, !required && hasRequiredDescendant(field), options)
}

func renderFieldUncommentedWithOptional(name string, field *docschema.Structural, depth int, path string, required bool, optional bool, options Options) []Line {
	indent := strings.Repeat("  ", depth)
	comment := compactComment(field, optional)

	switch effectiveType(field) {
	case "object":
		childNames := sortedProperties(field)
		if len(childNames) == 0 {
			if mapValue := mapValueSchema(field); mapValue != nil {
				lines := []Line{line(withRequiredLabel(fmt.Sprintf("%s%s:%s", indent, name, comment), required), depth, path, name, required)}
				lines = appendBlock(lines, renderFieldUncommented("<key>", mapValue, depth+1, path+".<key>", false, options), false)
				return withDescription(field, depth, path, required, options, lines)
			}
			return withDescription(field, depth, path, required, options, []Line{
				line(withRequiredLabel(fmt.Sprintf("%s%s: %s%s", indent, name, scalarValue(field), comment), required), depth, path, name, required),
			})
		}
		if depth >= options.ExpandDepth {
			return withDescription(field, depth, path, required, options, []Line{
				line(withRequiredLabel(fmt.Sprintf("%s%s: {}%s", indent, name, collapsedComment(field, depth, !required && hasRequiredDescendant(field))), required), depth, path, name, required),
			})
		}

		lines := []Line{line(withRequiredLabel(fmt.Sprintf("%s%s:%s", indent, name, comment), required), depth, path, name, required)}
		childRequired := requiredSet(field)
		for i, childName := range orderProperties(childNames, childRequired) {
			child := field.Properties[childName]
			lines = appendBlock(lines, renderField(childName, &child, depth+1, path+"."+childName, childRequired[childName], options), i > 0)
		}
		return withDescription(field, depth, path, required, options, lines)
	case "array":
		if hasInlineValue(field) {
			return withDescription(field, depth, path, required, options, []Line{
				line(withRequiredLabel(fmt.Sprintf("%s%s: %s%s", indent, name, scalarValue(field), comment), required), depth, path, name, required),
			})
		}
		lines := []Line{line(withRequiredLabel(fmt.Sprintf("%s%s:%s", indent, name, comment), required), depth, path, name, required)}
		item := field.Items
		if item == nil {
			lines = append(lines, line(fmt.Sprintf("%s  - {}", indent), depth+2, path+"[]", "", true))
			return withDescription(field, depth, path, required, options, lines)
		}
		if effectiveType(item) == "object" && len(item.Properties) > 0 && depth < options.ExpandDepth {
			itemRequired := requiredSet(item)
			first := true
			for i, childName := range orderProperties(sortedProperties(item), itemRequired) {
				child := item.Properties[childName]
				childLines := renderField(childName, &child, depth+2, path+"[]."+childName, itemRequired[childName], options)
				if first && len(childLines) > 0 {
					childLines = markSequenceItem(childLines, depth)
					first = false
				}
				lines = appendBlock(lines, childLines, i > 0)
			}
			return withDescription(field, depth, path, required, options, lines)
		}
		itemValue := scalarValue(item)
		if effectiveType(item) == "object" && len(item.Properties) > 0 {
			itemValue += collapsedComment(item, depth+1, false)
		}
		lines = append(lines, line(fmt.Sprintf("%s  - %s", indent, itemValue), depth+2, path+"[]", "", true))
		return withDescription(field, depth, path, required, options, lines)
	default:
		return withDescription(field, depth, path, required, options, []Line{
			line(withRequiredLabel(fmt.Sprintf("%s%s: %s%s", indent, name, scalarValue(field), comment), required), depth, path, name, required),
		})
	}
}

func line(text string, depth int, path, field string, required bool) Line {
	return Line{
		Text:     text,
		Depth:    depth,
		Path:     path,
		Field:    field,
		Code:     true,
		Required: required,
	}
}

func commentLine(text string, depth int, path string, required bool) Line {
	return Line{
		Text:     text,
		Depth:    depth,
		Path:     path,
		Required: required,
	}
}

func descriptionLine(text, description string, depth int, path string, required bool, group string) Line {
	line := commentLine(text, depth, path, required)
	line.Description = description
	line.CommentGroup = group
	return line
}

func blankLine() Line {
	return Line{}
}

type WrappedText struct {
	Text     string
	Code     bool
	Metadata bool
}

func WrapInlineCommentText(text string, code bool, columns int) []WrappedText {
	if !code || columns <= 0 || len(text) <= columns {
		return []WrappedText{{Text: text, Code: code}}
	}

	index := inlineCommentIndex(text)
	if index < 0 {
		return []WrappedText{{Text: text, Code: code}}
	}

	beforeHash := text[:index+1]
	body := strings.TrimSpace(text[index+3:])
	if body == "" {
		return []WrappedText{{Text: text, Code: code}}
	}

	firstPrefix := beforeHash + "# "
	continuationPrefix := strings.Repeat(" ", len(beforeHash)) + "# "
	wrapped := wrapInlineCommentBody(firstPrefix, continuationPrefix, body, columns)
	if len(wrapped) <= 1 {
		return []WrappedText{{Text: text, Code: code}}
	}

	out := make([]WrappedText, 0, len(wrapped))
	out = append(out, WrappedText{Text: wrapped[0], Code: true})
	for _, line := range wrapped[1:] {
		out = append(out, WrappedText{Text: line, Metadata: true})
	}
	return out
}

func wrapInlineComments(lines []Line, columns int) []Line {
	if columns <= 0 {
		return lines
	}
	var out []Line
	for _, current := range lines {
		wrapped := WrapInlineCommentText(current.Text, current.Code, columns)
		if len(wrapped) == 1 {
			out = append(out, current)
			continue
		}
		for i, text := range wrapped {
			next := current
			next.Text = text.Text
			next.Code = text.Code
			next.Metadata = text.Metadata
			if i > 0 {
				next.Field = ""
				next.Foldable = false
				next.Collapsed = false
			}
			out = append(out, next)
		}
	}
	return out
}

func inlineCommentIndex(text string) int {
	indentLength := len(text) - len(strings.TrimLeft(text, " "))
	trimmed := text[indentLength:]
	searchStart := 0
	for _, prefix := range []string{"# - # ", "# - ", "# ", "- # "} {
		if strings.HasPrefix(trimmed, prefix) {
			searchStart = len(prefix)
			break
		}
	}
	index := strings.Index(trimmed[searchStart:], " # ")
	if index < 0 {
		return -1
	}
	return indentLength + searchStart + index
}

func wrapInlineCommentBody(firstPrefix, continuationPrefix, body string, columns int) []string {
	if len(firstPrefix) >= columns || len(continuationPrefix) >= columns {
		return []string{firstPrefix + body}
	}

	var lines []string
	prefix := firstPrefix
	limit := columns - len(prefix)
	var current strings.Builder
	for _, word := range strings.Fields(body) {
		if current.Len() == 0 {
			current.WriteString(word)
			continue
		}
		if current.Len()+1+len(word) > limit {
			lines = append(lines, prefix+current.String())
			prefix = continuationPrefix
			limit = columns - len(prefix)
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

func reindex(lines []Line) []Line {
	for i := range lines {
		lines[i].Index = i
	}
	return lines
}

func markFoldable(lines []Line) {
	for i := range lines {
		if strings.TrimSpace(lines[i].Text) == "" || lines[i].Field == "" {
			continue
		}
		nextDepth, ok := nextContentDepth(lines, i)
		lines[i].Foldable = ok && nextDepth > lines[i].Depth
	}
}

func nextContentDepth(lines []Line, index int) (int, bool) {
	for i := index + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i].Text) == "" || lines[i].Metadata {
			continue
		}
		return lines[i].Depth, true
	}
	return 0, false
}

func withRequiredLabel(text string, required bool) string {
	if !required {
		return text
	}
	if index := strings.Index(text, " # "); index >= 0 {
		return text[:index] + " # " + addRequiredComment(text[index+3:])
	}
	return text + " # required"
}

func addRequiredComment(comment string) string {
	if strings.HasPrefix(comment, "default") || strings.HasPrefix(comment, "example") {
		if index := strings.Index(comment, ", "); index >= 0 {
			return comment[:index] + ", required" + comment[index:]
		}
		return comment + ", required"
	}
	return "required, " + comment
}

func appendBlock(lines, block []Line, separator bool) []Line {
	if len(block) == 0 {
		return lines
	}
	if separator && needsBlockSeparator(lines, block) {
		lines = append(lines, blankLine())
	}
	return append(lines, block...)
}

func needsBlockSeparator(lines, block []Line) bool {
	previousDepth, previousOK := trailingOneLineFieldDepth(lines)
	currentDepth, currentOK := oneLineFieldBlockDepth(block)
	return !previousOK || !currentOK || previousDepth != currentDepth
}

func trailingOneLineFieldDepth(lines []Line) (int, bool) {
	depth := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i].Text) == "" {
			break
		}
		depthAtLine := lines[i].Depth
		if depth >= 0 && depthAtLine != depth {
			break
		}
		if lines[i].Field == "" {
			return 0, false
		}
		depth = depthAtLine
	}
	return depth, depth >= 0
}

func oneLineFieldBlockDepth(block []Line) (int, bool) {
	seen := false
	depth := -1
	for _, line := range block {
		if strings.TrimSpace(line.Text) == "" {
			continue
		}
		if seen || line.Field == "" {
			return 0, false
		}
		depth = line.Depth
		seen = true
	}
	return depth, seen
}

func withDescription(field *docschema.Structural, depth int, path string, required bool, options Options, lines []Line) []Line {
	description := ""
	if field != nil {
		description = strings.TrimSpace(field.Description)
	}
	if description != "" {
		for i := range lines {
			if lines[i].Field != "" && lines[i].Path == path {
				lines[i].Description = description
				break
			}
		}
	}
	if !options.Descriptions.show(required) {
		return lines
	}
	comments := descriptionComments(field, depth, path, required, options.Columns)
	if len(comments) == 0 {
		return lines
	}
	out := make([]Line, 0, len(comments)+len(lines))
	out = append(out, comments...)
	out = append(out, lines...)
	return out
}

func descriptionComments(field *docschema.Structural, depth int, path string, required bool, columns int) []Line {
	if field == nil || strings.TrimSpace(field.Description) == "" {
		return nil
	}

	indent := strings.Repeat("  ", depth)
	var comments []Line
	var paragraph []string
	paragraphIndex := 0
	flush := func() {
		if len(paragraph) == 0 {
			return
		}
		group := "description-" + strconv.Itoa(paragraphIndex)
		paragraphIndex++
		for _, wrapped := range wrapCommentParagraph(indent, strings.Join(paragraph, " "), columns) {
			comments = append(comments, descriptionLine(wrapped.Text, wrapped.Description, depth, path, required, group))
		}
		paragraph = nil
	}

	for _, raw := range strings.Split(strings.TrimSpace(field.Description), "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			flush()
			comments = append(comments, commentLine(indent+"#", depth, path, required))
			continue
		}
		paragraph = append(paragraph, trimmed)
	}
	flush()
	return comments
}

type wrappedDescription struct {
	Text        string
	Description string
}

func wrapCommentParagraph(indent, paragraph string, columns int) []wrappedDescription {
	prefix := indent + "# "
	if columns <= 0 || len(prefix) >= columns {
		return []wrappedDescription{{Text: prefix + paragraph, Description: paragraph}}
	}

	width := columns - len(prefix)
	var lines []wrappedDescription
	var text strings.Builder
	for _, word := range strings.Fields(paragraph) {
		if text.Len() == 0 {
			text.WriteString(word)
			continue
		}
		if text.Len()+1+len(word) > width {
			lines = append(lines, wrappedDescription{Text: prefix + text.String(), Description: text.String()})
			text.Reset()
			text.WriteString(word)
			continue
		}
		text.WriteByte(' ')
		text.WriteString(word)
	}
	if text.Len() > 0 {
		lines = append(lines, wrappedDescription{Text: prefix + text.String(), Description: text.String()})
	}
	return lines
}

func markSequenceItem(lines []Line, depth int) []Line {
	childIndent := strings.Repeat("  ", depth+2)
	itemIndent := strings.Repeat("  ", depth+1) + "- "
	out := append([]Line(nil), lines...)
	for i, line := range out {
		if strings.TrimSpace(line.Text) == "" {
			continue
		}
		if strings.HasPrefix(line.Text, childIndent) {
			out[i].Text = itemIndent + strings.TrimPrefix(line.Text, childIndent)
		}
		return out
	}
	return out
}

func commentLines(lines []Line) []Line {
	commented := make([]Line, 0, len(lines))
	for _, current := range lines {
		if strings.TrimSpace(current.Text) == "" {
			commented = append(commented, current)
			continue
		}
		indent := current.Text[:len(current.Text)-len(strings.TrimLeft(current.Text, " "))]
		trimmed := strings.TrimLeft(current.Text, " ")
		if uncommented, ok := commentSequenceFieldLine(trimmed); ok {
			current.Text = indent + "# - " + uncommented
			commented = append(commented, current)
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			commented = append(commented, current)
			continue
		}
		current.Text = indent + "# " + trimmed
		commented = append(commented, current)
	}
	return commented
}

func commentSequenceFieldLine(text string) (string, bool) {
	if !strings.HasPrefix(text, "- # ") {
		return "", false
	}
	uncommented := strings.TrimPrefix(text, "- # ")
	if !looksLikeRenderedFieldLine(uncommented) {
		return "", false
	}
	return uncommented, true
}

func looksLikeRenderedFieldLine(text string) bool {
	if fieldName(text) == "" {
		return false
	}
	colon := strings.Index(text, ":")
	if colon <= 0 {
		return false
	}
	value := strings.TrimSpace(text[colon+1:])
	if commentIndex := strings.Index(value, " # "); commentIndex >= 0 {
		value = strings.TrimSpace(value[:commentIndex])
	}
	if value == "" || strings.HasPrefix(value, "#") {
		return true
	}
	switch {
	case strings.HasPrefix(value, `"`):
		return true
	case strings.HasPrefix(value, "<"):
		return true
	case strings.HasPrefix(value, "{"):
		return true
	case strings.HasPrefix(value, "["):
		return true
	case value == "true" || value == "false" || value == "null":
		return true
	default:
		_, err := strconv.ParseFloat(value, 64)
		return err == nil
	}
}

func fieldName(text string) string {
	trimmed := strings.TrimSpace(text)
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

func scalarValue(field *docschema.Structural) string {
	if field == nil {
		return "{}"
	}
	if field.Default.Object != nil {
		return yamlScalar(field.Default.Object)
	}
	if example, ok := selectedExample(field); ok {
		return yamlScalar(example.Value.Object)
	}
	if field.ValueValidation != nil && len(field.ValueValidation.Enum) > 0 {
		return yamlScalar(field.ValueValidation.Enum[0].Object)
	}
	if field.XIntOrString {
		return "<int-or-string>"
	}

	switch field.Type {
	case "string":
		return `"<string>"`
	case "integer":
		if format := fieldFormat(field); format != "" {
			return "<" + format + ">"
		}
		return "<integer>"
	case "number":
		return "<number>"
	case "boolean":
		return "<boolean>"
	case "array":
		return "[]"
	case "object":
		return "{}"
	default:
		return "{}"
	}
}

func yamlScalar(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strconv.Quote(typed)
	case nil:
		return "null"
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return "{}"
		}
		return string(data)
	}
}

func compactComment(field *docschema.Structural, optional bool) string {
	var comments []string
	if field == nil {
		return ""
	}
	if optional {
		comments = append(comments, "optional")
	}
	if field.Default.Object != nil {
		comments = append(comments, "default")
	} else if example, ok := selectedExample(field); ok {
		comments = append(comments, exampleComment(example))
	}
	if field.ValueValidation != nil {
		if len(field.ValueValidation.Enum) > 1 {
			var values []string
			for _, enum := range field.ValueValidation.Enum[1:] {
				values = append(values, yamlScalar(enum.Object))
			}
			comments = append(comments, "enum: "+strings.Join(values, " | "))
		}
		if field.ValueValidation.MinLength != nil {
			comments = append(comments, fmt.Sprintf("minLength: %d", *field.ValueValidation.MinLength))
		}
		if field.ValueValidation.MaxLength != nil {
			comments = append(comments, fmt.Sprintf("maxLength: %d", *field.ValueValidation.MaxLength))
		}
		if field.ValueValidation.Minimum != nil {
			comments = append(comments, fmt.Sprintf("minimum: %s", trimFloat(*field.ValueValidation.Minimum)))
		}
		if field.ValueValidation.Maximum != nil {
			comments = append(comments, fmt.Sprintf("maximum: %s", trimFloat(*field.ValueValidation.Maximum)))
		}
	}
	if field.Nullable {
		comments = append(comments, "nullable")
	}
	if field.XPreserveUnknownFields {
		comments = append(comments, "preserveUnknownFields")
	}
	if field.XEmbeddedResource {
		comments = append(comments, "embeddedResource")
	}
	if field.XIntOrString {
		comments = append(comments, "intOrString")
	}
	if field.XListType != nil {
		comments = append(comments, "listType: "+*field.XListType)
	}
	if len(field.XListMapKeys) > 0 {
		comments = append(comments, "listMapKeys: "+strings.Join(field.XListMapKeys, ", "))
	}
	if field.XMapType != nil {
		comments = append(comments, "mapType: "+*field.XMapType)
	}
	if len(comments) == 0 {
		return ""
	}
	return " # " + strings.Join(comments, ", ")
}

func selectedExample(field *docschema.Structural) (docschema.Example, bool) {
	if field == nil {
		return docschema.Example{}, false
	}
	for _, example := range field.Examples {
		if example.Value.Object != nil {
			return example, true
		}
	}
	return docschema.Example{}, false
}

func hasInlineValue(field *docschema.Structural) bool {
	if field == nil {
		return false
	}
	if field.Default.Object != nil {
		return true
	}
	_, ok := selectedExample(field)
	return ok
}

func exampleComment(example docschema.Example) string {
	parts := []string{"example", exampleValueType(example.Value.Object)}
	if example.Name != "" {
		parts = append(parts, example.Name)
	}
	return strings.Join(parts, " ")
}

func exampleValueType(value interface{}) string {
	switch value.(type) {
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, float32, int, int32, int64, uint, uint32, uint64:
		return "number"
	default:
		return "value"
	}
}

func collapsedComment(field *docschema.Structural, depth int, optional bool) string {
	comment := compactComment(field, optional)
	hint := fmt.Sprintf("show with --expand-depth %d", depth+1)
	if comment == "" {
		return " # " + hint
	}
	return comment + ", " + hint
}

func trimFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
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

func orderProperties(names []string, required map[string]bool) []string {
	ordered := make([]string, 0, len(names))
	for _, name := range names {
		if required[name] {
			ordered = append(ordered, name)
		}
	}
	for _, name := range names {
		if !required[name] {
			ordered = append(ordered, name)
		}
	}
	return ordered
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

func hasRequiredDescendant(field *docschema.Structural) bool {
	if field == nil {
		return false
	}
	if len(requiredSet(field)) > 0 {
		return true
	}
	if hasRequiredDescendant(field.Items) {
		return true
	}
	if field.AdditionalProperties != nil && hasRequiredDescendant(field.AdditionalProperties.Structural) {
		return true
	}
	for _, child := range field.Properties {
		if hasRequiredDescendant(&child) {
			return true
		}
	}
	return false
}

func mapValueSchema(field *docschema.Structural) *docschema.Structural {
	if field == nil || field.AdditionalProperties == nil || field.AdditionalProperties.Structural == nil {
		return nil
	}
	return field.AdditionalProperties.Structural
}

func fieldFormat(field *docschema.Structural) string {
	if field == nil || field.ValueValidation == nil {
		return ""
	}
	return field.ValueValidation.Format
}
