package markdownrender

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/crd"
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
		"spec:",
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
		"---\ntitle: Widget\n---\n\n",
		"# Widget\n",
		`<Accordion title="YAML" defaultOpen={true}>`,
		"```yaml title=\"example.io/v1 Widget\" wordWrap showLineNumbers={false}\napiVersion: example.io/v1\nkind: Widget\n",
		`<Accordion title="Field Details">`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected Fern Markdown to contain %q, got:\n%s", expected, rendered)
		}
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

func TestRenderWrapsYAMLDescriptionComments(t *testing.T) {
	var out bytes.Buffer
	doc := testDocument()
	spec := doc.Schema.Properties["spec"]
	spec.Description = "This description wraps across columns."
	doc.Schema.Properties["spec"] = spec

	if err := (Renderer{Dialect: DialectGitHub, ExpandDepth: 1, Columns: 24}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	expected := "# This description wraps\n# across columns.\nspec:"
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
