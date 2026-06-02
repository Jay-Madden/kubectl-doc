package krorender

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/crd"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
	"sigs.k8s.io/yaml"
)

func TestRenderKroSimpleSchema(t *testing.T) {
	var out bytes.Buffer
	if err := (Renderer{}).Render(&out, testDocument()); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	for _, expected := range []string{
		"apiVersion: example.io/v1\n",
		"kind: Widget\n",
		`spec: # required=true description="WidgetSpec configures the widget."`,
		`mode: string | required=true default="Auto" enum="Auto,Manual" minLength=1 description="Mode selects the widget behavior."`,
		`labels: "map[string]string"`,
		`ports: "[]PortsItem"`,
		"types:\n",
		"  PortsItem:\n",
		`    name: string | required=true description="Port name."`,
		`    number: integer | required=true minimum=1 format=int32`,
		`status: # description="Widget status."`,
		`phase: string | enum="Pending,Ready"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected Kro output to contain %q, got:\n%s", expected, rendered)
		}
	}
	assertParsesAsYAML(t, rendered)
}

func TestRenderKroDescriptionModes(t *testing.T) {
	var out bytes.Buffer
	if err := (Renderer{Descriptions: yamlrender.DescriptionRequired}).Render(&out, testDocument()); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	if !strings.Contains(rendered, `mode: string | required=true default="Auto" enum="Auto,Manual" minLength=1 description="Mode selects the widget behavior."`) {
		t.Fatalf("expected required field description, got:\n%s", rendered)
	}
	if strings.Contains(rendered, `status: # description="Widget status."`) {
		t.Fatalf("did not expect optional status description, got:\n%s", rendered)
	}
}

func TestRenderAllKroSimpleSchema(t *testing.T) {
	var out bytes.Buffer
	v1 := testDocument()
	v2 := testDocument()
	v2.Version = "v2"

	if err := (Renderer{}).RenderAll(&out, []*crd.Document{v2, v1}); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	if strings.Count(rendered, "apiVersion: example.io/") != 2 {
		t.Fatalf("expected two rendered versions, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "---\napiVersion: example.io/v1\n") {
		t.Fatalf("expected YAML document separator before second version, got:\n%s", rendered)
	}
}

func testDocument() *crd.Document {
	return &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Plural:  "widgets",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{
						Description: "WidgetSpec configures the widget.",
						Type:        "object",
					},
					Properties: map[string]docschema.Structural{
						"labels": {
							Generic: docschema.Generic{Type: "object"},
							AdditionalProperties: &docschema.StructuralOrBool{
								Structural: &docschema.Structural{
									Generic: docschema.Generic{Type: "string"},
								},
							},
						},
						"mode": {
							Generic: docschema.Generic{
								Description: "Mode selects the widget behavior.",
								Type:        "string",
								Default:     docschema.JSON{Object: "Auto"},
							},
							ValueValidation: &docschema.ValueValidation{
								Enum: []docschema.JSON{
									{Object: "Auto"},
									{Object: "Manual"},
								},
								MinLength: ptrInt64(1),
							},
						},
						"ports": {
							Generic: docschema.Generic{Type: "array"},
							Items: &docschema.Structural{
								Generic: docschema.Generic{Type: "object"},
								Properties: map[string]docschema.Structural{
									"name": {
										Generic: docschema.Generic{
											Description: "Port name.",
											Type:        "string",
										},
									},
									"number": {
										Generic: docschema.Generic{Type: "integer"},
										ValueValidation: &docschema.ValueValidation{
											Format:  "int32",
											Minimum: ptrFloat64(1),
										},
									},
								},
								ValueValidation: &docschema.ValueValidation{
									Required: []string{"name", "number"},
								},
							},
						},
					},
					ValueValidation: &docschema.ValueValidation{
						Required: []string{"mode"},
					},
				},
				"status": {
					Generic: docschema.Generic{
						Description: "Widget status.",
						Type:        "object",
					},
					Properties: map[string]docschema.Structural{
						"phase": {
							Generic: docschema.Generic{Type: "string"},
							ValueValidation: &docschema.ValueValidation{
								Enum: []docschema.JSON{
									{Object: "Pending"},
									{Object: "Ready"},
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
}

func assertParsesAsYAML(t *testing.T, rendered string) {
	t.Helper()

	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(rendered), &parsed); err != nil {
		t.Fatalf("rendered output is not valid YAML: %v\n%s", err, rendered)
	}
}

func ptrInt64(value int64) *int64 {
	return &value
}

func ptrFloat64(value float64) *float64 {
	return &value
}
