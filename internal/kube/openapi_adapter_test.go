package kube

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDocumentFromOpenAPIV3RendersNativeSchema(t *testing.T) {
	data := []byte(`{
		"openapi": "3.0.0",
		"components": {
			"schemas": {
				"io.k8s.api.apps.v1.Deployment": {
					"type": "object",
					"x-kubernetes-group-version-kind": [
						{"group": "apps", "version": "v1", "kind": "Deployment"}
					],
					"properties": {
						"metadata": {"$ref": "#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta"},
						"spec": {"$ref": "#/components/schemas/io.k8s.api.apps.v1.DeploymentSpec"},
						"status": {"type": "object", "description": "Deployment status."}
					}
				},
				"io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta": {
					"type": "object",
					"properties": {
						"name": {"type": "string"}
					}
				},
				"io.k8s.api.apps.v1.DeploymentSpec": {
					"type": "object",
					"description": "DeploymentSpec is the desired state.",
					"required": ["selector", "template"],
					"properties": {
						"replicas": {"type": "integer", "format": "int32", "default": 1, "minimum": 0},
						"selector": {"type": "object", "description": "Label selector."},
						"template": {"$ref": "#/components/schemas/io.k8s.api.core.v1.PodTemplateSpec"}
					}
				},
				"io.k8s.api.core.v1.PodTemplateSpec": {
					"type": "object",
					"required": ["spec"],
					"properties": {
						"spec": {"type": "object"}
					}
				}
			}
		}
	}`)
	identity := ResourceIdentity{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}

	doc, err := BuildDocumentFromOpenAPIV3(data, identity)
	if err != nil {
		t.Fatal(err)
	}

	if doc.Group != "apps" || doc.Version != "v1" || doc.Kind != "Deployment" || doc.Plural != "deployments" {
		t.Fatalf("unexpected document identity: %#v", doc)
	}

	spec := doc.Schema.Properties["spec"]
	if spec.Description != "DeploymentSpec is the desired state." {
		t.Fatalf("unexpected spec description: %q", spec.Description)
	}
	required := map[string]bool{}
	for _, name := range spec.ValueValidation.Required {
		required[name] = true
	}
	if !required["selector"] || !required["template"] {
		t.Fatalf("missing required fields: %#v", spec.ValueValidation.Required)
	}
	replicas := spec.Properties["replicas"]
	if replicas.Type != "integer" || replicas.ValueValidation.Format != "int32" {
		t.Fatalf("unexpected replicas schema: %#v", replicas)
	}
	if replicas.Default.Object != float64(1) {
		t.Fatalf("unexpected replicas default: %#v", replicas.Default.Object)
	}
}

func TestBuildDocumentFromOpenAPIV3ReportsMissingSchema(t *testing.T) {
	_, err := BuildDocumentFromOpenAPIV3([]byte(`{"components":{"schemas":{}}}`), ResourceIdentity{Kind: "Deployment"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildDocumentFromKubernetesAppsV1Fixture(t *testing.T) {
	data := readKubeOpenAPIFixture(t, "pkg/util/proto/testdata/openapi_v3_0_0/apps/v1.json")

	count := assertConvertsGVKSchemas(t, data)
	if count < 16 {
		t.Fatalf("expected at least 16 GVK schemas, got %d", count)
	}

	doc, err := BuildDocumentFromOpenAPIV3(data, ResourceIdentity{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
		Kind:     "Deployment",
	})
	if err != nil {
		t.Fatal(err)
	}
	spec := doc.Schema.Properties["spec"]
	for _, field := range []string{"replicas", "selector", "template"} {
		if _, ok := spec.Properties[field]; !ok {
			t.Fatalf("real Deployment spec is missing %q", field)
		}
	}
}

func TestBuildDocumentFromKubernetesBatchV1Fixture(t *testing.T) {
	data := readKubeOpenAPIFixture(t, "pkg/openapiconv/testdata_generated_from_k8s/v3_batch.v1.json")

	count := assertConvertsGVKSchemas(t, data)
	if count < 8 {
		t.Fatalf("expected at least 8 GVK schemas, got %d", count)
	}
}

func assertConvertsGVKSchemas(t *testing.T, data []byte) int {
	t.Helper()

	var document openAPIDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}

	count := 0
	for name, schema := range document.Components.Schemas {
		for _, gvk := range schema.XKubernetesGroupVersionKind {
			count++
			_, err := BuildDocumentFromOpenAPIV3(data, ResourceIdentity{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: resourceNameForKind(gvk.Kind),
				Kind:     gvk.Kind,
			})
			if err != nil {
				t.Fatalf("convert %s for %s/%s %s: %v", name, gvk.Group, gvk.Version, gvk.Kind, err)
			}
		}
	}
	return count
}

func resourceNameForKind(kind string) string {
	lower := strings.ToLower(kind)
	switch {
	case strings.HasSuffix(lower, "s"):
		return lower + "es"
	case strings.HasSuffix(lower, "y"):
		return strings.TrimSuffix(lower, "y") + "ies"
	default:
		return lower + "s"
	}
}

func readKubeOpenAPIFixture(t *testing.T, relativePath string) []byte {
	t.Helper()

	path := filepath.Join(kubeOpenAPIModuleDir(t), filepath.FromSlash(relativePath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read kube-openapi fixture %s: %v", path, err)
	}
	return data
}

func kubeOpenAPIModuleDir(t *testing.T) string {
	t.Helper()

	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "k8s.io/kube-openapi")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("locate k8s.io/kube-openapi module: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		t.Fatal("go list returned empty k8s.io/kube-openapi module dir")
	}
	return dir
}
