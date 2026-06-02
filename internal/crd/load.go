package crd

import (
	"fmt"
	"io"
	"os"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/sttts/kubectl-doc/internal/kubeversion"
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
	source, obj, err := loadCRDFile(filenames)
	if err != nil {
		return nil, err
	}

	selected, err := SelectVersion(obj.Spec.Versions, version)
	if err != nil {
		return nil, err
	}
	return buildDocument(source, obj, selected)
}

func LoadAllVersions(filenames []string) ([]*Document, error) {
	source, obj, err := loadCRDFile(filenames)
	if err != nil {
		return nil, err
	}

	var docs []*Document
	for _, version := range servedVersionNames(obj.Spec.Versions) {
		selected, err := SelectVersion(obj.Spec.Versions, version)
		if err != nil {
			return nil, err
		}
		doc, err := buildDocument(source, obj, selected)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("CRD has no served versions")
	}
	return docs, nil
}

func loadCRDFile(filenames []string) (string, *apiextensionsv1.CustomResourceDefinition, error) {
	if len(filenames) != 1 {
		return "", nil, fmt.Errorf("expected exactly one CRD file, got %d", len(filenames))
	}

	file, err := os.Open(filenames[0])
	if err != nil {
		return "", nil, fmt.Errorf("open CRD file %q: %w", filenames[0], err)
	}
	defer func() {
		_ = file.Close()
	}()

	obj, err := decodeSingleCRD(file)
	if err != nil {
		return "", nil, fmt.Errorf("read CRD file %q: %w", filenames[0], err)
	}
	return filenames[0], obj, nil
}

func buildDocument(source string, obj *apiextensionsv1.CustomResourceDefinition, selected apiextensionsv1.CustomResourceDefinitionVersion) (*Document, error) {
	if selected.Schema == nil || selected.Schema.OpenAPIV3Schema == nil {
		return nil, fmt.Errorf("CRD %s version %s has no structural schema", obj.Name, selected.Name)
	}

	structural, err := toStructural(selected.Schema.OpenAPIV3Schema)
	if err != nil {
		return nil, fmt.Errorf("convert CRD %s version %s to structural schema: %w", obj.Name, selected.Name, err)
	}

	return &Document{
		Source:   source,
		Group:    obj.Spec.Group,
		Kind:     obj.Spec.Names.Kind,
		Plural:   obj.Spec.Names.Plural,
		Version:  selected.Name,
		Schema:   structural,
		Versions: servedVersionNames(obj.Spec.Versions),
	}, nil
}

func servedVersionNames(versions []apiextensionsv1.CustomResourceDefinitionVersion) []string {
	served := make([]string, 0, len(versions))
	for _, version := range versions {
		if version.Served {
			served = append(served, version.Name)
		}
	}
	kubeversion.SortLatestFirst(served)
	return served
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
