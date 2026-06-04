package kube

import (
	"sort"

	"github.com/sttts/kubectl-doc/internal/kubeversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const CoreGroup = "core"

type Overview struct {
	Groups []Group
}

type Group struct {
	Name      string
	Resources []Resource
}

type Resource struct {
	Name       string
	Versions   []string
	ShortNames []string
}

func BuildOverview(lists []*metav1.APIResourceList) (*Overview, error) {
	resources, err := BuildResources(lists)
	if err != nil {
		return nil, err
	}

	sets := map[string]map[string]*overviewResourceSet{}
	for _, resource := range resources {
		groupName := resource.Group
		if groupName == "" {
			groupName = CoreGroup
		}
		groupResources := sets[groupName]
		if groupResources == nil {
			groupResources = map[string]*overviewResourceSet{}
			sets[groupName] = groupResources
		}
		resourceSet := groupResources[resource.Resource]
		if resourceSet == nil {
			resourceSet = &overviewResourceSet{
				versions:   map[string]struct{}{},
				shortNames: map[string]struct{}{},
			}
			groupResources[resource.Resource] = resourceSet
		}
		resourceSet.versions[resource.Version] = struct{}{}
		for _, shortName := range resource.ShortNames {
			resourceSet.shortNames[shortName] = struct{}{}
		}
	}

	return overviewFromSets(sets), nil
}

type overviewResourceSet struct {
	versions   map[string]struct{}
	shortNames map[string]struct{}
}

func overviewFromSets(sets map[string]map[string]*overviewResourceSet) *Overview {
	groups := make([]Group, 0, len(sets))
	for groupName, resourceSets := range sets {
		resources := make([]Resource, 0, len(resourceSets))
		for resourceName, resourceSet := range resourceSets {
			versions := make([]string, 0, len(resourceSet.versions))
			for version := range resourceSet.versions {
				versions = append(versions, version)
			}
			kubeversion.SortLatestFirst(versions)
			shortNames := make([]string, 0, len(resourceSet.shortNames))
			for shortName := range resourceSet.shortNames {
				shortNames = append(shortNames, shortName)
			}
			sort.Strings(shortNames)
			if len(shortNames) == 0 {
				shortNames = nil
			}
			resources = append(resources, Resource{Name: resourceName, Versions: versions, ShortNames: shortNames})
		}
		sort.Slice(resources, func(i, j int) bool {
			return resources[i].Name < resources[j].Name
		})
		groups = append(groups, Group{Name: groupName, Resources: resources})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Name == CoreGroup {
			return true
		}
		if groups[j].Name == CoreGroup {
			return false
		}
		return groups[i].Name < groups[j].Name
	})
	return &Overview{Groups: groups}
}
