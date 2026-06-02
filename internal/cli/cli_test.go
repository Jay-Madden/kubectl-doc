package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/sttts/kubectl-doc/internal/kube"
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

func TestClusterResourceSelectorsAreNotImplemented(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommandWithDeps(&out, &errOut, Dependencies{
		LoadOverview: func() (*kube.Overview, error) {
			t.Fatal("should not load overview for resource selectors")
			return nil, nil
		},
	})
	cmd.SetArgs([]string{"deployments"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "cluster resource schema rendering is not implemented yet; omit the resource to show the discovery overview" {
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

      # sharedMemorySize: "<string>" # intOrString

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
