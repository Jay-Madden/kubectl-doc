package yamlrender

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/kube"
	"github.com/sttts/kubectl-doc/internal/render/tree"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func TestColorLineUsesANSIStyles(t *testing.T) {
	colored := ColorLine(`spec: "<string>" # comment`)

	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("expected ANSI style sequence, got %q", colored)
	}
	if !strings.Contains(colored, "spec") || !strings.Contains(colored, "<string>") || !strings.Contains(colored, "# comment") {
		t.Fatalf("colored line lost content: %q", colored)
	}
}

func TestColorLineStylesYAMLPunctuation(t *testing.T) {
	colored := ColorLine(`deployments: ["v2","v1"]`)

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

func TestColorLineStylesRequiredLabel(t *testing.T) {
	for _, line := range []string{
		`spec: "<string>" # required, minLength: 1`,
		`key: "" # default, required`,
	} {
		colored := ColorLine(line)
		if !strings.Contains(colored, requiredStyle.Render("required")) {
			t.Fatalf("expected required label to be required-styled in %q, got %q", line, colored)
		}
	}

	colored := ColorLine(`spec: "<string>" # required, minLength: 1`)
	if !strings.Contains(colored, noteStyle.Render(", minLength: 1")) {
		t.Fatalf("expected normal comment to stay note-styled, got %q", colored)
	}
}

func TestColorLineStylesCommentedFieldsAsYAML(t *testing.T) {
	for _, line := range []string{
		`# replicas: 1 # default`,
		`# managedFields: # listType: atomic`,
		`# values: # listType: atomic`,
		`# - cluster_location: "<string>"`,
		`- # cluster_name: "<string>"`,
	} {
		colored := ColorTreeLine(tree.Line{Text: line, Code: true})
		if !strings.Contains(colored, noteStyle.Render("#")) {
			t.Fatalf("expected leading comment marker to be note-styled in %q, got %q", line, colored)
		}
		if !strings.Contains(colored, syntaxStyle.Render(":")) {
			t.Fatalf("expected commented field separator to be syntax-styled in %q, got %q", line, colored)
		}
	}

	colored := ColorTreeLine(tree.Line{Text: `# replicas: 1 # default`, Code: true})
	if !strings.Contains(colored, keyStyle.Render("replicas")) {
		t.Fatalf("expected commented field key to be key-styled, got %q", colored)
	}
	if !strings.Contains(colored, scalarStyle.Render("1")) {
		t.Fatalf("expected commented field value to be scalar-styled, got %q", colored)
	}
	if !strings.Contains(colored, noteStyle.Render(" # default")) {
		t.Fatalf("expected inline metadata to stay note-styled, got %q", colored)
	}
}

func TestColorLineDoesNotStyleProseCommentsAsYAML(t *testing.T) {
	for _, line := range []string{
		`# syntax: i.e. "$$(VAR_NAME)" will produce the string literal "$(VAR_NAME)".`,
		`# Required: resource to select`,
		`# Optional: Host name to connect to, defaults to the pod IP.`,
	} {
		colored := ColorTreeLine(tree.Line{Text: line})
		if strings.Contains(colored, keyStyle.Render(strings.TrimPrefix(strings.Split(line, ":")[0], "# "))) {
			t.Fatalf("prose comment must not be treated as YAML key in %q, got %q", line, colored)
		}
		if !strings.Contains(colored, noteStyle.Render(line)) {
			t.Fatalf("prose comment should stay note-styled in %q, got %q", line, colored)
		}
	}
}

func TestColorLineStylesURLsInComments(t *testing.T) {
	for _, line := range []tree.Line{
		{Text: `# https://example.com/path`},
		{Text: `# More info: https://example.com/path`},
		{Text: `metadata: {} # see https://example.com/path`, Code: true},
	} {
		colored := ColorTreeLine(line)
		if !strings.Contains(colored, urlStyle.Render("https://example.com/path")) {
			t.Fatalf("expected URL to be URL-styled in %q, got %q", line.Text, colored)
		}
		if strings.Contains(colored, keyStyle.Render("https")) {
			t.Fatalf("URL comment must not be treated as YAML key in %q, got %q", line.Text, colored)
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
spec: {} # required
`
	if !strings.Contains(out.String(), expected) {
		t.Fatalf("expected wrapped description block\nwant contains:\n%s\ngot:\n%s", expected, out.String())
	}
}

func TestRenderWrapsInlineValidationComments(t *testing.T) {
	var out bytes.Buffer
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"stopSignal": {
					Generic: docschema.Generic{Type: "string"},
					ValueValidation: &docschema.ValueValidation{
						Enum: []docschema.JSON{
							{Object: "SIGABRT"},
							{Object: "SIGALRM"},
							{Object: "SIGBUS"},
							{Object: "SIGCHLD"},
						},
					},
				},
			},
			ValueValidation: &docschema.ValueValidation{
				Required: []string{"stopSignal"},
			},
		},
	}

	if err := (Renderer{Columns: 44}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	lines := strings.Split(rendered, "\n")
	var first, continuation string
	for i, line := range lines {
		if strings.Contains(line, `stopSignal:`) {
			first = line
			if i+1 < len(lines) {
				continuation = lines[i+1]
			}
			break
		}
	}
	firstHash := strings.Index(first, "#")
	continuationHash := strings.Index(continuation, "#")
	if firstHash < 0 || continuationHash < 0 || firstHash != continuationHash {
		t.Fatalf("expected inline validation continuation to align under #\nfirst: %q\nnext:  %q\nfull:\n%s", first, continuation, rendered)
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
		`arrayExample: ["blue","green"] # example array, required`,
		`defaulted: "default" # default, required`,
		`objectExample: {"mode":"active"} # example object primary, required`,
		`scalarExample: "prod" # example string, required`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected rendered YAML to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, "example string, default") || strings.Contains(rendered, `"example"`) {
		t.Fatalf("expected default to take precedence over example, got:\n%s", rendered)
	}
}

func TestRenderCompactsAdjacentOneLineFields(t *testing.T) {
	var out bytes.Buffer
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{
						Type: "object",
					},
					Properties: map[string]docschema.Structural{
						"enabled": {
							Generic: docschema.Generic{
								Type:    "boolean",
								Default: docschema.JSON{Object: true},
							},
						},
						"env": {
							Generic: docschema.Generic{
								Type:    "string",
								Default: docschema.JSON{Object: "non-prod"},
							},
						},
						"selector": {
							Generic: docschema.Generic{
								Type: "object",
							},
							Properties: map[string]docschema.Structural{
								"matchLabels": {
									Generic: docschema.Generic{
										Type: "object",
									},
									AdditionalProperties: &docschema.StructuralOrBool{
										Structural: &docschema.Structural{
											Generic: docschema.Generic{Type: "string"},
										},
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

	if err := (Renderer{ExpandDepth: 2, Descriptions: DescriptionFalse}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	expected := `  # enabled: true # default
  # env: "non-prod" # default

  # selector:
    # matchLabels:
      # <key>: "<string>"`
	if !strings.Contains(out.String(), expected) {
		t.Fatalf("expected adjacent one-line fields to be compact before object block\nwant contains:\n%s\ngot:\n%s", expected, out.String())
	}
}

func TestRenderCommentsOptionalArrayItemFieldsWithoutDoubleCommentMarker(t *testing.T) {
	var out bytes.Buffer
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"spec": {
					Generic: docschema.Generic{Type: "object"},
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
									"project_id": {
										Generic: docschema.Generic{Type: "string"},
									},
									"replicas": {
										Generic: docschema.Generic{
											Type:    "integer",
											Default: docschema.JSON{Object: int64(1)},
										},
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

	if err := (Renderer{ExpandDepth: 3, Descriptions: DescriptionFalse}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	expected := `  # management_clusters:
    # - cluster_location: "<string>"
      # cluster_name: "<string>"
      # project_id: "<string>"
      # replicas: 1 # default`
	if !strings.Contains(rendered, expected) {
		t.Fatalf("expected optional array item fields to be commented as YAML fields\nwant contains:\n%s\ngot:\n%s", expected, rendered)
	}
	if strings.Contains(rendered, "# - # cluster_location") {
		t.Fatalf("did not expect double comment marker for array item field, got:\n%s", rendered)
	}
}

func TestRenderStatusMode(t *testing.T) {
	doc := &crd.Document{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Widget",
		Schema: &docschema.Structural{
			Properties: map[string]docschema.Structural{
				"status": {
					Generic: docschema.Generic{
						Type: "object",
					},
					Properties: map[string]docschema.Structural{
						"phase": {
							Generic: docschema.Generic{
								Type: "string",
							},
						},
					},
				},
			},
		},
	}

	var defaultOut bytes.Buffer
	if err := (Renderer{ExpandDepth: 2}).Render(&defaultOut, doc); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(defaultOut.String(), "# status: {}\n") || strings.Contains(defaultOut.String(), "phase") {
		t.Fatalf("expected default status to stay a folded comment, got:\n%s", defaultOut.String())
	}

	var statusOut bytes.Buffer
	if err := (Renderer{ExpandDepth: 2, RenderStatus: true}).Render(&statusOut, doc); err != nil {
		t.Fatal(err)
	}
	expected := `status: # optional
  # phase: "<string>"
`
	if !strings.Contains(statusOut.String(), expected) {
		t.Fatalf("expected generated status tree\nwant contains:\n%s\ngot:\n%s", expected, statusOut.String())
	}
}
