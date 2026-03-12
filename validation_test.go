package gorestdocs_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	gorestdocs "github.com/alek-sys/go-rest-docs"
	"gopkg.in/yaml.v3"
)

// TestValidation_SampleRESTAPI is a manual-style validation test that creates a sample
// REST API with 3-4 endpoints, records interactions via the middleware, generates
// an OpenAPI spec, and verifies the YAML is valid OpenAPI 3.1.
func TestValidation_SampleRESTAPI(t *testing.T) {
	gorestdocs.ResetDefaultRegistry()
	gorestdocs.ResetDefaultPatterns()
	gorestdocs.RegisterPatterns("/users/{id}", "/users/{userId}/posts/{postId}")

	mux := http.NewServeMux()

	// Endpoint 1: GET /users - list users
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "name": "Alice", "email": "alice@example.com"},
				{"id": 2, "name": "Bob", "email": "bob@example.com"},
			})
		case http.MethodPost:
			// Endpoint 2: POST /users - create user
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["id"] = 3
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(body)
		}
	})

	// Endpoint 3: GET /users/{id} - get single user
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/users/"), "/")
		if len(parts) >= 3 && parts[1] == "posts" {
			// Endpoint 4: GET /users/{userId}/posts/{postId}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"postId": parts[2],
				"userId": parts[0],
				"title":  "Hello World",
				"body":   "This is a post",
			})
			return
		}

		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "name": "Alice", "email": "alice@example.com",
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	})

	srv := httptest.NewServer(gorestdocs.Handler(mux))
	defer srv.Close()

	// Make test requests exercising all endpoints
	// GET /users?limit=10
	resp, err := http.Get(srv.URL + "/users?limit=10")
	assertNoError(t, err)
	_ = resp.Body.Close()

	// POST /users
	resp, err = http.Post(srv.URL+"/users", "application/json",
		strings.NewReader(`{"name":"Charlie","email":"charlie@example.com"}`))
	assertNoError(t, err)
	_ = resp.Body.Close()

	// GET /users/1
	resp, err = http.Get(srv.URL + "/users/1")
	assertNoError(t, err)
	_ = resp.Body.Close()

	// GET /users/2 (second user, same pattern)
	resp, err = http.Get(srv.URL + "/users/2")
	assertNoError(t, err)
	_ = resp.Body.Close()

	// DELETE /users/1
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/users/1", nil)
	resp, err = http.DefaultClient.Do(req)
	assertNoError(t, err)
	_ = resp.Body.Close()

	// GET /users/1/posts/42
	resp, err = http.Get(srv.URL + "/users/1/posts/42")
	assertNoError(t, err)
	_ = resp.Body.Close()

	// Generate the spec
	var buf bytes.Buffer
	err = gorestdocs.GenerateSpec(&buf, gorestdocs.Info{
		Title:       "Sample User API",
		Version:     "1.0.0",
		Description: "A sample REST API for validation testing",
	})
	assertNoError(t, err)

	yamlOutput := buf.String()
	t.Logf("Generated OpenAPI spec:\n%s", yamlOutput)

	// Parse the YAML and validate structure
	var spec map[string]interface{}
	err = yaml.Unmarshal([]byte(yamlOutput), &spec)
	assertNoError(t, err)

	// Verify OpenAPI version
	openapi, ok := spec["openapi"].(string)
	if !ok || openapi != "3.1.0" {
		t.Fatalf("expected openapi 3.1.0, got %v", spec["openapi"])
	}

	// Verify info section
	info, ok := spec["info"].(map[string]interface{})
	if !ok {
		t.Fatal("missing info section")
	}
	if info["title"] != "Sample User API" {
		t.Errorf("expected title 'Sample User API', got %v", info["title"])
	}
	if info["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %v", info["version"])
	}
	if info["description"] != "A sample REST API for validation testing" {
		t.Errorf("expected description, got %v", info["description"])
	}

	// Verify paths section
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("missing paths section")
	}

	// Should have /users, /users/{id}, /users/{userId}/posts/{postId}
	expectedPaths := []string{"/users", "/users/{id}", "/users/{userId}/posts/{postId}"}
	for _, ep := range expectedPaths {
		if _, ok := paths[ep]; !ok {
			t.Errorf("missing expected path %s; paths present: %v", ep, pathKeys(paths))
		}
	}

	// Verify /users has GET and POST
	usersPath, ok := paths["/users"].(map[string]interface{})
	if !ok {
		t.Fatal("cannot parse /users path")
	}
	if _, ok := usersPath["get"]; !ok {
		t.Error("/users missing GET operation")
	}
	if _, ok := usersPath["post"]; !ok {
		t.Error("/users missing POST operation")
	}

	// Verify /users/{id} has GET and DELETE
	usersIdPath, ok := paths["/users/{id}"].(map[string]interface{})
	if !ok {
		t.Fatal("cannot parse /users/{id} path")
	}
	if _, ok := usersIdPath["get"]; !ok {
		t.Error("/users/{id} missing GET operation")
	}
	if _, ok := usersIdPath["delete"]; !ok {
		t.Error("/users/{id} missing DELETE operation")
	}

	// Verify GET /users has query parameter 'limit'
	getUsersOp, ok := usersPath["get"].(map[string]interface{})
	if !ok {
		t.Fatal("cannot parse GET /users")
	}
	params, _ := getUsersOp["parameters"].([]interface{})
	foundLimit := false
	for _, p := range params {
		pm, _ := p.(map[string]interface{})
		if pm["name"] == "limit" && pm["in"] == "query" {
			foundLimit = true
		}
	}
	if !foundLimit {
		t.Error("GET /users missing 'limit' query parameter")
	}

	// Verify /users/{id} GET has path parameter
	getUserByIdOp, ok := usersIdPath["get"].(map[string]interface{})
	if !ok {
		t.Fatal("cannot parse GET /users/{id}")
	}
	idParams, _ := getUserByIdOp["parameters"].([]interface{})
	foundPathParam := false
	for _, p := range idParams {
		pm, _ := p.(map[string]interface{})
		if pm["name"] == "id" && pm["in"] == "path" {
			foundPathParam = true
		}
	}
	if !foundPathParam {
		t.Error("GET /users/{id} missing 'id' path parameter")
	}

	// Verify responses exist with status codes
	getUsersResponses, _ := getUsersOp["responses"].(map[string]interface{})
	if _, ok := getUsersResponses["200"]; !ok {
		t.Error("GET /users missing 200 response")
	}

	// Verify POST /users has request body and 201 response
	postUsersOp, ok := usersPath["post"].(map[string]interface{})
	if !ok {
		t.Fatal("cannot parse POST /users")
	}
	if _, ok := postUsersOp["requestBody"]; !ok {
		t.Error("POST /users missing requestBody")
	}
	postResponses, _ := postUsersOp["responses"].(map[string]interface{})
	if _, ok := postResponses["201"]; !ok {
		t.Error("POST /users missing 201 response")
	}

	// Verify the nested path /users/{userId}/posts/{postId}
	nestedPath, ok := paths["/users/{userId}/posts/{postId}"].(map[string]interface{})
	if !ok {
		t.Fatal("cannot parse /users/{userId}/posts/{postId} path")
	}
	if _, ok := nestedPath["get"]; !ok {
		t.Error("/users/{userId}/posts/{postId} missing GET operation")
	}

	// Final structural validation: ensure the YAML can round-trip
	var roundTrip map[string]interface{}
	remarshaled, err := yaml.Marshal(spec)
	assertNoError(t, err)
	err = yaml.Unmarshal(remarshaled, &roundTrip)
	assertNoError(t, err)
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func pathKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestValidation_WriteSpecToFile tests the flag-based output by directly invoking GenerateSpec
// to write to a temp file, simulating the -gorestdocs.output workflow.
func TestValidation_WriteSpecToFile(t *testing.T) {
	gorestdocs.ResetDefaultRegistry()
	gorestdocs.ResetDefaultPatterns()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	srv := httptest.NewServer(gorestdocs.Handler(mux))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	assertNoError(t, err)
	_ = resp.Body.Close()

	tmpFile := t.TempDir() + "/api.yaml"

	// Use GenerateSpec to write to a file (same mechanism as WriteSpecIfEnabled)
	var buf bytes.Buffer
	err = gorestdocs.GenerateSpec(&buf, gorestdocs.Info{
		Title:   "Health API",
		Version: "0.1.0",
	})
	assertNoError(t, err)

	// Write to file
	err = writeFile(tmpFile, buf.Bytes())
	assertNoError(t, err)

	// Read back and validate
	data, err := readFile(tmpFile)
	assertNoError(t, err)

	var spec map[string]interface{}
	err = yaml.Unmarshal(data, &spec)
	assertNoError(t, err)

	if spec["openapi"] != "3.1.0" {
		t.Errorf("expected openapi 3.1.0, got %v", spec["openapi"])
	}

	paths, _ := spec["paths"].(map[string]interface{})
	if _, ok := paths["/health"]; !ok {
		t.Error("missing /health path in generated spec")
	}

	fmt.Printf("Spec written to %s (%d bytes)\n", tmpFile, len(data))
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
