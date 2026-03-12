package gorestdocs

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuildSpec_EmptyRegistry(t *testing.T) {
	reg := NewRegistry()
	spec := BuildSpec(reg, Info{Title: "Test API", Version: "1.0.0"})

	if spec.OpenAPI != "3.1.0" {
		t.Errorf("expected openapi 3.1.0, got %s", spec.OpenAPI)
	}
	if spec.Info.Title != "Test API" {
		t.Errorf("expected title Test API, got %s", spec.Info.Title)
	}
	if len(spec.Paths) != 0 {
		t.Errorf("expected empty paths, got %d", len(spec.Paths))
	}
}

func TestBuildSpec_SingleEndpoint(t *testing.T) {
	reg := NewRegistry()
	reg.Record(Interaction{
		Method:         "GET",
		Path:           "/users",
		ResponseStatus: 200,
		ResponseBody:   []byte(`[{"id": 1, "name": "alice"}]`),
	})

	spec := BuildSpec(reg, Info{Title: "Test", Version: "1.0"})

	pathItem, ok := spec.Paths["/users"]
	if !ok {
		t.Fatal("expected /users path")
	}
	if pathItem.Get == nil {
		t.Fatal("expected GET operation")
	}
	if pathItem.Post != nil {
		t.Error("expected no POST operation")
	}

	resp, ok := pathItem.Get.Responses["200"]
	if !ok {
		t.Fatal("expected 200 response")
	}
	mt, ok := resp.Content["application/json"]
	if !ok {
		t.Fatal("expected application/json content")
	}
	assertType(t, mt.Schema, "array")
}

func TestBuildSpec_MultipleEndpoints(t *testing.T) {
	reg := NewRegistry()
	reg.Record(Interaction{
		Method:         "GET",
		Path:           "/users",
		ResponseStatus: 200,
		ResponseBody:   []byte(`[{"id": 1}]`),
	})
	reg.Record(Interaction{
		Method:         "POST",
		Path:           "/users",
		RequestBody:    []byte(`{"name": "alice"}`),
		ResponseStatus: 201,
		ResponseBody:   []byte(`{"id": 1, "name": "alice"}`),
	})
	reg.Record(Interaction{
		Method:         "GET",
		Path:           "/posts",
		ResponseStatus: 200,
		ResponseBody:   []byte(`[]`),
	})

	spec := BuildSpec(reg, Info{Title: "Test", Version: "1.0"})

	if len(spec.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(spec.Paths))
	}

	usersPath := spec.Paths["/users"]
	if usersPath.Get == nil {
		t.Error("expected GET /users")
	}
	if usersPath.Post == nil {
		t.Error("expected POST /users")
	}
	if usersPath.Post.RequestBody == nil {
		t.Error("expected request body for POST /users")
	}

	postsPath := spec.Paths["/posts"]
	if postsPath.Get == nil {
		t.Error("expected GET /posts")
	}
}

func TestBuildSpec_PathParams(t *testing.T) {
	reg := NewRegistry()
	reg.Record(Interaction{
		Method:         "GET",
		Path:           "/users/123",
		ResponseStatus: 200,
		ResponseBody:   []byte(`{"id": 123, "name": "alice"}`),
	})
	reg.Record(Interaction{
		Method:         "GET",
		Path:           "/users/456",
		ResponseStatus: 200,
		ResponseBody:   []byte(`{"id": 456, "name": "bob"}`),
	})

	spec := BuildSpec(reg, Info{Title: "Test", Version: "1.0"})

	// Should detect /users/{param0} pattern
	var foundPattern string
	for path := range spec.Paths {
		if strings.Contains(path, "{") {
			foundPattern = path
			break
		}
	}
	if foundPattern == "" {
		t.Fatal("expected parameterized path pattern, got paths:", specPaths(spec))
	}

	pathItem := spec.Paths[foundPattern]
	if pathItem.Get == nil {
		t.Fatal("expected GET operation on parameterized path")
	}

	// Should have a path parameter
	hasPathParam := false
	for _, p := range pathItem.Get.Parameters {
		if p.In == "path" {
			hasPathParam = true
			if !p.Required {
				t.Error("path parameter should be required")
			}
		}
	}
	if !hasPathParam {
		t.Error("expected path parameter")
	}
}

func TestBuildSpec_QueryParams(t *testing.T) {
	reg := NewRegistry()
	reg.Record(Interaction{
		Method:      "GET",
		Path:        "/search",
		QueryParams: map[string][]string{"q": {"golang"}, "page": {"1"}},
		ResponseStatus: 200,
		ResponseBody:   []byte(`{"results": []}`),
	})

	spec := BuildSpec(reg, Info{Title: "Test", Version: "1.0"})

	pathItem := spec.Paths["/search"]
	if pathItem.Get == nil {
		t.Fatal("expected GET operation")
	}

	queryParams := make(map[string]bool)
	for _, p := range pathItem.Get.Parameters {
		if p.In == "query" {
			queryParams[p.Name] = true
		}
	}
	if !queryParams["q"] {
		t.Error("expected query param 'q'")
	}
	if !queryParams["page"] {
		t.Error("expected query param 'page'")
	}
}

func TestBuildSpec_RequestAndResponseBodies(t *testing.T) {
	reg := NewRegistry()
	reg.Record(Interaction{
		Method:         "POST",
		Path:           "/items",
		RequestBody:    []byte(`{"name": "widget", "price": 9.99}`),
		ResponseStatus: 201,
		ResponseBody:   []byte(`{"id": 1, "name": "widget", "price": 9.99}`),
	})

	spec := BuildSpec(reg, Info{Title: "Test", Version: "1.0"})

	pathItem := spec.Paths["/items"]
	if pathItem.Post == nil {
		t.Fatal("expected POST operation")
	}

	// Request body
	if pathItem.Post.RequestBody == nil {
		t.Fatal("expected request body")
	}
	reqMT, ok := pathItem.Post.RequestBody.Content["application/json"]
	if !ok {
		t.Fatal("expected application/json request content")
	}
	assertType(t, reqMT.Schema, "object")
	reqProps := reqMT.Schema["properties"].(map[string]interface{})
	if _, ok := reqProps["name"]; !ok {
		t.Error("expected name property in request schema")
	}
	if _, ok := reqProps["price"]; !ok {
		t.Error("expected price property in request schema")
	}

	// Response body
	resp201, ok := pathItem.Post.Responses["201"]
	if !ok {
		t.Fatal("expected 201 response")
	}
	respMT, ok := resp201.Content["application/json"]
	if !ok {
		t.Fatal("expected application/json response content")
	}
	assertType(t, respMT.Schema, "object")
	respProps := respMT.Schema["properties"].(map[string]interface{})
	if _, ok := respProps["id"]; !ok {
		t.Error("expected id property in response schema")
	}
}

func TestBuildSpec_MergesResponseSchemas(t *testing.T) {
	reg := NewRegistry()
	reg.Record(Interaction{
		Method:         "GET",
		Path:           "/items",
		ResponseStatus: 200,
		ResponseBody:   []byte(`{"id": 1, "name": "widget"}`),
	})
	reg.Record(Interaction{
		Method:         "GET",
		Path:           "/items",
		ResponseStatus: 200,
		ResponseBody:   []byte(`{"id": 2, "name": "gadget", "description": "cool"}`),
	})

	spec := BuildSpec(reg, Info{Title: "Test", Version: "1.0"})

	pathItem := spec.Paths["/items"]
	resp := pathItem.Get.Responses["200"]
	mt := resp.Content["application/json"]
	props := mt.Schema["properties"].(map[string]interface{})

	// description only in second interaction, should be nullable
	descSchema := toSchema(props["description"])
	if descSchema == nil {
		t.Fatal("expected description property")
	}
	if !isNullable(descSchema) {
		t.Error("expected description to be nullable (only in one interaction)")
	}
}

func TestBuildSpec_NoResponseBody(t *testing.T) {
	reg := NewRegistry()
	reg.Record(Interaction{
		Method:         "DELETE",
		Path:           "/items/1",
		ResponseStatus: 204,
	})

	spec := BuildSpec(reg, Info{Title: "Test", Version: "1.0"})

	pathItem := spec.Paths["/items/1"]
	if pathItem.Delete == nil {
		t.Fatal("expected DELETE operation")
	}
	resp, ok := pathItem.Delete.Responses["204"]
	if !ok {
		t.Fatal("expected 204 response")
	}
	if resp.Content != nil {
		t.Error("expected no content for 204 response")
	}
}

func TestMarshalYAML(t *testing.T) {
	reg := NewRegistry()
	reg.Record(Interaction{
		Method:         "GET",
		Path:           "/health",
		ResponseStatus: 200,
		ResponseBody:   []byte(`{"status": "ok"}`),
	})

	spec := BuildSpec(reg, Info{Title: "My API", Version: "1.0.0"})
	data, err := MarshalYAML(spec)
	if err != nil {
		t.Fatalf("MarshalYAML error: %v", err)
	}

	// Parse back to verify it's valid YAML
	var parsed map[string]interface{}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("generated YAML is not valid: %v", err)
	}

	if parsed["openapi"] != "3.1.0" {
		t.Errorf("expected openapi 3.1.0, got %v", parsed["openapi"])
	}

	info, ok := parsed["info"].(map[string]interface{})
	if !ok {
		t.Fatal("expected info map")
	}
	if info["title"] != "My API" {
		t.Errorf("expected title My API, got %v", info["title"])
	}

	paths, ok := parsed["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("expected paths map")
	}
	if _, ok := paths["/health"]; !ok {
		t.Error("expected /health path in YAML output")
	}
}

func TestExtractPathParams(t *testing.T) {
	params := extractPathParams("/users/{id}/posts/{postId}")
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(params))
	}
	if params[0] != "id" {
		t.Errorf("expected first param 'id', got %s", params[0])
	}
	if params[1] != "postId" {
		t.Errorf("expected second param 'postId', got %s", params[1])
	}
}

// helpers

func specPaths(spec OpenAPI) []string {
	var paths []string
	for p := range spec.Paths {
		paths = append(paths, p)
	}
	return paths
}
