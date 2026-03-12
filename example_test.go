package gorestdocs_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	gorestdocs "github.com/alek-sys/go-rest-docs"
)

func ExampleHandler() {
	gorestdocs.ResetDefaultRegistry()

	// Set up your API handler
	mux := http.NewServeMux()
	mux.HandleFunc("/pets", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"id": "1", "name": "Fido", "species": "dog"},
			})
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "2", "name": "Whiskers", "species": "cat"})
		}
	})

	// Wrap with gorestdocs middleware
	srv := httptest.NewServer(gorestdocs.Handler(mux))
	defer srv.Close()

	// Make test requests (these get recorded)
	_, _ = http.Get(srv.URL + "/pets")
	_, _ = http.Post(srv.URL+"/pets", "application/json",
		strings.NewReader(`{"name":"Whiskers","species":"cat"}`))

	// Generate the spec
	var buf bytes.Buffer
	_ = gorestdocs.GenerateSpec(&buf, gorestdocs.Info{
		Title:   "Pet Store API",
		Version: "1.0.0",
	})

	// The output is a valid OpenAPI 3.1 YAML spec
	output := buf.String()
	fmt.Println(strings.Contains(output, "openapi:") && strings.Contains(output, "3.1.0"))
	fmt.Println(strings.Contains(output, "title: Pet Store API"))
	fmt.Println(strings.Contains(output, "/pets"))
	// Output:
	// true
	// true
	// true
}
