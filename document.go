package gorestdocs

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// FieldDoc describes a single field in a request or response body.
type FieldDoc struct {
	Path        string // dot-separated path, e.g. "address.city" or "[].id"
	Type        string // "string", "integer", "number", "boolean", "object", "array"
	Description string
}

// ParamDoc describes a path or query parameter.
type ParamDoc struct {
	Name        string
	Description string
}

// EndpointDoc holds all documentation for a single operation (method + path pattern).
type EndpointDoc struct {
	Summary        string
	Description    string
	RequestFields  []FieldDoc
	ResponseFields []FieldDoc
	PathParams     []ParamDoc
	QueryParams    []ParamDoc
}

// DocOption configures an EndpointDoc.
type DocOption func(*EndpointDoc)

// Summary sets the operation summary.
func Summary(s string) DocOption {
	return func(d *EndpointDoc) { d.Summary = s }
}

// OpDescription sets the operation description (longer form).
func OpDescription(s string) DocOption {
	return func(d *EndpointDoc) { d.Description = s }
}

// RequestFields documents and validates request body fields.
func RequestFields(fields ...FieldDoc) DocOption {
	return func(d *EndpointDoc) {
		d.RequestFields = append(d.RequestFields, fields...)
	}
}

// ResponseFields documents and validates response body fields.
func ResponseFields(fields ...FieldDoc) DocOption {
	return func(d *EndpointDoc) {
		d.ResponseFields = append(d.ResponseFields, fields...)
	}
}

// PathParams documents path parameters.
func PathParams(params ...ParamDoc) DocOption {
	return func(d *EndpointDoc) {
		d.PathParams = append(d.PathParams, params...)
	}
}

// QueryParams documents query parameters.
func QueryParams(params ...ParamDoc) DocOption {
	return func(d *EndpointDoc) {
		d.QueryParams = append(d.QueryParams, params...)
	}
}

// Field creates a FieldDoc.
func Field(path, typ, description string) FieldDoc {
	return FieldDoc{Path: path, Type: typ, Description: description}
}

// Param creates a ParamDoc.
func Param(name, description string) ParamDoc {
	return ParamDoc{Name: name, Description: description}
}

// endpointKey identifies an endpoint by method and path pattern.
type endpointKey struct {
	method  string
	pattern string
}

// Docs is a thread-safe collection of endpoint documentation.
type Docs struct {
	mu    sync.Mutex
	items map[endpointKey]*EndpointDoc
}

// NewDocs creates a new empty Docs store.
func NewDocs() *Docs {
	return &Docs{items: make(map[endpointKey]*EndpointDoc)}
}

// store adds or merges documentation for an endpoint.
func (d *Docs) store(method, pattern string, opts ...DocOption) {
	d.mu.Lock()
	defer d.mu.Unlock()
	key := endpointKey{method: strings.ToUpper(method), pattern: pattern}
	doc := d.items[key]
	if doc == nil {
		doc = &EndpointDoc{}
		d.items[key] = doc
	}
	for _, opt := range opts {
		opt(doc)
	}
}

// Lookup returns the EndpointDoc for a method+pattern, or nil.
func (d *Docs) Lookup(method, pattern string) *EndpointDoc {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.items[endpointKey{method: strings.ToUpper(method), pattern: pattern}]
}

// Reset clears all stored documentation.
func (d *Docs) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.items = make(map[endpointKey]*EndpointDoc)
}

// Document validates the HTTP response against documented fields and stores
// the documentation for spec generation. It uses the default registry to find
// the recorded interaction matching the response.
//
// Validation rules:
//   - Every field in the response body must have a corresponding FieldDoc (no undocumented fields)
//   - Every documented FieldDoc must be present in the response body (no phantom docs)
//   - Field types must match (string, integer, number, boolean, object, array)
//   - Same rules apply to request fields when RequestFields is provided
func Document(t testing.TB, resp *http.Response, opts ...DocOption) {
	t.Helper()
	DocumentWith(t, defaultRegistry, defaultDocs, resp, opts...)
}

// DocumentWith is like Document but uses explicit registry and docs stores.
func DocumentWith(t testing.TB, registry *Registry, docs *Docs, resp *http.Response, opts ...DocOption) {
	t.Helper()

	if resp == nil || resp.Request == nil {
		t.Errorf("gorestdocs.Document: resp or resp.Request is nil")
		return
	}

	method := resp.Request.Method
	path := resp.Request.URL.Path

	// Find the matching interaction (last one for this method+path)
	ix, ok := findInteraction(registry, method, path)
	if !ok {
		t.Errorf("gorestdocs.Document: no recorded interaction for %s %s", method, path)
		return
	}

	// Build the EndpointDoc from options
	doc := &EndpointDoc{}
	for _, opt := range opts {
		opt(doc)
	}

	// Validate response fields
	if len(doc.ResponseFields) > 0 {
		validateFields(t, ix.ResponseBody, doc.ResponseFields, "response")
	}

	// Validate request fields
	if len(doc.RequestFields) > 0 {
		validateFields(t, ix.RequestBody, doc.RequestFields, "request")
	}

	// Store docs for spec generation
	docs.store(method, path, opts...)
}

// findInteraction returns the last recorded interaction matching method and path.
func findInteraction(registry *Registry, method, path string) (Interaction, bool) {
	all := registry.All()
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Method == method && all[i].Path == path {
			return all[i], true
		}
	}
	return Interaction{}, false
}

// validateFields checks that the documented fields match the actual JSON body.
func validateFields(t testing.TB, body []byte, fields []FieldDoc, direction string) {
	t.Helper()

	if len(body) == 0 {
		if len(fields) > 0 {
			t.Errorf("gorestdocs: %s body is empty but %d fields are documented", direction, len(fields))
		}
		return
	}

	var parsed interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Errorf("gorestdocs: failed to parse %s body as JSON: %v", direction, err)
		return
	}

	// Flatten actual field paths with their types
	actual := make(map[string]string) // path -> JSON type
	flattenPaths(parsed, "", actual)

	// Build documented set
	documented := make(map[string]FieldDoc)
	for _, f := range fields {
		documented[f.Path] = f
	}

	// Check for undocumented fields
	for path := range actual {
		if _, ok := documented[path]; !ok {
			t.Errorf("gorestdocs: undocumented %s field: %s", direction, path)
		}
	}

	// Check for missing fields and type mismatches
	for _, f := range fields {
		actualType, exists := actual[f.Path]
		if !exists {
			t.Errorf("gorestdocs: documented %s field missing from body: %s", direction, f.Path)
			continue
		}
		if f.Type != "" && actualType != f.Type {
			t.Errorf("gorestdocs: %s field %s: expected type %s, got %s", direction, f.Path, f.Type, actualType)
		}
	}
}

// flattenPaths recursively walks a parsed JSON value and collects all field paths
// with their inferred types.
func flattenPaths(v interface{}, prefix string, result map[string]string) {
	switch val := v.(type) {
	case map[string]interface{}:
		// Record the object path, but not for array item containers (prefix ending in "[]")
		// since those are documented via the array field itself.
		if prefix != "" && !strings.HasSuffix(prefix, "[]") {
			result[prefix] = "object"
		}
		for k, child := range val {
			childPath := k
			if prefix != "" {
				childPath = prefix + "." + k
			}
			flattenPaths(child, childPath, result)
		}
	case []interface{}:
		// Flatten array items using [] notation.
		// For nested arrays (prefix != ""), also record the array container.
		arrayPrefix := "[]"
		if prefix != "" {
			result[prefix] = "array"
			arrayPrefix = prefix + "[]"
		}
		// Merge paths from all array elements
		for _, item := range val {
			flattenPaths(item, arrayPrefix, result)
		}
	case float64:
		if val == float64(int64(val)) {
			result[prefix] = "integer"
		} else {
			result[prefix] = "number"
		}
	case string:
		result[prefix] = "string"
	case bool:
		result[prefix] = "boolean"
	case nil:
		result[prefix] = "null"
	}
}

// applyDocs enriches an Operation with documentation from an EndpointDoc.
// It adds descriptions to operations, parameters, and schema properties
// without changing the schema structure.
func applyDocs(op *Operation, doc *EndpointDoc) {
	if doc.Summary != "" {
		op.Summary = doc.Summary
	}
	if doc.Description != "" {
		op.Description = doc.Description
	}

	// Build param description lookup
	paramDescs := make(map[string]string)
	for _, p := range doc.PathParams {
		paramDescs[p.Name] = p.Description
	}
	for _, p := range doc.QueryParams {
		paramDescs[p.Name] = p.Description
	}

	// Apply parameter descriptions
	for i, p := range op.Parameters {
		if desc, ok := paramDescs[p.Name]; ok {
			op.Parameters[i].Description = desc
		}
	}

	// Apply request field descriptions
	if op.RequestBody != nil && len(doc.RequestFields) > 0 {
		for ct, mt := range op.RequestBody.Content {
			applyFieldDocs(mt.Schema, doc.RequestFields)
			op.RequestBody.Content[ct] = mt
		}
	}

	// Apply response field descriptions
	if len(doc.ResponseFields) > 0 {
		for statusStr, resp := range op.Responses {
			for ct, mt := range resp.Content {
				applyFieldDocs(mt.Schema, doc.ResponseFields)
				resp.Content[ct] = mt
			}
			op.Responses[statusStr] = resp
		}
	}
}

// applyFieldDocs sets description on schema properties matching the field paths.
func applyFieldDocs(schema Schema, fields []FieldDoc) {
	for _, f := range fields {
		setFieldDescription(schema, f.Path, f.Description)
	}
}

// setFieldDescription navigates a dot-path into a schema and sets the description.
// Supports: "name", "address.city", "[].name", "items[].name"
func setFieldDescription(schema Schema, path, description string) {
	parts := splitFieldPath(path)
	current := schema

	for i, part := range parts {
		isLast := i == len(parts)-1

		if part == "[]" {
			// Navigate into array items
			items := getItems(current)
			if items == nil {
				return
			}
			current = items
			continue
		}

		// Check if part ends with [] (e.g., "items[]")
		if strings.HasSuffix(part, "[]") {
			propName := strings.TrimSuffix(part, "[]")
			props := getProperties(current)
			if props == nil {
				return
			}
			propSchema := toSchema(props[propName])
			if propSchema == nil {
				return
			}
			if isLast {
				propSchema["description"] = description
				props[propName] = propSchema
				return
			}
			items := getItems(propSchema)
			if items == nil {
				return
			}
			current = items
			continue
		}

		// Regular property navigation
		props := getProperties(current)
		if props == nil {
			return
		}
		propSchema := toSchema(props[part])
		if propSchema == nil {
			return
		}

		if isLast {
			propSchema["description"] = description
			props[part] = propSchema
		} else {
			pt := primaryType(propSchema)
			if pt == "object" || pt == "array" {
				current = propSchema
			} else {
				return
			}
		}
	}
}

// splitFieldPath splits a field path like "a.b[].c" into parts.
// It splits on "." but keeps "[]" attached to the preceding segment.
func splitFieldPath(path string) []string {
	if path == "" {
		return nil
	}
	// Handle leading [] (top-level array)
	if strings.HasPrefix(path, "[].") {
		rest := splitFieldPath(path[3:])
		return append([]string{"[]"}, rest...)
	}
	if path == "[]" {
		return []string{"[]"}
	}
	return strings.Split(path, ".")
}

// toSchema converts an interface{} to Schema if possible.
func toSchema(v interface{}) Schema {
	switch s := v.(type) {
	case Schema:
		return s
	case map[string]interface{}:
		return Schema(s)
	}
	return nil
}
