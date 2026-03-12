package gorestdocs

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHandler(t *testing.T) {
	ResetDefaultRegistry()

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	srv := httptest.NewServer(Handler(mux))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ping")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	interactions := DefaultRegistry().All()
	if len(interactions) != 1 {
		t.Fatalf("expected 1 interaction, got %d", len(interactions))
	}
	if interactions[0].Path != "/ping" {
		t.Errorf("expected path /ping, got %s", interactions[0].Path)
	}
	if interactions[0].ResponseStatus != 200 {
		t.Errorf("expected status 200, got %d", interactions[0].ResponseStatus)
	}
}

func TestGenerateSpec(t *testing.T) {
	ResetDefaultRegistry()

	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "name": "Alice"},
			})
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 2, "name": "Bob"})
		}
	})

	srv := httptest.NewServer(Handler(mux))
	defer srv.Close()

	// GET /users
	resp, err := http.Get(srv.URL + "/users")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	_ = resp.Body.Close()

	// POST /users
	body := strings.NewReader(`{"name":"Bob"}`)
	resp, err = http.Post(srv.URL+"/users", "application/json", body)
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	_ = resp.Body.Close()

	var buf bytes.Buffer
	err = GenerateSpec(&buf, Info{Title: "Test API", Version: "1.0.0"})
	if err != nil {
		t.Fatalf("GenerateSpec error: %v", err)
	}

	// Parse the YAML output and verify structure
	var spec OpenAPI
	if err := yaml.Unmarshal(buf.Bytes(), &spec); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	if spec.OpenAPI != "3.1.0" {
		t.Errorf("expected openapi 3.1.0, got %s", spec.OpenAPI)
	}
	if spec.Info.Title != "Test API" {
		t.Errorf("expected title 'Test API', got %s", spec.Info.Title)
	}

	usersPath, ok := spec.Paths["/users"]
	if !ok {
		t.Fatal("expected /users path in spec")
	}
	if usersPath.Get == nil {
		t.Error("expected GET operation for /users")
	}
	if usersPath.Post == nil {
		t.Error("expected POST operation for /users")
	}
	if usersPath.Post != nil && usersPath.Post.RequestBody == nil {
		t.Error("expected request body for POST /users")
	}
}

func TestWriteSpecIfEnabled_Disabled(t *testing.T) {
	// When flag is empty, WriteSpecIfEnabled should do nothing
	outputFlag = ""
	err := WriteSpecIfEnabled(Info{Title: "Test", Version: "1.0.0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteSpecIfEnabled_Enabled(t *testing.T) {
	ResetDefaultRegistry()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"healthy":true}`))
	})

	srv := httptest.NewServer(Handler(mux))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	_ = resp.Body.Close()

	// Set output flag to a temp file
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "openapi.yaml")
	outputFlag = outPath

	err = WriteSpecIfEnabled(Info{Title: "Health API", Version: "0.1.0"})
	if err != nil {
		t.Fatalf("WriteSpecIfEnabled error: %v", err)
	}

	// Verify file was written
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var spec OpenAPI
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatalf("failed to parse written YAML: %v", err)
	}

	if spec.Info.Title != "Health API" {
		t.Errorf("expected title 'Health API', got %s", spec.Info.Title)
	}
	if _, ok := spec.Paths["/health"]; !ok {
		t.Error("expected /health path in spec")
	}

	// Reset the flag so it doesn't interfere with other tests
	outputFlag = ""
}

func TestWriteSpecIfEnabled_FlagOverrides(t *testing.T) {
	ResetDefaultRegistry()

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	srv := httptest.NewServer(Handler(mux))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ping")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	_ = resp.Body.Close()

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "openapi.yaml")
	outputFlag = outPath
	titleFlag = "Overridden Title"
	versionFlag = "2.0.0"
	descriptionFlag = "Overridden description"

	err = WriteSpecIfEnabled(Info{Title: "Original", Version: "1.0.0"})
	if err != nil {
		t.Fatalf("WriteSpecIfEnabled error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var spec OpenAPI
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	if spec.Info.Title != "Overridden Title" {
		t.Errorf("expected title 'Overridden Title', got %s", spec.Info.Title)
	}
	if spec.Info.Version != "2.0.0" {
		t.Errorf("expected version '2.0.0', got %s", spec.Info.Version)
	}
	if spec.Info.Description != "Overridden description" {
		t.Errorf("expected description 'Overridden description', got %s", spec.Info.Description)
	}

	// Reset flags
	outputFlag = ""
	titleFlag = ""
	versionFlag = ""
	descriptionFlag = ""
}

func TestResetDefaultRegistry(t *testing.T) {
	defaultRegistry.Record(Interaction{Method: "GET", Path: "/test"})
	if len(DefaultRegistry().All()) == 0 {
		t.Fatal("expected at least one interaction")
	}
	ResetDefaultRegistry()
	if len(DefaultRegistry().All()) != 0 {
		t.Error("expected empty registry after reset")
	}
}
