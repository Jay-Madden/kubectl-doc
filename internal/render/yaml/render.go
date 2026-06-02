package yamlrender

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/sttts/kubectl-doc/internal/crd"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

type Renderer struct {
	ExpandDepth int
}

func (r Renderer) Render(out io.Writer, doc *crd.Document) error {
	lines := []string{
		fmt.Sprintf("apiVersion: %s/%s", doc.Group, doc.Version),
		fmt.Sprintf("kind: %s", doc.Kind),
		"metadata:",
		`  name: "<name>"`,
	}

	rootRequired := requiredSet(doc.Schema)
	for _, name := range sortedProperties(doc.Schema) {
		if name == "apiVersion" || name == "kind" || name == "metadata" {
			continue
		}
		child := doc.Schema.Properties[name]
		required := rootRequired[name]
		if name == "status" && !required {
			lines = append(lines, fmt.Sprintf("# status: {}%s", compactComment(&child)))
			continue
		}
		fieldLines := renderField(name, &child, 0, r.ExpandDepth, required)
		lines = append(lines, fieldLines...)
	}

	_, err := fmt.Fprintln(out, strings.Join(lines, "\n"))
	return err
}

func renderField(name string, field *docschema.Structural, depth, expandDepth int, required bool) []string {
	lines := renderFieldUncommented(name, field, depth, expandDepth)
	if required {
		return lines
	}
	return commentLines(lines)
}

func renderFieldUncommented(name string, field *docschema.Structural, depth, expandDepth int) []string {
	indent := strings.Repeat("  ", depth)
	comment := compactComment(field)

	switch effectiveType(field) {
	case "object":
		childNames := sortedProperties(field)
		if len(childNames) == 0 {
			if mapValue := mapValueSchema(field); mapValue != nil {
				lines := []string{fmt.Sprintf("%s%s:%s", indent, name, comment)}
				lines = append(lines, renderFieldUncommented("<key>", mapValue, depth+1, expandDepth)...)
				return lines
			}
			return []string{fmt.Sprintf("%s%s: {}%s", indent, name, comment)}
		}
		if depth >= expandDepth {
			return []string{fmt.Sprintf("%s%s: {}%s", indent, name, comment)}
		}

		lines := []string{fmt.Sprintf("%s%s:%s", indent, name, comment)}
		required := requiredSet(field)
		for _, childName := range orderProperties(childNames, required) {
			child := field.Properties[childName]
			lines = append(lines, renderField(childName, &child, depth+1, expandDepth, required[childName])...)
		}
		return lines
	case "array":
		lines := []string{fmt.Sprintf("%s%s:%s", indent, name, comment)}
		item := field.Items
		if item == nil {
			return append(lines, fmt.Sprintf("%s  - {}", indent))
		}
		if effectiveType(item) == "object" && len(item.Properties) > 0 && depth < expandDepth {
			itemRequired := requiredSet(item)
			first := true
			for _, childName := range orderProperties(sortedProperties(item), itemRequired) {
				child := item.Properties[childName]
				childLines := renderField(childName, &child, depth+2, expandDepth, itemRequired[childName])
				if first && len(childLines) > 0 {
					childLines[0] = strings.Replace(childLines[0], strings.Repeat("  ", depth+2), strings.Repeat("  ", depth+1)+"- ", 1)
					first = false
				}
				lines = append(lines, childLines...)
			}
			return lines
		}
		return append(lines, fmt.Sprintf("%s  - %s", indent, scalarValue(item)))
	default:
		return []string{fmt.Sprintf("%s%s: %s%s", indent, name, scalarValue(field), comment)}
	}
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
		return `"<string>"`
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

func compactComment(field *docschema.Structural) string {
	var comments []string
	if field == nil {
		return ""
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
	if len(comments) == 0 {
		return ""
	}
	return " # " + strings.Join(comments, ", ")
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
