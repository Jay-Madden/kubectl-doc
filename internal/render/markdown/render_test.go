package markdownrender

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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
		`<KubeSchemaDoc data={kubectlDocSchemas[0]} filtering={true} />`,
		`"complete": true`,
		`"text": "spec: # required"`,
		`"text": "  mode: \"\u003cstring\u003e\" # required, minLength: 1"`,
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
	if strings.Contains(rendered, `<Accordion title={"YAML"}`) {
		t.Fatalf("interactive Fern Markdown should not wrap the schema tree in a Fern accordion:\n%s", rendered)
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

func TestRenderFernMarkdownOmitsLineLevelDescriptionIndexes(t *testing.T) {
	var out bytes.Buffer
	if err := (Renderer{Dialect: DialectFern, ExpandDepth: 1, HideFieldDetails: true}).Render(&out, testDocument()); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	payloads := embeddedFernPayloads(t, rendered)
	if len(payloads) != 1 {
		t.Fatalf("expected one payload, got %d", len(payloads))
	}
	if strings.Contains(rendered, `"filterText"`) {
		t.Fatalf("Fern payload should not duplicate filter text in line records:\n%s", rendered)
	}
	lineJSON, err := json.Marshal(payloads[0].Lines)
	if err != nil {
		t.Fatal(err)
	}
	var lines []map[string]any
	if err := json.Unmarshal(lineJSON, &lines); err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		if _, ok := line["description"]; ok {
			t.Fatalf("line records should not duplicate field descriptions:\n%s", lineJSON)
		}
		if _, ok := line["filterText"]; ok {
			t.Fatalf("line records should not duplicate filter text:\n%s", lineJSON)
		}
	}
	field := fernFieldByPath(payloads[0].Fields, "spec.mode")
	if field == nil || field.Description != "Mode selects the widget behavior." {
		t.Fatalf("field record should retain description for details/filtering, got %#v", field)
	}
}

func TestRenderFernMarkdownCanWriteFullSchemaSidecars(t *testing.T) {
	var out bytes.Buffer
	dir := t.TempDir()
	if err := (Renderer{
		Dialect:           DialectFern,
		ExpandDepth:       0,
		HideFieldDetails:  true,
		FernSchemaDir:     dir,
		FernSchemaURLPath: "./schemas",
	}).Render(&out, testDocument()); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	payloads := embeddedFernPayloads(t, rendered)
	if len(payloads) != 1 {
		t.Fatalf("expected one embedded shallow payload, got %d", len(payloads))
	}
	shallow := payloads[0]
	if shallow.Complete {
		t.Fatalf("embedded payload should be shallow, got complete=true")
	}
	if shallow.FullURL != "./schemas/widget-schema-0-full.md" {
		t.Fatalf("unexpected full payload URL %q", shallow.FullURL)
	}
	if metadata := fernLineByPath(shallow.Lines, "metadata"); metadata == nil || !metadata.Collapsed {
		t.Fatalf("shallow payload should keep metadata collapsed, got %#v", metadata)
	}
	if hasFernLinePath(shallow.Lines, "spec.mode") {
		t.Fatalf("shallow payload should not include collapsed descendant spec.mode")
	}

	full := fernPayloadFile(t, filepath.Join(dir, "widget-schema-0-full.md"))
	if !full.Complete {
		t.Fatalf("full sidecar payload should be complete")
	}
	if full.FullURL != "" {
		t.Fatalf("full sidecar payload should not point to another full payload, got %q", full.FullURL)
	}
	if !hasFernLinePath(full.Lines, "spec.mode") {
		t.Fatalf("full payload should include collapsed descendant spec.mode")
	}
	if len(shallow.Lines) >= len(full.Lines) {
		t.Fatalf("expected shallow payload to be smaller than full payload, got %d >= %d", len(shallow.Lines), len(full.Lines))
	}
	for _, line := range shallow.Lines {
		fullLine := full.Lines[line.Index]
		if fullLine.Index != line.Index || fullLine.Text != line.Text || fullLine.Path != line.Path {
			t.Fatalf("shallow line index %d does not match full payload line: %#v vs %#v", line.Index, line, fullLine)
		}
	}
}

func TestRenderAllFernMarkdownCanWriteVersionedSchemaSidecars(t *testing.T) {
	var out bytes.Buffer
	dir := t.TempDir()
	v1 := testDocument()
	v2 := testDocument()
	v2.Version = "v2"

	if err := (Renderer{
		Dialect:          DialectFern,
		ExpandDepth:      0,
		HideFieldDetails: true,
		FernSchemaDir:    dir,
	}).RenderAll(&out, []*crd.Document{v2, v1}); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	for _, expected := range []string{
		"<Tabs>",
		`<Tab title={"example.io/v2"}>`,
		`<Tab title={"example.io/v1"}>`,
		`<KubeSchemaDoc data={kubectlDocSchemas[0]} filtering={true} />`,
		`<KubeSchemaDoc data={kubectlDocSchemas[1]} filtering={true} />`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected all-version sidecar output to contain %q, got:\n%s", expected, rendered)
		}
	}

	payloads := embeddedFernPayloads(t, rendered)
	if len(payloads) != 2 {
		t.Fatalf("expected two embedded shallow payloads, got %d", len(payloads))
	}
	for i, payload := range payloads {
		index := strconv.Itoa(i)
		expectedURL := "./widget-schema-" + index + "-full.md"
		if payload.Complete {
			t.Fatalf("payload %d should be shallow", i)
		}
		if payload.FullURL != expectedURL {
			t.Fatalf("payload %d full URL: expected %q, got %q", i, expectedURL, payload.FullURL)
		}
		if _, err := os.Stat(filepath.Join(dir, "widget-schema-"+index+"-full.md")); err != nil {
			t.Fatalf("expected full sidecar %d: %v", i, err)
		}
	}
}

func TestRenderDynamoGraphDeploymentFernSidecarPayloadSize(t *testing.T) {
	var out bytes.Buffer
	dir := t.TempDir()
	doc, err := crd.Load([]string{"../../cli/testdata/dynamographdeployment-crd.yaml"}, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := (Renderer{
		Dialect:          DialectFern,
		ExpandDepth:      0,
		Descriptions:     yamlrender.DescriptionFalse,
		HideFieldDetails: true,
		FernSchemaDir:    dir,
	}).Render(&out, doc); err != nil {
		t.Fatal(err)
	}

	shallow := embeddedFernPayloads(t, out.String())[0]
	full := fernPayloadFile(t, filepath.Join(dir, "dynamo-graph-deployment-schema-0-full.md"))
	if shallow.Complete || !full.Complete {
		t.Fatalf("expected shallow embedded payload and complete sidecar, got shallow=%t full=%t", shallow.Complete, full.Complete)
	}
	if len(shallow.Lines) > 10 {
		t.Fatalf("DynamoGraphDeployment shallow payload grew unexpectedly: %d lines", len(shallow.Lines))
	}
	if len(full.Lines) < 50 {
		t.Fatalf("DynamoGraphDeployment full payload is unexpectedly small: %d lines", len(full.Lines))
	}
	if len(shallow.Fields) > 10 {
		t.Fatalf("DynamoGraphDeployment shallow field payload grew unexpectedly: %d fields", len(shallow.Fields))
	}
	if len(full.Fields) < 50 {
		t.Fatalf("DynamoGraphDeployment full field payload is unexpectedly small: %d fields", len(full.Fields))
	}
	if hasFernLinePath(shallow.Lines, "spec.components[].podTemplate") {
		t.Fatalf("shallow payload should not include collapsed Dynamo podTemplate")
	}
	if fernFieldByPath(shallow.Fields, "spec.components[].podTemplate") != nil {
		t.Fatalf("shallow payload should not include collapsed Dynamo podTemplate details")
	}
	if status := fernLineByPath(shallow.Lines, "status"); status == nil || !status.Collapsed {
		t.Fatalf("shallow payload should include collapsed Dynamo status, got %#v", status)
	}
	if metadata := fernLineByPath(shallow.Lines, "metadata"); metadata == nil || !metadata.Collapsed {
		t.Fatalf("shallow payload should include collapsed Dynamo metadata, got %#v", metadata)
	}
	if status := fernLineByPath(full.Lines, "status"); status == nil || !status.Collapsed {
		t.Fatalf("full payload should keep Dynamo status collapsed, got %#v", status)
	}
	if metadata := fernLineByPath(full.Lines, "metadata"); metadata == nil || !metadata.Collapsed {
		t.Fatalf("full payload should keep Dynamo metadata collapsed, got %#v", metadata)
	}
	if !hasFernLinePath(full.Lines, "spec.components[].podTemplate") {
		t.Fatalf("full payload should include Dynamo podTemplate")
	}
	if fernFieldByPath(full.Fields, "spec.components[].podTemplate") == nil {
		t.Fatalf("full payload should include Dynamo podTemplate details for filtering")
	}
	if shallowSize, fullSize := len(jsonCompact(shallow)), len(jsonCompact(full)); shallowSize*4 >= fullSize {
		t.Fatalf("DynamoGraphDeployment shallow payload is not small enough: shallow=%d full=%d", shallowSize, fullSize)
	}
}

func TestRenderFernMarkdownSidecarsAreDeterministic(t *testing.T) {
	render := func(t *testing.T) (string, string) {
		t.Helper()
		var out bytes.Buffer
		dir := t.TempDir()
		if err := (Renderer{
			Dialect:          DialectFern,
			ExpandDepth:      0,
			HideFieldDetails: true,
			FernSchemaDir:    dir,
		}).Render(&out, testDocument()); err != nil {
			t.Fatal(err)
		}
		sidecar, err := os.ReadFile(filepath.Join(dir, "widget-schema-0-full.md"))
		if err != nil {
			t.Fatal(err)
		}
		return out.String(), string(sidecar)
	}

	firstMDX, firstSidecar := render(t)
	secondMDX, secondSidecar := render(t)
	if firstMDX != secondMDX {
		t.Fatalf("embedded Fern MDX payload is not deterministic\nfirst:\n%s\nsecond:\n%s", firstMDX, secondMDX)
	}
	if firstSidecar != secondSidecar {
		t.Fatalf("Fern sidecar payload is not deterministic\nfirst:\n%s\nsecond:\n%s", firstSidecar, secondSidecar)
	}
}

func TestRenderFernMarkdownSidecarsDoNotReferenceLiveOpenAPI(t *testing.T) {
	var out bytes.Buffer
	dir := t.TempDir()
	if err := (Renderer{
		Dialect:          DialectFern,
		ExpandDepth:      0,
		HideFieldDetails: true,
		FernSchemaDir:    dir,
	}).Render(&out, testDocument()); err != nil {
		t.Fatal(err)
	}

	sidecar, err := os.ReadFile(filepath.Join(dir, "widget-schema-0-full.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{out.String(), string(sidecar)} {
		for _, forbidden := range []string{"localhost", "/openapi", "openapi/v2", "openapi/v3"} {
			if strings.Contains(strings.ToLower(output), forbidden) {
				t.Fatalf("Fern output should not reference live OpenAPI endpoint %q:\n%s", forbidden, output)
			}
		}
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

func embeddedFernPayloads(t *testing.T, rendered string) []fernDocumentPayload {
	t.Helper()
	const prefix = "export const kubectlDocSchemas = "
	start := strings.Index(rendered, prefix)
	if start < 0 {
		t.Fatalf("embedded Fern payload export not found:\n%s", rendered)
	}
	start += len(prefix)
	end := strings.Index(rendered[start:], ";\n\n")
	if end < 0 {
		t.Fatalf("embedded Fern payload terminator not found:\n%s", rendered[start:])
	}
	var payloads []fernDocumentPayload
	if err := json.Unmarshal([]byte(rendered[start:start+end]), &payloads); err != nil {
		t.Fatalf("decode embedded Fern payloads: %v", err)
	}
	return payloads
}

func fernPayloadFile(t *testing.T, path string) fernDocumentPayload {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Fern payload file %s: %v", path, err)
	}
	text := string(data)
	const startFence = "```kubectl-doc-schema"
	start := strings.Index(text, startFence)
	if start < 0 {
		t.Fatalf("payload fence not found in %s:\n%s", path, text)
	}
	start += len(startFence)
	end := strings.Index(text[start:], "```")
	if end < 0 {
		t.Fatalf("payload fence terminator not found in %s:\n%s", path, text)
	}
	encoded := strings.Join(strings.Fields(text[start:start+end]), "")
	payloadJSON, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode base64 payload %s: %v", path, err)
	}
	var payload fernDocumentPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		t.Fatalf("decode Fern payload JSON %s: %v", path, err)
	}
	return payload
}

func hasFernLinePath(lines []fernLinePayload, path string) bool {
	return fernLineByPath(lines, path) != nil
}

func fernLineByPath(lines []fernLinePayload, path string) *fernLinePayload {
	for i := range lines {
		if lines[i].Path == path {
			return &lines[i]
		}
	}
	return nil
}

func fernFieldByPath(fields []fernFieldPayload, path string) *fernFieldPayload {
	for i := range fields {
		if fields[i].Path == path {
			return &fields[i]
		}
	}
	return nil
}
