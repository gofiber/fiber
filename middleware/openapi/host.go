package openapi

import (
	"strings"
)

// effectiveHost combines the host patterns a route is behind, innermost first, into the one pattern a
// hostname must match to reach it: label by label, a literal wins over a parameter. ok is false when no
// hostname can match them all (a different number of labels, or two different literals), so the route
// answers nowhere.
func effectiveHost(patterns []string) (pattern string, ok bool) { //nolint:nonamedreturns // a string and a bool read better named
	switch len(patterns) {
	case 0:
		return "", true
	case 1:
		return patterns[0], true
	}
	labels := strings.Split(patterns[0], ".")
	for _, other := range patterns[1:] {
		otherLabels := strings.Split(other, ".")
		if len(otherLabels) != len(labels) {
			return "", false
		}
		for i, label := range otherLabels {
			switch {
			case strings.HasPrefix(label, ":"):
			case strings.HasPrefix(labels[i], ":"):
				labels[i] = label
			case labels[i] != label:
				return "", false
			}
		}
	}
	return strings.Join(labels, "."), true
}

// hostCovers reports whether every hostname matching route also matches cover; "" is any host.
func hostCovers(cover, route string) bool {
	if cover == "" {
		return true
	}
	if route == "" {
		return false
	}
	coverLabels, routeLabels := strings.Split(cover, "."), strings.Split(route, ".")
	if len(coverLabels) != len(routeLabels) {
		return false
	}
	for i, label := range coverLabels {
		if !strings.HasPrefix(label, ":") && label != routeLabels[i] {
			return false
		}
	}
	return true
}
