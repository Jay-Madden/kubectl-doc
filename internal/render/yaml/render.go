package yamlrender

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"

	"github.com/sttts/kubectl-doc/internal/crd"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

type Renderer struct {
	ExpandDepth  int
	Color        bool
	Descriptions DescriptionMode
}

type DescriptionMode string

const (
	DescriptionFalse    DescriptionMode = "false"
	DescriptionRequired DescriptionMode = "required"
	DescriptionTrue     DescriptionMode = "true"
)

func (r Renderer) Render(out io.Writer, doc *crd.Document) error {
	lines := []string{
		fmt.Sprintf("apiVersion: %s/%s", doc.Group, doc.Version),
		fmt.Sprintf("kind: %s", doc.Kind),
		"metadata:",
		`  name: "<name>"`,
	}

	descriptions := r.descriptionMode()
	rootRequired := requiredSet(doc.Schema)
	rootFields := 0
	for _, name := range sortedProperties(doc.Schema) {
		if name == "apiVersion" || name == "kind" || name == "metadata" {
			continue
		}
		child := doc.Schema.Properties[name]
		required := rootRequired[name]
		var fieldLines []string
		if name == "status" && !required {
			fieldLines = []string{fmt.Sprintf("# status: {}%s", compactComment(&child, false))}
			fieldLines = withDescription(&child, 0, required, descriptions, fieldLines)
		} else {
			fieldLines = renderField(name, &child, 0, r.ExpandDepth, required, descriptions)
		}
		lines = appendBlock(lines, fieldLines, rootFields > 0)
		rootFields++
	}

	if r.Color {
		for i, line := range lines {
			lines[i] = colorLine(line)
		}
	}

	_, err := fmt.Fprintln(out, strings.Join(lines, "\n"))
	return err
}

func (r Renderer) descriptionMode() DescriptionMode {
	if r.Descriptions == "" {
		return DescriptionTrue
	}
	return r.Descriptions
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

func renderField(name string, field *docschema.Structural, depth, expandDepth int, required bool, descriptions DescriptionMode) []string {
	lines := renderFieldUncommented(name, field, depth, expandDepth, required, descriptions)
	if required || hasRequiredDescendant(field) {
		return lines
	}
	return commentLines(lines)
}

func renderFieldUncommented(name string, field *docschema.Structural, depth, expandDepth int, required bool, descriptions DescriptionMode) []string {
	indent := strings.Repeat("  ", depth)
	comment := compactComment(field, !required && hasRequiredDescendant(field))

	switch effectiveType(field) {
	case "object":
		childNames := sortedProperties(field)
		if len(childNames) == 0 {
			if mapValue := mapValueSchema(field); mapValue != nil {
				lines := []string{fmt.Sprintf("%s%s:%s", indent, name, comment)}
				lines = appendBlock(lines, renderFieldUncommented("<key>", mapValue, depth+1, expandDepth, false, descriptions), false)
				return withDescription(field, depth, required, descriptions, lines)
			}
			return withDescription(field, depth, required, descriptions, []string{fmt.Sprintf("%s%s: {}%s", indent, name, comment)})
		}
		if depth >= expandDepth {
			return withDescription(field, depth, required, descriptions, []string{fmt.Sprintf("%s%s: {}%s", indent, name, collapsedComment(field, depth, !required && hasRequiredDescendant(field)))})
		}

		lines := []string{fmt.Sprintf("%s%s:%s", indent, name, comment)}
		childRequired := requiredSet(field)
		for i, childName := range orderProperties(childNames, childRequired) {
			child := field.Properties[childName]
			lines = appendBlock(lines, renderField(childName, &child, depth+1, expandDepth, childRequired[childName], descriptions), i > 0)
		}
		return withDescription(field, depth, required, descriptions, lines)
	case "array":
		lines := []string{fmt.Sprintf("%s%s:%s", indent, name, comment)}
		item := field.Items
		if item == nil {
			lines = append(lines, fmt.Sprintf("%s  - {}", indent))
			return withDescription(field, depth, required, descriptions, lines)
		}
		if effectiveType(item) == "object" && len(item.Properties) > 0 && depth < expandDepth {
			itemRequired := requiredSet(item)
			first := true
			for i, childName := range orderProperties(sortedProperties(item), itemRequired) {
				child := item.Properties[childName]
				childLines := renderField(childName, &child, depth+2, expandDepth, itemRequired[childName], descriptions)
				if first && len(childLines) > 0 {
					childLines = markSequenceItem(childLines, depth)
					first = false
				}
				lines = appendBlock(lines, childLines, i > 0)
			}
			return withDescription(field, depth, required, descriptions, lines)
		}
		itemValue := scalarValue(item)
		if effectiveType(item) == "object" && len(item.Properties) > 0 {
			itemValue += collapsedComment(item, depth+1, false)
		}
		lines = append(lines, fmt.Sprintf("%s  - %s", indent, itemValue))
		return withDescription(field, depth, required, descriptions, lines)
	default:
		return withDescription(field, depth, required, descriptions, []string{fmt.Sprintf("%s%s: %s%s", indent, name, scalarValue(field), comment)})
	}
}

func appendBlock(lines, block []string, separator bool) []string {
	if len(block) == 0 {
		return lines
	}
	if separator {
		lines = append(lines, "")
	}
	return append(lines, block...)
}

func withDescription(field *docschema.Structural, depth int, required bool, descriptions DescriptionMode, lines []string) []string {
	if !descriptions.show(required) {
		return lines
	}
	comments := descriptionComments(field, depth)
	if len(comments) == 0 {
		return lines
	}
	out := make([]string, 0, len(comments)+len(lines))
	out = append(out, comments...)
	out = append(out, lines...)
	return out
}

func descriptionComments(field *docschema.Structural, depth int) []string {
	if field == nil || strings.TrimSpace(field.Description) == "" {
		return nil
	}

	indent := strings.Repeat("  ", depth)
	var comments []string
	for _, line := range strings.Split(strings.TrimSpace(field.Description), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			comments = append(comments, indent+"#")
			continue
		}
		comments = append(comments, indent+"# "+line)
	}
	return comments
}

func markSequenceItem(lines []string, depth int) []string {
	childIndent := strings.Repeat("  ", depth+2)
	itemIndent := strings.Repeat("  ", depth+1) + "- "
	out := append([]string(nil), lines...)
	for i, line := range out {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, childIndent) {
			out[i] = itemIndent + strings.TrimPrefix(line, childIndent)
		}
		return out
	}
	return out
}

func commentLines(lines []string) []string {
	commented := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			commented = append(commented, line)
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, "#") {
			commented = append(commented, line)
			continue
		}
		commented = append(commented, indent+"# "+trimmed)
	}
	return commented
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

var (
	keyStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	stringStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	scalarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	syntaxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	noteStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func colorLine(line string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	trimmed := strings.TrimLeft(line, " ")
	if trimmed == "" {
		return line
	}
	if strings.HasPrefix(trimmed, "#") {
		return indent + noteStyle.Render(trimmed)
	}

	code := line
	comment := ""
	if index := strings.Index(line, " # "); index >= 0 {
		code = line[:index]
		comment = noteStyle.Render(line[index:])
	}
	return colorCode(code) + comment
}

func colorCode(code string) string {
	indent := code[:len(code)-len(strings.TrimLeft(code, " "))]
	trimmed := strings.TrimLeft(code, " ")
	prefix := ""
	if strings.HasPrefix(trimmed, "- ") {
		prefix = "- "
		trimmed = strings.TrimPrefix(trimmed, "- ")
	}

	colon := strings.Index(trimmed, ":")
	if colon < 0 {
		return code
	}

	key := trimmed[:colon]
	rest := trimmed[colon:]
	if strings.TrimSpace(strings.TrimPrefix(rest, ":")) == "" {
		return indent + prefix + keyStyle.Render(key) + colorMappingSeparator(rest)
	}
	return indent + prefix + keyStyle.Render(key) + colorValue(rest)
}

func colorMappingSeparator(rest string) string {
	if rest == "" {
		return rest
	}
	return syntaxStyle.Render(rest[:1]) + rest[1:]
}

func colorValue(rest string) string {
	value := strings.TrimPrefix(rest, ":")
	space := value[:len(value)-len(strings.TrimLeft(value, " "))]
	trimmed := strings.TrimLeft(value, " ")
	if trimmed == "" {
		return colorMappingSeparator(rest)
	}

	style := scalarStyle
	if strings.HasPrefix(trimmed, `"`) {
		style = stringStyle
	}
	if strings.HasPrefix(trimmed, "[") {
		return syntaxStyle.Render(":") + space + colorFlowValue(trimmed)
	}
	return syntaxStyle.Render(":") + space + style.Render(trimmed)
}

func colorFlowValue(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		switch value[i] {
		case '[', ']', ',':
			out.WriteString(syntaxStyle.Render(value[i : i+1]))
			i++
		case '"':
			start := i
			i++
			for i < len(value) {
				if value[i] == '\\' {
					i += 2
					continue
				}
				if value[i] == '"' {
					i++
					break
				}
				i++
			}
			out.WriteString(stringStyle.Render(value[start:i]))
		case ' ', '\t':
			out.WriteByte(value[i])
			i++
		default:
			start := i
			for i < len(value) && !strings.ContainsRune("[],\" \t", rune(value[i])) {
				i++
			}
			out.WriteString(scalarStyle.Render(value[start:i]))
		}
	}
	return out.String()
}
