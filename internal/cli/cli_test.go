package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sttts/kubectl-doc/internal/crd"
	"github.com/sttts/kubectl-doc/internal/kube"
	"github.com/sttts/kubectl-doc/internal/tui"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestRendersClusterOverviewWhenNoCRDFile(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		LoadOverview: func() (*kube.Overview, error) {
			return &kube.Overview{
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
							{Name: "daemonsets", Versions: []string{"v1"}},
							{Name: "deployments", Versions: []string{"v1", "v1beta1"}},
						},
					},
				},
			}, nil
		},
		LoadResourceResolver: func() (*kube.ResourceResolver, error) {
			t.Fatal("should not load resolver for overview")
			return nil, nil
		},
	})
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	expected := `core:
  pods: v1
apps:
  daemonsets: v1
  deployments: ["v1","v1beta1"]
`
	if out.String() != expected {
		t.Fatalf("unexpected output\nwant:\n%s\ngot:\n%s", expected, out.String())
	}
	assertParsesAsYAML(t, out.Bytes())
}

func TestClusterOverviewReportsDiscoveryErrors(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	expected := errors.New("discovery failed")
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		LoadOverview: func() (*kube.Overview, error) {
			return nil, expected
		},
	})
	cmd.SetArgs(nil)

	err := cmd.Execute()
	if !errors.Is(err, expected) {
		t.Fatalf("expected discovery error, got %v", err)
	}
}

func TestRendersClusterResourceFromOpenAPI(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		LoadOverview: func() (*kube.Overview, error) {
			t.Fatal("should not load overview for resource selectors")
			return nil, nil
		},
		LoadResourceResolver: func() (*kube.ResourceResolver, error) {
			return testResourceResolver(t), nil
		},
		LoadOpenAPIClient: func() (*kube.OpenAPIClient, error) {
			return testOpenAPIClient(t), nil
		},
	})
	cmd.SetArgs([]string{"deployments"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	expected := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: "<name>"
  namespace: "<namespace>"
# DeploymentSpec is the desired state.
spec: # optional
  # Label selector.
  selector: {} # required

  # replicas: 1 # default, minimum: 0

# Deployment status.
# status: {}
`
	if out.String() != expected {
		t.Fatalf("unexpected output\nwant:\n%s\ngot:\n%s", expected, out.String())
	}
	assertParsesAsYAML(t, out.Bytes())
}

func TestClusterResourceSelectorReportsResolutionErrors(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		LoadResourceResolver: func() (*kube.ResourceResolver, error) {
			return testResourceResolver(t), nil
		},
		LoadOpenAPIClient: func() (*kube.OpenAPIClient, error) {
			t.Fatal("should not load OpenAPI for unresolved resources")
			return nil, nil
		},
	})
	cmd.SetArgs([]string{"does-not-exist"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRendersCRDFileAsYAML(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "yaml"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	expected := `apiVersion: stable.example.com/v1
kind: CronTab
metadata:
  name: "<name>"
  namespace: "<namespace>"
# CronTabSpec describes the desired cron job.
spec: # required
  # Cron expression for running the job.
  cronSpec: "<string>" # required, minLength: 1

  # Container image used by the job.
  image: "<string>" # required

  # concurrencyPolicy: "Allow" # default, enum: "Forbid" | "Replace"

  # labels:
    # <key>: "<string>"

  ports: # optional
    - # Port exposed by the container.
      containerPort: <int32> # required

      # Port name.
      name: "<string>" # required

      # protocol: "TCP" # default, enum: "UDP"

  # replicas: 1 # default, minimum: 0

# status: {}
`
	if out.String() != expected {
		t.Fatalf("unexpected output\nwant:\n%s\ngot:\n%s", expected, out.String())
	}
	assertParsesAsYAML(t, out.Bytes())
}

func TestRendersRequestedCRDVersion(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "--version", "v1alpha1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	expected := `apiVersion: stable.example.com/v1alpha1
kind: CronTab
metadata:
  name: "<name>"
  namespace: "<namespace>"
spec: # required
  # Cron expression for running the job.
  cronSpec: "<string>" # required, minLength: 1

  # Container image used by the job.
  # image: "<string>"
`
	if out.String() != expected {
		t.Fatalf("unexpected output\nwant:\n%s\ngot:\n%s", expected, out.String())
	}
	assertParsesAsYAML(t, out.Bytes())
}

func TestRendersCRDFileAsMarkdown(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "markdown", "--version", "v1alpha1", "--descriptions=false"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	rendered := out.String()
	for _, expected := range []string{
		"# CronTab\n",
		"| API Version | `stable.example.com/v1alpha1` |",
		"| Kind | `CronTab` |",
		"| Resource | `crontabs` |",
		"```yaml\napiVersion: stable.example.com/v1alpha1\nkind: CronTab\n",
		`cronSpec: "<string>" # required, minLength: 1`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected Markdown to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.HasPrefix(rendered, "---\n") {
		t.Fatalf("markdown alias should render GitHub Markdown without Fern frontmatter:\n%s", rendered)
	}
	if strings.Contains(rendered, "## Field Details") {
		t.Fatalf("Markdown should hide field details by default:\n%s", rendered)
	}
}

func TestRendersCRDFileAsFernMarkdown(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "markdown-fern", "--version", "v1alpha1", "--descriptions=false"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	rendered := out.String()
	for _, expected := range []string{
		"---\ntitle: \"CronTab\"\n---\n\n",
		`import { KubeSchemaDoc } from "@/components/kubectl-doc/KubeSchemaDoc";`,
		"# CronTab\n",
		`<KubeSchemaDoc data={kubectlDocSchemas[0]} filtering={true} />`,
		`"apiVersion": "stable.example.com/v1alpha1"`,
		`"tokens": [`,
		`"k": "key"`,
		`"t": "apiVersion"`,
		`"t": "spec"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected Fern Markdown to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, "<ParamField") {
		t.Fatalf("Fern Markdown should hide static field details by default:\n%s", rendered)
	}
}

func TestFernMarkdownCanDisableFiltering(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "markdown-fern", "--version", "v1alpha1", "--disable-filtering"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	rendered := out.String()
	if !strings.Contains(rendered, `<KubeSchemaDoc data={kubectlDocSchemas[0]} filtering={false} />`) {
		t.Fatalf("expected disabled filtering component flag, got:\n%s", rendered)
	}
	if strings.Contains(rendered, `"filterText"`) {
		t.Fatalf("disabled filtering should omit filterText indexes, got:\n%s", rendered)
	}
}

func TestFernMarkdownCanWriteSchemaSidecars(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	dir := t.TempDir()
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{
		"-f", "testdata/crontab-crd.yaml",
		"-o", "markdown-fern",
		"--version", "v1alpha1",
		"--expand-depth", "0",
		"--fern-schema-dir", dir,
		"--fern-schema-url-path", "./schemas",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	rendered := out.String()
	for _, expected := range []string{
		`"complete": false`,
		`"fullPayloadURL": "./schemas/cron-tab-schema-0-full.json"`,
		`<KubeSchemaDoc data={kubectlDocSchemas[0]} filtering={true} />`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected Fern Markdown sidecar output to contain %q, got:\n%s", expected, rendered)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "cron-tab-schema-0-full.json")); err != nil {
		t.Fatalf("expected full schema sidecar: %v", err)
	}
}

func TestFernSchemaDirRequiresFernOutput(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "markdown", "--fern-schema-dir", t.TempDir()})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--fern-schema-dir requires -o markdown-fern") {
		t.Fatalf("expected --fern-schema-dir validation error, got %v\nstderr:\n%s", err, errOut.String())
	}
}

func TestFernSchemaURLPathRequiresSchemaDir(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "markdown-fern", "--fern-schema-url-path", "./schemas"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--fern-schema-url-path requires --fern-schema-dir") {
		t.Fatalf("expected --fern-schema-url-path validation error, got %v\nstderr:\n%s", err, errOut.String())
	}
}

func TestFernMarkdownCanRenderAllVersionsAndFieldDetails(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "markdown-fern", "--all-versions", "--descriptions=false", "--field-details"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	rendered := out.String()
	for _, expected := range []string{
		"| Versions | `stable.example.com/v1`, `stable.example.com/v1alpha1` |",
		"<Tabs>",
		`<Tab title={"stable.example.com/v1"}>`,
		`<Tab title={"stable.example.com/v1alpha1"}>`,
		`<KubeSchemaDoc data={kubectlDocSchemas[0]} filtering={true} />`,
		`<KubeSchemaDoc data={kubectlDocSchemas[1]} filtering={true} />`,
		`<ParamField path={"spec.cronSpec"} type={"string"} required={true}>`,
		`- minLength: 1`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected all-version Fern Markdown to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestRendersCRDFileAsHTML(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "html", "--version", "v1alpha1", "--descriptions=false"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	rendered := out.String()
	for _, expected := range []string{
		"<!doctype html>",
		"<title>CronTab</title>",
		"class=\"kubectl-doc\"",
		"<h1>CronTab <small>stable.example.com/v1alpha1</small></h1>",
		"data-kdoc-toggle",
		`<span class="kdoc-yaml-key">apiVersion</span><span class="kdoc-yaml-punct">:</span> <span class="kdoc-yaml-scalar">stable.example.com/v1alpha1</span>`,
		`<span class="kdoc-yaml-key">cronSpec</span><span class="kdoc-yaml-punct">:</span> <span class="kdoc-yaml-string">&#34;&lt;string&gt;&#34;</span><span class="kdoc-yaml-comment"> # </span><span class="kdoc-required-label">required</span><span class="kdoc-yaml-comment">, minLength: 1</span>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected HTML to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.Contains(strings.ToLower(rendered), "copy") {
		t.Fatalf("HTML must not contain copy controls, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "data-kdoc-search") {
		t.Fatalf("HTML must use browser search instead of custom search controls, got:\n%s", rendered)
	}
}

func TestRendersCRDFileAsKro(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "kro", "--version", "v1alpha1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	rendered := out.String()
	for _, expected := range []string{
		"apiVersion: stable.example.com/v1alpha1\n",
		"kind: CronTab\n",
		"spec: # required=true",
		`cronSpec: string | required=true minLength=1 description="Cron expression for running the job."`,
		`image: string | description="Container image used by the job."`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected Kro output to contain %q, got:\n%s", expected, rendered)
		}
	}
	assertParsesAsYAML(t, out.Bytes())
}

func TestRendersCRDFileAsAllVersionsKro(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "kro", "--all-versions", "--descriptions=false"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	rendered := out.String()
	for _, expected := range []string{
		"apiVersion: stable.example.com/v1\n",
		"---\napiVersion: stable.example.com/v1alpha1\n",
		`concurrencyPolicy: string | default="Allow" enum="Allow,Forbid,Replace"`,
		"ports:\n",
		`    - containerPort: integer | required=true format=int32`,
		`      name: string | required=true`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected all-version Kro output to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestRendersCRDFileAsJSONSchema(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "jsonschema", "--version", "v1alpha1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	rendered := out.String()
	for _, unexpected := range []string{"apiVersion:", "kind:", "# required", "# Cron expression"} {
		if strings.Contains(rendered, unexpected) {
			t.Fatalf("JSON Schema output must not contain rendered YAML markup %q, got:\n%s", unexpected, rendered)
		}
	}

	var parsed map[string]interface{}
	if err := yaml.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("expected JSON Schema output to parse as YAML: %v\n%s", err, rendered)
	}
	if parsed["type"] != "object" {
		t.Fatalf("expected root object schema, got %#v", parsed["type"])
	}
	properties := parsed["properties"].(map[string]interface{})
	spec := properties["spec"].(map[string]interface{})
	specProperties := spec["properties"].(map[string]interface{})
	cronSpec := specProperties["cronSpec"].(map[string]interface{})
	if cronSpec["type"] != "string" || cronSpec["description"] != "Cron expression for running the job." {
		t.Fatalf("expected plain cronSpec JSON Schema, got %#v", cronSpec)
	}
	if cronSpec["minLength"] != float64(1) {
		t.Fatalf("expected cronSpec minLength, got %#v", cronSpec["minLength"])
	}
}

func TestRendersCRDFileAsAllVersionsMarkdown(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "markdown", "--all-versions", "--descriptions=false", "--field-details=true"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	rendered := out.String()
	for _, expected := range []string{
		"| Versions | `stable.example.com/v1`, `stable.example.com/v1alpha1` |",
		"## stable.example.com/v1\n",
		"## stable.example.com/v1alpha1\n",
		"<summary>YAML: stable.example.com/v1</summary>",
		"### Field details: stable.example.com/v1\n",
		`<a id="field-stable-example-com-v1-spec-cronspec"></a>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected all-version Markdown to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestRendersClusterResourceAsAllVersionsMarkdown(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		LoadResourceResolver: func() (*kube.ResourceResolver, error) {
			return testResourceResolver(t), nil
		},
		LoadOpenAPIClient: func() (*kube.OpenAPIClient, error) {
			return testOpenAPIClient(t), nil
		},
	})
	cmd.SetArgs([]string{"-o", "markdown", "--all-versions", "--descriptions=false", "deployments"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	rendered := out.String()
	for _, expected := range []string{
		"| Versions | `apps/v1`, `apps/v1beta1` |",
		"## apps/v1\n",
		"## apps/v1beta1\n",
		"apiVersion: apps/v1beta1\nkind: Deployment\n",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected cluster all-version Markdown to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestRendersClusterResourceAsAllVersionsKro(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		LoadResourceResolver: func() (*kube.ResourceResolver, error) {
			return testResourceResolver(t), nil
		},
		LoadOpenAPIClient: func() (*kube.OpenAPIClient, error) {
			return testOpenAPIClient(t), nil
		},
	})
	cmd.SetArgs([]string{"-o", "kro", "--all-versions", "deployments"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	rendered := out.String()
	for _, expected := range []string{
		"apiVersion: apps/v1\n",
		"---\napiVersion: apps/v1beta1\n",
		"kind: Deployment\n",
		`selector: object | required=true description="Label selector."`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected cluster all-version Kro output to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestAllVersionsConflictsWithVersion(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "markdown", "--all-versions", "--version", "v1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "--all-versions conflicts with --version" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkdownColumnsFlagWrapsDescriptions(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "markdown", "--version", "v1alpha1", "--columns", "24"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	expected := "  # Cron expression for\n  # running the job.\n  cronSpec:"
	if !strings.Contains(out.String(), expected) {
		t.Fatalf("expected Markdown to contain wrapped description %q, got:\n%s", expected, out.String())
	}
}

func TestYAMLColumnsFlagWrapsDescriptions(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "yaml", "--version", "v1alpha1", "--columns", "24"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	expected := "  # Cron expression for\n  # running the job.\n  cronSpec:"
	if !strings.Contains(out.String(), expected) {
		t.Fatalf("expected YAML to contain wrapped description %q, got:\n%s", expected, out.String())
	}
}

func TestMarkdownRequiresResourceSelectorInClusterMode(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		LoadOverview: func() (*kube.Overview, error) {
			t.Fatal("should not render discovery overview for markdown")
			return nil, nil
		},
	})
	cmd.SetArgs([]string{"-o", "markdown"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "resource selector required for -o markdown" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTMLRequiresResourceSelectorInClusterMode(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		LoadOverview: func() (*kube.Overview, error) {
			t.Fatal("should not render discovery overview for static html")
			return nil, nil
		},
	})
	cmd.SetArgs([]string{"-o", "html"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "resource selector required for -o html" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestColumnsRejectsNegativeValues(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "markdown", "--columns", "-1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "--columns must be non-negative" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRendersRequiredDescriptionsOnly(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "--version", "v1alpha1", "--descriptions=required"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	expected := `apiVersion: stable.example.com/v1alpha1
kind: CronTab
metadata:
  name: "<name>"
  namespace: "<namespace>"
spec: # required
  # Cron expression for running the job.
  cronSpec: "<string>" # required, minLength: 1

  # image: "<string>"
`
	if out.String() != expected {
		t.Fatalf("unexpected output\nwant:\n%s\ngot:\n%s", expected, out.String())
	}
	assertParsesAsYAML(t, out.Bytes())
}

func TestCanDisableDescriptions(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "--version", "v1alpha1", "--descriptions=false"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	expected := `apiVersion: stable.example.com/v1alpha1
kind: CronTab
metadata:
  name: "<name>"
  namespace: "<namespace>"
spec: # required
  cronSpec: "<string>" # required, minLength: 1
  # image: "<string>"
`
	if out.String() != expected {
		t.Fatalf("unexpected output\nwant:\n%s\ngot:\n%s", expected, out.String())
	}
	assertParsesAsYAML(t, out.Bytes())
}

func TestInteractiveShortcutNormalizesToTUI(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	var called bool
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		RunTUI: func(ctx context.Context, out io.Writer, doc *crd.Document, config tui.Config) error {
			called = true
			if doc.Kind != "CronTab" {
				t.Fatalf("expected CronTab document, got %s", doc.Kind)
			}
			if config.ExpandDepth != 2 {
				t.Fatalf("expected default expand depth 2, got %d", config.ExpandDepth)
			}
			_, err := fmt.Fprintf(out, "tui %s\n", doc.Kind)
			return err
		},
	})
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-i"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}
	if !called {
		t.Fatalf("expected TUI runner to be called")
	}
	if out.String() != "tui CronTab\n" {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestInteractiveTerminalDefaultsToTUI(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	var called bool
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		IsInteractive: func(out io.Writer) bool {
			return true
		},
		RunTUI: func(ctx context.Context, out io.Writer, doc *crd.Document, config tui.Config) error {
			called = true
			_, err := fmt.Fprintf(out, "tui %s\n", doc.Kind)
			return err
		},
	})
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}
	if !called {
		t.Fatalf("expected TUI runner to be called")
	}
	if out.String() != "tui CronTab\n" {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestExplicitYAMLKeepsYAMLOutputOnInteractiveTerminal(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		IsInteractive: func(out io.Writer) bool {
			return true
		},
		RunTUI: func(ctx context.Context, out io.Writer, doc *crd.Document, config tui.Config) error {
			t.Fatalf("TUI runner should not be called for explicit -o yaml")
			return nil
		},
	})
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "yaml", "--descriptions=false"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "apiVersion: stable.example.com/v1\nkind: CronTab\n") {
		t.Fatalf("expected YAML output, got %q", out.String())
	}
}

func TestExplicitYAMLUsesTerminalWidthForCommentWrapping(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		IsInteractive: func(out io.Writer) bool {
			return true
		},
		TerminalWidth: func(out io.Writer) int {
			return 24
		},
		RunTUI: func(ctx context.Context, out io.Writer, doc *crd.Document, config tui.Config) error {
			t.Fatalf("TUI runner should not be called for explicit -o yaml")
			return nil
		},
	})
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "yaml", "--version", "v1alpha1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	expected := "  # Cron expression for\n  # running the job.\n  cronSpec:"
	if !strings.Contains(out.String(), expected) {
		t.Fatalf("expected explicit YAML to wrap comments to terminal width, got:\n%s", out.String())
	}
}

func TestInteractiveShortcutRunsClusterOverviewWithoutResource(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	var called bool
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		LoadOverview: func() (*kube.Overview, error) {
			return &kube.Overview{
				Groups: []kube.Group{
					{
						Name: "apps",
						Resources: []kube.Resource{
							{Name: "deployments", Versions: []string{"v1"}},
						},
					},
				},
			}, nil
		},
		LoadResourceResolver: func() (*kube.ResourceResolver, error) {
			return testResourceResolver(t), nil
		},
		LoadOpenAPIClient: func() (*kube.OpenAPIClient, error) {
			return testOpenAPIClient(t), nil
		},
		RunTUIOverview: func(ctx context.Context, out io.Writer, overview *kube.Overview, config tui.OverviewConfig) error {
			called = true
			if len(overview.Groups) != 1 || overview.Groups[0].Name != "apps" {
				t.Fatalf("unexpected overview: %#v", overview)
			}
			if config.LoadDocument == nil {
				t.Fatal("expected overview document loader")
			}
			doc, err := config.LoadDocument(ctx, "apps", "v1", "deployments")
			if err != nil {
				t.Fatal(err)
			}
			if doc.Kind != "Deployment" {
				t.Fatalf("expected Deployment document, got %s", doc.Kind)
			}
			_, err = fmt.Fprintln(out, "tui overview")
			return err
		},
	})
	cmd.SetArgs([]string{"-i"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}
	if !called {
		t.Fatalf("expected TUI overview runner to be called")
	}
	if out.String() != "tui overview\n" {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestInteractiveShortcutConflictsWithDifferentOutput(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-i", "-o", "yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "--interactive conflicts with -o yaml" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebShortcutServesClusterOverviewAndLazySchema(t *testing.T) {
	var out lockedBuffer
	var errOut lockedBuffer
	var opened lockedBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		LoadOverview: func() (*kube.Overview, error) {
			return &kube.Overview{
				Groups: []kube.Group{
					{
						Name: "apps",
						Resources: []kube.Resource{
							{Name: "deployments", Versions: []string{"v1"}},
						},
					},
				},
			}, nil
		},
		LoadResourceResolver: func() (*kube.ResourceResolver, error) {
			return testResourceResolver(t), nil
		},
		LoadOpenAPIClient: func() (*kube.OpenAPIClient, error) {
			return testOpenAPIClient(t), nil
		},
		OpenBrowser: func(rawURL string) error {
			_, _ = opened.Write([]byte(rawURL))
			return errors.New("open failed")
		},
	})
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-w"})

	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Execute()
	}()

	baseURL := waitForBrowserURL(t, &out, errCh)
	if opened.String() != baseURL {
		t.Fatalf("expected browser opener to receive %q, got %q", baseURL, opened.String())
	}
	overview := httpGet(t, baseURL)
	for _, expected := range []string{
		"Kubernetes resources",
		"deployments",
		`<div class="kdoc-resource" data-kdoc-overview-resource data-resource-name="deployments" data-shortnames=""><span class="kdoc-resource-name">deployments</span><span class="kdoc-version"><a href="/?group=apps&amp;resource=deployments&amp;version=v1" data-kdoc-overview-item data-index="0" data-version="v1">v1</a></span>`,
		"?group=apps&amp;resource=deployments&amp;version=v1",
		`data-kdoc-overview-root`,
		`data-kdoc-filter-overlay hidden`,
		`function applyOverviewFilter()`,
		`function applyOverviewHighlights()`,
		`function pageDistance()`,
		`function selectGroup(direction)`,
		`case "ArrowDown":`,
		`case "ArrowLeft":`,
		`case "ArrowRight":`,
		`case "PageDown":`,
		`case "Enter":`,
		`kubectl-doc-overview-focus`,
		`.kdoc-group h2{color:#007c89;`,
	} {
		if !strings.Contains(overview, expected) {
			t.Fatalf("expected browser overview to contain %q, got:\n%s", expected, overview)
		}
	}
	for _, unwanted := range []string{
		"<strong>deployments</strong>",
		".kdoc-resource strong",
	} {
		if strings.Contains(overview, unwanted) {
			t.Fatalf("browser overview should render resources inline without bold markup %q, got:\n%s", unwanted, overview)
		}
	}

	schema := httpGet(t, baseURL+"?group=apps&resource=deployments&version=v1")
	for _, expected := range []string{
		"<!doctype html>",
		"Deployment",
		"<h1>Deployment <small>apps/v1</small></h1>",
		`data-kdoc-back-url="/"`,
		`case "Escape":`,
		`<span class="kdoc-yaml-key">apiVersion</span><span class="kdoc-yaml-punct">:</span> <span class="kdoc-yaml-scalar">apps/v1</span>`,
		"DeploymentSpec is the desired state.",
	} {
		if !strings.Contains(schema, expected) {
			t.Fatalf("expected browser schema page to contain %q, got:\n%s", expected, schema)
		}
	}
	if strings.Contains(strings.ToLower(schema), "copy") {
		t.Fatalf("browser page must not contain copy controls, got:\n%s", schema)
	}
	if strings.Contains(schema, "data-kdoc-search") {
		t.Fatalf("browser page must use browser search instead of custom search controls, got:\n%s", schema)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("browser command returned error: %v\nstderr:\n%s", err, errOut.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("browser command did not stop after context cancellation")
	}
}

func TestWebShortcutExplicitResourcePageCanQuitServer(t *testing.T) {
	var out lockedBuffer
	var errOut lockedBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		LoadResourceResolver: func() (*kube.ResourceResolver, error) {
			return testResourceResolver(t), nil
		},
		LoadOpenAPIClient: func() (*kube.OpenAPIClient, error) {
			return testOpenAPIClient(t), nil
		},
	})
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-w", "deployments"})

	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Execute()
	}()

	baseURL := waitForBrowserURL(t, &out, errCh)
	page := httpGet(t, baseURL)
	for _, expected := range []string{
		"Deployment",
		`data-kdoc-quit-url="/__kubectl-doc/quit"`,
		`function requestQuit()`,
	} {
		if !strings.Contains(page, expected) {
			t.Fatalf("expected explicit browser schema page to contain %q, got:\n%s", expected, page)
		}
	}
	if strings.Contains(page, `data-kdoc-back-url="`) {
		t.Fatalf("explicit browser schema page must not render overview back navigation, got:\n%s", page)
	}

	httpPost(t, baseURL+"__kubectl-doc/quit", http.StatusAccepted)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("browser command returned error after quit: %v\nstderr:\n%s", err, errOut.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("browser command did not stop after quit request")
	}
}

func TestRendersDynamoGraphDeploymentExtensions(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/dynamographdeployment-crd.yaml", "-o", "yaml", "--descriptions=false"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	expected := `apiVersion: nvidia.com/v1beta1
kind: DynamoGraphDeployment
metadata:
  name: "<name>"
  namespace: "<namespace>"
spec: # required
  components: # required, listType: map, listMapKeys: name
    - name: "<string>" # required, minLength: 1, maxLength: 63
      # podTemplate: {} # preserveUnknownFields
      # replicas: 1 # default, minimum: 0
      # resources: {} # show with --expand-depth 4

      services: # optional
        - {} # show with --expand-depth 5

      # sharedMemorySize: <int-or-string> # intOrString

  # annotations:
    # <key>: "<string>"

  # backendFramework: "sglang" # default, enum: "vllm" | "trtllm"

  envs: # optional
    - name: "<string>" # required, minLength: 1
      # value: "<string>"
      valueFrom: {} # optional, show with --expand-depth 4

# status: {}
`
	if out.String() != expected {
		t.Fatalf("unexpected output\nwant:\n%s\ngot:\n%s", expected, out.String())
	}
	assertParsesAsYAML(t, out.Bytes())
}

func TestRendersDynamoGraphDeploymentRequestExtensions(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/dynamographdeploymentrequest-crd.yaml", "-o", "yaml"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	expected := `apiVersion: nvidia.com/v1beta1
kind: DynamoGraphDeploymentRequest
metadata:
  name: "<name>"
  namespace: "<namespace>"
spec: # required
  model: "<string>" # required, minLength: 1
  # autoApply: true # default

  overrides: # optional
    # dgd: {} # preserveUnknownFields, embeddedResource
    profilingJob: {} # optional, show with --expand-depth 3
`
	if out.String() != expected {
		t.Fatalf("unexpected output\nwant:\n%s\ngot:\n%s", expected, out.String())
	}
	assertParsesAsYAML(t, out.Bytes())
}

func assertParsesAsYAML(t *testing.T, data []byte) {
	t.Helper()

	var parsed map[string]interface{}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("rendered output is not valid YAML: %v\n%s", err, string(data))
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitForBrowserURL(t *testing.T, out *lockedBuffer, errCh <-chan error) string {
	t.Helper()

	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case err := <-errCh:
			t.Fatalf("browser command exited before printing URL: %v", err)
		case <-deadline:
			t.Fatalf("timed out waiting for browser URL; stdout:\n%s", out.String())
		case <-ticker.C:
			for _, field := range strings.Fields(out.String()) {
				if strings.HasPrefix(field, "http://127.0.0.1:") {
					return field
				}
			}
		}
	}
}

func httpGet(t *testing.T, requestURL string) string {
	t.Helper()

	resp, err := http.Get(requestURL)
	if err != nil {
		t.Fatalf("GET %s: %v", requestURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", requestURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d: %s", requestURL, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return string(data)
}

func httpPost(t *testing.T, requestURL string, expectedStatus int) {
	t.Helper()

	resp, err := http.Post(requestURL, "text/plain", bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("POST %s: %v", requestURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", requestURL, err)
	}
	if resp.StatusCode != expectedStatus {
		t.Fatalf("POST %s: status %d: %s", requestURL, resp.StatusCode, strings.TrimSpace(string(data)))
	}
}

func testResourceResolver(t *testing.T) *kube.ResourceResolver {
	t.Helper()

	resolver, err := kube.BuildResourceResolver([]*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", SingularName: "deployment", Kind: "Deployment", Namespaced: true, ShortNames: []string{"deploy"}},
			},
		},
		{
			GroupVersion: "apps/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", SingularName: "deployment", Kind: "Deployment", Namespaced: true, ShortNames: []string{"deploy"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return resolver
}

func testOpenAPIClient(t *testing.T) *kube.OpenAPIClient {
	t.Helper()

	baseURL, err := url.Parse("https://cluster.example")
	if err != nil {
		t.Fatal(err)
	}
	transport := &fakeRoundTripper{
		responses: map[string]string{
			"https://cluster.example/openapi/v3": `{
				"paths": {
					"apis/apps/v1": {"serverRelativeURL": "/openapi/v3/apis/apps/v1?hash=apps"},
					"apis/apps/v1beta1": {"serverRelativeURL": "/openapi/v3/apis/apps/v1beta1?hash=apps"}
				}
			}`,
			"https://cluster.example/openapi/v3/apis/apps/v1?hash=apps": `{
				"openapi": "3.0.0",
				"components": {
					"schemas": {
						"io.k8s.api.apps.v1.Deployment": {
							"type": "object",
							"x-kubernetes-group-version-kind": [
								{"group": "apps", "version": "v1", "kind": "Deployment"}
							],
							"properties": {
								"spec": {"$ref": "#/components/schemas/io.k8s.api.apps.v1.DeploymentSpec"},
								"status": {"type": "object", "description": "Deployment status."}
							}
						},
						"io.k8s.api.apps.v1.DeploymentSpec": {
							"type": "object",
							"description": "DeploymentSpec is the desired state.",
							"required": ["selector"],
							"properties": {
								"replicas": {"type": "integer", "format": "int32", "default": 1, "minimum": 0},
								"selector": {"type": "object", "description": "Label selector."}
							}
						}
					}
				}
			}`,
			"https://cluster.example/openapi/v3/apis/apps/v1beta1?hash=apps": `{
				"openapi": "3.0.0",
				"components": {
					"schemas": {
						"io.k8s.api.apps.v1beta1.Deployment": {
							"type": "object",
							"x-kubernetes-group-version-kind": [
								{"group": "apps", "version": "v1beta1", "kind": "Deployment"}
							],
							"properties": {
								"spec": {"$ref": "#/components/schemas/io.k8s.api.apps.v1beta1.DeploymentSpec"}
							}
						},
						"io.k8s.api.apps.v1beta1.DeploymentSpec": {
							"type": "object",
							"description": "DeploymentSpec beta desired state.",
							"properties": {
								"selector": {"type": "object", "description": "Beta label selector."}
							},
							"required": ["selector"]
						}
					}
				}
			}`,
		},
	}
	return kube.NewOpenAPIClient(baseURL, &http.Client{Transport: transport})
}

type fakeRoundTripper struct {
	responses map[string]string
}

func (rt *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body, ok := rt.responses[req.URL.String()]
	if !ok {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("not found")),
			Header:     http.Header{},
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
		Request:    req,
	}, nil
}
