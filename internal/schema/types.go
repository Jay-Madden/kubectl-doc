package schema

import "k8s.io/apimachinery/pkg/runtime"

// Structural is kubectl-doc's schema model. Its initial shape mirrors
// Kubernetes' CRD structural schema and can be extended for native resources.
type Structural struct {
	Items                *Structural
	Properties           map[string]Structural
	AdditionalProperties *StructuralOrBool

	Generic
	Extensions
	ValidationExtensions

	ValueValidation *ValueValidation
}

// StructuralOrBool is either a structural schema or a boolean.
type StructuralOrBool struct {
	Structural *Structural
	Bool       bool
}

// Generic contains generic schema fields that describe the value itself.
type Generic struct {
	Description string
	Type        string
	Title       string
	Default     JSON
	Nullable    bool
}

// Extensions contains Kubernetes OpenAPI v3 vendor extensions.
type Extensions struct {
	XPreserveUnknownFields bool
	XEmbeddedResource      bool
	XIntOrString           bool
	XListMapKeys           []string
	XListType              *string
	XMapType               *string
}

// ValidationExtensions contains Kubernetes validation extensions.
type ValidationExtensions struct {
	XValidations ValidationRules
}

// ValidationRules describes CEL validation rules without depending on CRD API types.
type ValidationRules []ValidationRule

// ValidationRule describes one CEL validation rule.
type ValidationRule struct {
	Rule              string
	Message           string
	MessageExpression string
	Reason            *string
	FieldPath         string
	OptionalOldSelf   *bool
}

// ValueValidation contains schema fields not contributing to the structure.
type ValueValidation struct {
	Format           string
	Maximum          *float64
	ExclusiveMaximum bool
	Minimum          *float64
	ExclusiveMinimum bool
	MaxLength        *int64
	MinLength        *int64
	Pattern          string
	MaxItems         *int64
	MinItems         *int64
	UniqueItems      bool
	MultipleOf       *float64
	Enum             []JSON
	MaxProperties    *int64
	MinProperties    *int64
	Required         []string
	AllOf            []NestedValueValidation
	OneOf            []NestedValueValidation
	AnyOf            []NestedValueValidation
	Not              *NestedValueValidation
}

// NestedValueValidation contains validations usable below logical junctors.
type NestedValueValidation struct {
	ValueValidation
	ValidationExtensions

	Items                *NestedValueValidation
	Properties           map[string]NestedValueValidation
	AdditionalProperties *NestedValueValidation

	ForbiddenGenerics   Generic
	ForbiddenExtensions Extensions
}

// JSON wraps an arbitrary JSON value.
type JSON struct {
	Object interface{}
}

func (j JSON) DeepCopy() JSON {
	return JSON{Object: runtime.DeepCopyJSONValue(j.Object)}
}
