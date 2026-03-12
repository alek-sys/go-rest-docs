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
func BuildSpec(registry *Registry, info Info) OpenAPI {
	spec := OpenAPI{
		OpenAPI: "3.1.0",
		Info:    info,
		Paths:   make(map[string]PathItem),
	}

	interactions := registry.All()
	if len(interactions) == 0 {
		return spec
	}

	groups := groupInteractions(interactions)

	for _, g := range groups {
		pathItem := spec.Paths[g.pattern]
		op := buildOperation(g)
		setOperation(&pathItem, g.method, op)
		spec.Paths[g.pattern] = pathItem
	}

	return spec
}

// MarshalYAML marshals the OpenAPI spec to YAML bytes.
func MarshalYAML(spec OpenAPI) ([]byte, error) {
	return yaml.Marshal(spec)
}

// groupInteractions groups interactions by path pattern and method.
// It detects path parameters by finding segments that vary across interactions
// with the same structure.
func groupInteractions(interactions []Interaction) []interactionGroup {
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
		_ = method
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

	// First, find which segment positions vary
	// We group paths by their "static skeleton"
	type skeleton struct {
		parts string // static segments joined, with * for varying
	}

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
	hasRequestBody := false
	for _, ix := range g.interactions {
		if len(ix.RequestBody) > 0 {
			hasRequestBody = true
			s := InferSchema(ix.RequestBody)
			if s != nil {
				requestSchema = MergeSchemas(requestSchema, s)
			}
		}
	}
	if hasRequestBody && requestSchema != nil {
		op.RequestBody = &RequestBody{
			Required: true,
			Content: map[string]MediaType{
				"application/json": {Schema: requestSchema},
			},
		}
	}

	// Build response schemas grouped by status code
	responseSchemas := make(map[int]Schema)
	for _, ix := range g.interactions {
		status := ix.ResponseStatus
		if len(ix.ResponseBody) > 0 {
			s := InferSchema(ix.ResponseBody)
			if s != nil {
				responseSchemas[status] = MergeSchemas(responseSchemas[status], s)
			}
		} else {
			// Ensure the status code is represented even without a body
			if _, ok := responseSchemas[status]; !ok {
				responseSchemas[status] = nil
			}
		}
	}

	for status, schema := range responseSchemas {
		key := fmt.Sprintf("%d", status)
		resp := Response{
			Description: fmt.Sprintf("Response %d", status),
		}
		if schema != nil {
			resp.Content = map[string]MediaType{
				"application/json": {Schema: schema},
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
