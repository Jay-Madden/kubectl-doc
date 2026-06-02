package crd

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestSelectVersionOrdersByStabilityThenNumber(t *testing.T) {
	versions := []apiextensionsv1.CustomResourceDefinitionVersion{
		{Name: "v2beta1", Served: true},
		{Name: "v1", Served: true},
		{Name: "v9alpha1", Served: true},
	}

	selected, err := SelectVersion(versions, "")
	if err != nil {
		t.Fatal(err)
	}
	if selected.Name != "v1" {
		t.Fatalf("expected v1, got %s", selected.Name)
	}
}

func TestSelectVersionOrdersWithinTier(t *testing.T) {
	versions := []apiextensionsv1.CustomResourceDefinitionVersion{
		{Name: "v1beta1", Served: true},
		{Name: "v2beta1", Served: true},
		{Name: "v2beta3", Served: true},
	}

	selected, err := SelectVersion(versions, "")
	if err != nil {
		t.Fatal(err)
	}
	if selected.Name != "v2beta3" {
		t.Fatalf("expected v2beta3, got %s", selected.Name)
	}
}

func TestSelectVersionUsesRequestedServedVersion(t *testing.T) {
	versions := []apiextensionsv1.CustomResourceDefinitionVersion{
		{Name: "v1", Served: true},
		{Name: "v2", Served: true},
	}

	selected, err := SelectVersion(versions, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if selected.Name != "v1" {
		t.Fatalf("expected v1, got %s", selected.Name)
	}
}

func TestServedVersionNamesSortLatestFirst(t *testing.T) {
	versions := []apiextensionsv1.CustomResourceDefinitionVersion{
		{Name: "v1alpha1", Served: true},
		{Name: "v1", Served: true},
		{Name: "v2", Served: false},
	}

	served := servedVersionNames(versions)
	if len(served) != 2 || served[0] != "v1" || served[1] != "v1alpha1" {
		t.Fatalf("unexpected served versions: %#v", served)
	}
}
