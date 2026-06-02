package kube

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/sttts/kubectl-doc/internal/crd"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

type openAPIDocument struct {
	Paths      map[string]openAPIPathItem `json:"paths"`
	Components openAPIComponents          `json:"components"`
}

type openAPIComponents struct {
	Schemas map[string]*openAPISchema `json:"schemas"`
}

type openAPIPathItem struct {
	Get    *openAPIOperation `json:"get"`
	Put    *openAPIOperation `json:"put"`
	Post   *openAPIOperation `json:"post"`
	Patch  *openAPIOperation `json:"patch"`
	Delete *openAPIOperation `json:"delete"`
}

type openAPIOperation struct {
	RequestBody                 *openAPIRequestBody        `json:"requestBody"`
	Responses                   map[string]openAPIResponse `json:"responses"`
	XKubernetesAction           string                     `json:"x-kubernetes-action"`
	XKubernetesGroupVersionKind openAPIGVK                 `json:"x-kubernetes-group-version-kind"`
}

type openAPIRequestBody struct {
	Content map[string]openAPIMediaType `json:"content"`
}

type openAPIResponse struct {
	Content map[string]openAPIMediaType `json:"content"`
}

type openAPIMediaType struct {
	Schema *openAPISchema `json:"schema"`
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

	XKubernetesGroupVersionKind openAPIGVKList `json:"x-kubernetes-group-version-kind"`
	XPreserveUnknownFields      bool           `json:"x-kubernetes-preserve-unknown-fields"`
	XEmbeddedResource           bool           `json:"x-kubernetes-embedded-resource"`
	XIntOrString                bool           `json:"x-kubernetes-int-or-string"`
	XListMapKeys                []string       `json:"x-kubernetes-list-map-keys"`
	XListType                   *string        `json:"x-kubernetes-list-type"`
	XMapType                    *string        `json:"x-kubernetes-map-type"`
}

type openAPIGVK struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

type openAPIGVKList []openAPIGVK

func (l *openAPIGVKList) UnmarshalJSON(data []byte) error {
	var list []openAPIGVK
	if err := json.Unmarshal(data, &list); err == nil {
		*l = list
		return nil
	}

	var single openAPIGVK
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	*l = []openAPIGVK{single}
	return nil
}

func BuildDocumentFromOpenAPIV3(data []byte, identity ResourceIdentity) (*crd.Document, error) {
	var document openAPIDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode OpenAPI v3 document: %w", err)
	}
	if len(document.Components.Schemas) == 0 {
		return nil, fmt.Errorf("OpenAPI v3 document has no component schemas")
	}

	name, schema := findResourceSchema(document, identity)
	if schema == nil {
		return nil, &OpenAPISchemaNotFoundError{Identity: identity}
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

type OpenAPISchemaNotFoundError struct {
	Identity ResourceIdentity
}

func (e *OpenAPISchemaNotFoundError) Error() string {
	return fmt.Sprintf("OpenAPI v3 schema for %s not found", e.Identity.String())
}

func IsOpenAPISchemaNotFound(err error) bool {
	var notFound *OpenAPISchemaNotFoundError
	return errors.As(err, &notFound)
}

func BuildDocumentFromOpenAPIV3WithNativeFallback(data []byte, identity ResourceIdentity) (*crd.Document, error) {
	doc, err := BuildDocumentFromOpenAPIV3(data, identity)
	if err == nil {
		return doc, nil
	}
	if !IsOpenAPISchemaNotFound(err) {
		return nil, err
	}

	fallback, ok := NativeOpenAPIV3Document(identity.Group, identity.Version)
	if !ok {
		return nil, err
	}
	doc, fallbackErr := BuildDocumentFromOpenAPIV3(fallback, identity)
	if fallbackErr != nil {
		return nil, fmt.Errorf("%w; embedded native OpenAPI v3 fallback also failed: %v", err, fallbackErr)
	}
	doc.Source = "embedded-native-openapi"
	return doc, nil
}

func findResourceSchema(document openAPIDocument, identity ResourceIdentity) (string, *openAPISchema) {
	if name, schema := findComponentResourceSchema(document.Components.Schemas, identity); schema != nil {
		return name, schema
	}
	return findOperationResourceSchema(document.Paths, identity)
}

func findComponentResourceSchema(schemas map[string]*openAPISchema, identity ResourceIdentity) (string, *openAPISchema) {
	for name, schema := range schemas {
		for _, gvk := range schema.XKubernetesGroupVersionKind {
			if matchesGVK(gvk, identity) {
				return name, schema
			}
		}
	}
	return "", nil
}

func findOperationResourceSchema(paths map[string]openAPIPathItem, identity ResourceIdentity) (string, *openAPISchema) {
	priorities := []struct {
		action string
		method string
		schema func(*openAPIOperation) *openAPISchema
	}{
		{action: "post", method: "post", schema: requestBodySchema},
		{action: "put", method: "put", schema: requestBodySchema},
		{action: "get", method: "get", schema: okResponseSchema},
	}

	pathNames := sortedOpenAPIPathNames(paths)
	for _, priority := range priorities {
		for _, pathName := range pathNames {
			operation := paths[pathName].operation(priority.method)
			if operation == nil || operation.XKubernetesAction != priority.action || !matchesGVK(operation.XKubernetesGroupVersionKind, identity) {
				continue
			}
			schema := priority.schema(operation)
			if schema != nil {
				return fmt.Sprintf("%s %s %s", priority.method, pathName, priority.action), schema
			}
		}
	}
	return "", nil
}

func (p openAPIPathItem) operation(method string) *openAPIOperation {
	switch method {
	case "get":
		return p.Get
	case "put":
		return p.Put
	case "post":
		return p.Post
	case "patch":
		return p.Patch
	case "delete":
		return p.Delete
	default:
		return nil
	}
}

func requestBodySchema(operation *openAPIOperation) *openAPISchema {
	if operation == nil || operation.RequestBody == nil {
		return nil
	}
	return schemaFromContent(operation.RequestBody.Content)
}

func okResponseSchema(operation *openAPIOperation) *openAPISchema {
	if operation == nil {
		return nil
	}
	response, ok := operation.Responses["200"]
	if !ok {
		return nil
	}
	return schemaFromContent(response.Content)
}

func schemaFromContent(content map[string]openAPIMediaType) *openAPISchema {
	for _, contentType := range []string{"application/json", "*/*", "application/yaml", "application/vnd.kubernetes.protobuf"} {
		if mediaType, ok := content[contentType]; ok && mediaType.Schema != nil {
			return mediaType.Schema
		}
	}

	contentTypes := make([]string, 0, len(content))
	for contentType := range content {
		contentTypes = append(contentTypes, contentType)
	}
	sort.Strings(contentTypes)
	for _, contentType := range contentTypes {
		if content[contentType].Schema != nil {
			return content[contentType].Schema
		}
	}
	return nil
}

func matchesGVK(gvk openAPIGVK, identity ResourceIdentity) bool {
	return gvk.Group == identity.Group && gvk.Version == identity.Version && gvk.Kind == identity.Kind
}

func sortedOpenAPIPathNames(paths map[string]openAPIPathItem) []string {
	out := make([]string, 0, len(paths))
	for pathName := range paths {
		out = append(out, pathName)
	}
	sort.Strings(out)
	return out
}

type openAPIConverter struct {
	schemas map[string]*openAPISchema
}

func (c openAPIConverter) convert(in *openAPISchema, stack map[string]bool) (*docschema.Structural, error) {
	if in == nil {
		return nil, nil
	}
	if in.Ref != "" {
		out, err := c.convertRef(in.Ref, stack)
		if err != nil {
			return nil, err
		}
		applyRefWrapperMetadata(out, in)
		return out, nil
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
			XIntOrString:           isOpenAPIIntOrString(in),
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

func (c openAPIConverter) convertRef(ref string, stack map[string]bool) (*docschema.Structural, error) {
	name, err := componentNameFromRef(ref)
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

func applyRefWrapperMetadata(out *docschema.Structural, in *openAPISchema) {
	if out == nil || in == nil {
		return
	}
	if in.Description != "" {
		out.Description = in.Description
	}
	if in.Title != "" {
		out.Title = in.Title
	}
	if in.Type != "" {
		out.Type = in.Type
	}
	if in.Default != nil {
		out.Default = docschema.JSON{Object: in.Default}
	}
	if in.Nullable {
		out.Nullable = true
	}
	if in.XPreserveUnknownFields {
		out.XPreserveUnknownFields = true
	}
	if in.XEmbeddedResource {
		out.XEmbeddedResource = true
	}
	if isOpenAPIIntOrString(in) {
		out.XIntOrString = true
	}
	if len(in.XListMapKeys) > 0 {
		out.XListMapKeys = append([]string(nil), in.XListMapKeys...)
	}
	if in.XListType != nil {
		out.XListType = copyStringPtr(in.XListType)
	}
	if in.XMapType != nil {
		out.XMapType = copyStringPtr(in.XMapType)
	}
	if in.Format != "" || len(in.Required) > 0 {
		if out.ValueValidation == nil {
			out.ValueValidation = &docschema.ValueValidation{}
		}
		if in.Format != "" {
			out.ValueValidation.Format = in.Format
		}
		if len(in.Required) > 0 {
			out.ValueValidation.Required = append([]string(nil), in.Required...)
		}
	}
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

func isOpenAPIIntOrString(in *openAPISchema) bool {
	return in != nil && (in.XIntOrString || in.Format == "int-or-string")
}
