package gorestdocs

import (
	"strings"
	"sync"
)

// PathPatterns holds user-registered path patterns for parameterization.
// For example, registering "/users/{id}" tells the builder that any path
// matching /users/<something> should be grouped under /users/{id}.
type PathPatterns struct {
	mu       sync.Mutex
	patterns []string
}

// NewPathPatterns creates a new empty PathPatterns.
func NewPathPatterns() *PathPatterns {
	return &PathPatterns{}
}

// Register adds a path pattern. The pattern should use {name} for parameters,
// e.g. "/users/{id}" or "/users/{userId}/posts/{postId}".
func (pp *PathPatterns) Register(patterns ...string) {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.patterns = append(pp.patterns, patterns...)
}

// All returns all registered patterns.
func (pp *PathPatterns) All() []string {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	result := make([]string, len(pp.patterns))
	copy(result, pp.patterns)
	return result
}

// Reset clears all registered patterns.
func (pp *PathPatterns) Reset() {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.patterns = nil
}

// Match tries to match a concrete path against a registered pattern.
// Returns the pattern and true if matched, or empty string and false.
func (pp *PathPatterns) Match(path string) (string, bool) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	pathSegs := splitPath(path)
	for _, pattern := range pp.patterns {
		patternSegs := splitPath(pattern)
		if matchSegments(pathSegs, patternSegs) {
			return pattern, true
		}
	}
	return "", false
}

// splitPath splits a path into segments, ignoring leading/trailing slashes.
func splitPath(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}

// matchSegments checks if concrete path segments match a pattern's segments.
// A pattern segment like {id} matches any concrete segment.
func matchSegments(path, pattern []string) bool {
	if len(path) != len(pattern) {
		return false
	}
	for i, ps := range pattern {
		if isParam(ps) {
			continue // parameter segment matches anything
		}
		if path[i] != ps {
			return false
		}
	}
	return true
}

// isParam returns true if the segment is a path parameter placeholder like {id}.
func isParam(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

// AutoDetectPatterns groups a set of paths by segment count and static prefixes,
// then detects which segments vary (likely parameters). Returns a map from
// detected pattern to the original paths that match it.
func AutoDetectPatterns(paths []string) map[string][]string {
	// Group paths by segment count
	byLen := make(map[int][]string)
	for _, p := range paths {
		segs := splitPath(p)
		byLen[len(segs)] = append(byLen[len(segs)], p)
	}

	result := make(map[string][]string)

	for _, group := range byLen {
		if len(group) == 1 {
			// Single path, no pattern to detect
			result[group[0]] = group
			continue
		}

		// Split all paths into segments
		allSegs := make([][]string, len(group))
		for i, p := range group {
			allSegs[i] = splitPath(p)
		}

		segCount := len(allSegs[0])

		// Sub-group by static segments to avoid merging unrelated paths
		// e.g. /users/123 and /posts/456 should not merge into /{param0}/{param1}
		subGroups := subGroupByStaticPrefix(allSegs, group)

		for _, sg := range subGroups {
			if len(sg.paths) == 1 {
				result[sg.paths[0]] = sg.paths
				continue
			}

			// Find varying segments within this sub-group
			varying := make([]bool, segCount)
			for i := 0; i < segCount; i++ {
				first := sg.segs[0][i]
				for _, s := range sg.segs[1:] {
					if s[i] != first {
						varying[i] = true
						break
					}
				}
			}

			// Build pattern
			patternParts := make([]string, segCount)
			paramIdx := 0
			hasStatic := false
			for i := 0; i < segCount; i++ {
				if varying[i] {
					patternParts[i] = paramName(paramIdx)
					paramIdx++
				} else {
					patternParts[i] = sg.segs[0][i]
					hasStatic = true
				}
			}

			if hasStatic {
				pattern := "/" + strings.Join(patternParts, "/")
				result[pattern] = append(result[pattern], sg.paths...)
			} else {
				// All segments vary - treat each path individually
				for _, p := range sg.paths {
					result[p] = append(result[p], p)
				}
			}
		}
	}

	return result
}

type subGroup struct {
	segs  [][]string
	paths []string
}

// subGroupByStaticPrefix groups paths that share the same static segments.
// This prevents merging unrelated paths like /users/123 and /posts/456.
func subGroupByStaticPrefix(allSegs [][]string, paths []string) []subGroup {
	if len(allSegs) == 0 {
		return nil
	}

	segCount := len(allSegs[0])

	// Use a simple heuristic: group by the first static-looking segment.
	// A segment is "static" if it appears in more than one path at the same position.
	// For simplicity, group by the segments that look non-numeric/non-UUID.
	groups := make(map[string]*subGroup)
	var order []string

	for i, segs := range allSegs {
		// Build a key from segments that look static (not purely numeric)
		var keyParts []string
		for j := 0; j < segCount; j++ {
			if looksStatic(segs[j]) {
				keyParts = append(keyParts, segs[j])
			} else {
				keyParts = append(keyParts, "*")
			}
		}
		k := strings.Join(keyParts, "/")
		if _, ok := groups[k]; !ok {
			groups[k] = &subGroup{}
			order = append(order, k)
		}
		groups[k].segs = append(groups[k].segs, segs)
		groups[k].paths = append(groups[k].paths, paths[i])
	}

	var result []subGroup
	for _, k := range order {
		result = append(result, *groups[k])
	}
	return result
}

// looksStatic returns true if a path segment appears to be a static resource name
// rather than a dynamic parameter (ID, UUID, etc.).
func looksStatic(segment string) bool {
	// Purely numeric segments are likely IDs
	allDigits := true
	for _, c := range segment {
		if c < '0' || c > '9' {
			allDigits = false
			break
		}
	}
	if allDigits && len(segment) > 0 {
		return false
	}

	// UUIDs
	if len(segment) == 36 && strings.Count(segment, "-") == 4 {
		return false
	}

	return true
}

func paramName(idx int) string {
	return "{param" + strings.Repeat("", 0) + itoa(idx) + "}"
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var s string
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
