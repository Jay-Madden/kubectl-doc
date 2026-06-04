package fielddetail

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sttts/kubectl-doc/internal/crd"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

type Field struct {
	ID          string
	Path        string
	Type        string
	Required    bool
	Description string
	Metadata    []string
}

func Collect(doc *crd.Document) []Field {
	if doc == nil {
		return nil
	}

	var fields []Field
	collect(doc, &fields, "apiVersion", doc.APIVersionSchema(), true)
	collect(doc, &fields, "kind", doc.KindSchema(), true)
	collect(doc, &fields, "metadata", doc.MetadataSchema(), true)
	if doc.Schema == nil {
		return fields
	}

	required := requiredSet(doc.Schema)
	for _, name := range sortedProperties(doc.Schema) {
		if name == "apiVersion" || name == "kind" || name == "metadata" {
			continue
		}
		field := doc.Schema.Properties[name]
		collect(doc, &fields, name, &field, required[name])
	}
	return fields
}

func ByPath(doc *crd.Document) map[string]Field {
	fields := map[string]Field{}
	for _, field := range Collect(doc) {
		fields[field.Path] = field
		if strings.Contains(field.Path, "[]") {
			fields[strings.ReplaceAll(field.Path, "[]", "")] = field
		}
	}
	return fields
}

func ID(doc *crd.Document, path string) string {
	return "field-" + Slug(apiVersion(doc.Group, doc.Version)+"-"+path)
}

func Slug(value string) string {
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

func collect(doc *crd.Document, fields *[]Field, path string, field *docschema.Structural, required bool) {
	description := ""
	if field != nil {
		description = strings.TrimSpace(field.Description)
	}
	*fields = append(*fields, Field{
		ID:          ID(doc, path),
		Path:        path,
		Type:        Type(field),
		Required:    required,
		Description: description,
		Metadata:    Metadata(field),
	})

	switch effectiveType(field) {
	case "object":
		childRequired := requiredSet(field)
		for _, name := range sortedProperties(field) {
			child := field.Properties[name]
			collect(doc, fields, path+"."+name, &child, childRequired[name])
		}
		if field.AdditionalProperties != nil && field.AdditionalProperties.Structural != nil {
			collect(doc, fields, path+".<key>", field.AdditionalProperties.Structural, false)
		}
	case "array":
		if field.Items == nil {
			return
		}
		itemPath := path + "[]"
		if effectiveType(field.Items) != "object" || len(field.Items.Properties) == 0 {
			collect(doc, fields, itemPath, field.Items, true)
			return
		}
		itemRequired := requiredSet(field.Items)
		for _, name := range sortedProperties(field.Items) {
			child := field.Items.Properties[name]
			collect(doc, fields, itemPath+"."+name, &child, itemRequired[name])
		}
	}
}

func Type(field *docschema.Structural) string {
	if field == nil {
		return "object"
	}
	if field.XIntOrString {
		return "int-or-string"
	}
	if field.Type == "array" && field.Items != nil {
		return "array<" + Type(field.Items) + ">"
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
		return "array<" + Type(field.Items) + ">"
	}
	return "object"
}

func Metadata(field *docschema.Structural) []string {
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

func apiVersion(group, version string) string {
	if group == "" {
		return version
	}
	return group + "/" + version
}
