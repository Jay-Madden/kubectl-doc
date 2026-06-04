package krorender

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/sttts/kubectl-doc/internal/crd"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

type Renderer struct {
	Descriptions yamlrender.DescriptionMode
}

func (r Renderer) Render(out io.Writer, doc *crd.Document) error {
	return r.RenderAll(out, []*crd.Document{doc})
}

func (r Renderer) RenderAll(out io.Writer, docs []*crd.Document) error {
	docs = compactDocuments(docs)
	if len(docs) == 0 {
		return fmt.Errorf("at least one document is required")
	}

	for i, doc := range docs {
		if i > 0 {
			if _, err := fmt.Fprintln(out, "---"); err != nil {
				return err
			}
		}
		if err := r.renderDocument(out, doc); err != nil {
			return err
		}
	}
	return nil
}

func (r Renderer) renderDocument(out io.Writer, doc *crd.Document) error {
	lines := rootDescriptionLines(doc, r.descriptionMode())
	lines = append(lines,
		fmt.Sprintf("apiVersion: %s", apiVersion(doc.Group, doc.Version)),
		fmt.Sprintf("kind: %s", doc.Kind),
	)

	rootRequired := requiredSet(doc.Schema)
	for _, name := range orderProperties(sortedProperties(doc.Schema), rootRequired) {
		if name == "apiVersion" || name == "kind" || name == "metadata" {
			continue
		}
		child := doc.Schema.Properties[name]
		lines = append(lines, renderField(name, &child, 0, rootRequired[name], r.descriptionMode())...)
	}

	_, err := fmt.Fprintln(out, strings.Join(lines, "\n"))
	return err
}

func rootDescriptionLines(doc *crd.Document, descriptions yamlrender.DescriptionMode) []string {
	if !showDescription(descriptions, true) || doc == nil || doc.Schema == nil || strings.TrimSpace(doc.Schema.Description) == "" {
		return nil
	}

	var lines []string
	var paragraph []string
	flush := func() {
		if len(paragraph) == 0 {
			return
		}
		lines = append(lines, "# "+strings.Join(paragraph, " "))
		paragraph = nil
	}
	for _, raw := range strings.Split(strings.TrimSpace(doc.Schema.Description), "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			flush()
			lines = append(lines, "#")
			continue
		}
		paragraph = append(paragraph, trimmed)
	}
	flush()
	return lines
}

func (r Renderer) descriptionMode() yamlrender.DescriptionMode {
	if r.Descriptions == "" {
		return yamlrender.DescriptionTrue
	}
	return r.Descriptions
}

func renderField(name string, field *docschema.Structural, depth int, required bool, descriptions yamlrender.DescriptionMode) []string {
	indent := strings.Repeat("  ", depth)
	switch effectiveType(field) {
	case "object":
		if len(field.Properties) == 0 {
			return []string{fmt.Sprintf("%s%s: %s", indent, yamlKey(name), schemaValue(kroType(field), markers(field, required, descriptions)))}
		}
		line := fmt.Sprintf("%s%s:", indent, yamlKey(name))
		if suffix := commentSuffix(markers(field, required, descriptions), extensionNotes(field)); suffix != "" {
			line += suffix
		}
		lines := []string{line}
		requiredChildren := requiredSet(field)
		for _, childName := range orderProperties(sortedProperties(field), requiredChildren) {
			child := field.Properties[childName]
			lines = append(lines, renderField(childName, &child, depth+1, requiredChildren[childName], descriptions)...)
		}
		return lines
	case "array":
		if field.Items != nil && effectiveType(field.Items) == "object" && len(field.Items.Properties) > 0 {
			line := fmt.Sprintf("%s%s:", indent, yamlKey(name))
			if suffix := commentSuffix(markers(field, required, descriptions), extensionNotes(field)); suffix != "" {
				line += suffix
			}
			lines := []string{line}
			requiredChildren := requiredSet(field.Items)
			first := true
			for _, childName := range orderProperties(sortedProperties(field.Items), requiredChildren) {
				child := field.Items.Properties[childName]
				childLines := renderField(childName, &child, depth+2, requiredChildren[childName], descriptions)
				if first {
					childLines = markSequenceItem(childLines, depth)
					first = false
				}
				lines = append(lines, childLines...)
			}
			return lines
		}

		line := fmt.Sprintf("%s%s: %s", indent, yamlKey(name), schemaValue(kroType(field), markers(field, required, descriptions)))
		if suffix := commentSuffix(nil, extensionNotes(field)); suffix != "" {
			line += suffix
		}
		return []string{line}
	default:
		line := fmt.Sprintf("%s%s: %s", indent, yamlKey(name), schemaValue(kroType(field), markers(field, required, descriptions)))
		if suffix := commentSuffix(nil, extensionNotes(field)); suffix != "" {
			line += suffix
		}
		return []string{line}
	}
}

func kroType(field *docschema.Structural) string {
	if field == nil {
		return "object"
	}
	if field.XIntOrString {
		return "string"
	}
	switch effectiveType(field) {
	case "array":
		if field.Items == nil {
			return "[]object"
		}
		return "[]" + kroType(field.Items)
	case "object":
		if mapValue := mapValueSchema(field); mapValue != nil && len(field.Properties) == 0 {
			return "map[string]" + kroType(mapValue)
		}
		return "object"
	case "integer":
		return "integer"
	case "number":
		return "float"
	case "boolean":
		return "boolean"
	case "string":
		return "string"
	default:
		if field.Type != "" {
			return field.Type
		}
		return "object"
	}
}

func schemaValue(typeName string, markers []string) string {
	value := typeName
	if len(markers) > 0 {
		value += " | " + strings.Join(markers, " ")
	}
	if needsQuotedSchemaValue(value) {
		return strconv.Quote(value)
	}
	return value
}

func needsQuotedSchemaValue(value string) bool {
	return strings.HasPrefix(value, "[]") || strings.HasPrefix(value, "map[")
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

func markers(field *docschema.Structural, required bool, descriptions yamlrender.DescriptionMode) []string {
	var out []string
	if required {
		out = append(out, "required=true")
	}
	if field == nil {
		return out
	}
	if field.Default.Object != nil {
		out = append(out, "default="+jsonValue(field.Default.Object))
	} else if example, ok := selectedExample(field); ok {
		out = append(out, "default="+jsonValue(example.Value.Object))
	}
	if field.ValueValidation != nil {
		validation := field.ValueValidation
		if len(validation.Enum) > 0 {
			out = append(out, "enum="+quoteMarkerValue(enumValue(validation.Enum)))
		}
		if validation.Minimum != nil {
			out = append(out, "minimum="+trimFloat(*validation.Minimum))
		}
		if validation.Maximum != nil {
			out = append(out, "maximum="+trimFloat(*validation.Maximum))
		}
		if validation.MinLength != nil {
			out = append(out, fmt.Sprintf("minLength=%d", *validation.MinLength))
		}
		if validation.MaxLength != nil {
			out = append(out, fmt.Sprintf("maxLength=%d", *validation.MaxLength))
		}
		if validation.Pattern != "" {
			out = append(out, "pattern="+quoteMarkerValue(validation.Pattern))
		}
		if validation.MinItems != nil {
			out = append(out, fmt.Sprintf("minItems=%d", *validation.MinItems))
		}
		if validation.MaxItems != nil {
			out = append(out, fmt.Sprintf("maxItems=%d", *validation.MaxItems))
		}
		if validation.UniqueItems {
			out = append(out, "uniqueItems=true")
		}
		if validation.Format != "" {
			out = append(out, "format="+validation.Format)
		}
	}
	if showDescription(descriptions, required) && strings.TrimSpace(field.Description) != "" {
		out = append(out, "description="+quoteMarkerValue(compactDescription(field.Description)))
	}
	return out
}

func showDescription(mode yamlrender.DescriptionMode, required bool) bool {
	switch mode {
	case yamlrender.DescriptionFalse:
		return false
	case yamlrender.DescriptionRequired:
		return required
	default:
		return true
	}
}

func extensionNotes(field *docschema.Structural) []string {
	if field == nil {
		return nil
	}
	var notes []string
	if field.Nullable {
		notes = append(notes, "nullable")
	}
	if field.XPreserveUnknownFields {
		notes = append(notes, "x-kubernetes-preserve-unknown-fields")
	}
	if field.XEmbeddedResource {
		notes = append(notes, "x-kubernetes-embedded-resource")
	}
	if field.XIntOrString {
		notes = append(notes, "x-kubernetes-int-or-string")
	}
	if field.XListType != nil {
		notes = append(notes, "x-kubernetes-list-type="+*field.XListType)
	}
	if len(field.XListMapKeys) > 0 {
		notes = append(notes, "x-kubernetes-list-map-keys="+strings.Join(field.XListMapKeys, ","))
	}
	if field.XMapType != nil {
		notes = append(notes, "x-kubernetes-map-type="+*field.XMapType)
	}
	if len(field.XValidations) > 0 {
		notes = append(notes, "x-kubernetes-validations")
	}
	if field.ValueValidation != nil {
		if field.ValueValidation.ExclusiveMinimum {
			notes = append(notes, "exclusiveMinimum")
		}
		if field.ValueValidation.ExclusiveMaximum {
			notes = append(notes, "exclusiveMaximum")
		}
		if field.ValueValidation.MultipleOf != nil {
			notes = append(notes, "multipleOf="+trimFloat(*field.ValueValidation.MultipleOf))
		}
	}
	if len(notes) == 0 {
		return nil
	}
	return notes
}

func commentSuffix(markers, notes []string) string {
	parts := append([]string(nil), markers...)
	parts = append(parts, notes...)
	if len(parts) == 0 {
		return ""
	}
	return " # " + strings.Join(parts, " ")
}

func enumValue(values []docschema.JSON) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		switch typed := value.Object.(type) {
		case string:
			parts = append(parts, typed)
		default:
			parts = append(parts, jsonValue(typed))
		}
	}
	return strings.Join(parts, ",")
}

func quoteMarkerValue(value string) string {
	return strconv.Quote(value)
}

func jsonValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strconv.Quote(typed)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return "{}"
		}
		return string(data)
	}
}

func compactDescription(value string) string {
	return strings.Join(strings.Fields(value), " ")
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

func apiVersion(group, version string) string {
	if group == "" {
		return version
	}
	return group + "/" + version
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

func mapValueSchema(field *docschema.Structural) *docschema.Structural {
	if field == nil || field.AdditionalProperties == nil || field.AdditionalProperties.Structural == nil {
		return nil
	}
	return field.AdditionalProperties.Structural
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

func yamlKey(value string) string {
	if value == "" {
		return strconv.Quote(value)
	}
	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return strconv.Quote(value)
		}
	}
	return value
}

func trimFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
