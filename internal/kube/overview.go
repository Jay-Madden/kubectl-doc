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
	Name     string
	Versions []string
}

func BuildOverview(lists []*metav1.APIResourceList) (*Overview, error) {
	resources, err := BuildResources(lists)
	if err != nil {
		return nil, err
	}

	sets := map[string]map[string]map[string]struct{}{}
	for _, resource := range resources {
		groupName := resource.Group
		if groupName == "" {
			groupName = CoreGroup
		}
		groupResources := sets[groupName]
		if groupResources == nil {
			groupResources = map[string]map[string]struct{}{}
			sets[groupName] = groupResources
		}
		versions := groupResources[resource.Resource]
		if versions == nil {
			versions = map[string]struct{}{}
			groupResources[resource.Resource] = versions
		}
		versions[resource.Version] = struct{}{}
	}

	return overviewFromSets(sets), nil
}

func overviewFromSets(sets map[string]map[string]map[string]struct{}) *Overview {
	groups := make([]Group, 0, len(sets))
	for groupName, resourceSets := range sets {
		resources := make([]Resource, 0, len(resourceSets))
		for resourceName, versionSet := range resourceSets {
			versions := make([]string, 0, len(versionSet))
			for version := range versionSet {
				versions = append(versions, version)
			}
			kubeversion.SortLatestFirst(versions)
			resources = append(resources, Resource{Name: resourceName, Versions: versions})
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
