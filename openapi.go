package gorestdocs

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// OpenAPI represents an OpenAPI 3.1 specification document.
type OpenAPI struct {
	OpenAPI string              `yaml:"openapi" json:"openapi"`
	Info    Info                `yaml:"info" json:"info"`
	Paths   map[string]PathItem `yaml:"paths" json:"paths"`
}

// Info represents the metadata about the API.
type Info struct {
	Title       string `yaml:"title" json:"title"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// PathItem describes the operations available on a single path.
type PathItem struct {
	Get    *Operation `yaml:"get,omitempty" json:"get,omitempty"`
	Post   *Operation `yaml:"post,omitempty" json:"post,omitempty"`
	Put    *Operation `yaml:"put,omitempty" json:"put,omitempty"`
	Patch  *Operation `yaml:"patch,omitempty" json:"patch,omitempty"`
	Delete *Operation `yaml:"delete,omitempty" json:"delete,omitempty"`
}

// Operation describes a single API operation on a path.
type Operation struct {
	Summary     string            `yaml:"summary,omitempty" json:"summary,omitempty"`
	Parameters  []Parameter       `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	RequestBody *RequestBody      `yaml:"requestBody,omitempty" json:"requestBody,omitempty"`
	Responses   map[string]Response `yaml:"responses" json:"responses"`
}

// Parameter describes a single operation parameter.
type Parameter struct {
	Name     string `yaml:"name" json:"name"`
	In       string `yaml:"in" json:"in"`
	Required bool   `yaml:"required" json:"required"`
	Schema   Schema `yaml:"schema" json:"schema"`
}

// RequestBody describes a request body.
type RequestBody struct {
	Content  map[string]MediaType `yaml:"content" json:"content"`
	Required bool                 `yaml:"required,omitempty" json:"required,omitempty"`
}

// Response describes a single response from an API operation.
type Response struct {
	Description string               `yaml:"description" json:"description"`
	Content     map[string]MediaType `yaml:"content,omitempty" json:"content,omitempty"`
}

// MediaType describes a media type with a schema.
type MediaType struct {
	Schema Schema `yaml:"schema" json:"schema"`
}

// interactionGroup groups interactions by their normalized path and method.
type interactionGroup struct {
	method       string
	pattern      string
	pathParams   []string
	interactions []Interaction
}

// BuildSpec converts all recorded interactions into an OpenAPI 3.1 specification.
// It uses the provided PathPatterns (if any) to map concrete paths to parameterized patterns.
// If patterns is nil, automatic path parameter detection is used as a fallback.
func BuildSpec(registry *Registry, info Info, opts ...BuildOption) OpenAPI {
	cfg := buildConfig{}
	for _, o := range opts {
		o(&cfg)
	}

	spec := OpenAPI{
		OpenAPI: "3.1.0",
		Info:    info,
		Paths:   make(map[string]PathItem),
	}

	interactions := registry.All()
	if len(interactions) == 0 {
		return spec
	}

	groups := groupInteractions(interactions, cfg.patterns)

	for _, g := range groups {
		pathItem := spec.Paths[g.pattern]
		op := buildOperation(g)
		setOperation(&pathItem, g.method, op)
		spec.Paths[g.pattern] = pathItem
	}

	return spec
}

// BuildOption configures BuildSpec behavior.
type BuildOption func(*buildConfig)

type buildConfig struct {
	patterns *PathPatterns
}

// WithPatterns provides explicit path patterns to use when building the spec.
func WithPatterns(pp *PathPatterns) BuildOption {
	return func(c *buildConfig) {
		c.patterns = pp
	}
}

// MarshalYAML marshals the OpenAPI spec to YAML bytes.
func MarshalYAML(spec OpenAPI) ([]byte, error) {
	return yaml.Marshal(spec)
}

// groupInteractions groups interactions by path pattern and method.
// If patterns is provided, it uses registered patterns first, falling back
// to automatic detection for unmatched paths.
func groupInteractions(interactions []Interaction, patterns *PathPatterns) []interactionGroup {
	// If user-registered patterns exist, apply them first
	if patterns != nil && len(patterns.All()) > 0 {
		return groupWithPatterns(interactions, patterns)
	}

	// Fallback: automatic detection
	// Group by method + path segment count
	type groupKey struct {
		method   string
		segments int
	}
	byStructure := make(map[groupKey][]Interaction)
	for _, ix := range interactions {
		segments := strings.Split(strings.Trim(ix.Path, "/"), "/")
		key := groupKey{method: ix.Method, segments: len(segments)}
		byStructure[key] = append(byStructure[key], ix)
	}

	var result []interactionGroup

	for _, ixs := range byStructure {
		subgroups := detectPathPatterns(ixs)
		result = append(result, subgroups...)
	}

	// Sort for deterministic output
	sort.Slice(result, func(i, j int) bool {
		if result[i].pattern != result[j].pattern {
			return result[i].pattern < result[j].pattern
		}
		return result[i].method < result[j].method
	})

	return result
}

// groupWithPatterns groups interactions using user-registered path patterns.
// Interactions that don't match any registered pattern fall back to auto-detection.
func groupWithPatterns(interactions []Interaction, patterns *PathPatterns) []interactionGroup {
	type groupKey struct {
		pattern string
		method  string
	}

	grouped := make(map[groupKey][]Interaction)
	var unmatched []Interaction

	for _, ix := range interactions {
		if pattern, ok := patterns.Match(ix.Path); ok {
			key := groupKey{pattern: pattern, method: ix.Method}
			grouped[key] = append(grouped[key], ix)
		} else {
			unmatched = append(unmatched, ix)
		}
	}

	var result []interactionGroup
	for key, ixs := range grouped {
		pathParams := extractPathParams(key.pattern)
		result = append(result, interactionGroup{
			method:       key.method,
			pattern:      key.pattern,
			pathParams:   pathParams,
			interactions: ixs,
		})
	}

	// Auto-detect patterns for unmatched interactions
	if len(unmatched) > 0 {
		autoGroups := groupInteractions(unmatched, nil)
		result = append(result, autoGroups...)
	}

	// Sort for deterministic output
	sort.Slice(result, func(i, j int) bool {
		if result[i].pattern != result[j].pattern {
			return result[i].pattern < result[j].pattern
		}
		return result[i].method < result[j].method
	})

	return result
}

// detectPathPatterns takes interactions with the same method and segment count,
// and groups them by detecting which segments are static vs parameters.
func detectPathPatterns(interactions []Interaction) []interactionGroup {
	if len(interactions) == 0 {
		return nil
	}

	// Group by method first
	byMethod := make(map[string][]Interaction)
	for _, ix := range interactions {
		byMethod[ix.Method] = append(byMethod[ix.Method], ix)
	}

	var result []interactionGroup

	for method, ixs := range byMethod {
		patternGroups := groupByPattern(ixs)
		for pattern, grouped := range patternGroups {
			pathParams := extractPathParams(pattern)
			result = append(result, interactionGroup{
				method:       method,
				pattern:      pattern,
				pathParams:   pathParams,
				interactions: grouped,
			})
		}
	}

	return result
}

// groupByPattern groups interactions into path patterns by detecting varying segments.
func groupByPattern(interactions []Interaction) map[string][]Interaction {
	if len(interactions) == 0 {
		return nil
	}

	// Split all paths into segments
	type segmentedPath struct {
		segments []string
		ix       Interaction
	}
	var paths []segmentedPath
	for _, ix := range interactions {
		segs := strings.Split(strings.Trim(ix.Path, "/"), "/")
		paths = append(paths, segmentedPath{segments: segs, ix: ix})
	}

	segCount := len(paths[0].segments)

	// Group by static segments: find segments that are the same across all paths
	// with the same "static prefix pattern"
	// Simple approach: group paths that share the same static segments
	// and differ only in parameter segments

	// Try to find a single pattern that fits all paths
	if segCount > 0 && len(paths) > 1 {
		varying := make([]bool, segCount)
		for i := 0; i < segCount; i++ {
			first := paths[0].segments[i]
			for _, p := range paths[1:] {
				if p.segments[i] != first {
					varying[i] = true
					break
				}
			}
		}

		// Build pattern
		patternParts := make([]string, segCount)
		paramIdx := 0
		for i := 0; i < segCount; i++ {
			if varying[i] {
				patternParts[i] = fmt.Sprintf("{param%d}", paramIdx)
				paramIdx++
			} else {
				patternParts[i] = paths[0].segments[i]
			}
		}

		// Only merge into a pattern if at least one segment is static
		hasStatic := false
		for i := 0; i < segCount; i++ {
			if !varying[i] {
				hasStatic = true
				break
			}
		}

		// Check if all paths fit this single pattern (same static segments)
		allFit := hasStatic
		if allFit {
			for _, p := range paths {
				for i := 0; i < segCount; i++ {
					if !varying[i] && p.segments[i] != paths[0].segments[i] {
						allFit = false
						break
					}
				}
				if !allFit {
					break
				}
			}
		}

		if allFit {
			pattern := "/" + strings.Join(patternParts, "/")
			result := make(map[string][]Interaction)
			for _, p := range paths {
				result[pattern] = append(result[pattern], p.ix)
			}
			return result
		}
	}

	// Fallback: each unique path is its own pattern
	result := make(map[string][]Interaction)
	for _, p := range paths {
		result[p.ix.Path] = append(result[p.ix.Path], p.ix)
	}
	return result
}

// extractPathParams returns the parameter names from a path pattern.
func extractPathParams(pattern string) []string {
	var params []string
	for _, seg := range strings.Split(pattern, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			params = append(params, seg[1:len(seg)-1])
		}
	}
	return params
}

// buildOperation creates an Operation from a group of interactions.
func buildOperation(g interactionGroup) *Operation {
	op := &Operation{
		Responses: make(map[string]Response),
	}

	// Add path parameters
	for _, p := range g.pathParams {
		op.Parameters = append(op.Parameters, Parameter{
			Name:     p,
			In:       "path",
			Required: true,
			Schema:   Schema{"type": "string"},
		})
	}

	// Collect query parameters across all interactions
	queryParams := make(map[string]bool)
	for _, ix := range g.interactions {
		for k := range ix.QueryParams {
			queryParams[k] = true
		}
	}
	// Sort query param names for deterministic output
	sortedQueryParams := make([]string, 0, len(queryParams))
	for k := range queryParams {
		sortedQueryParams = append(sortedQueryParams, k)
	}
	sort.Strings(sortedQueryParams)
	for _, name := range sortedQueryParams {
		op.Parameters = append(op.Parameters, Parameter{
			Name:   name,
			In:     "query",
			Schema: Schema{"type": "string"},
		})
	}

	// Build request body schema by merging across interactions
	var requestSchema Schema
	hasRequestBody := 0
	totalInteractions := len(g.interactions)
	requestContentType := "application/json"
	for _, ix := range g.interactions {
		if len(ix.RequestBody) > 0 {
			hasRequestBody++
			if ct := ix.RequestHeaders.Get("Content-Type"); ct != "" {
				// Use the first non-empty Content-Type; strip parameters
				if idx := strings.Index(ct, ";"); idx != -1 {
					ct = strings.TrimSpace(ct[:idx])
				}
				requestContentType = ct
			}
			s := InferSchema(ix.RequestBody)
			if s != nil {
				requestSchema = MergeSchemas(requestSchema, s)
			}
		}
	}
	if hasRequestBody > 0 && requestSchema != nil {
		op.RequestBody = &RequestBody{
			Required: hasRequestBody == totalInteractions,
			Content: map[string]MediaType{
				requestContentType: {Schema: requestSchema},
			},
		}
	}

	// Build response schemas grouped by status code
	type responseInfo struct {
		schema      Schema
		contentType string
	}
	responseData := make(map[int]*responseInfo)
	for _, ix := range g.interactions {
		status := ix.ResponseStatus
		if len(ix.ResponseBody) > 0 {
			s := InferSchema(ix.ResponseBody)
			if s != nil {
				ri := responseData[status]
				if ri == nil {
					ct := "application/json"
					if h := ix.ResponseHeaders.Get("Content-Type"); h != "" {
						if idx := strings.Index(h, ";"); idx != -1 {
							h = strings.TrimSpace(h[:idx])
						}
						ct = h
					}
					ri = &responseInfo{contentType: ct}
					responseData[status] = ri
				}
				ri.schema = MergeSchemas(ri.schema, s)
			}
		} else {
			// Ensure the status code is represented even without a body
			if _, ok := responseData[status]; !ok {
				responseData[status] = nil
			}
		}
	}

	for status, ri := range responseData {
		key := fmt.Sprintf("%d", status)
		resp := Response{
			Description: fmt.Sprintf("Response %d", status),
		}
		if ri != nil && ri.schema != nil {
			resp.Content = map[string]MediaType{
				ri.contentType: {Schema: ri.schema},
			}
		}
		op.Responses[key] = resp
	}

	return op
}

// setOperation sets the operation on the PathItem for the given HTTP method.
func setOperation(item *PathItem, method string, op *Operation) {
	switch strings.ToUpper(method) {
	case "GET":
		item.Get = op
	case "POST":
		item.Post = op
	case "PUT":
		item.Put = op
	case "PATCH":
		item.Patch = op
	case "DELETE":
		item.Delete = op
	}
}
