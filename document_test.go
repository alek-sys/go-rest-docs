package gorestdocs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeT captures test errors without failing the real test.
type fakeT struct {
	testing.TB
	errors []string
}

func newFakeT(t *testing.T) *fakeT { return &fakeT{TB: t} }
func (f *fakeT) Helper()           {}
func (f *fakeT) Errorf(format string, args ...interface{}) {
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}

func (f *fakeT) hasError(substr string) bool {
	for _, e := range f.errors {
		if strings.Contains(e, substr) {
			return true
		}
	}
	return false
}

// newTestServer creates a test server with a JSON handler and recording middleware.
func newTestServer(registry *Registry, handler http.Handler) *httptest.Server {
	return httptest.NewServer(Middleware(handler, registry))
}

// jsonHandler returns an http.Handler that responds with the given JSON for GET
// and echoes back the request body for POST with additional fields.
func jsonHandler(responseJSON string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, responseJSON)
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, responseJSON)
		}
	})
}

func TestDocument_AllFieldsDocumented(t *testing.T) {
	reg := NewRegistry()
	docs := NewDocs()
	handler := jsonHandler(`{"id":"1","name":"Fido","species":"dog"}`)
	srv := newTestServer(reg, handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/pets/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	ft := newFakeT(t)
	DocumentWith(ft, reg, docs, resp,
		Summary("Get a pet"),
		ResponseFields(
			Field("id", "string", "Pet ID"),
			Field("name", "string", "Pet name"),
			Field("species", "string", "Animal species"),
		),
	)

	if len(ft.errors) > 0 {
		t.Errorf("expected no errors, got: %v", ft.errors)
	}
}

func TestDocument_UndocumentedField(t *testing.T) {
	reg := NewRegistry()
	docs := NewDocs()
	handler := jsonHandler(`{"id":"1","name":"Fido","species":"dog"}`)
	srv := newTestServer(reg, handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/pets/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	ft := newFakeT(t)
	DocumentWith(ft, reg, docs, resp,
		ResponseFields(
			Field("id", "string", "Pet ID"),
			Field("name", "string", "Pet name"),
			// species is missing → should fail
		),
	)

	if !ft.hasError("undocumented response field: species") {
		t.Errorf("expected undocumented field error, got: %v", ft.errors)
	}
}

func TestDocument_MissingField(t *testing.T) {
	reg := NewRegistry()
	docs := NewDocs()
	handler := jsonHandler(`{"id":"1","name":"Fido"}`)
	srv := newTestServer(reg, handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/pets/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	ft := newFakeT(t)
	DocumentWith(ft, reg, docs, resp,
		ResponseFields(
			Field("id", "string", "Pet ID"),
			Field("name", "string", "Pet name"),
			Field("species", "string", "Animal species"), // not in response
		),
	)

	if !ft.hasError("documented response field missing from body: species") {
		t.Errorf("expected missing field error, got: %v", ft.errors)
	}
}

func TestDocument_TypeMismatch(t *testing.T) {
	reg := NewRegistry()
	docs := NewDocs()
	handler := jsonHandler(`{"id":1,"name":"Fido"}`)
	srv := newTestServer(reg, handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/pets/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	ft := newFakeT(t)
	DocumentWith(ft, reg, docs, resp,
		ResponseFields(
			Field("id", "string", "Pet ID"), // actual is integer
			Field("name", "string", "Pet name"),
		),
	)

	if !ft.hasError("expected type string, got integer") {
		t.Errorf("expected type mismatch error, got: %v", ft.errors)
	}
}

func TestDocument_NestedFields(t *testing.T) {
	reg := NewRegistry()
	docs := NewDocs()
	handler := jsonHandler(`{"id":"1","address":{"city":"NYC","zip":"10001"}}`)
	srv := newTestServer(reg, handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/users/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	ft := newFakeT(t)
	DocumentWith(ft, reg, docs, resp,
		ResponseFields(
			Field("id", "string", "User ID"),
			Field("address", "object", "Address"),
			Field("address.city", "string", "City"),
			Field("address.zip", "string", "ZIP code"),
		),
	)

	if len(ft.errors) > 0 {
		t.Errorf("expected no errors, got: %v", ft.errors)
	}
}

func TestDocument_ArrayFields(t *testing.T) {
	reg := NewRegistry()
	docs := NewDocs()
	handler := jsonHandler(`[{"id":"1","name":"Fido"},{"id":"2","name":"Rex"}]`)
	srv := newTestServer(reg, handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/pets")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	ft := newFakeT(t)
	DocumentWith(ft, reg, docs, resp,
		ResponseFields(
			Field("[].id", "string", "Pet ID"),
			Field("[].name", "string", "Pet name"),
		),
	)

	if len(ft.errors) > 0 {
		t.Errorf("expected no errors, got: %v", ft.errors)
	}
}

func TestDocument_NestedArrayFields(t *testing.T) {
	reg := NewRegistry()
	docs := NewDocs()
	handler := jsonHandler(`{"items":[{"id":"1","name":"Widget"}],"total":1}`)
	srv := newTestServer(reg, handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/products")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	ft := newFakeT(t)
	DocumentWith(ft, reg, docs, resp,
		ResponseFields(
			Field("items", "array", "Product list"),
			Field("items[].id", "string", "Product ID"),
			Field("items[].name", "string", "Product name"),
			Field("total", "integer", "Total count"),
		),
	)

	if len(ft.errors) > 0 {
		t.Errorf("expected no errors, got: %v", ft.errors)
	}
}

func TestDocument_RequestFieldValidation(t *testing.T) {
	reg := NewRegistry()
	docs := NewDocs()
	handler := jsonHandler(`{"id":"1","name":"Fido","species":"dog"}`)
	srv := newTestServer(reg, handler)
	defer srv.Close()

	resp, err := http.Post(
		srv.URL+"/pets",
		"application/json",
		strings.NewReader(`{"name":"Fido","species":"dog"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	ft := newFakeT(t)
	DocumentWith(ft, reg, docs, resp,
		RequestFields(
			Field("name", "string", "Pet name"),
			Field("species", "string", "Animal species"),
		),
		ResponseFields(
			Field("id", "string", "Pet ID"),
			Field("name", "string", "Pet name"),
			Field("species", "string", "Animal species"),
		),
	)

	if len(ft.errors) > 0 {
		t.Errorf("expected no errors, got: %v", ft.errors)
	}
}

func TestDocument_RequestFieldUndocumented(t *testing.T) {
	reg := NewRegistry()
	docs := NewDocs()
	handler := jsonHandler(`{"id":"1"}`)
	srv := newTestServer(reg, handler)
	defer srv.Close()

	resp, err := http.Post(
		srv.URL+"/pets",
		"application/json",
		strings.NewReader(`{"name":"Fido","species":"dog"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	ft := newFakeT(t)
	DocumentWith(ft, reg, docs, resp,
		RequestFields(
			Field("name", "string", "Pet name"),
			// species undocumented in request
		),
		ResponseFields(
			Field("id", "string", "Pet ID"),
		),
	)

	if !ft.hasError("undocumented request field: species") {
		t.Errorf("expected undocumented request field error, got: %v", ft.errors)
	}
}

func TestDocument_SpecEnrichment(t *testing.T) {
	reg := NewRegistry()
	docs := NewDocs()
	patterns := NewPathPatterns()
	patterns.Register("/pets/{id}")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && r.URL.Path == "/pets":
			json.NewEncoder(w).Encode([]map[string]string{
				{"id": "1", "name": "Fido"},
			})
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/pets/"):
			json.NewEncoder(w).Encode(map[string]string{"id": "1", "name": "Fido"})
		case r.Method == "POST" && r.URL.Path == "/pets":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": "2", "name": "Rex"})
		}
	})
	srv := newTestServer(reg, handler)
	defer srv.Close()

	// GET /pets
	resp, _ := http.Get(srv.URL + "/pets")
	resp.Body.Close()
	DocumentWith(t, reg, docs, resp,
		Summary("List all pets"),
		ResponseFields(
			Field("[].id", "string", "Pet ID"),
			Field("[].name", "string", "Pet name"),
		),
	)

	// GET /pets/1
	resp, _ = http.Get(srv.URL + "/pets/1")
	resp.Body.Close()
	DocumentWith(t, reg, docs, resp,
		Summary("Get a pet by ID"),
		PathParams(Param("id", "The unique pet identifier")),
		ResponseFields(
			Field("id", "string", "Pet ID"),
			Field("name", "string", "Pet name"),
		),
	)

	// POST /pets
	resp, _ = http.Post(srv.URL+"/pets", "application/json",
		strings.NewReader(`{"name":"Rex"}`))
	resp.Body.Close()
	DocumentWith(t, reg, docs, resp,
		Summary("Create a pet"),
		RequestFields(
			Field("name", "string", "The pet's name"),
		),
		ResponseFields(
			Field("id", "string", "Assigned ID"),
			Field("name", "string", "Pet name"),
		),
	)

	// Build spec with docs
	spec := BuildSpec(reg, Info{Title: "Test API", Version: "1.0.0"},
		WithPatterns(patterns), WithDocs(docs))

	// Check operation summaries
	if spec.Paths["/pets"].Get.Summary != "List all pets" {
		t.Errorf("GET /pets summary: got %q", spec.Paths["/pets"].Get.Summary)
	}
	if spec.Paths["/pets"].Post.Summary != "Create a pet" {
		t.Errorf("POST /pets summary: got %q", spec.Paths["/pets"].Post.Summary)
	}
	if spec.Paths["/pets/{id}"].Get.Summary != "Get a pet by ID" {
		t.Errorf("GET /pets/{id} summary: got %q", spec.Paths["/pets/{id}"].Get.Summary)
	}

	// Check path parameter description
	getByID := spec.Paths["/pets/{id}"].Get
	found := false
	for _, p := range getByID.Parameters {
		if p.Name == "id" && p.Description == "The unique pet identifier" {
			found = true
		}
	}
	if !found {
		t.Errorf("path param 'id' description not applied: %+v", getByID.Parameters)
	}

	// Check field descriptions in response schema
	listResp := spec.Paths["/pets"].Get.Responses["200"]
	if listResp.Content != nil {
		schema := listResp.Content["application/json"].Schema
		items := getItems(schema)
		if items != nil {
			props := getProperties(items)
			if idSchema := toSchema(props["id"]); idSchema != nil {
				if idSchema["description"] != "Pet ID" {
					t.Errorf("GET /pets [].id description: got %q", idSchema["description"])
				}
			}
		}
	}

	// Check request field descriptions
	postOp := spec.Paths["/pets"].Post
	if postOp.RequestBody != nil {
		reqSchema := postOp.RequestBody.Content["application/json"].Schema
		props := getProperties(reqSchema)
		if nameSchema := toSchema(props["name"]); nameSchema != nil {
			if nameSchema["description"] != "The pet's name" {
				t.Errorf("POST /pets request name description: got %q", nameSchema["description"])
			}
		}
	}
}

func TestDocument_NoDocsBackwardCompatible(t *testing.T) {
	reg := NewRegistry()
	handler := jsonHandler(`{"id":"1","name":"Fido"}`)
	srv := newTestServer(reg, handler)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/pets")
	resp.Body.Close()

	// Build spec without any docs — should work exactly as before
	spec := BuildSpec(reg, Info{Title: "Test", Version: "1.0.0"})

	if spec.Paths["/pets"].Get == nil {
		t.Fatal("expected GET /pets operation")
	}
	if spec.Paths["/pets"].Get.Summary != "" {
		t.Errorf("expected empty summary without docs, got %q", spec.Paths["/pets"].Get.Summary)
	}
}

func TestDocument_NoInteractionFound(t *testing.T) {
	reg := NewRegistry()
	docs := NewDocs()

	// Create a fake response without making any actual request through the middleware
	resp := &http.Response{
		Request: httptest.NewRequest("GET", "/nonexistent", nil),
	}

	ft := newFakeT(t)
	DocumentWith(ft, reg, docs, resp, Summary("Should fail"))

	if !ft.hasError("no recorded interaction") {
		t.Errorf("expected 'no recorded interaction' error, got: %v", ft.errors)
	}
}

func TestFlattenPaths_TopLevelObject(t *testing.T) {
	var v interface{}
	json.Unmarshal([]byte(`{"a":"x","b":1,"c":true,"d":null,"e":1.5}`), &v)

	result := make(map[string]string)
	flattenPaths(v, "", result)

	expected := map[string]string{
		"a": "string",
		"b": "integer",
		"c": "boolean",
		"d": "null",
		"e": "number",
	}

	for k, expectedType := range expected {
		if result[k] != expectedType {
			t.Errorf("field %s: expected %s, got %s", k, expectedType, result[k])
		}
	}
}

func TestFlattenPaths_NestedObject(t *testing.T) {
	var v interface{}
	json.Unmarshal([]byte(`{"user":{"name":"Alice","address":{"city":"NYC"}}}`), &v)

	result := make(map[string]string)
	flattenPaths(v, "", result)

	expected := map[string]string{
		"user":              "object",
		"user.name":         "string",
		"user.address":      "object",
		"user.address.city": "string",
	}

	for k, expectedType := range expected {
		if result[k] != expectedType {
			t.Errorf("field %s: expected %s, got %s", k, expectedType, result[k])
		}
	}
}

func TestFlattenPaths_TopLevelArray(t *testing.T) {
	var v interface{}
	json.Unmarshal([]byte(`[{"id":"1","name":"A"},{"id":"2","name":"B"}]`), &v)

	result := make(map[string]string)
	flattenPaths(v, "", result)

	expected := map[string]string{
		"[].id":   "string",
		"[].name": "string",
	}

	for k, expectedType := range expected {
		if result[k] != expectedType {
			t.Errorf("field %s: expected %s, got %s", k, expectedType, result[k])
		}
	}
}

func TestFlattenPaths_NestedArray(t *testing.T) {
	var v interface{}
	json.Unmarshal([]byte(`{"items":[{"id":"1"}],"total":5}`), &v)

	result := make(map[string]string)
	flattenPaths(v, "", result)

	expected := map[string]string{
		"items":     "array",
		"items[].id": "string",
		"total":      "integer",
	}

	for k, expectedType := range expected {
		if result[k] != expectedType {
			t.Errorf("field %s: expected %s, got %s", k, expectedType, result[k])
		}
	}
}
