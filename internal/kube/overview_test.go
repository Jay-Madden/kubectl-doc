package kube

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildOverviewGroupsResourcesAndVersions(t *testing.T) {
	overview, err := BuildOverview([]*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments"},
				{Name: "deployments/status"},
				{Name: "daemonsets"},
			},
		},
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", ShortNames: []string{"po"}},
				{Name: "pods/log"},
			},
		},
		{
			GroupVersion: "apps/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "deployments"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	expected := &Overview{
		Groups: []Group{
			{
				Name: CoreGroup,
				Resources: []Resource{
					{Name: "pods", Versions: []string{"v1"}, ShortNames: []string{"po"}},
				},
			},
			{
				Name: "apps",
				Resources: []Resource{
					{Name: "daemonsets", Versions: []string{"v1"}},
					{Name: "deployments", Versions: []string{"v1", "v1beta1"}},
				},
			},
		},
	}
	if !reflect.DeepEqual(overview, expected) {
		t.Fatalf("unexpected overview\nwant: %#v\ngot:  %#v", expected, overview)
	}
}

func TestBuildOverviewRejectsInvalidGroupVersion(t *testing.T) {
	_, err := BuildOverview([]*metav1.APIResourceList{{GroupVersion: "apps/"}})
	if err == nil {
		t.Fatal("expected error")
	}
}
