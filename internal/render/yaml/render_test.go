package yamlrender

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/kube"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func TestColorLineUsesANSIStyles(t *testing.T) {
	colored := colorLine(`spec: "<string>" # comment`)

	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("expected ANSI style sequence, got %q", colored)
	}
	if !strings.Contains(colored, "spec") || !strings.Contains(colored, "<string>") || !strings.Contains(colored, "# comment") {
		t.Fatalf("colored line lost content: %q", colored)
	}
}

func TestColorLineStylesYAMLPunctuation(t *testing.T) {
	colored := colorLine(`deployments: ["v2","v1"]`)

	for _, token := range []string{":", "[", ",", "]"} {
		expected := syntaxStyle.Render(token)
		if !strings.Contains(colored, expected) {
			t.Fatalf("expected syntax-styled %q in %q", token, colored)
		}
	}
	for _, token := range []string{`"v2"`, `"v1"`} {
		expected := stringStyle.Render(token)
		if !strings.Contains(colored, expected) {
			t.Fatalf("expected string-styled %q in %q", token, colored)
		}
	}
}

func TestRenderOverview(t *testing.T) {
	var out bytes.Buffer
	overview := &kube.Overview{
		Groups: []kube.Group{
			{
				Name: kube.CoreGroup,
				Resources: []kube.Resource{
					{Name: "pods", Versions: []string{"v1"}},
				},
			},
			{
				Name: "apps",
				Resources: []kube.Resource{
					{Name: "deployments", Versions: []string{"v1", "v1beta1"}},
				},
			},
		},
	}

	renderer := OverviewRenderer{}
	if err := renderer.Render(&out, overview); err != nil {
		t.Fatal(err)
	}

	expected := `core:
  pods: v1
apps:
  deployments: ["v1","v1beta1"]
`
	if out.String() != expected {
		t.Fatalf("unexpected output\nwant:\n%s\ngot:\n%s", expected, out.String())
	}
}

func TestRenderOverviewColor(t *testing.T) {
	var out bytes.Buffer
	overview := &kube.Overview{
		Groups: []kube.Group{
			{
				Name: kube.CoreGroup,
				Resources: []kube.Resource{
					{Name: "pods", Versions: []string{"v2", "v1"}},
				},
			},
		},
	}

	renderer := OverviewRenderer{Color: true}
	if err := renderer.Render(&out, overview); err != nil {
		t.Fatal(err)
	}

	colored := out.String()
	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("expected ANSI style sequence, got %q", colored)
	}
	if !strings.Contains(colored, "core") || !strings.Contains(colored, "pods") || !strings.Contains(colored, "v2") {
		t.Fatalf("colored overview lost content: %q", colored)
	}
	for _, token := range []string{":", "[", ",", "]"} {
		if !strings.Contains(colored, syntaxStyle.Render(token)) {
			t.Fatalf("expected syntax-styled %q in %q", token, colored)
		}
	}
}

func TestRenderCoreAPIVersion(t *testing.T) {
	var out bytes.Buffer
	doc := &crd.Document{
		Version: "v1",
		Kind:    "Pod",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{},
		},
	}

	if err := (Renderer{}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(out.String(), "apiVersion: v1\n") {
		t.Fatalf("expected core apiVersion without leading slash, got:\n%s", out.String())
	}
}

func TestRenderWrapsDescriptionComments(t *testing.T) {
	var out bytes.Buffer
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{
						Type:        "object",
						Description: "This description wraps across columns.\n\nSecond paragraph wraps too.",
					},
				},
			},
			ValueValidation: &docschema.ValueValidation{
				Required: []string{"spec"},
			},
		},
	}

	if err := (Renderer{Columns: 24}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	expected := `# This description wraps
# across columns.
#
# Second paragraph wraps
# too.
spec: {}
`
	if !strings.Contains(out.String(), expected) {
		t.Fatalf("expected wrapped description block\nwant contains:\n%s\ngot:\n%s", expected, out.String())
	}
}

func TestRenderExamples(t *testing.T) {
	var out bytes.Buffer
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"arrayExample": {
					Generic: docschema.Generic{
						Type: "array",
						Examples: []docschema.Example{{
							Value: docschema.JSON{Object: []interface{}{"blue", "green"}},
						}},
					},
				},
				"defaulted": {
					Generic: docschema.Generic{
						Type:    "string",
						Default: docschema.JSON{Object: "default"},
						Examples: []docschema.Example{{
							Value: docschema.JSON{Object: "example"},
						}},
					},
				},
				"objectExample": {
					Generic: docschema.Generic{
						Type: "object",
						Examples: []docschema.Example{{
							Name:  "primary",
							Value: docschema.JSON{Object: map[string]interface{}{"mode": "active"}},
						}},
					},
				},
				"scalarExample": {
					Generic: docschema.Generic{
						Type: "string",
						Examples: []docschema.Example{{
							Value: docschema.JSON{Object: "prod"},
						}},
					},
				},
			},
			ValueValidation: &docschema.ValueValidation{
				Required: []string{"arrayExample", "defaulted", "objectExample", "scalarExample"},
			},
		},
	}

	if err := (Renderer{}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	for _, expected := range []string{
		`arrayExample: ["blue","green"] # example array`,
		`defaulted: "default" # default`,
		`objectExample: {"mode":"active"} # example object primary`,
		`scalarExample: "prod" # example string`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected rendered YAML to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, "example string, default") || strings.Contains(rendered, `"example"`) {
		t.Fatalf("expected default to take precedence over example, got:\n%s", rendered)
	}
}
