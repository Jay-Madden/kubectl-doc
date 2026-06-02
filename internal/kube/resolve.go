package kube

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sttts/kubectl-doc/internal/kubeversion"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apischema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"
)

type ResourceIdentity struct {
	Group        string
	Version      string
	Resource     string
	SingularName string
	Kind         string
	ShortNames   []string
	Namespaced   bool
	Verbs        []string
}

func (r ResourceIdentity) GroupVersionResource() apischema.GroupVersionResource {
	return apischema.GroupVersionResource{
		Group:    r.Group,
		Version:  r.Version,
		Resource: r.Resource,
	}
}

func (r ResourceIdentity) String() string {
	if r.Group == "" {
		return fmt.Sprintf("%s %s/%s", r.Kind, r.Version, r.Resource)
	}
	return fmt.Sprintf("%s %s/%s/%s", r.Kind, r.Group, r.Version, r.Resource)
}

type ResourceResolver struct {
	mapper    meta.RESTMapper
	resources []ResourceIdentity
	byGVR     map[apischema.GroupVersionResource]ResourceIdentity
}

func BuildResourceResolver(lists []*metav1.APIResourceList) (*ResourceResolver, error) {
	resources, err := BuildResources(lists)
	if err != nil {
		return nil, err
	}
	groupResources, err := BuildAPIGroupResources(lists)
	if err != nil {
		return nil, err
	}
	return newResourceResolver(resources, restmapper.NewDiscoveryRESTMapper(groupResources)), nil
}

func BuildResourceResolverFromDiscovery(discoveryClient discovery.DiscoveryInterface) (*ResourceResolver, error) {
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return nil, err
	}
	lists, err := LoadAPIResourceLists(discoveryClient)
	if err != nil {
		return nil, err
	}
	resources, err := BuildResources(lists)
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	mapper = restmapper.NewShortcutExpander(mapper, discoveryClient, nil)
	return newResourceResolver(resources, mapper), nil
}

func BuildResources(lists []*metav1.APIResourceList) ([]ResourceIdentity, error) {
	var resources []ResourceIdentity
	for _, list := range lists {
		if list == nil {
			continue
		}
		gv, err := apischema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			return nil, fmt.Errorf("parse API groupVersion %q: %w", list.GroupVersion, err)
		}
		if gv.Version == "" {
			return nil, fmt.Errorf("API groupVersion %q has no version", list.GroupVersion)
		}

		for _, apiResource := range list.APIResources {
			if apiResource.Name == "" || strings.Contains(apiResource.Name, "/") {
				continue
			}
			resources = append(resources, ResourceIdentity{
				Group:        gv.Group,
				Version:      gv.Version,
				Resource:     apiResource.Name,
				SingularName: apiResource.SingularName,
				Kind:         apiResource.Kind,
				ShortNames:   append([]string(nil), apiResource.ShortNames...),
				Namespaced:   apiResource.Namespaced,
				Verbs:        append([]string(nil), apiResource.Verbs...),
			})
		}
	}

	sortResourceIdentities(resources)
	return resources, nil
}

func BuildAPIGroupResources(lists []*metav1.APIResourceList) ([]*restmapper.APIGroupResources, error) {
	versionsByGroup := map[string][]string{}
	resourcesByGroupVersion := map[apischema.GroupVersion][]metav1.APIResource{}
	for _, list := range lists {
		if list == nil {
			continue
		}
		gv, err := apischema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			return nil, fmt.Errorf("parse API groupVersion %q: %w", list.GroupVersion, err)
		}
		if gv.Version == "" {
			return nil, fmt.Errorf("API groupVersion %q has no version", list.GroupVersion)
		}
		if _, ok := resourcesByGroupVersion[gv]; !ok {
			versionsByGroup[gv.Group] = append(versionsByGroup[gv.Group], gv.Version)
		}
		resourcesByGroupVersion[gv] = append(resourcesByGroupVersion[gv], list.APIResources...)
	}

	groupNames := make([]string, 0, len(versionsByGroup))
	for group := range versionsByGroup {
		groupNames = append(groupNames, group)
	}
	sort.Slice(groupNames, func(i, j int) bool {
		if groupNames[i] == "" {
			return true
		}
		if groupNames[j] == "" {
			return false
		}
		return groupNames[i] < groupNames[j]
	})

	groupResources := make([]*restmapper.APIGroupResources, 0, len(groupNames))
	for _, groupName := range groupNames {
		versions := versionsByGroup[groupName]
		kubeversion.SortLatestFirst(versions)

		apiGroup := metav1.APIGroup{Name: groupName}
		for _, version := range versions {
			gv := apischema.GroupVersion{Group: groupName, Version: version}
			apiGroup.Versions = append(apiGroup.Versions, metav1.GroupVersionForDiscovery{
				GroupVersion: gv.String(),
				Version:      version,
			})
		}
		if len(apiGroup.Versions) > 0 {
			apiGroup.PreferredVersion = apiGroup.Versions[0]
		}

		versionedResources := map[string][]metav1.APIResource{}
		for _, version := range versions {
			gv := apischema.GroupVersion{Group: groupName, Version: version}
			versionedResources[version] = resourcesByGroupVersion[gv]
		}
		groupResources = append(groupResources, &restmapper.APIGroupResources{
			Group:              apiGroup,
			VersionedResources: versionedResources,
		})
	}
	return groupResources, nil
}

func newResourceResolver(resources []ResourceIdentity, mapper meta.RESTMapper) *ResourceResolver {
	byGVR := make(map[apischema.GroupVersionResource]ResourceIdentity, len(resources))
	for _, resource := range resources {
		byGVR[resource.GroupVersionResource()] = resource
	}
	return &ResourceResolver{
		mapper:    mapper,
		resources: resources,
		byGVR:     byGVR,
	}
}

func (r *ResourceResolver) Resolve(selector string) (ResourceIdentity, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return ResourceIdentity{}, fmt.Errorf("resource selector must not be empty")
	}

	var lastErr error
	gvr, groupResource := apischema.ParseResourceArg(selector)
	for _, candidate := range resourceCandidates(gvr, groupResource) {
		if identity, ok := r.preferredUnqualifiedResource(candidate); ok {
			return identity, nil
		}

		resolved, err := r.mapper.ResourceFor(candidate)
		if err == nil {
			return r.identityForGVR(resolved)
		}
		if !meta.IsNoMatchError(err) {
			return ResourceIdentity{}, err
		}
		lastErr = err
	}

	gvk, groupKind := apischema.ParseKindArg(selector)
	for _, candidate := range kindCandidates(gvk, groupKind) {
		mapping, err := r.mapper.RESTMapping(candidate.GroupKind(), candidate.Version)
		if err == nil {
			return r.identityForGVR(mapping.Resource)
		}
		if !meta.IsNoMatchError(err) {
			return ResourceIdentity{}, err
		}
		lastErr = err
	}
	if lastErr != nil {
		return ResourceIdentity{}, lastErr
	}
	return ResourceIdentity{}, fmt.Errorf("resource %q not found", selector)
}

func resourceCandidates(gvr *apischema.GroupVersionResource, groupResource apischema.GroupResource) []apischema.GroupVersionResource {
	var candidates []apischema.GroupVersionResource
	if gvr != nil {
		candidates = append(candidates, *gvr)
	}
	candidates = append(candidates, groupResource.WithVersion(""))
	return dedupeGVRs(candidates)
}

func kindCandidates(gvk *apischema.GroupVersionKind, groupKind apischema.GroupKind) []apischema.GroupVersionKind {
	var candidates []apischema.GroupVersionKind
	if gvk != nil {
		candidates = append(candidates, *gvk)
	}
	candidates = append(candidates, groupKind.WithVersion(""))
	return dedupeGVKs(candidates)
}

func (r *ResourceResolver) identityForGVR(gvr apischema.GroupVersionResource) (ResourceIdentity, error) {
	identity, ok := r.byGVR[gvr]
	if ok {
		return identity, nil
	}
	return ResourceIdentity{}, fmt.Errorf("resolved resource %s is not present in discovery", gvr.String())
}

func (r *ResourceResolver) preferredUnqualifiedResource(candidate apischema.GroupVersionResource) (ResourceIdentity, bool) {
	if candidate.Group != "" || candidate.Version != "" || candidate.Resource == "" {
		return ResourceIdentity{}, false
	}

	var matches []ResourceIdentity
	for _, resource := range r.resources {
		if resource.matchesName(candidate.Resource) {
			matches = append(matches, resource)
		}
	}
	if len(matches) == 0 {
		return ResourceIdentity{}, false
	}
	sortResourceIdentities(matches)

	preferred := matches[0]
	for _, match := range matches[1:] {
		if match.Group == preferred.Group && match.Version == preferred.Version {
			return ResourceIdentity{}, false
		}
	}
	return preferred, true
}

func (r ResourceIdentity) matchesName(name string) bool {
	if r.Resource == name || r.SingularName == name {
		return true
	}
	for _, shortName := range r.ShortNames {
		if shortName == name {
			return true
		}
	}
	return false
}

func dedupeGVRs(candidates []apischema.GroupVersionResource) []apischema.GroupVersionResource {
	seen := map[apischema.GroupVersionResource]struct{}{}
	var out []apischema.GroupVersionResource
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func dedupeGVKs(candidates []apischema.GroupVersionKind) []apischema.GroupVersionKind {
	seen := map[apischema.GroupVersionKind]struct{}{}
	var out []apischema.GroupVersionKind
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func sortResourceIdentities(resources []ResourceIdentity) {
	sort.SliceStable(resources, func(i, j int) bool {
		left := resources[i]
		right := resources[j]
		if left.Group != right.Group {
			if left.Group == "" {
				return true
			}
			if right.Group == "" {
				return false
			}
			return left.Group < right.Group
		}
		if left.Resource != right.Resource {
			return left.Resource < right.Resource
		}
		return kubeversion.Compare(left.Version, right.Version) > 0
	})
}
