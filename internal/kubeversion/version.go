package kubeversion

import (
	"regexp"
	"sort"
	"strconv"
)

var kubeVersionRE = regexp.MustCompile(`^v([0-9]+)(?:(alpha|beta)([0-9]+))?$`)

func Compare(left, right string) int {
	l := parse(left)
	r := parse(right)
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

func SortLatestFirst(versions []string) {
	sort.SliceStable(versions, func(i, j int) bool {
		return Compare(versions[i], versions[j]) > 0
	})
}

func SortDisplay(versions []string) {
	sort.SliceStable(versions, func(i, j int) bool {
		return Compare(versions[i], versions[j]) < 0
	})
}

type parts struct {
	tier  int
	major int
	pre   int
}

func parse(version string) parts {
	matches := kubeVersionRE.FindStringSubmatch(version)
	if matches == nil {
		return parts{}
	}

	major, _ := strconv.Atoi(matches[1])
	parts := parts{tier: 3, major: major}
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
