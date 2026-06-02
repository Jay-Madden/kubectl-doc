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
		"```yaml\napiVersion: example.io/v1\nkind: Widget\n",
		"spec:",
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
		"```yaml\napiVersion: example.io/v1\nkind: Widget\n",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected Fern Markdown to contain %q, got:\n%s", expected, rendered)
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
							Generic: docschema.Generic{Type: "string"},
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
