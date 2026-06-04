package crd

import docschema "github.com/sttts/kubectl-doc/internal/schema"

// APIVersionSchema returns the schema used for the manifest apiVersion field.
func (d *Document) APIVersionSchema() *docschema.Structural {
	return d.typeMetaFieldSchema("apiVersion", "APIVersion defines the versioned schema of this representation of an object.")
}

// KindSchema returns the schema used for the manifest kind field.
func (d *Document) KindSchema() *docschema.Structural {
	return d.typeMetaFieldSchema("kind", "Kind is a string value representing the REST resource this object represents.")
}

func (d *Document) typeMetaFieldSchema(name, description string) *docschema.Structural {
	var field *docschema.Structural
	if d != nil && d.Schema != nil {
		if existing, ok := d.Schema.Properties[name]; ok {
			field = copyDocumentStructural(&existing)
		}
	}
	if field == nil {
		field = copyDocumentStructural(&docschema.Structural{
			Generic: docschema.Generic{
				Type:        "string",
				Description: description,
			},
		})
	}
	if field.Type == "" {
		field.Type = "string"
	}
	if field.Description == "" {
		field.Description = description
	}
	return field
}

// MetadataSchema returns the resource metadata schema used for authoring views.
func (d *Document) MetadataSchema() *docschema.Structural {
	fallback := objectMetaSchema()
	var metadata *docschema.Structural
	if d != nil && d.Schema != nil {
		if field, ok := d.Schema.Properties["metadata"]; ok {
			metadata = copyDocumentStructural(&field)
		}
	}
	if metadata == nil {
		metadata = fallback
	} else {
		mergeMetadataFallback(metadata, fallback)
	}

	if metadata.Type == "" {
		metadata.Type = "object"
	}
	if metadata.Properties == nil {
		metadata.Properties = map[string]docschema.Structural{}
	}

	// Native OpenAPI ObjectMeta wrapper defaults are schema artifacts, not authoring defaults.
	metadata.Default = docschema.JSON{}
	ensureMetadataField(metadata, "name", fallback.Properties["name"])
	if d != nil && d.Namespaced {
		ensureMetadataField(metadata, "namespace", fallback.Properties["namespace"])
	}
	requireMetadataFields(metadata, d != nil && d.Namespaced)
	return metadata
}

func objectMetaSchema() *docschema.Structural {
	return &docschema.Structural{
		Generic: docschema.Generic{
			Type:        "object",
			Description: "Standard Kubernetes object metadata.",
		},
		Properties: map[string]docschema.Structural{
			"annotations":                stringMapField("Annotations is an unstructured key value map stored with a resource."),
			"creationTimestamp":          timeField("CreationTimestamp is set by the server when a resource is created."),
			"deletionGracePeriodSeconds": int64Field("Number of seconds allowed for graceful deletion."),
			"deletionTimestamp":          timeField("DeletionTimestamp is set by the server when graceful deletion is requested."),
			"finalizers":                 stringArrayField("Finalizers must be empty before the object is deleted from the registry."),
			"generateName":               stringField("GenerateName is an optional prefix used by the server to generate a unique name."),
			"generation":                 int64Field("Generation is a sequence number representing a specific desired state."),
			"labels":                     stringMapField("Labels are key value pairs used to organize and select objects."),
			"managedFields":              managedFieldsField(),
			"name":                       stringField("Name must be unique within a namespace."),
			"namespace":                  stringField("Namespace defines the space within which each name must be unique."),
			"ownerReferences":            ownerReferencesField(),
			"resourceVersion":            stringField("ResourceVersion is an opaque internal version value."),
			"selfLink":                   stringField("SelfLink is a deprecated read-only field."),
			"uid":                        stringField("UID is the unique in time and space value for this object."),
		},
	}
}

func ensureMetadataField(metadata *docschema.Structural, name string, fallback docschema.Structural) {
	if metadata == nil {
		return
	}
	if existing, ok := metadata.Properties[name]; ok {
		mergeMetadataFallback(&existing, &fallback)
		metadata.Properties[name] = existing
		return
	}
	metadata.Properties[name] = fallback
}

func mergeMetadataFallback(field, fallback *docschema.Structural) {
	if field == nil || fallback == nil {
		return
	}
	if field.Description == "" {
		field.Description = fallback.Description
	}
	if field.Type == "" {
		field.Type = fallback.Type
	}
	if field.ValueValidation == nil {
		field.ValueValidation = copyDocumentValueValidation(fallback.ValueValidation)
	}
	if field.AdditionalProperties == nil {
		field.AdditionalProperties = copyDocumentStructuralOrBool(fallback.AdditionalProperties)
	} else if field.AdditionalProperties.Structural != nil && fallback.AdditionalProperties != nil && fallback.AdditionalProperties.Structural != nil {
		mergeMetadataFallback(field.AdditionalProperties.Structural, fallback.AdditionalProperties.Structural)
	}
	if field.Items == nil {
		field.Items = copyDocumentStructural(fallback.Items)
	} else {
		mergeMetadataFallback(field.Items, fallback.Items)
	}
	if len(fallback.Properties) == 0 {
		return
	}
	if field.Properties == nil {
		field.Properties = map[string]docschema.Structural{}
	}
	for name, fallbackChild := range fallback.Properties {
		if child, ok := field.Properties[name]; ok {
			mergeMetadataFallback(&child, &fallbackChild)
			field.Properties[name] = child
			continue
		}
		field.Properties[name] = *copyDocumentStructural(&fallbackChild)
	}
}

func requireMetadataFields(metadata *docschema.Structural, namespaced bool) {
	if metadata.ValueValidation == nil {
		metadata.ValueValidation = &docschema.ValueValidation{}
	}
	required := append([]string(nil), metadata.ValueValidation.Required...)
	required = appendRequired(required, "name")
	if namespaced {
		required = appendRequired(required, "namespace")
	}
	metadata.ValueValidation.Required = required
}

func appendRequired(required []string, name string) []string {
	for _, existing := range required {
		if existing == name {
			return required
		}
	}
	return append(required, name)
}

func stringField(description string) docschema.Structural {
	return docschema.Structural{
		Generic: docschema.Generic{
			Type:        "string",
			Description: description,
		},
	}
}

func timeField(description string) docschema.Structural {
	field := stringField(description)
	field.ValueValidation = &docschema.ValueValidation{Format: "date-time"}
	return field
}

func int64Field(description string) docschema.Structural {
	return docschema.Structural{
		Generic: docschema.Generic{
			Type:        "integer",
			Description: description,
		},
		ValueValidation: &docschema.ValueValidation{Format: "int64"},
	}
}

func stringArrayField(description string) docschema.Structural {
	return docschema.Structural{
		Generic: docschema.Generic{
			Type:        "array",
			Description: description,
		},
		Items: &docschema.Structural{
			Generic: docschema.Generic{Type: "string"},
		},
	}
}

func stringMapField(description string) docschema.Structural {
	return docschema.Structural{
		Generic: docschema.Generic{
			Type:        "object",
			Description: description,
		},
		AdditionalProperties: &docschema.StructuralOrBool{
			Structural: &docschema.Structural{
				Generic: docschema.Generic{Type: "string"},
			},
		},
	}
}

func ownerReferencesField() docschema.Structural {
	return docschema.Structural{
		Generic: docschema.Generic{
			Type:        "array",
			Description: "OwnerReferences lists objects depended on by this object.",
		},
		Items: &docschema.Structural{
			Generic: docschema.Generic{Type: "object"},
			Properties: map[string]docschema.Structural{
				"apiVersion":         stringField("API version of the referent."),
				"blockOwnerDeletion": boolField("BlockOwnerDeletion controls foreground deletion behavior."),
				"controller":         boolField("Controller marks the managing controller owner reference."),
				"kind":               stringField("Kind of the referent."),
				"name":               stringField("Name of the referent."),
				"uid":                stringField("UID of the referent."),
			},
			ValueValidation: &docschema.ValueValidation{
				Required: []string{"apiVersion", "kind", "name", "uid"},
			},
		},
	}
}

func managedFieldsField() docschema.Structural {
	return docschema.Structural{
		Generic: docschema.Generic{
			Type:        "array",
			Description: "ManagedFields records which actor manages which fields.",
		},
		Items: &docschema.Structural{
			Generic: docschema.Generic{Type: "object"},
			Properties: map[string]docschema.Structural{
				"apiVersion": stringField("APIVersion defines the version of this field set."),
				"fieldsType": stringField("FieldsType is the discriminator for the fields format."),
				"fieldsV1": {
					Generic: docschema.Generic{
						Type:        "object",
						Description: "FieldsV1 stores a versioned field set.",
					},
					Extensions: docschema.Extensions{XPreserveUnknownFields: true},
				},
				"manager":     stringField("Manager identifies the workflow managing these fields."),
				"operation":   stringField("Operation is the type of operation that produced this managedFields entry."),
				"subresource": stringField("Subresource is the name of the subresource used to update the object."),
				"time":        timeField("Time is when this managedFields entry was added."),
			},
		},
	}
}

func boolField(description string) docschema.Structural {
	return docschema.Structural{
		Generic: docschema.Generic{
			Type:        "boolean",
			Description: description,
		},
	}
}

func copyDocumentStructural(in *docschema.Structural) *docschema.Structural {
	if in == nil {
		return nil
	}

	return &docschema.Structural{
		Items:                copyDocumentStructural(in.Items),
		Properties:           copyDocumentStructuralProperties(in.Properties),
		AdditionalProperties: copyDocumentStructuralOrBool(in.AdditionalProperties),
		Generic: docschema.Generic{
			Description: in.Description,
			Type:        in.Type,
			Title:       in.Title,
			Default:     in.Default.DeepCopy(),
			Nullable:    in.Nullable,
			Examples:    copyDocumentExamples(in.Examples),
		},
		Extensions: docschema.Extensions{
			XPreserveUnknownFields: in.XPreserveUnknownFields,
			XEmbeddedResource:      in.XEmbeddedResource,
			XIntOrString:           in.XIntOrString,
			XListMapKeys:           append([]string(nil), in.XListMapKeys...),
			XListType:              copyDocumentStringPtr(in.XListType),
			XMapType:               copyDocumentStringPtr(in.XMapType),
		},
		ValidationExtensions: docschema.ValidationExtensions{
			XValidations: copyDocumentValidationRules(in.XValidations),
		},
		ValueValidation: copyDocumentValueValidation(in.ValueValidation),
	}
}

func copyDocumentStructuralProperties(in map[string]docschema.Structural) map[string]docschema.Structural {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]docschema.Structural, len(in))
	for name, field := range in {
		out[name] = *copyDocumentStructural(&field)
	}
	return out
}

func copyDocumentStructuralOrBool(in *docschema.StructuralOrBool) *docschema.StructuralOrBool {
	if in == nil {
		return nil
	}
	return &docschema.StructuralOrBool{
		Structural: copyDocumentStructural(in.Structural),
		Bool:       in.Bool,
	}
}

func copyDocumentValueValidation(in *docschema.ValueValidation) *docschema.ValueValidation {
	if in == nil {
		return nil
	}
	return &docschema.ValueValidation{
		Format:           in.Format,
		Maximum:          copyDocumentFloat64Ptr(in.Maximum),
		ExclusiveMaximum: in.ExclusiveMaximum,
		Minimum:          copyDocumentFloat64Ptr(in.Minimum),
		ExclusiveMinimum: in.ExclusiveMinimum,
		MaxLength:        copyDocumentInt64Ptr(in.MaxLength),
		MinLength:        copyDocumentInt64Ptr(in.MinLength),
		Pattern:          in.Pattern,
		MaxItems:         copyDocumentInt64Ptr(in.MaxItems),
		MinItems:         copyDocumentInt64Ptr(in.MinItems),
		UniqueItems:      in.UniqueItems,
		MultipleOf:       copyDocumentFloat64Ptr(in.MultipleOf),
		Enum:             copyDocumentJSONList(in.Enum),
		MaxProperties:    copyDocumentInt64Ptr(in.MaxProperties),
		MinProperties:    copyDocumentInt64Ptr(in.MinProperties),
		Required:         append([]string(nil), in.Required...),
	}
}

func copyDocumentExamples(in []docschema.Example) []docschema.Example {
	if len(in) == 0 {
		return nil
	}
	out := make([]docschema.Example, 0, len(in))
	for _, example := range in {
		out = append(out, docschema.Example{Name: example.Name, Value: example.Value.DeepCopy()})
	}
	return out
}

func copyDocumentJSONList(in []docschema.JSON) []docschema.JSON {
	if len(in) == 0 {
		return nil
	}
	out := make([]docschema.JSON, 0, len(in))
	for _, value := range in {
		out = append(out, value.DeepCopy())
	}
	return out
}

func copyDocumentValidationRules(in docschema.ValidationRules) docschema.ValidationRules {
	if len(in) == 0 {
		return nil
	}
	out := make(docschema.ValidationRules, 0, len(in))
	for _, rule := range in {
		out = append(out, docschema.ValidationRule{
			Rule:              rule.Rule,
			Message:           rule.Message,
			MessageExpression: rule.MessageExpression,
			Reason:            copyDocumentStringPtr(rule.Reason),
			FieldPath:         rule.FieldPath,
			OptionalOldSelf:   copyDocumentBoolPtr(rule.OptionalOldSelf),
		})
	}
	return out
}

func copyDocumentStringPtr(in *string) *string {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func copyDocumentBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func copyDocumentFloat64Ptr(in *float64) *float64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func copyDocumentInt64Ptr(in *int64) *int64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
