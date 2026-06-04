package tui

import "strings"

type pathFilterQuery struct {
	anchored bool
	tokens   []string
	suffix   string
}

func parsePathFilter(query string) (pathFilterQuery, bool) {
	query = strings.ToLower(query)
	anchored := strings.HasPrefix(query, ".") && !strings.HasPrefix(query, "...")
	if anchored {
		query = strings.TrimPrefix(query, ".")
	}
	if query == "" || (!anchored && !strings.Contains(query, ".")) {
		return pathFilterQuery{}, false
	}

	filter := pathFilterQuery{anchored: anchored}
	for i := 0; i < len(query); {
		if strings.HasPrefix(query[i:], "...") {
			filter.tokens = append(filter.tokens, "...")
			i += 3
			continue
		}
		if query[i] == '.' {
			i++
			continue
		}

		start := i
		for i < len(query) && query[i] != '.' {
			i++
		}
		token := query[start:i]
		if token == "" {
			continue
		}
		if strings.ContainsAny(token, " \t\r\n") {
			filter.suffix = query[start:]
			break
		}
		filter.tokens = append(filter.tokens, token)
	}
	if len(filter.tokens) == 0 && filter.suffix == "" {
		return pathFilterQuery{}, false
	}
	return filter, true
}

func pathFilterHighlight(path, query string) (string, bool) {
	filter, ok := parsePathFilter(query)
	if !ok || path == "" {
		return "", false
	}
	parts := strings.Split(strings.ToLower(path), ".")
	if len(parts) == 0 {
		return "", false
	}

	if filter.anchored {
		return matchPathFilter(parts, 0, filter.tokens, 0, filter.suffix)
	}
	for start := 0; start < len(parts); start++ {
		if highlight, ok := matchPathFilter(parts, start, filter.tokens, 0, filter.suffix); ok {
			return highlight, true
		}
	}
	return "", false
}

func matchPathFilter(parts []string, partIndex int, tokens []string, tokenIndex int, suffix string) (string, bool) {
	if tokenIndex == len(tokens) {
		if suffix != "" {
			return pathSuffixHighlight(parts[partIndex:], suffix)
		}
		return "", partIndex == len(parts)
	}
	if tokens[tokenIndex] == "..." {
		if tokenIndex == len(tokens)-1 && suffix == "" {
			return cleanPathComponent(parts[len(parts)-1]), true
		}
		for skip := partIndex; skip <= len(parts); skip++ {
			if highlight, ok := matchPathFilter(parts, skip, tokens, tokenIndex+1, suffix); ok {
				return highlight, true
			}
		}
		return "", false
	}
	if partIndex >= len(parts) {
		return "", false
	}

	token := tokens[tokenIndex]
	if tokenIndex == len(tokens)-1 && suffix == "" {
		if partIndex == len(parts)-1 && pathComponentContains(parts[partIndex], token) {
			return token, true
		}
		return "", false
	}
	if !pathComponentEqual(parts[partIndex], token) {
		return "", false
	}
	return matchPathFilter(parts, partIndex+1, tokens, tokenIndex+1, suffix)
}

func pathSuffixHighlight(parts []string, suffix string) (string, bool) {
	if len(parts) == 0 {
		return "", false
	}
	if pathSuffixOverlapsFinalComponent(parts, suffix) || pathSuffixOverlapsFinalComponent(cleanPathComponents(parts), suffix) {
		if index := strings.LastIndex(suffix, "."); index >= 0 {
			return suffix[index+1:], true
		}
		return suffix, true
	}
	return "", false
}

func pathSuffixOverlapsFinalComponent(parts []string, suffix string) bool {
	text := strings.Join(parts, ".")
	finalStart := len(text) - len(parts[len(parts)-1])
	for offset := 0; offset <= len(text); {
		index := strings.Index(text[offset:], suffix)
		if index < 0 {
			return false
		}
		start := offset + index
		if start+len(suffix) > finalStart {
			return true
		}
		offset = start + 1
	}
	return false
}

func cleanPathComponents(parts []string) []string {
	clean := make([]string, len(parts))
	for i, part := range parts {
		clean[i] = cleanPathComponent(part)
	}
	return clean
}

func pathComponentEqual(component, token string) bool {
	return component == token || cleanPathComponent(component) == token
}

func pathComponentContains(component, token string) bool {
	return strings.Contains(component, token) || strings.Contains(cleanPathComponent(component), token)
}

func cleanPathComponent(component string) string {
	return strings.TrimSuffix(component, "[]")
}
