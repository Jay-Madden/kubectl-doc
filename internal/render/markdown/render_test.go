package markdownrender

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/crd"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func TestRenderGitHubMarkdown(t *testing.T) {
	var out bytes.Buffer
	if err := (Renderer{Dialect: DialectGitHub, ExpandDepth: 1}).Render(&out, testDocument()); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	for _, expected := range []string{
		"# Widget\n",
		"| API Version | `example.io/v1` |",
		"| Kind | `Widget` |",
		"| Resource | `widgets` |",
		"<details open>\n<summary>YAML</summary>",
		"```yaml\napiVersion: example.io/v1\nkind: Widget\n",
		"spec: # required",
		`mode: "<string>" # required, minLength: 1`,
		"## Field Details\n",
		`<a id="field-example-io-v1-spec-mode"></a>`,
		"### `spec.mode`",
		"- Required: `yes`",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected GitHub Markdown to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.HasPrefix(rendered, "---\n") {
		t.Fatalf("GitHub Markdown should not render Fern frontmatter:\n%s", rendered)
	}
}

func TestRenderFernMarkdown(t *testing.T) {
	var out bytes.Buffer
	if err := (Renderer{Dialect: DialectFern, ExpandDepth: 1}).Render(&out, testDocument()); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	for _, expected := range []string{
		"---\ntitle: \"Widget\"\n---\n\n",
		`import { KubeSchemaDoc } from "@/components/kubectl-doc/KubeSchemaDoc";`,
		"export const kubectlDocSchemas = [",
		"# Widget\n",
		`<Accordion title={"YAML"} defaultOpen={true}>`,
		`<KubeSchemaDoc data={kubectlDocSchemas[0]} filtering={true} />`,
		`"text": "spec: # required"`,
		`"text": "  mode: \"\u003cstring\u003e\" # required, minLength: 1"`,
		`"filterText": "mode\nMode selects the widget behavior."`,
		`"metadata": [
          "minLength: 1"
        ]`,
		`<Accordion title={"Field Details"}>`,
		`<ParamField path={"spec.mode"} type={"string"} required={true}>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected Fern Markdown to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, "```yaml") {
		t.Fatalf("interactive Fern Markdown should not fall back to fenced YAML by default:\n%s", rendered)
	}
}

func TestRenderAllGitHubMarkdown(t *testing.T) {
	var out bytes.Buffer
	v1 := testDocument()
	v2 := testDocument()
	v2.Version = "v2"

	if err := (Renderer{Dialect: DialectGitHub, ExpandDepth: 1}).RenderAll(&out, []*crd.Document{v2, v1}); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	for _, expected := range []string{
		"| Versions | `example.io/v2`, `example.io/v1` |",
		"## example.io/v2\n",
		"## example.io/v1\n",
		"<summary>YAML: example.io/v2</summary>",
		"### Field details: example.io/v2\n",
		`<a id="field-example-io-v2-spec-mode"></a>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected multi-version Markdown to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestRenderAllFernMarkdown(t *testing.T) {
	var out bytes.Buffer
	v1 := testDocument()
	v2 := testDocument()
	v2.Version = "v2"

	if err := (Renderer{Dialect: DialectFern, ExpandDepth: 1, HideFieldDetails: true}).RenderAll(&out, []*crd.Document{v2, v1}); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	for _, expected := range []string{
		"| Versions | `example.io/v2`, `example.io/v1` |",
		"<Tabs>",
		`<Tab title={"example.io/v2"}>`,
		`<Tab title={"example.io/v1"}>`,
		`<KubeSchemaDoc data={kubectlDocSchemas[0]} filtering={true} />`,
		`<KubeSchemaDoc data={kubectlDocSchemas[1]} filtering={true} />`,
		`"apiVersion": "example.io/v2"`,
		`"apiVersion": "example.io/v1"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected all-version Fern Markdown to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestRenderFernMarkdownCanDisableFiltering(t *testing.T) {
	var out bytes.Buffer
	if err := (Renderer{Dialect: DialectFern, ExpandDepth: 1, HideFieldDetails: true, DisableFiltering: true}).Render(&out, testDocument()); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	if !strings.Contains(rendered, `<KubeSchemaDoc data={kubectlDocSchemas[0]} filtering={false} />`) {
		t.Fatalf("expected disabled filtering component flag, got:\n%s", rendered)
	}
	if strings.Contains(rendered, `"filterText"`) {
		t.Fatalf("disabled filtering should omit filterText indexes, got:\n%s", rendered)
	}
}

func TestRenderFernMarkdownEscapesMDX(t *testing.T) {
	var out bytes.Buffer
	doc := testDocument()
	spec := doc.Schema.Properties["spec"]
	mode := spec.Properties["mode"]
	mode.Description = `Mode accepts <fast> values with {braces}.`
	spec.Properties["mode"] = mode
	doc.Schema.Properties["spec"] = spec

	if err := (Renderer{Dialect: DialectFern, ExpandDepth: 1}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	for _, expected := range []string{
		`"description": "Mode accepts \u003cfast\u003e values with {braces}."`,
		`Mode accepts &lt;fast&gt; values with \{braces\}.`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected escaped Fern Markdown to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestRenderDynamoGraphDeploymentFernPayloadGolden(t *testing.T) {
	var out bytes.Buffer
	doc, err := crd.Load([]string{"../../cli/testdata/dynamographdeployment-crd.yaml"}, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := (Renderer{
		Dialect:          DialectFern,
		ExpandDepth:      3,
		Descriptions:     yamlrender.DescriptionFalse,
		HideFieldDetails: true,
	}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	for _, expected := range []string{
		`# DynamoGraphDeployment`,
		`"apiVersion": "nvidia.com/v1beta1"`,
		`"kind": "DynamoGraphDeployment"`,
		`"path": "spec.components[].podTemplate"`,
		`"x-kubernetes-preserve-unknown-fields"`,
		`"path": "spec.components[].sharedMemorySize"`,
		`"type": "int-or-string"`,
		`"x-kubernetes-int-or-string"`,
		`"x-kubernetes-list-type: map"`,
		`"x-kubernetes-list-map-keys: name"`,
		`<KubeSchemaDoc data={kubectlDocSchemas[0]} filtering={true} />`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected Dynamo Fern golden output to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestRenderWrapsYAMLDescriptionComments(t *testing.T) {
	var out bytes.Buffer
	doc := testDocument()
	spec := doc.Schema.Properties["spec"]
	spec.Description = "This description wraps across columns."
	doc.Schema.Properties["spec"] = spec

	if err := (Renderer{Dialect: DialectGitHub, ExpandDepth: 1, Columns: 24}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	expected := "# This description wraps\n# across columns.\nspec: # required"
	if !strings.Contains(out.String(), expected) {
		t.Fatalf("expected Markdown YAML block to contain wrapped comments %q, got:\n%s", expected, out.String())
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
					Generic: docschema.Generic{Type: "object"},
					Properties: map[string]docschema.Structural{
						"mode": {
							Generic: docschema.Generic{
								Description: "Mode selects the widget behavior.",
								Type:        "string",
							},
							ValueValidation: &docschema.ValueValidation{
								MinLength: ptrInt64(1),
							},
						},
					},
					ValueValidation: &docschema.ValueValidation{
						Required: []string{"mode"},
					},
				},
			},
			ValueValidation: &docschema.ValueValidation{
				Required: []string{"spec"},
			},
		},
	}
}

func ptrInt64(value int64) *int64 {
	return &value
}
