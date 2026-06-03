package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/tree"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func TestModelNavigatesFieldsAndFolds(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})

	if model.FocusPath() != "metadata" {
		t.Fatalf("expected initial focus on metadata, got %q", model.FocusPath())
	}
	if !model.IsCollapsed("metadata") {
		t.Fatalf("expected metadata to start collapsed")
	}

	model = press(model, tea.Key{Code: tea.KeyRight})
	if model.FocusPath() != "metadata.name" {
		t.Fatalf("right should expand metadata and focus first child, got %q", model.FocusPath())
	}
	if model.IsCollapsed("metadata") {
		t.Fatalf("right should expand collapsed metadata")
	}

	model = press(model, tea.Key{Code: tea.KeyDown})
	if model.FocusPath() != "metadata.namespace" {
		t.Fatalf("down should move to next visible field, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyLeft})
	if model.FocusPath() != "metadata" {
		t.Fatalf("left should move to parent field, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyEnter})
	if !model.IsCollapsed("metadata") {
		t.Fatalf("enter should collapse a foldable focused field")
	}

	model = press(model, tea.Key{Code: tea.KeyTab})
	if model.FocusPath() != "spec" {
		t.Fatalf("tab should jump to next visible foldable field, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyTab, Mod: tea.ModShift})
	if model.FocusPath() != "metadata" {
		t.Fatalf("shift-tab should jump to previous visible foldable field, got %q", model.FocusPath())
	}
}

func TestModelHomeEndAndSearch(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})

	model = press(model, tea.Key{Code: tea.KeyEnd})
	if model.FocusPath() != "status" {
		t.Fatalf("end should focus last visible field, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyHome})
	if model.FocusPath() != "metadata" {
		t.Fatalf("home should focus first visible field, got %q", model.FocusPath())
	}

	model = pressText(model, "/")
	for _, r := range "template" {
		model = pressText(model, string(r))
	}
	if model.SearchQuery() != "template" {
		t.Fatalf("expected search query to be recorded, got %q", model.SearchQuery())
	}
	if model.FocusPath() != "spec.template" {
		t.Fatalf("search should focus matching field, got %q", model.FocusPath())
	}

	model = press(model, tea.Key{Code: tea.KeyEsc})
	model = pressText(model, "p")
	if model.FocusPath() != "spec.template" {
		t.Fatalf("single search match should stay focused on previous, got %q", model.FocusPath())
	}
}

func TestModelRendersDetailsAndResponsiveLayout(t *testing.T) {
	model := NewModel(testDocument(), Config{
		ExpandDepth:  2,
		Descriptions: tree.DescriptionTrue,
		Columns:      120,
	})
	model = press(model, tea.Key{Code: tea.KeyTab})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	model = updated.(Model)
	wide := model.view()
	for _, expected := range []string{
		"Widget example.io/v1",
		"Details",
		"Path: spec",
		"Spec configures the widget.",
	} {
		if !strings.Contains(wide, expected) {
			t.Fatalf("expected wide view to contain %q, got:\n%s", expected, wide)
		}
	}

	updated, _ = model.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
	model = updated.(Model)
	narrow := model.view()
	if !strings.Contains(narrow, "\n\nDetails") {
		t.Fatalf("expected narrow view to place details below schema, got:\n%s", narrow)
	}
}

func press(model Model, key tea.Key) Model {
	updated, _ := model.Update(tea.KeyPressMsg(key))
	return updated.(Model)
}

func pressText(model Model, text string) Model {
	return press(model, tea.Key{Code: []rune(text)[0], Text: text})
}

func testDocument() *crd.Document {
	return &crd.Document{
		Group:      "example.io",
		Version:    "v1",
		Kind:       "Widget",
		Plural:     "widgets",
		Namespaced: true,
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{
						Type:        "object",
						Description: "Spec configures the widget.",
					},
					Properties: map[string]docschema.Structural{
						"labels": {
							Generic: docschema.Generic{
								Type:        "object",
								Description: "Labels are copied to generated pods.",
							},
							AdditionalProperties: &docschema.StructuralOrBool{
								Structural: &docschema.Structural{
									Generic: docschema.Generic{Type: "string"},
								},
							},
						},
						"replicas": {
							Generic: docschema.Generic{
								Type:        "integer",
								Description: "Desired replica count.",
							},
						},
						"template": {
							Generic: docschema.Generic{
								Type:        "object",
								Description: "Template for generated pods.",
							},
							Properties: map[string]docschema.Structural{
								"image": {
									Generic: docschema.Generic{
										Type:        "string",
										Description: "Container image.",
									},
								},
							},
							ValueValidation: &docschema.ValueValidation{
								Required: []string{"image"},
							},
						},
					},
					ValueValidation: &docschema.ValueValidation{
						Required: []string{"template"},
					},
				},
				"status": {
					Generic: docschema.Generic{
						Type:        "object",
						Description: "Observed widget state.",
					},
					Properties: map[string]docschema.Structural{
						"phase": {
							Generic: docschema.Generic{Type: "string"},
						},
					},
				},
			},
			ValueValidation: &docschema.ValueValidation{
				Required: []string{"spec"},
			},
		},
	}
}
