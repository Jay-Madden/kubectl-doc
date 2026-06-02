package kube

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sttts/kubectl-doc/internal/kubeversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apischema "k8s.io/apimachinery/pkg/runtime/schema"
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
	sets := map[string]map[string]map[string]struct{}{}
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

		groupName := gv.Group
		if groupName == "" {
			groupName = CoreGroup
		}
		for _, apiResource := range list.APIResources {
			if apiResource.Name == "" || strings.Contains(apiResource.Name, "/") {
				continue
			}
			resources := sets[groupName]
			if resources == nil {
				resources = map[string]map[string]struct{}{}
				sets[groupName] = resources
			}
			versions := resources[apiResource.Name]
			if versions == nil {
				versions = map[string]struct{}{}
				resources[apiResource.Name] = versions
			}
			versions[gv.Version] = struct{}{}
		}
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
