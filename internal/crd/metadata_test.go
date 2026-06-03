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
