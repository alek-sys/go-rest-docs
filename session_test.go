package gorestdocs

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteReadSessionRoundTrip(t *testing.T) {
	dir := t.TempDir()

	registry := NewRegistry()
	registry.Record(Interaction{
		Method:      "GET",
		Path:        "/pets",
		QueryParams: http.Header{"limit": {"10"}},
		RequestHeaders: http.Header{
			"Accept": {"application/json"},
		},
		ResponseStatus: 200,
		ResponseHeaders: http.Header{
			"Content-Type": {"application/json"},
		},
		ResponseBody: []byte(`[{"id":1,"name":"Fido"}]`),
	})
	registry.Record(Interaction{
		Method:      "POST",
		Path:        "/pets",
		RequestHeaders: http.Header{
			"Content-Type": {"application/json"},
		},
		RequestBody:    []byte(`{"name":"Rex"}`),
		ResponseStatus: 201,
		ResponseHeaders: http.Header{
			"Content-Type": {"application/json"},
		},
		ResponseBody: []byte(`{"id":2,"name":"Rex"}`),
	})
	registry.Record(Interaction{
		Method:         "GET",
		Path:           "/pets/1",
		ResponseStatus: 200,
		ResponseBody:   []byte(`{"id":1,"name":"Fido"}`),
	})

	patterns := []string{"/pets/{id}"}

	if err := WriteSession(dir, registry, patterns); err != nil {
		t.Fatalf("WriteSession: %v", err)
	}

	// Manifest file should exist
	if _, err := os.Stat(filepath.Join(dir, "session.json")); err != nil {
		t.Fatalf("session.json missing: %v", err)
	}

	manifest, interactions, err := ReadSession(dir)
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}

	if manifest.Version != sessionVersion {
		t.Errorf("manifest version = %q, want %q", manifest.Version, sessionVersion)
	}
	if manifest.InteractionCount != 3 {
		t.Errorf("manifest interaction count = %d, want 3", manifest.InteractionCount)
	}
	if len(manifest.Patterns) != 1 || manifest.Patterns[0] != "/pets/{id}" {
		t.Errorf("manifest patterns = %v, want [/pets/{id}]", manifest.Patterns)
	}
	if manifest.RecordedAt.IsZero() {
		t.Error("manifest recorded_at is zero")
	}

	if len(interactions) != 3 {
		t.Fatalf("loaded %d interactions, want 3", len(interactions))
	}

	// Build a map by method+path for order-independent checks
	byKey := make(map[string]Interaction)
	for _, i := range interactions {
		byKey[i.Method+":"+i.Path] = i
	}

	get := byKey["GET:/pets"]
	if get.ResponseStatus != 200 {
		t.Errorf("GET /pets status = %d, want 200", get.ResponseStatus)
	}
	if string(get.ResponseBody) != `[{"id":1,"name":"Fido"}]` {
		t.Errorf("GET /pets body = %s", get.ResponseBody)
	}
	if vals := get.QueryParams["limit"]; len(vals) == 0 || vals[0] != "10" {
		t.Errorf("GET /pets query limit = %v, want [10]", get.QueryParams["limit"])
	}

	post := byKey["POST:/pets"]
	if post.ResponseStatus != 201 {
		t.Errorf("POST /pets status = %d, want 201", post.ResponseStatus)
	}
	if string(post.RequestBody) != `{"name":"Rex"}` {
		t.Errorf("POST /pets request body = %s", post.RequestBody)
	}

	getOne := byKey["GET:/pets/1"]
	if getOne.ResponseStatus != 200 {
		t.Errorf("GET /pets/1 status = %d, want 200", getOne.ResponseStatus)
	}
}

func TestWriteSessionCreatesDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "sub", "session")

	registry := NewRegistry()
	registry.Record(Interaction{
		Method:         "DELETE",
		Path:           "/items/42",
		ResponseStatus: 204,
	})

	if err := WriteSession(dir, registry, nil); err != nil {
		t.Fatalf("WriteSession: %v", err)
	}

	manifest, interactions, err := ReadSession(dir)
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if manifest.InteractionCount != 1 {
		t.Errorf("interaction count = %d, want 1", manifest.InteractionCount)
	}
	if len(interactions) != 1 {
		t.Fatalf("loaded %d interactions, want 1", len(interactions))
	}
	if interactions[0].Method != "DELETE" || interactions[0].Path != "/items/42" {
		t.Errorf("unexpected interaction: %+v", interactions[0])
	}
	if interactions[0].ResponseStatus != 204 {
		t.Errorf("status = %d, want 204", interactions[0].ResponseStatus)
	}
}

func TestWriteSessionEmptyRegistry(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry()

	if err := WriteSession(dir, registry, nil); err != nil {
		t.Fatalf("WriteSession: %v", err)
	}

	manifest, interactions, err := ReadSession(dir)
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if manifest.InteractionCount != 0 {
		t.Errorf("interaction count = %d, want 0", manifest.InteractionCount)
	}
	if len(interactions) != 0 {
		t.Errorf("loaded %d interactions, want 0", len(interactions))
	}
}

func TestReadSessionMissingDir(t *testing.T) {
	_, _, err := ReadSession("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error reading missing session dir, got nil")
	}
}

func TestSanitizePath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/pets", "pets"},
		{"/pets/1", "pets_1"},
		{"/users/{id}/posts", "users__id__posts"},
		{"", "root"},
		{"/", "root"},
	}
	for _, c := range cases {
		got := sanitizePath(c.in)
		if got != c.want {
			t.Errorf("sanitizePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestManifestTimestamp(t *testing.T) {
	dir := t.TempDir()
	before := time.Now().UTC().Add(-time.Second)

	registry := NewRegistry()
	registry.Record(Interaction{Method: "GET", Path: "/", ResponseStatus: 200})

	if err := WriteSession(dir, registry, nil); err != nil {
		t.Fatalf("WriteSession: %v", err)
	}

	after := time.Now().UTC().Add(time.Second)

	manifest, _, err := ReadSession(dir)
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}

	if manifest.RecordedAt.Before(before) || manifest.RecordedAt.After(after) {
		t.Errorf("recorded_at %v is outside [%v, %v]", manifest.RecordedAt, before, after)
	}
}
