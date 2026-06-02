package cli

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/sttts/kubectl-doc/internal/kube"
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
# DeploymentSpec is the desired state.
spec: # optional
  # Label selector.
  selector: {}

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
# CronTabSpec describes the desired cron job.
spec:
  # Cron expression for running the job.
  cronSpec: "<string>" # minLength: 1

  # Container image used by the job.
  image: "<string>"

  # concurrencyPolicy: "Allow" # default, enum: "Forbid" | "Replace"

  # labels:
    # <key>: "<string>"

  ports: # optional
    - # Port exposed by the container.
      containerPort: <int32>

      # Port name.
      name: "<string>"

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
spec:
  # Cron expression for running the job.
  cronSpec: "<string>" # minLength: 1

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
		`cronSpec: "<string>" # minLength: 1`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected Markdown to contain %q, got:\n%s", expected, rendered)
		}
	}
	if strings.HasPrefix(rendered, "---\n") {
		t.Fatalf("markdown alias should render GitHub Markdown without Fern frontmatter:\n%s", rendered)
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
		"---\ntitle: CronTab\n---\n\n",
		"# CronTab\n",
		"```yaml title=\"stable.example.com/v1alpha1 CronTab\" wordWrap showLineNumbers={false}\napiVersion: stable.example.com/v1alpha1\nkind: CronTab\n",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected Fern Markdown to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestRendersCRDFileAsAllVersionsMarkdown(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-o", "markdown", "--all-versions", "--descriptions=false"})

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
spec:
  # Cron expression for running the job.
  cronSpec: "<string>" # minLength: 1

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
spec:
  cronSpec: "<string>" # minLength: 1

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
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-i"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "-o tui is not implemented yet" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebShortcutNormalizesToBrowser(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/crontab-crd.yaml", "-w"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "-o browser is not implemented yet" {
		t.Fatalf("unexpected error: %v", err)
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

func TestRendersDynamoGraphDeploymentExtensions(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs([]string{"-f", "testdata/dynamographdeployment-crd.yaml", "-o", "yaml"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr:\n%s", err, errOut.String())
	}

	expected := `apiVersion: nvidia.com/v1beta1
kind: DynamoGraphDeployment
metadata:
  name: "<name>"
spec:
  components: # listType: map, listMapKeys: name
    - name: "<string>" # minLength: 1, maxLength: 63

      # sharedMemorySize: <int-or-string> # intOrString

  # backendFramework: "sglang" # enum: "vllm" | "trtllm"
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
spec:
  model: "<string>" # minLength: 1

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

func testResourceResolver(t *testing.T) *kube.ResourceResolver {
	t.Helper()

	resolver, err := kube.BuildResourceResolver([]*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", SingularName: "deployment", Kind: "Deployment", ShortNames: []string{"deploy"}},
			},
		},
		{
			GroupVersion: "apps/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", SingularName: "deployment", Kind: "Deployment", ShortNames: []string{"deploy"}},
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
