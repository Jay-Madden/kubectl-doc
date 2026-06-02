package kube

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sttts/kubectl-doc/internal/crd"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

type openAPIDocument struct {
	Components openAPIComponents `json:"components"`
}

type openAPIComponents struct {
	Schemas map[string]*openAPISchema `json:"schemas"`
}

type openAPISchema struct {
	Ref                  string                    `json:"$ref"`
	Description          string                    `json:"description"`
	Type                 string                    `json:"type"`
	Title                string                    `json:"title"`
	Format               string                    `json:"format"`
	Default              interface{}               `json:"default"`
	Nullable             bool                      `json:"nullable"`
	Required             []string                  `json:"required"`
	Enum                 []interface{}             `json:"enum"`
	Properties           map[string]*openAPISchema `json:"properties"`
	Items                *openAPISchema            `json:"items"`
	AdditionalProperties json.RawMessage           `json:"additionalProperties"`

	Maximum          *float64 `json:"maximum"`
	ExclusiveMaximum bool     `json:"exclusiveMaximum"`
	Minimum          *float64 `json:"minimum"`
	ExclusiveMinimum bool     `json:"exclusiveMinimum"`
	MaxLength        *int64   `json:"maxLength"`
	MinLength        *int64   `json:"minLength"`
	Pattern          string   `json:"pattern"`
	MaxItems         *int64   `json:"maxItems"`
	MinItems         *int64   `json:"minItems"`
	UniqueItems      bool     `json:"uniqueItems"`
	MultipleOf       *float64 `json:"multipleOf"`
	MaxProperties    *int64   `json:"maxProperties"`
	MinProperties    *int64   `json:"minProperties"`

	XKubernetesGroupVersionKind []openAPIGVK `json:"x-kubernetes-group-version-kind"`
	XPreserveUnknownFields      bool         `json:"x-kubernetes-preserve-unknown-fields"`
	XEmbeddedResource           bool         `json:"x-kubernetes-embedded-resource"`
	XIntOrString                bool         `json:"x-kubernetes-int-or-string"`
	XListMapKeys                []string     `json:"x-kubernetes-list-map-keys"`
	XListType                   *string      `json:"x-kubernetes-list-type"`
	XMapType                    *string      `json:"x-kubernetes-map-type"`
}

type openAPIGVK struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

func BuildDocumentFromOpenAPIV3(data []byte, identity ResourceIdentity) (*crd.Document, error) {
	var document openAPIDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode OpenAPI v3 document: %w", err)
	}
	if len(document.Components.Schemas) == 0 {
		return nil, fmt.Errorf("OpenAPI v3 document has no component schemas")
	}

	name, schema := findResourceSchema(document.Components.Schemas, identity)
	if schema == nil {
		return nil, fmt.Errorf("OpenAPI v3 schema for %s not found", identity.String())
	}
	structural, err := openAPIConverter{schemas: document.Components.Schemas}.convert(schema, map[string]bool{})
	if err != nil {
		return nil, fmt.Errorf("convert OpenAPI schema %s: %w", name, err)
	}

	return &crd.Document{
		Source:  "cluster",
		Group:   identity.Group,
		Kind:    identity.Kind,
		Plural:  identity.Resource,
		Version: identity.Version,
		Schema:  structural,
	}, nil
}

func findResourceSchema(schemas map[string]*openAPISchema, identity ResourceIdentity) (string, *openAPISchema) {
	for name, schema := range schemas {
		for _, gvk := range schema.XKubernetesGroupVersionKind {
			if gvk.Group == identity.Group && gvk.Version == identity.Version && gvk.Kind == identity.Kind {
				return name, schema
			}
		}
	}
	return "", nil
}

type openAPIConverter struct {
	schemas map[string]*openAPISchema
}

func (c openAPIConverter) convert(in *openAPISchema, stack map[string]bool) (*docschema.Structural, error) {
	if in == nil {
		return nil, nil
	}
	if in.Ref != "" {
		name, err := componentNameFromRef(in.Ref)
		if err != nil {
			return nil, err
		}
		if stack[name] {
			return &docschema.Structural{
				Generic: docschema.Generic{
					Description: fmt.Sprintf("recursive reference to %s", name),
					Type:        "object",
				},
			}, nil
		}
		target := c.schemas[name]
		if target == nil {
			return nil, fmt.Errorf("component schema %q not found", name)
		}
		nextStack := copyRefStack(stack)
		nextStack[name] = true
		return c.convert(target, nextStack)
	}

	out := &docschema.Structural{
		Generic: docschema.Generic{
			Description: in.Description,
			Type:        in.Type,
			Title:       in.Title,
			Default:     docschema.JSON{Object: in.Default},
			Nullable:    in.Nullable,
		},
		Extensions: docschema.Extensions{
			XPreserveUnknownFields: in.XPreserveUnknownFields,
			XEmbeddedResource:      in.XEmbeddedResource,
			XIntOrString:           in.XIntOrString,
			XListMapKeys:           append([]string(nil), in.XListMapKeys...),
			XListType:              copyStringPtr(in.XListType),
			XMapType:               copyStringPtr(in.XMapType),
		},
		ValueValidation: &docschema.ValueValidation{
			Format:           in.Format,
			Maximum:          copyFloat64Ptr(in.Maximum),
			ExclusiveMaximum: in.ExclusiveMaximum,
			Minimum:          copyFloat64Ptr(in.Minimum),
			ExclusiveMinimum: in.ExclusiveMinimum,
			MaxLength:        copyInt64Ptr(in.MaxLength),
			MinLength:        copyInt64Ptr(in.MinLength),
			Pattern:          in.Pattern,
			MaxItems:         copyInt64Ptr(in.MaxItems),
			MinItems:         copyInt64Ptr(in.MinItems),
			UniqueItems:      in.UniqueItems,
			MultipleOf:       copyFloat64Ptr(in.MultipleOf),
			Enum:             copyOpenAPIEnum(in.Enum),
			MaxProperties:    copyInt64Ptr(in.MaxProperties),
			MinProperties:    copyInt64Ptr(in.MinProperties),
			Required:         append([]string(nil), in.Required...),
		},
	}

	if out.Type == "" && (len(in.Properties) > 0 || len(in.AdditionalProperties) > 0) {
		out.Type = "object"
	}
	if out.Type == "" && in.Items != nil {
		out.Type = "array"
	}

	var err error
	out.Properties, err = c.convertProperties(in.Properties, stack)
	if err != nil {
		return nil, err
	}
	out.Items, err = c.convert(in.Items, stack)
	if err != nil {
		return nil, err
	}
	out.AdditionalProperties, err = c.convertAdditionalProperties(in.AdditionalProperties, stack)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c openAPIConverter) convertProperties(in map[string]*openAPISchema, stack map[string]bool) (map[string]docschema.Structural, error) {
	if len(in) == 0 {
		return nil, nil
	}

	out := make(map[string]docschema.Structural, len(in))
	for name, schema := range in {
		converted, err := c.convert(schema, stack)
		if err != nil {
			return nil, fmt.Errorf("property %s: %w", name, err)
		}
		if converted != nil {
			out[name] = *converted
		}
	}
	return out, nil
}

func (c openAPIConverter) convertAdditionalProperties(in json.RawMessage, stack map[string]bool) (*docschema.StructuralOrBool, error) {
	if len(in) == 0 {
		return nil, nil
	}

	var boolean bool
	if err := json.Unmarshal(in, &boolean); err == nil {
		return &docschema.StructuralOrBool{Bool: boolean}, nil
	}

	var schema openAPISchema
	if err := json.Unmarshal(in, &schema); err != nil {
		return nil, fmt.Errorf("additionalProperties: %w", err)
	}
	converted, err := c.convert(&schema, stack)
	if err != nil {
		return nil, fmt.Errorf("additionalProperties: %w", err)
	}
	return &docschema.StructuralOrBool{Structural: converted}, nil
}

func componentNameFromRef(ref string) (string, error) {
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return "", fmt.Errorf("unsupported ref %q", ref)
	}
	name := strings.TrimPrefix(ref, prefix)
	if name == "" {
		return "", fmt.Errorf("empty component ref %q", ref)
	}
	return name, nil
}

func copyRefStack(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in)+1)
	for name, value := range in {
		out[name] = value
	}
	return out
}

func copyOpenAPIEnum(in []interface{}) []docschema.JSON {
	if len(in) == 0 {
		return nil
	}

	out := make([]docschema.JSON, 0, len(in))
	for _, value := range in {
		out = append(out, docschema.JSON{Object: value})
	}
	return out
}

func copyStringPtr(in *string) *string {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func copyFloat64Ptr(in *float64) *float64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func copyInt64Ptr(in *int64) *int64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
