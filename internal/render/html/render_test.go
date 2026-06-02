package htmlrender

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/crd"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func TestRenderFoldableSearchableHTML(t *testing.T) {
	var out bytes.Buffer
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Plural:  "widgets",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{
						Type:        "object",
						Description: "Spec describes the widget.",
					},
					Properties: map[string]docschema.Structural{
						"template": {
							Generic: docschema.Generic{
								Type:        "object",
								Description: "Template controls generated pods.",
							},
							Properties: map[string]docschema.Structural{
								"image": {
									Generic: docschema.Generic{
										Type:        "string",
										Description: "Container image.",
									},
								},
							},
						},
					},
				},
			},
			ValueValidation: &docschema.ValueValidation{
				Required: []string{"spec"},
			},
		},
	}

	if err := (Renderer{ExpandDepth: 1, Descriptions: yamlrender.DescriptionTrue}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	for _, expected := range []string{
		"<!doctype html>",
		"class=\"kubectl-doc\"",
		"data-kdoc-search",
		"data-kdoc-toggle",
		"aria-expanded=\"false\"",
		"Spec describes the widget.",
		"template:",
		"# Container image.",
		"#ff8c00",
		"fieldOnly",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected HTML to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.Contains(strings.ToLower(rendered), "copy") {
		t.Fatalf("HTML must not contain copy controls, got:\n%s", rendered)
	}
}
