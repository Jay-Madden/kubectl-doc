package htmlrender

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/render/tree"
	yamlrender "github.com/sttts/kubectl-doc/internal/render/yaml"
	"github.com/sttts/kubectl-doc/internal/render/yamltokens"
	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

func TestRenderFoldableHTML(t *testing.T) {
	var out bytes.Buffer
	minReplicas := float64(0)
	maxReplicas := float64(10)
	minLength := int64(1)
	maxItems := int64(4)
	listType := "map"
	doc := &crd.Document{
		Group:      "example.io",
		Version:    "v1",
		Kind:       "Widget",
		Plural:     "widgets",
		Namespaced: true,
		Schema: &docschema.Structural{
			Generic: docschema.Generic{
				Type:        "object",
				Description: "Widget declares the root object.",
			},
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
						Required: []string{"replicas", "template"},
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
		"<h1>Widget <small>example.io/v1</small></h1>",
		"class=\"kdoc-view-controls\"",
		`<input type="checkbox" data-kdoc-wrap-comments checked><span class="kdoc-switch" aria-hidden="true"></span><span class="kdoc-wrap-label">wrap</span>`,
		".kdoc-view-controls{bottom:calc(12px + 2.5em);",
		".kdoc-wrap-toggle{align-items:center;background:transparent;border:0;",
		".kdoc-switch{background:#d0d7de;",
		".kdoc-wrap-toggle input:checked + .kdoc-switch{background:#34c759;",
		".kubectl-doc.kdoc-wrap-comments .kdoc-yaml-comment-text{display:block;flex:1 1 auto;white-space:normal}",
		".kubectl-doc.kdoc-wrap-comments .kdoc-comment-line{display:block;white-space:pre}",
		".kdoc-layout{align-items:start;",
		".kdoc-details{align-self:start;background:#fff;",
		"max-height:calc(100vh - 24px);min-width:0;overflow:auto;",
		"scrollbar-gutter:stable;top:12px;z-index:2",
		"@media(max-width:900px){.kubectl-doc{padding:16px}.kdoc-layout{grid-template-columns:1fr}.kdoc-details{max-height:calc(100vh - 16px);top:8px}}",
		"@media(max-width:640px){.kdoc-view-controls{bottom:calc(8px + 2.5em)}}",
		`data-kdoc-comment-wrap-prefix`,
		`data-kdoc-comment-group="description-0"`,
		`data-kdoc-field`,
		`data-kdoc-comment-text="Name must be unique within a namespace."`,
		`function wrapCommentText(text, firstLimit, nextLimit)`,
		`function buildCommentGroups(states)`,
		`function renderCommentGroup(group, wrapped, lineChars)`,
		`kdoc-comment-reflow-hidden`,
		`function mount(root, options)`,
		`global.KubectlDoc.mount = mount`,
		`root.__kubectlDocController`,
		`destroy: function()`,
		`snapshot: function()`,
		`function visibleFieldLines()`,
		`function collapseOrParent()`,
		`function expandOrChild()`,
		`function handleCursorKey(event)`,
		`case "ArrowUp":`,
		`case "Tab":`,
		`case "PageDown":`,
		`renderCommentLine(index === 0 ? state.firstPrefix : state.nextPrefix, chunk);`,
		`function nextContentDepth(index)`,
		`if(blank && followingDepth !== null && followingDepth <= parentDepth){ break; }`,
		`applyCommentWrap();
      applyFolds();`,
		"data-kdoc-toggle",
		"aria-expanded=\"false\"",
		"Spec describes the widget.",
		`<span class="kdoc-yaml-key">template</span><span class="kdoc-yaml-punct">:</span>`,
		`aria-expanded="false" data-kdoc-toggle></button><span class="kdoc-yaml-text"><span class="kdoc-yaml-key">metadata</span><span class="kdoc-yaml-punct">:</span>`,
		`data-path="metadata.namespace"`,
		`data-path="metadata.ownerReferences[].kind"`,
		`<span class="kdoc-yaml-key">namespace</span><span class="kdoc-yaml-punct">:</span> <span class="kdoc-yaml-string">&#34;&lt;string&gt;&#34;</span><span class="kdoc-yaml-comment"> # </span><span class="kdoc-required-label">required</span>`,
		`data-kdoc-comment-text="Widget declares the root object."`,
		`data-detail-id="root-description-example-io-v1"`,
		`data-kdoc-comment-text="Container image."`,
		"--kdoc-yaml-key",
		"class=\"kdoc-yaml-key\"",
		"class=\"kdoc-yaml-punct\"",
		"class=\"kdoc-yaml-comment\"",
		"font:13px/1.3",
		".kdoc-line{align-items:flex-start;",
		".kdoc-version.kdoc-filtering .kdoc-line{display:none}",
		".kdoc-version.kdoc-filtering .kdoc-line.kdoc-filter-visible{display:flex}",
		".kdoc-fold,.kdoc-gutter{background:transparent;border:0;color:var(--kdoc-muted);display:block;",
		".kdoc-fold:focus{outline:0}",
		".kdoc-fold:focus-visible::before{color:var(--kdoc-yaml-key)}",
		".kdoc-fold::before{content:\"▶\";display:block;line-height:inherit}",
		"--kdoc-required",
		"--kdoc-ok",
		"kdoc-required-label",
		".kdoc-required-label{background:#ffebe9;",
		".kdoc-selected .kdoc-required-label{color:var(--kdoc-required)}",
		"class=\"kdoc-yaml-comment\"> # </span><span class=\"kdoc-required-label\">required</span><span class=\"kdoc-yaml-comment\">, enum:",
		"class=\"kdoc-yaml-comment\"> # default, </span><span class=\"kdoc-required-label\">required</span><span class=\"kdoc-yaml-comment\">, minimum:",
		".kdoc-detail-row{align-items:baseline;",
		".kdoc-detail-code,.kdoc-detail-list code{font:12px/1.45",
		"vertical-align:baseline",
		".kdoc-detail-badge-required{background:#ffebe9;",
		".kdoc-detail-badge-optional{background:#dafbe1;",
		"kdoc-detail-body",
		"kdoc-detail-grid",
		`<section class="kdoc-docs"><div class="kdoc-filter-overlay" data-kdoc-filter-overlay hidden></div>`,
		"data-kdoc-filter-overlay hidden",
		"data-kdoc-filter-text",
		"var filterQuery = \"\"",
		"var highlightedElements = []",
		"var selectedLines = []",
		"var filterVisibleLines = []",
		"var detailLineStates = new Map()",
		`detailLineStates.get(detailID).push(state);`,
		"descendants: []",
		`state.ancestors.forEach(function(ancestor){ ancestor.descendants.push(state); });`,
		"function setLineHidden(state, value)",
		"var filterScopeSection = null",
		"function currentVersionSection()",
		"state.version !== scope",
		"var highlightLineStates = new Set();",
		"function lineVisible(line)",
		"function setFilterVisibleLines(allowed)",
		"function groupedLineStates(state)",
		"function directFilterMatchLines()",
		"function selectFilterMatch(delta)",
		"filterQuery ? selectFilterMatch(event.shiftKey ? -1 : 1) : selectFoldable",
		`function currentFilterState()`,
		`pathFilterHighlightForState(state, pathFilter)`,
		`filterState.highlightLineStates.forEach(function(state){`,
		`var pathHit = fieldState.pathHit || "";`,
		`var commentColumnCache = 0;`,
		`function commentLineChars()`,
		"function applyFilterHighlights()",
		"function highlightElementIfContains(element, textLower, query)",
		`highlightElementIfContains(text, state.textLower, query);`,
		"function clearFilter()",
		"function acceptFilter()",
		".kdoc-filter-hit{background:var(--kdoc-filter);",
		`highlightedElements.forEach(function(element){`,
		`highlightedElements = [];`,
		".kdoc-filter-overlay{background:var(--kdoc-filter);border:1px solid rgba(17,17,17,.18);border-radius:6px;box-shadow:",
		"position:sticky;top:8px;width:max-content;z-index:6",
		".kdoc-embedded-host{background:transparent;max-width:100%;padding:0;position:relative}",
		`width = contentWidth(text) || width;`,
		".kdoc-detail-section{border-top:1px solid var(--kdoc-border);min-width:0;",
		".kdoc-detail-description{margin:0;overflow-wrap:anywhere}",
		"data-detail-html",
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
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected HTML to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, "data-detail=\"#") {
		t.Fatalf("field details must not be rendered from YAML comments, got:\n%s", rendered)
	}
	rootIndex := strings.Index(rendered, `data-kdoc-comment-text="Widget declares the root object."`)
	apiVersionIndex := strings.Index(rendered, `data-kdoc-field-name="apiVersion"`)
	if rootIndex < 0 || apiVersionIndex < 0 || rootIndex > apiVersionIndex {
		t.Fatalf("expected root description before apiVersion, root=%d apiVersion=%d:\n%s", rootIndex, apiVersionIndex, rendered)
	}
	rootLineStart := strings.LastIndex(rendered[:rootIndex], "<div")
	if rootLineStart < 0 {
		t.Fatalf("expected root description to render inside a line element, got:\n%s", rendered)
	}
	rootLine := rendered[rootLineStart:rootIndex]
	if strings.Contains(rootLine, `data-kdoc-field`) {
		t.Fatalf("root description must not be selectable as a field, got:\n%s", rootLine)
	}
	apiVersionLineEnd := strings.Index(rendered[apiVersionIndex:], "</div>")
	if apiVersionLineEnd < 0 {
		t.Fatalf("expected apiVersion line to end, got:\n%s", rendered[apiVersionIndex:])
	}
	apiVersionLine := rendered[apiVersionIndex : apiVersionIndex+apiVersionLineEnd]
	if strings.Contains(apiVersionLine, "Widget declares the root object.") {
		t.Fatalf("root description must not be attached to apiVersion details, got:\n%s", apiVersionLine)
	}
	for _, unwanted := range []string{
		`data-kdoc-toggle>▼</button>`,
		`data-kdoc-toggle>▶</button>`,
		`data-kdoc-search`,
		`data-search=`,
		`data-field=`,
		`kdoc-search-hit`,
		`kdoc-current`,
		`kdoc-metadata`,
		`fieldOnly`,
		`focusResult`,
		`.kdoc-details{position:static}`,
		`data-kdoc-back-url="`,
		`data-kdoc-quit-url="`,
		`# required</span> <span class="kdoc-required-label"># required`,
		`metadata.ownerReferences.apiVersion.kind`,
		`highlightElement(text, fieldName);`,
		`root.querySelectorAll("mark.kdoc-filter-hit")`,
		`lines.forEach(function(item){ item.classList.remove("kdoc-selected"); });`,
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

func TestRenderAllVersionsScopesFilterAndRootDescription(t *testing.T) {
	doc := func(version, description string) *crd.Document {
		return &crd.Document{
			Group:   "example.io",
			Version: version,
			Kind:    "Widget",
			Plural:  "widgets",
			Schema: &docschema.Structural{
				Generic: docschema.Generic{
					Type:        "object",
					Description: description,
				},
				Properties: map[string]docschema.Structural{
					"spec": {
						Generic: docschema.Generic{Type: "object", Description: "Spec describes this version."},
						Properties: map[string]docschema.Structural{
							"mode": {Generic: docschema.Generic{Type: "string", Description: "Mode selects behavior."}},
						},
					},
				},
			},
		}
	}

	var out bytes.Buffer
	if err := (Renderer{ExpandDepth: 1, Descriptions: yamlrender.DescriptionTrue, Columns: 34}).RenderAll(&out, []*crd.Document{
		doc("v1", "Widget v1 root description wraps as one selectable documentation block."),
		doc("v2", "Widget v2 root description wraps as another selectable documentation block."),
	}); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	for _, expected := range []string{
		`<section class="kdoc-version"><h2>example.io/v1</h2>`,
		`<section class="kdoc-version"><h2>example.io/v2</h2>`,
		`data-detail-id="root-description-example-io-v1"`,
		`data-detail-id="root-description-example-io-v2"`,
		".kdoc-version.kdoc-filtering .kdoc-line{display:none}",
		"var filterScopeSection = null",
		"state.version !== scope",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected all-version HTML to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, `data-detail-id="root-description"`) {
		t.Fatalf("all-version root descriptions must be version-qualified, got:\n%s", rendered)
	}
	if count := strings.Count(rendered, `data-detail-id="root-description-example-io-v1"`); count < 2 {
		t.Fatalf("expected wrapped v1 root description lines to share one detail id, count=%d:\n%s", count, rendered)
	}
	if count := strings.Count(rendered, `data-detail-id="root-description-example-io-v2"`); count < 2 {
		t.Fatalf("expected wrapped v2 root description lines to share one detail id, count=%d:\n%s", count, rendered)
	}
}

func TestRenderDoesNotExposeMetadataWrapperDefault(t *testing.T) {
	var out bytes.Buffer
	doc := &crd.Document{
		Group:      "apps",
		Version:    "v1",
		Kind:       "Deployment",
		Plural:     "deployments",
		Namespaced: true,
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

	if err := (Renderer{ExpandDepth: 1}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	if rendered := out.String(); strings.Contains(rendered, "# default") {
		t.Fatalf("metadata wrapper default must not be exposed, got:\n%s", rendered)
	}
}

func TestRenderYAMLCommentHighlightsRequiredToken(t *testing.T) {
	rendered := yamltokens.RenderHTML("replicas: 1 # default, required, minimum: 0", true)
	expected := `<span class="kdoc-yaml-comment"> # default, </span><span class="kdoc-required-label">required</span><span class="kdoc-yaml-comment">, minimum: 0</span>`
	if !strings.Contains(rendered, expected) {
		t.Fatalf("unexpected required comment rendering\nwant to contain: %s\ngot:             %s", expected, rendered)
	}

	rendered = yamltokens.RenderHTML("field: value # requiredFields: name", true)
	if strings.Contains(rendered, "kdoc-required-label") {
		t.Fatalf("requiredFields must not be highlighted as required label, got %s", rendered)
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
			expected: `<span class="kdoc-yaml-type-number">&lt;int32&gt;</span>`,
		},
		{
			name:     "boolean",
			token:    "<boolean>",
			expected: `<span class="kdoc-yaml-bool">&lt;boolean&gt;</span>`,
		},
		{
			name:     "int or string",
			token:    "<int-or-string>",
			expected: `<span class="kdoc-yaml-placeholder">&lt;int-or-string&gt;</span>`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := yamltokens.RenderHTML("field: "+tc.token, true)
			if !strings.Contains(got, tc.expected) {
				t.Fatalf("expected %s to render as %q, got %q", tc.token, tc.expected, got)
			}
		})
	}
}

func TestBuildLinesTreatsCommentedListItemFieldAsField(t *testing.T) {
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

	lines := tree.Build(doc, tree.Options{
		ExpandDepth:  3,
		Descriptions: tree.DescriptionFalse,
	})
	location, ok := findLine(lines, "spec.management_clusters[].cluster_location")
	if !ok {
		t.Fatalf("expected cluster_location line, got %#v", lines)
	}
	name, ok := findLine(lines, "spec.management_clusters[].cluster_name")
	if !ok {
		t.Fatalf("expected cluster_name line, got %#v", lines)
	}
	if location.Field != "cluster_location" || !strings.Contains(location.Text, `# - cluster_location: "<string>"`) {
		t.Fatalf("expected commented list item to be a field line, got %#v", location)
	}
	if location.Foldable {
		t.Fatalf("did not expect scalar array item field to be foldable, got %#v", location)
	}
	if name.Depth != location.Depth {
		t.Fatalf("expected sibling field depth after commented list marker, got %d and %d", location.Depth, name.Depth)
	}
}

func findLine(lines []tree.Line, path string) (tree.Line, bool) {
	for _, line := range lines {
		if line.Path == path {
			return line, true
		}
	}
	return tree.Line{}, false
}

func TestRenderStandaloneCommentsCarryWrapPrefixes(t *testing.T) {
	for _, tc := range []struct {
		line     string
		expected string
	}{
		{
			line:     "  # plain comment",
			expected: `data-kdoc-comment-prefix="  # " data-kdoc-comment-wrap-prefix="  # " data-kdoc-comment-text="plain comment"`,
		},
		{
			line:     "  - # list comment",
			expected: `data-kdoc-comment-prefix="  - # " data-kdoc-comment-wrap-prefix="    # " data-kdoc-comment-text="list comment"`,
		},
		{
			line:     "  # - # commented list comment",
			expected: `data-kdoc-comment-prefix="  # - # " data-kdoc-comment-wrap-prefix="  #   # " data-kdoc-comment-text="commented list comment"`,
		},
	} {
		if rendered := renderYAMLText(htmlLine{Line: tree.Line{Text: tc.line}}); !strings.Contains(rendered, tc.expected) {
			t.Fatalf("expected wrapped comment metadata %q, got %q", tc.expected, rendered)
		}
	}
}
