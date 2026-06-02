package crd

import (
	apiextensionsinternal "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	upstreamschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apimachinery/pkg/runtime"

	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func toStructural(in *apiextensionsv1.JSONSchemaProps) (*docschema.Structural, error) {
	var internal apiextensionsinternal.JSONSchemaProps
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(in, &internal, nil); err != nil {
		return nil, err
	}
	structural, err := upstreamschema.NewStructural(&internal)
	if err != nil {
		return nil, err
	}
	return copyStructural(structural), nil
}

func copyStructural(in *upstreamschema.Structural) *docschema.Structural {
	if in == nil {
		return nil
	}

	return &docschema.Structural{
		Items:                copyStructural(in.Items),
		Properties:           copyStructuralProperties(in.Properties),
		AdditionalProperties: copyStructuralOrBool(in.AdditionalProperties),
		Generic: docschema.Generic{
			Description: in.Description,
			Type:        in.Type,
			Title:       in.Title,
			Default:     copyJSON(in.Default),
			Nullable:    in.Nullable,
			Examples:    copyExample(in.Example),
		},
		Extensions: docschema.Extensions{
			XPreserveUnknownFields: in.XPreserveUnknownFields,
			XEmbeddedResource:      in.XEmbeddedResource,
			XIntOrString:           in.XIntOrString,
			XListMapKeys:           append([]string(nil), in.XListMapKeys...),
			XListType:              copyStringPtr(in.XListType),
			XMapType:               copyStringPtr(in.XMapType),
		},
		ValidationExtensions: docschema.ValidationExtensions{
			XValidations: copyValidationRules(in.XValidations),
		},
		ValueValidation: copyValueValidation(in.ValueValidation),
	}
}

func copyStructuralProperties(in map[string]upstreamschema.Structural) map[string]docschema.Structural {
	if len(in) == 0 {
		return nil
	}

	out := make(map[string]docschema.Structural, len(in))
	for name, value := range in {
		out[name] = *copyStructural(&value)
	}
	return out
}

func copyStructuralOrBool(in *upstreamschema.StructuralOrBool) *docschema.StructuralOrBool {
	if in == nil {
		return nil
	}
	return &docschema.StructuralOrBool{
		Structural: copyStructural(in.Structural),
		Bool:       in.Bool,
	}
}

func copyValueValidation(in *upstreamschema.ValueValidation) *docschema.ValueValidation {
	if in == nil {
		return nil
	}
	return &docschema.ValueValidation{
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
		Enum:             copyJSONList(in.Enum),
		MaxProperties:    copyInt64Ptr(in.MaxProperties),
		MinProperties:    copyInt64Ptr(in.MinProperties),
		Required:         append([]string(nil), in.Required...),
	}
}

func copyValidationRules(in apiextensionsv1.ValidationRules) docschema.ValidationRules {
	if len(in) == 0 {
		return nil
	}

	out := make(docschema.ValidationRules, 0, len(in))
	for _, rule := range in {
		var reason *string
		if rule.Reason != nil {
			value := string(*rule.Reason)
			reason = &value
		}
		out = append(out, docschema.ValidationRule{
			Rule:              rule.Rule,
			Message:           rule.Message,
			MessageExpression: rule.MessageExpression,
			Reason:            reason,
			FieldPath:         rule.FieldPath,
			OptionalOldSelf:   copyBoolPtr(rule.OptionalOldSelf),
		})
	}
	return out
}

func copyJSONList(in []upstreamschema.JSON) []docschema.JSON {
	if len(in) == 0 {
		return nil
	}

	out := make([]docschema.JSON, 0, len(in))
	for _, value := range in {
		out = append(out, copyJSON(value))
	}
	return out
}

func copyExample(in upstreamschema.JSON) []docschema.Example {
	if in.Object == nil {
		return nil
	}
	return []docschema.Example{{Value: copyJSON(in)}}
}

func copyJSON(in upstreamschema.JSON) docschema.JSON {
	return docschema.JSON{Object: runtime.DeepCopyJSONValue(in.Object)}
}

func copyStringPtr(in *string) *string {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func copyBoolPtr(in *bool) *bool {
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
