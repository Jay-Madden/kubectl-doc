package webschema

import (
	"testing"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/tree"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func TestBuildAndShallowPayload(t *testing.T) {
	minLength := int64(1)
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Plural:  "widgets",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{Type: "object", Description: "Spec describes the widget."},
					Properties: map[string]docschema.Structural{
						"mode": {
							Generic: docschema.Generic{Type: "string", Description: "Mode selects behavior."},
							ValueValidation: &docschema.ValueValidation{
								MinLength: &minLength,
							},
						},
					},
					ValueValidation: &docschema.ValueValidation{Required: []string{"mode"}},
				},
				"status": {
					Generic: docschema.Generic{Type: "object", Description: "Observed state."},
					Properties: map[string]docschema.Structural{
						"phase": {Generic: docschema.Generic{Type: "string", Description: "Current phase."}},
					},
				},
			},
			ValueValidation: &docschema.ValueValidation{Required: []string{"spec"}},
		},
	}

	full := Build(doc, Options{
		ExpandDepth:    1,
		Descriptions:   tree.DescriptionTrue,
		RenderStatus:   true,
		RenderMetadata: true,
	})
	if !full.Complete {
		t.Fatalf("full payload must be complete")
	}
	if full.APIVersion != "example.io/v1" || full.Kind != "Widget" || full.Resource != "widgets" {
		t.Fatalf("unexpected resource identity: %#v", full)
	}
	if field := fieldByPath(full.Fields, "spec.mode"); field == nil || !field.Required || field.Description == "" {
		t.Fatalf("expected spec.mode details to include required and description metadata, got %#v", field)
	}
	if !lineByPath(full.Lines, "status.phase") {
		t.Fatalf("full payload must include collapsed descendants")
	}

	shallow := Shallow(full, "./widget-schema-0-full.md")
	if shallow.Complete {
		t.Fatalf("shallow payload must not be complete")
	}
	if shallow.FullURL != "./widget-schema-0-full.md" {
		t.Fatalf("unexpected full payload URL: %q", shallow.FullURL)
	}
	if lineByPath(shallow.Lines, "status.phase") {
		t.Fatalf("shallow payload must omit descendants hidden by collapsed parents")
	}
	if field := fieldByPath(shallow.Fields, "spec.mode"); field == nil || !field.Required {
		t.Fatalf("shallow payload must keep referenced field details, got %#v", field)
	}
}

func fieldByPath(fields []FieldPayload, path string) *FieldPayload {
	for i := range fields {
		if fields[i].Path == path {
			return &fields[i]
		}
	}
	return nil
}

func lineByPath(lines []LinePayload, path string) bool {
	for _, line := range lines {
		if line.Path == path {
			return true
		}
	}
	return false
}
