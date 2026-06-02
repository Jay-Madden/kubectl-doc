package crd

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestToStructuralPreservesExample(t *testing.T) {
	example := &apiextensionsv1.JSON{Raw: []byte(`"prod"`)}
	structural, err := toStructural(&apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"mode": {
				Type:    "string",
				Example: example,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	examples := structural.Properties["mode"].Examples
	if len(examples) != 1 || examples[0].Value.Object != "prod" {
		t.Fatalf("unexpected examples: %#v", examples)
	}
}
