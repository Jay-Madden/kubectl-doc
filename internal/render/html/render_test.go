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
	minReplicas := float64(0)
	maxReplicas := float64(10)
	minLength := int64(1)
	maxItems := int64(4)
	listType := "map"
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
									ValueValidation: &docschema.ValueValidation{
										Enum: []docschema.JSON{
											{Object: "nginx:latest"},
											{Object: "busybox:latest"},
										},
										MinLength: &minLength,
										Pattern:   "^.+:.+$",
									},
								},
							},
							ValueValidation: &docschema.ValueValidation{
								Required: []string{"image"},
							},
						},
						"replicas": {
							Generic: docschema.Generic{
								Type:        "integer",
								Description: "Desired number of pod replicas for the component.",
								Default:     docschema.JSON{Object: int64(1)},
							},
							ValueValidation: &docschema.ValueValidation{
								Format:  "int32",
								Minimum: &minReplicas,
								Maximum: &maxReplicas,
							},
						},
						"ports": {
							Items: &docschema.Structural{
								Generic: docschema.Generic{
									Type: "object",
								},
								Properties: map[string]docschema.Structural{
									"name": {
										Generic: docschema.Generic{
											Type:        "string",
											Description: "Port name.",
										},
									},
								},
							},
							Generic: docschema.Generic{
								Type:        "array",
								Description: "Published ports.",
							},
							Extensions: docschema.Extensions{
								XListMapKeys: []string{"name"},
								XListType:    &listType,
							},
							ValueValidation: &docschema.ValueValidation{
								MaxItems:    &maxItems,
								UniqueItems: true,
							},
						},
					},
					ValueValidation: &docschema.ValueValidation{
						Required: []string{"template"},
					},
				},
				"status": {
					Generic: docschema.Generic{
						Type:        "object",
						Description: "Widget status.",
					},
					Properties: map[string]docschema.Structural{
						"phase": {
							Generic: docschema.Generic{
								Type:        "string",
								Description: "Current phase.",
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
		"--kdoc-yaml-key",
		"class=\"kdoc-yaml-key\"",
		"class=\"kdoc-yaml-punct\"",
		"class=\"kdoc-yaml-comment\"",
		"font:13px/1.3",
		".kdoc-line{align-items:flex-start;",
		".kdoc-fold,.kdoc-gutter{background:transparent;border:0;color:var(--kdoc-muted);display:block;",
		".kdoc-fold::before{content:\"▶\";display:block;line-height:inherit}",
		"--kdoc-required",
		"--kdoc-ok",
		"kdoc-required-label",
		"# required",
		"class=\"kdoc-required-label\"># required</span><span class=\"kdoc-yaml-comment\">, enum:",
		".kdoc-detail-row{align-items:baseline;",
		".kdoc-detail-code,.kdoc-detail-list code{font:12px/1.45",
		"vertical-align:baseline",
		".kdoc-detail-badge-required{background:#ffebe9;",
		".kdoc-detail-badge-optional{background:#dafbe1;",
		"kdoc-detail-body",
		"kdoc-detail-grid",
		"data-detail-html",
		"fieldOnly",
		"Path: spec.template.image",
		"kdoc-detail-description",
		"Description:\nContainer image.",
		"enum: &#34;nginx:latest&#34; | &#34;busybox:latest&#34;",
		"minLength: 1",
		"pattern: ^.+:.+$",
		"format: int32",
		"minimum: 0",
		"maximum: 10",
		"x-kubernetes-list-type: map",
		"x-kubernetes-list-map-keys: name",
		`aria-expanded="false" data-kdoc-toggle></button><span class="kdoc-yaml-text"><span class="kdoc-yaml-key">status</span><span class="kdoc-yaml-punct">:</span><span class="kdoc-yaml-comment"> # optional</span>`,
		`data-path="status.phase"`,
		`<span class="kdoc-yaml-comment"># </span><span class="kdoc-yaml-key">phase</span><span class="kdoc-yaml-punct">:</span> <span class="kdoc-yaml-string">&#34;&lt;string&gt;&#34;</span>`,
		"kdoc-search-hit",
		"event.key === \"ArrowDown\"",
		"tag !== \"INPUT\" && tag !== \"TEXTAREA\" && (event.key === \"n\"",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected HTML to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, "data-detail=\"#") {
		t.Fatalf("field details must not be rendered from YAML comments, got:\n%s", rendered)
	}
	for _, unwanted := range []string{
		`data-kdoc-toggle>▼</button>`,
		`data-kdoc-toggle>▶</button>`,
		`# required</span> <span class="kdoc-required-label"># required`,
	} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("unexpected selectable or duplicate UI text %q, got:\n%s", unwanted, rendered)
		}
	}
	if count := strings.Count(rendered, `data-detail-id="field-example-io-v1-spec-replicas"`); count < 2 {
		t.Fatalf("expected replicas description and field lines to share a detail id, count=%d:\n%s", count, rendered)
	}
	if count := strings.Count(rendered, `data-detail-id="field-example-io-v1-spec-ports-name"`); count < 2 {
		t.Fatalf("expected array item description and field lines to share a detail id, count=%d:\n%s", count, rendered)
	}
	if strings.Contains(strings.ToLower(rendered), "copy") {
		t.Fatalf("HTML must not contain copy controls, got:\n%s", rendered)
	}
}

func TestRenderKeepsSearchTypingKeysInInput(t *testing.T) {
	script := scriptElement()
	for _, unwanted := range []string{
		`if(event.key === "n" || event.key === "ArrowDown")`,
		`if(event.key === "p" || event.key === "ArrowUp")`,
	} {
		if strings.Contains(script, unwanted) {
			t.Fatalf("search input must not consume typing key %q:\n%s", unwanted, script)
		}
	}
}

func TestRenderScalarTokenStylesTypedPlaceholders(t *testing.T) {
	for _, tc := range []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "string",
			token:    "<string>",
			expected: `<span class="kdoc-yaml-string">&lt;string&gt;</span>`,
		},
		{
			name:     "integer format",
			token:    "<int32>",
			expected: `<span class="kdoc-yaml-number">&lt;int32&gt;</span>`,
		},
		{
			name:     "boolean",
			token:    "<boolean>",
			expected: `<span class="kdoc-yaml-bool">&lt;boolean&gt;</span>`,
		},
		{
			name:     "int or string",
			token:    "<int-or-string>",
			expected: `<span class="kdoc-yaml-number">&lt;int</span><span class="kdoc-yaml-punct">-or-</span><span class="kdoc-yaml-string">string&gt;</span>`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderScalarToken(tc.token); got != tc.expected {
				t.Fatalf("expected %s to render as %q, got %q", tc.token, tc.expected, got)
			}
		})
	}
}

func TestSearchTextDoesNotMatchParentPathForChildren(t *testing.T) {
	lines := buildLines(strings.Join([]string{
		"spec:",
		"  # Annotations propagated to generated workloads.",
		"  # annotations:",
		"    # <key>: \"<string>\"",
	}, "\n"), 10, map[string]fieldDetail{
		"spec": {
			ID:   "field-spec",
			Path: "spec",
			Type: "object",
		},
		"spec.annotations": {
			ID:          "field-spec-annotations",
			Path:        "spec.annotations",
			Type:        "object",
			Description: "Annotations propagated to generated workloads.",
		},
		"spec.annotations.<key>": {
			ID:   "field-spec-annotations-key",
			Path: "spec.annotations.<key>",
			Type: "string",
		},
	})

	for _, line := range lines {
		if line.Path == "spec.annotations.<key>" && strings.Contains(strings.ToLower(line.SearchText), "annotation") {
			t.Fatalf("map child search text must not match parent path: %#v", line)
		}
	}
}
