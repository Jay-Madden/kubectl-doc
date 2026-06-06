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
		Group:      "example.io",
		Version:    "v1",
		Kind:       "Widget",
		Plural:     "widgets",
		Namespaced: true,
		Schema: &docschema.Structural{
			Generic: docschema.Generic{
				Description: "Widget declares a deliberately long root description that wraps across several logical YAML comment lines.",
			},
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
		Columns:        48,
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
	if !lineByPath(full.Lines, "metadata.name") || !lineByPath(full.Lines, "metadata.namespace") {
		t.Fatalf("full payload must include generated metadata descendants")
	}
	rootDescriptionLines := 0
	rootDescriptionGroup := ""
	for _, line := range full.Lines {
		if line.Path == "" && line.Comment != nil {
			rootDescriptionLines++
			if line.DetailID != rootDescriptionDetailID {
				t.Fatalf("root description lines must share %q, got %#v", rootDescriptionDetailID, line)
			}
			if line.CommentGroup == "" {
				t.Fatalf("root description lines must carry a comment group, got %#v", line)
			}
			if rootDescriptionGroup == "" {
				rootDescriptionGroup = line.CommentGroup
			} else if line.CommentGroup != rootDescriptionGroup {
				t.Fatalf("root description paragraph lines must share one comment group, got %q and %q", rootDescriptionGroup, line.CommentGroup)
			}
		}
	}
	if rootDescriptionLines < 2 {
		t.Fatalf("expected wrapped root description payload lines, got %#v", full.Lines)
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
	if !lineByPath(shallow.Lines, "metadata.name") || !lineByPath(shallow.Lines, "metadata.namespace") {
		t.Fatalf("shallow payload must keep metadata descendants for local expansion")
	}
	if field := fieldByPath(shallow.Fields, "spec.mode"); field == nil || !field.Required {
		t.Fatalf("shallow payload must keep referenced field details, got %#v", field)
	}
	if field := fieldByPath(shallow.Fields, "metadata.name"); field == nil || !field.Required {
		t.Fatalf("shallow payload must keep metadata field details, got %#v", field)
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
