package jsonschemarender

import (
	"fmt"
	"io"

	"github.com/sttts/kubectl-doc/internal/crd"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
	"sigs.k8s.io/yaml"
)

type Renderer struct{}

func (r Renderer) Render(out io.Writer, doc *crd.Document) error {
	return r.RenderAll(out, []*crd.Document{doc})
}

func (r Renderer) RenderAll(out io.Writer, docs []*crd.Document) error {
	if len(docs) == 0 {
		return fmt.Errorf("at least one document is required")
	}
	for i, doc := range docs {
		if i > 0 {
			if _, err := fmt.Fprintln(out, "---"); err != nil {
				return err
			}
		}
		data, err := yaml.Marshal(schemaObject(doc.Schema))
		if err != nil {
			return err
		}
		if _, err := out.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func schemaObject(schema *docschema.Structural) map[string]interface{} {
	out := map[string]interface{}{}
	if schema == nil {
		return out
	}

	addString(out, "title", schema.Title)
	addString(out, "description", schema.Description)
	addString(out, "type", schema.Type)
	if schema.Default.Object != nil {
		out["default"] = schema.Default.Object
	}
	if schema.Nullable {
		out["nullable"] = true
	}
	addExamples(out, schema.Examples)
	addValidation(out, schema.ValueValidation)
	addExtensions(out, schema)

	if schema.Items != nil {
		out["items"] = schemaObject(schema.Items)
	}
	if len(schema.Properties) > 0 {
		properties := map[string]interface{}{}
		for name, field := range schema.Properties {
			field := field
			properties[name] = schemaObject(&field)
		}
		out["properties"] = properties
	}
	if schema.AdditionalProperties != nil {
		switch {
		case schema.AdditionalProperties.Structural != nil:
			out["additionalProperties"] = schemaObject(schema.AdditionalProperties.Structural)
		case schema.AdditionalProperties.Bool:
			out["additionalProperties"] = true
		default:
			out["additionalProperties"] = false
		}
	}
	return out
}

func addString(out map[string]interface{}, name, value string) {
	if value != "" {
		out[name] = value
	}
}

func addExamples(out map[string]interface{}, examples []docschema.Example) {
	if len(examples) == 0 {
		return
	}
	if len(examples) == 1 && examples[0].Name == "" {
		out["example"] = examples[0].Value.Object
		return
	}

	values := make([]interface{}, 0, len(examples))
	for _, example := range examples {
		if example.Name == "" {
			values = append(values, example.Value.Object)
			continue
		}
		values = append(values, map[string]interface{}{
			"name":  example.Name,
			"value": example.Value.Object,
		})
	}
	out["examples"] = values
}

func addValidation(out map[string]interface{}, validation *docschema.ValueValidation) {
	if validation == nil {
		return
	}
	addString(out, "format", validation.Format)
	addFloat(out, "maximum", validation.Maximum)
	if validation.ExclusiveMaximum {
		out["exclusiveMaximum"] = true
	}
	addFloat(out, "minimum", validation.Minimum)
	if validation.ExclusiveMinimum {
		out["exclusiveMinimum"] = true
	}
	addInt(out, "maxLength", validation.MaxLength)
	addInt(out, "minLength", validation.MinLength)
	addString(out, "pattern", validation.Pattern)
	addInt(out, "maxItems", validation.MaxItems)
	addInt(out, "minItems", validation.MinItems)
	if validation.UniqueItems {
		out["uniqueItems"] = true
	}
	addFloat(out, "multipleOf", validation.MultipleOf)
	if len(validation.Enum) > 0 {
		values := make([]interface{}, 0, len(validation.Enum))
		for _, value := range validation.Enum {
			values = append(values, value.Object)
		}
		out["enum"] = values
	}
	addInt(out, "maxProperties", validation.MaxProperties)
	addInt(out, "minProperties", validation.MinProperties)
	if len(validation.Required) > 0 {
		out["required"] = append([]string(nil), validation.Required...)
	}
}

func addExtensions(out map[string]interface{}, schema *docschema.Structural) {
	if schema.XPreserveUnknownFields {
		out["x-kubernetes-preserve-unknown-fields"] = true
	}
	if schema.XEmbeddedResource {
		out["x-kubernetes-embedded-resource"] = true
	}
	if schema.XIntOrString {
		out["x-kubernetes-int-or-string"] = true
	}
	if len(schema.XListMapKeys) > 0 {
		out["x-kubernetes-list-map-keys"] = append([]string(nil), schema.XListMapKeys...)
	}
	if schema.XListType != nil {
		out["x-kubernetes-list-type"] = *schema.XListType
	}
	if schema.XMapType != nil {
		out["x-kubernetes-map-type"] = *schema.XMapType
	}
	if len(schema.XValidations) > 0 {
		validations := make([]interface{}, 0, len(schema.XValidations))
		for _, validation := range schema.XValidations {
			item := map[string]interface{}{"rule": validation.Rule}
			addString(item, "message", validation.Message)
			addString(item, "messageExpression", validation.MessageExpression)
			if validation.Reason != nil {
				item["reason"] = *validation.Reason
			}
			addString(item, "fieldPath", validation.FieldPath)
			if validation.OptionalOldSelf != nil {
				item["optionalOldSelf"] = *validation.OptionalOldSelf
			}
			validations = append(validations, item)
		}
		out["x-kubernetes-validations"] = validations
	}
}

func addFloat(out map[string]interface{}, name string, value *float64) {
	if value != nil {
		out[name] = *value
	}
}

func addInt(out map[string]interface{}, name string, value *int64) {
	if value != nil {
		out[name] = *value
	}
}
