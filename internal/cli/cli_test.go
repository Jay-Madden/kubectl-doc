package cli

import (
	"bytes"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestRequiresCRDFileForYAML(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewCommand(&out, &errOut)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error")
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
spec:
  cronSpec: "<string>" # minLength: 1
  image: "<string>"
  # concurrencyPolicy: "Allow" # default, enum: "Forbid" | "Replace"
  # labels:
    # <key>: "<string>"
  # ports:
    # - containerPort: <int32>
      # name: "<string>"
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
  cronSpec: "<string>" # minLength: 1
  # image: "<string>"
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
