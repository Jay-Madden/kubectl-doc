package tree

import (
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/crd"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func TestBuildLinesCarrySchemaMetadata(t *testing.T) {
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{
						Type:        "object",
						Description: "Spec configures the widget.",
					},
					Properties: map[string]docschema.Structural{
						"management_clusters": {
							Generic: docschema.Generic{Type: "array"},
							Items: &docschema.Structural{
								Generic: docschema.Generic{Type: "object"},
								Properties: map[string]docschema.Structural{
									"cluster_location": {
										Generic: docschema.Generic{Type: "string"},
									},
									"cluster_name": {
										Generic: docschema.Generic{Type: "string"},
									},
								},
							},
						},
						"template": {
							Generic: docschema.Generic{Type: "object"},
							Properties: map[string]docschema.Structural{
								"image": {
									Generic: docschema.Generic{Type: "string"},
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
			},
			ValueValidation: &docschema.ValueValidation{
				Required: []string{"spec"},
			},
		},
	}

	lines := Build(doc, Options{
		ExpandDepth:  3,
		Descriptions: DescriptionTrue,
	})
	spec, ok := findLine(lines, "spec")
	if !ok {
		t.Fatalf("expected spec line, got %#v", lines)
	}
	if !spec.Foldable || !spec.Required {
		t.Fatalf("expected required foldable spec line, got %#v", spec)
	}

	specDescription, ok := findText(lines, "# Spec configures the widget.")
	if !ok {
		t.Fatalf("expected spec description line, got %#v", lines)
	}
	if specDescription.Path != "spec" || specDescription.Field != "" {
		t.Fatalf("expected description to share spec path without becoming a field, got %#v", specDescription)
	}

	location, ok := findLine(lines, "spec.management_clusters[].cluster_location")
	if !ok {
		t.Fatalf("expected cluster_location line, got %#v", lines)
	}
	name, ok := findLine(lines, "spec.management_clusters[].cluster_name")
	if !ok {
		t.Fatalf("expected cluster_name line, got %#v", lines)
	}
	if location.Field != "cluster_location" || !strings.Contains(location.Text, `# - cluster_location: "<string>"`) {
		t.Fatalf("expected commented array item field, got %#v", location)
	}
	if location.Foldable {
		t.Fatalf("did not expect scalar array item field to be foldable, got %#v", location)
	}
	if location.Depth != name.Depth {
		t.Fatalf("expected array item sibling depths to match, got %d and %d", location.Depth, name.Depth)
	}
}

func findLine(lines []Line, path string) (Line, bool) {
	for _, line := range lines {
		if line.Path == path && line.Field != "" {
			return line, true
		}
	}
	return Line{}, false
}

func findText(lines []Line, text string) (Line, bool) {
	for _, line := range lines {
		if strings.TrimSpace(line.Text) == text {
			return line, true
		}
	}
	return Line{}, false
}
