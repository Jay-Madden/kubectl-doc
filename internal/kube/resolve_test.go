package kube

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	discoveryfake "k8s.io/client-go/discovery/fake"
	kubetesting "k8s.io/client-go/testing"
)

func TestResolveResourceSelectsLatestVersion(t *testing.T) {
	resolver := newTestResolver(t)

	resolved, err := resolver.Resolve("deployments")
	if err != nil {
		t.Fatal(err)
	}
	assertResolved(t, resolved, "apps", "v1", "deployments", "Deployment")
}

func TestResolveResourceSupportsQualifiedSelectors(t *testing.T) {
	resolver := newTestResolver(t)

	tests := []struct {
		name     string
		selector string
		group    string
		version  string
		resource string
		kind     string
	}{
		{name: "group qualified", selector: "deployments.apps", group: "apps", version: "v1", resource: "deployments", kind: "Deployment"},
		{name: "version and group qualified", selector: "deployments.v1beta1.apps", group: "apps", version: "v1beta1", resource: "deployments", kind: "Deployment"},
		{name: "group prefix", selector: "deployments.app", group: "apps", version: "v1", resource: "deployments", kind: "Deployment"},
		{name: "singular", selector: "deployment", group: "apps", version: "v1", resource: "deployments", kind: "Deployment"},
		{name: "short name", selector: "deploy", group: "apps", version: "v1", resource: "deployments", kind: "Deployment"},
		{name: "kind", selector: "Deployment", group: "apps", version: "v1", resource: "deployments", kind: "Deployment"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := resolver.Resolve(test.selector)
			if err != nil {
				t.Fatal(err)
			}
			assertResolved(t, resolved, test.group, test.version, test.resource, test.kind)
		})
	}
}

func TestResolveResourceUsesUpstreamPriority(t *testing.T) {
	resolver := newResolverFromDiscovery(t, []*metav1.APIResourceList{
		resourceList("apps/v1", resource("deployments", "Deployment", "deploy")),
		resourceList("custom.example.com/v1", resource("deployments", "Deployment", "deploy")),
	})

	resolved, err := resolver.Resolve("deployments")
	if err != nil {
		t.Fatal(err)
	}
	assertResolved(t, resolved, "apps", "v1", "deployments", "Deployment")
}

func TestResolveResourceReportsMissingResource(t *testing.T) {
	resolver := newTestResolver(t)

	_, err := resolver.Resolve("does-not-exist")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `does-not-exist`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveResourceFollowsUpstreamVersionSyntax(t *testing.T) {
	resolver := newTestResolver(t)

	_, err := resolver.Resolve("pods.v1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `pods`) || !strings.Contains(err.Error(), `v1`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTestResolver(t *testing.T) *ResourceResolver {
	t.Helper()

	return newResolverFromDiscovery(t, []*metav1.APIResourceList{
		resourceList("v1", resource("pods", "Pod", "po")),
		resourceList("apps/v1", resource("deployments", "Deployment", "deploy"), resource("deployments/status", "Deployment", "")),
		resourceList("apps/v1beta1", resource("deployments", "Deployment", "deploy")),
	})
}

func newResolverFromDiscovery(t *testing.T, lists []*metav1.APIResourceList) *ResourceResolver {
	t.Helper()

	discoveryClient := &discoveryfake.FakeDiscovery{
		Fake: &kubetesting.Fake{Resources: lists},
	}
	resolver, err := BuildResourceResolverFromDiscovery(discoveryClient)
	if err != nil {
		t.Fatal(err)
	}
	return resolver
}

func resourceList(groupVersion string, resources ...metav1.APIResource) *metav1.APIResourceList {
	return &metav1.APIResourceList{
		GroupVersion: groupVersion,
		APIResources: resources,
	}
}

func resource(name, kind, shortName string) metav1.APIResource {
	apiResource := metav1.APIResource{
		Name:         name,
		SingularName: strings.TrimSuffix(name, "s"),
		Kind:         kind,
		Namespaced:   true,
		Verbs:        []string{"get", "list"},
	}
	if shortName != "" {
		apiResource.ShortNames = []string{shortName}
	}
	return apiResource
}

func assertResolved(t *testing.T, resolved ResourceIdentity, group, version, resource, kind string) {
	t.Helper()

	if resolved.Group != group || resolved.Version != version || resolved.Resource != resource || resolved.Kind != kind {
		t.Fatalf("expected %s/%s %s %s, got %#v", group, version, resource, kind, resolved)
	}
}
