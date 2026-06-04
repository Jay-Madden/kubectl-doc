package crd

import (
	"testing"

	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func TestMetadataSchemaDropsWrapperDefault(t *testing.T) {
	doc := &Document{
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"metadata": {
					Generic: docschema.Generic{
						Type:    "object",
						Default: docschema.JSON{Object: map[string]interface{}{}},
					},
					Properties: map[string]docschema.Structural{
						"name": {
							Generic: docschema.Generic{Type: "string"},
						},
					},
				},
			},
		},
	}

	metadata := doc.MetadataSchema()
	if metadata.Default.Object != nil {
		t.Fatalf("metadata wrapper default must not be exposed, got %#v", metadata.Default.Object)
	}
}

func TestMetadataSchemaFillsMissingFieldDescriptions(t *testing.T) {
	doc := &Document{
		Namespaced: true,
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"metadata": {
					Generic: docschema.Generic{Type: "object"},
					Properties: map[string]docschema.Structural{
						"name": {
							Generic: docschema.Generic{Type: "string"},
						},
						"labels": {
							Generic: docschema.Generic{
								Type:        "object",
								Description: "Custom labels description.",
							},
						},
					},
				},
			},
		},
	}

	metadata := doc.MetadataSchema()
	name := metadata.Properties["name"]
	if name.Description != "Name must be unique within a namespace." {
		t.Fatalf("expected fallback name description, got %#v", name)
	}
	namespace := metadata.Properties["namespace"]
	if namespace.Description != "Namespace defines the space within which each name must be unique." {
		t.Fatalf("expected fallback namespace description, got %#v", namespace)
	}
	labels := metadata.Properties["labels"]
	if labels.Description != "Custom labels description." {
		t.Fatalf("expected custom labels description to be preserved, got %#v", labels)
	}
}
