package crd

import (
	"fmt"
	"sort"

	"github.com/sttts/kubectl-doc/internal/kubeversion"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func SelectVersion(versions []apiextensionsv1.CustomResourceDefinitionVersion, requested string) (apiextensionsv1.CustomResourceDefinitionVersion, error) {
	var served []apiextensionsv1.CustomResourceDefinitionVersion
	for _, version := range versions {
		if !version.Served {
			continue
		}
		if requested != "" && version.Name == requested {
			return version, nil
		}
		served = append(served, version)
	}

	if requested != "" {
		return apiextensionsv1.CustomResourceDefinitionVersion{}, fmt.Errorf("served CRD version %q not found", requested)
	}
	if len(served) == 0 {
		return apiextensionsv1.CustomResourceDefinitionVersion{}, fmt.Errorf("CRD has no served versions")
	}

	sort.SliceStable(served, func(i, j int) bool {
		return kubeversion.Compare(served[i].Name, served[j].Name) > 0
	})
	return served[0], nil
}
