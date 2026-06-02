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
						"port": {"type": "string", "format": "int-or-string"},
						"selector": {"type": "object", "description": "Label selector."},
						"template": {"description": "Pod template wrapper.", "$ref": "#/components/schemas/io.k8s.api.core.v1.PodTemplateSpec"}
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
	port := spec.Properties["port"]
	if !port.XIntOrString {
		t.Fatalf("expected int-or-string format to set XIntOrString: %#v", port)
	}
	template := spec.Properties["template"]
	if template.Description != "Pod template wrapper." || template.Properties["spec"].Type != "object" {
		t.Fatalf("expected ref wrapper metadata and target structure, got %#v", template)
	}
}

func TestBuildDocumentFromOpenAPIV3UnwrapsSingleRefAllOf(t *testing.T) {
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
						"spec": {
							"description": "Specification of the desired behavior of the Deployment.",
							"default": {},
							"allOf": [
								{"$ref": "#/components/schemas/io.k8s.api.apps.v1.DeploymentSpec"}
							]
						}
					}
				},
				"io.k8s.api.apps.v1.DeploymentSpec": {
					"type": "object",
					"required": ["selector", "template"],
					"properties": {
						"replicas": {"type": "integer", "format": "int32"},
						"selector": {"type": "object"},
						"template": {"type": "object"}
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

	spec := doc.Schema.Properties["spec"]
	if spec.Description != "Specification of the desired behavior of the Deployment." {
		t.Fatalf("expected allOf wrapper description, got %q", spec.Description)
	}
	if spec.Default.Object == nil {
		t.Fatalf("expected allOf wrapper default to be preserved")
	}
	for _, field := range []string{"replicas", "selector", "template"} {
		if _, ok := spec.Properties[field]; !ok {
			t.Fatalf("expected allOf ref target field %q, got %#v", field, spec.Properties)
		}
	}
}

func TestBuildDocumentFromOpenAPIV3PreservesExamples(t *testing.T) {
	data := []byte(`{
		"openapi": "3.0.0",
		"components": {
			"schemas": {
				"example.io.v1.Widget": {
					"type": "object",
					"x-kubernetes-group-version-kind": [
						{"group": "example.io", "version": "v1", "kind": "Widget"}
					],
					"properties": {
						"mode": {
							"type": "string",
							"example": "prod"
						},
						"config": {
							"type": "object",
							"examples": {
								"secondary": {"value": {"mode": "standby"}},
								"primary": {"value": {"mode": "active"}},
								"external": {"externalValue": "https://example.invalid/config.json"}
							}
						}
					}
				}
			}
		}
	}`)

	doc, err := BuildDocumentFromOpenAPIV3(data, ResourceIdentity{
		Group:    "example.io",
		Version:  "v1",
		Resource: "widgets",
		Kind:     "Widget",
	})
	if err != nil {
		t.Fatal(err)
	}

	modeExamples := doc.Schema.Properties["mode"].Examples
	if len(modeExamples) != 1 || modeExamples[0].Value.Object != "prod" {
		t.Fatalf("unexpected mode examples: %#v", modeExamples)
	}

	configExamples := doc.Schema.Properties["config"].Examples
	if len(configExamples) != 2 {
		t.Fatalf("expected two local config examples, got %#v", configExamples)
	}
	if configExamples[0].Name != "primary" {
		t.Fatalf("expected examples to be sorted by name, got %#v", configExamples)
	}
	value, ok := configExamples[0].Value.Object.(map[string]interface{})
	if !ok || value["mode"] != "active" {
		t.Fatalf("unexpected primary config example: %#v", configExamples[0].Value.Object)
	}
}

func TestBuildDocumentFromOpenAPIV3UsesOperationGVKFallback(t *testing.T) {
	data := []byte(`{
		"openapi": "3.0.0",
		"paths": {
			"/api/v1/namespaces/{namespace}/pods": {
				"get": {
					"x-kubernetes-action": "list",
					"x-kubernetes-group-version-kind": {"group": "", "version": "v1", "kind": "Pod"},
					"responses": {
						"200": {
							"content": {
								"application/json": {
									"schema": {"$ref": "#/components/schemas/io.k8s.api.core.v1.PodList"}
								}
							}
						}
					}
				},
				"post": {
					"x-kubernetes-action": "post",
					"x-kubernetes-group-version-kind": {"group": "", "version": "v1", "kind": "Pod"},
					"requestBody": {
						"content": {
							"*/*": {
								"schema": {"$ref": "#/components/schemas/io.k8s.api.core.v1.Pod"}
							}
						}
					}
				}
			}
		},
		"components": {
			"schemas": {
				"io.k8s.api.core.v1.Pod": {
					"type": "object",
					"properties": {
						"spec": {"$ref": "#/components/schemas/io.k8s.api.core.v1.PodSpec"}
					}
				},
				"io.k8s.api.core.v1.PodList": {
					"type": "object",
					"properties": {
						"items": {"type": "array", "items": {"$ref": "#/components/schemas/io.k8s.api.core.v1.Pod"}}
					}
				},
				"io.k8s.api.core.v1.PodSpec": {
					"type": "object",
					"properties": {
						"containers": {"type": "array"}
					}
				}
			}
		}
	}`)
	identity := ResourceIdentity{Version: "v1", Resource: "pods", Kind: "Pod"}

	doc, err := BuildDocumentFromOpenAPIV3(data, identity)
	if err != nil {
		t.Fatal(err)
	}
	spec := doc.Schema.Properties["spec"]
	if spec.Properties["containers"].Type != "array" {
		t.Fatalf("expected operation fallback to select Pod schema, got %#v", doc.Schema)
	}
	if _, ok := doc.Schema.Properties["items"]; ok {
		t.Fatalf("operation fallback selected PodList instead of Pod: %#v", doc.Schema)
	}
}

func TestBuildDocumentFromOpenAPIV3WithNativeFallbackRendersMissingBuiltIn(t *testing.T) {
	data := []byte(`{
		"openapi": "3.0.0",
		"paths": {
			"/api/v1/configmaps": {
				"post": {
					"x-kubernetes-action": "post",
					"x-kubernetes-group-version-kind": {"group": "", "version": "v1", "kind": "ConfigMap"},
					"requestBody": {
						"content": {
							"*/*": {
								"schema": {"$ref": "#/components/schemas/io.k8s.api.core.v1.ConfigMap"}
							}
						}
					}
				}
			}
		},
		"components": {
			"schemas": {
				"io.k8s.api.core.v1.ConfigMap": {
					"type": "object",
					"x-kubernetes-group-version-kind": [
						{"group": "", "version": "v1", "kind": "ConfigMap"}
					]
				}
			}
		}
	}`)
	identity := ResourceIdentity{Version: "v1", Resource: "pods", Kind: "Pod"}

	doc, err := BuildDocumentFromOpenAPIV3WithNativeFallback(data, identity)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Source != "embedded-native-openapi" {
		t.Fatalf("expected embedded fallback source, got %q", doc.Source)
	}
	spec := doc.Schema.Properties["spec"]
	if spec.Properties["containers"].Type != "array" {
		t.Fatalf("expected embedded Pod schema, got %#v", doc.Schema)
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
	data := readKubeOpenAPIFixture(t, "pkg/util/proto/testdata/openapi_v3_0_0/batch/v1.json")

	count := assertConvertsGVKSchemas(t, data)
	if count < 8 {
		t.Fatalf("expected at least 8 GVK schemas, got %d", count)
	}
}

func TestBuildDocumentFromKubernetesOpenAPIV3Fixtures(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		minGVKs int
	}{
		{name: "core", path: "pkg/util/proto/testdata/openapi_v3_0_0/v1.json", minGVKs: 20},
		{name: "apiextensions", path: "pkg/util/proto/testdata/openapi_v3_0_0/apiextensions.k8s.io/v1.json", minGVKs: 3},
		{name: "apps", path: "pkg/util/proto/testdata/openapi_v3_0_0/apps/v1.json", minGVKs: 16},
		{name: "batch", path: "pkg/util/proto/testdata/openapi_v3_0_0/batch/v1.json", minGVKs: 8},
		{name: "batch beta", path: "pkg/util/proto/testdata/openapi_v3_0_0/batch/v1beta1.json", minGVKs: 4},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := readKubeOpenAPIFixture(t, test.path)
			count := assertConvertsGVKSchemas(t, data)
			if count < test.minGVKs {
				t.Fatalf("expected at least %d GVK schemas, got %d", test.minGVKs, count)
			}
		})
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
