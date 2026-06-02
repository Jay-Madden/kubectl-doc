package crd

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var kubeVersionRE = regexp.MustCompile(`^v([0-9]+)(?:(alpha|beta)([0-9]+))?$`)

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
		return compareVersions(served[i].Name, served[j].Name) > 0
	})
	return served[0], nil
}

func compareVersions(left, right string) int {
	l := parseVersion(left)
	r := parseVersion(right)
	if l.tier != r.tier {
		return compareInt(l.tier, r.tier)
	}
	if l.major != r.major {
		return compareInt(l.major, r.major)
	}
	if l.pre != r.pre {
		return compareInt(l.pre, r.pre)
	}
	return compareString(left, right)
}

type versionParts struct {
	tier  int
	major int
	pre   int
}

func parseVersion(version string) versionParts {
	matches := kubeVersionRE.FindStringSubmatch(version)
	if matches == nil {
		return versionParts{}
	}

	major, _ := strconv.Atoi(matches[1])
	parts := versionParts{tier: 3, major: major}
	switch matches[2] {
	case "beta":
		parts.tier = 2
	case "alpha":
		parts.tier = 1
	}
	if matches[3] != "" {
		parts.pre, _ = strconv.Atoi(matches[3])
	}
	return parts
}

func compareInt(left, right int) int {
	switch {
	case left > right:
		return 1
	case left < right:
		return -1
	default:
		return 0
	}
}

func compareString(left, right string) int {
	switch {
	case left > right:
		return 1
	case left < right:
		return -1
	default:
		return 0
	}
}
