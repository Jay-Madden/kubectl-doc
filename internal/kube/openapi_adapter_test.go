package kube

import (
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
