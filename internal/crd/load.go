package crd

import (
	"fmt"
	"io"
	"os"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	docschema "github.com/sttts/kubectl-doc/internal/schema"
)

type Document struct {
	Source   string
	Group    string
	Kind     string
	Plural   string
	Version  string
	Schema   *docschema.Structural
	Versions []string
}

func Load(filenames []string, version string) (*Document, error) {
	if len(filenames) != 1 {
		return nil, fmt.Errorf("expected exactly one CRD file, got %d", len(filenames))
	}

	file, err := os.Open(filenames[0])
	if err != nil {
		return nil, fmt.Errorf("open CRD file %q: %w", filenames[0], err)
	}
	defer file.Close()

	crd, err := decodeSingleCRD(file)
	if err != nil {
		return nil, fmt.Errorf("read CRD file %q: %w", filenames[0], err)
	}

	selected, err := SelectVersion(crd.Spec.Versions, version)
	if err != nil {
		return nil, err
	}
	if selected.Schema == nil || selected.Schema.OpenAPIV3Schema == nil {
		return nil, fmt.Errorf("CRD %s version %s has no structural schema", crd.Name, selected.Name)
	}

	structural, err := toStructural(selected.Schema.OpenAPIV3Schema)
	if err != nil {
		return nil, fmt.Errorf("convert CRD %s version %s to structural schema: %w", crd.Name, selected.Name, err)
	}

	served := make([]string, 0, len(crd.Spec.Versions))
	for _, v := range crd.Spec.Versions {
		if v.Served {
			served = append(served, v.Name)
		}
	}

	return &Document{
		Source:   filenames[0],
		Group:    crd.Spec.Group,
		Kind:     crd.Spec.Names.Kind,
		Plural:   crd.Spec.Names.Plural,
		Version:  selected.Name,
		Schema:   structural,
		Versions: served,
	}, nil
}

func decodeSingleCRD(reader io.Reader) (*apiextensionsv1.CustomResourceDefinition, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(reader, 4096)
	var found *apiextensionsv1.CustomResourceDefinition

	for {
		var obj apiextensionsv1.CustomResourceDefinition
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if obj.Kind == "" && obj.APIVersion == "" && obj.Name == "" {
			continue
		}
		if obj.Kind != "CustomResourceDefinition" {
			return nil, fmt.Errorf("expected CustomResourceDefinition, got %q", obj.Kind)
		}
		if found != nil {
			return nil, fmt.Errorf("expected exactly one CustomResourceDefinition")
		}
		found = &obj
	}

	if found == nil {
		return nil, fmt.Errorf("no CustomResourceDefinition found")
	}
	return found, nil
}
