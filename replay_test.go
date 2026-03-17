package gorestdocs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func makeTestSession(t *testing.T, interactions []Interaction, patterns []string) string {
	t.Helper()
	dir := t.TempDir()
	reg := NewRegistry()
	for _, i := range interactions {
		reg.Record(i)
	}
	if err := WriteSession(dir, reg, patterns); err != nil {
		t.Fatalf("WriteSession: %v", err)
	}
	return dir
}

func TestReplayServer_BasicMatch(t *testing.T) {
	dir := makeTestSession(t, []Interaction{
		{
			Method:          "GET",
			Path:            "/pets",
			ResponseStatus:  200,
			ResponseHeaders: http.Header{"Content-Type": {"application/json"}},
			ResponseBody:    []byte(`[{"id":1,"name":"Fluffy"}]`),
		},
	}, nil)

	srv, err := NewReplayServer(dir)
	if err != nil {
		t.Fatalf("NewReplayServer: %v", err)
	}

	req := httptest.NewRequest("GET", "/pets", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("want 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("want Content-Type application/json, got %q", ct)
	}
	if body := rec.Body.String(); body != `[{"id":1,"name":"Fluffy"}]` {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestReplayServer_PatternMatch(t *testing.T) {
	dir := makeTestSession(t, []Interaction{
		{
			Method:         "GET",
			Path:           "/pets/42",
			ResponseStatus: 200,
			ResponseBody:   []byte(`{"id":42}`),
		},
	}, []string{"/pets/{id}"})

	srv, err := NewReplayServer(dir)
	if err != nil {
		t.Fatalf("NewReplayServer: %v", err)
	}

	req := httptest.NewRequest("GET", "/pets/99", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("want 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != `{"id":42}` {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestReplayServer_ExactPathPreferred(t *testing.T) {
	dir := makeTestSession(t, []Interaction{
		{
			Method:         "GET",
			Path:           "/pets/1",
			ResponseStatus: 200,
			ResponseBody:   []byte(`{"id":1}`),
		},
		{
			Method:         "GET",
			Path:           "/pets/2",
			ResponseStatus: 200,
			ResponseBody:   []byte(`{"id":2}`),
		},
	}, []string{"/pets/{id}"})

	srv, err := NewReplayServer(dir)
	if err != nil {
		t.Fatalf("NewReplayServer: %v", err)
	}

	// /pets/2 should be preferred over /pets/1 by exact match
	req := httptest.NewRequest("GET", "/pets/2", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("want 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != `{"id":2}` {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestReplayServer_NotFound(t *testing.T) {
	dir := makeTestSession(t, []Interaction{
		{
			Method:         "GET",
			Path:           "/pets",
			ResponseStatus: 200,
			ResponseBody:   []byte(`[]`),
		},
	}, nil)

	srv, err := NewReplayServer(dir)
	if err != nil {
		t.Fatalf("NewReplayServer: %v", err)
	}

	req := httptest.NewRequest("GET", "/unknown", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Errorf("want 404, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "no recorded interaction") {
		t.Errorf("expected helpful 404 message, got: %q", body)
	}
	if !strings.Contains(body, "GET /pets") {
		t.Errorf("expected available endpoint in 404 message, got: %q", body)
	}
}

func TestReplayServer_MethodNotFound(t *testing.T) {
	dir := makeTestSession(t, []Interaction{
		{
			Method:         "GET",
			Path:           "/pets",
			ResponseStatus: 200,
			ResponseBody:   []byte(`[]`),
		},
	}, nil)

	srv, err := NewReplayServer(dir)
	if err != nil {
		t.Fatalf("NewReplayServer: %v", err)
	}

	req := httptest.NewRequest("POST", "/pets", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Errorf("want 404, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "no recorded interaction") {
		t.Errorf("expected helpful 404 message, got: %q", body)
	}
	if !strings.Contains(body, "GET /pets") {
		t.Errorf("expected available endpoint in 404 message, got: %q", body)
	}
}

func TestReplayServer_Endpoints(t *testing.T) {
	dir := makeTestSession(t, []Interaction{
		{Method: "GET", Path: "/pets", ResponseStatus: 200},
		{Method: "POST", Path: "/pets", ResponseStatus: 201},
		{Method: "GET", Path: "/pets/1", ResponseStatus: 200},
	}, []string{"/pets/{id}"})

	srv, err := NewReplayServer(dir)
	if err != nil {
		t.Fatalf("NewReplayServer: %v", err)
	}

	endpoints := srv.Endpoints()
	want := []string{"GET /pets", "GET /pets/{id}", "POST /pets"}
	if len(endpoints) != len(want) {
		t.Fatalf("want %d endpoints, got %d: %v", len(want), len(endpoints), endpoints)
	}
	for i, ep := range endpoints {
		if ep != want[i] {
			t.Errorf("endpoint[%d] = %q, want %q", i, ep, want[i])
		}
	}
}

func TestNewReplayServer_InvalidDir(t *testing.T) {
	_, err := NewReplayServer("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for invalid session dir")
	}
}

func TestReplayServer_QueryParamMatch(t *testing.T) {
	dir := makeTestSession(t, []Interaction{
		{
			Method:         "GET",
			Path:           "/pets",
			QueryParams:    map[string][]string{"species": {"cat"}},
			ResponseStatus: 200,
			ResponseBody:   []byte(`[{"name":"Whiskers"}]`),
		},
		{
			Method:         "GET",
			Path:           "/pets",
			QueryParams:    map[string][]string{"species": {"dog"}},
			ResponseStatus: 200,
			ResponseBody:   []byte(`[{"name":"Rex"}]`),
		},
	}, nil)

	srv, err := NewReplayServer(dir)
	if err != nil {
		t.Fatalf("NewReplayServer: %v", err)
	}

	req := httptest.NewRequest("GET", "/pets?species=dog", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if body := rec.Body.String(); body != `[{"name":"Rex"}]` {
		t.Errorf("query param match: want Rex body, got %q", body)
	}
}

func TestReplayServer_FallbackToFirst(t *testing.T) {
	dir := makeTestSession(t, []Interaction{
		{
			Method:         "GET",
			Path:           "/pets/42",
			ResponseStatus: 200,
			ResponseBody:   []byte(`{"id":42}`),
		},
		{
			Method:         "GET",
			Path:           "/pets/99",
			ResponseStatus: 200,
			ResponseBody:   []byte(`{"id":99}`),
		},
	}, []string{"/pets/{id}"})

	srv, err := NewReplayServer(dir)
	if err != nil {
		t.Fatalf("NewReplayServer: %v", err)
	}

	// /pets/77 matches neither exact path; should fall back to first recorded
	req := httptest.NewRequest("GET", "/pets/77", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("want 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != `{"id":42}` {
		t.Errorf("want first candidate body, got %q", body)
	}
}

func TestReplayServer_Handler(t *testing.T) {
	dir := makeTestSession(t, []Interaction{
		{
			Method:         "GET",
			Path:           "/ping",
			ResponseStatus: 200,
			ResponseBody:   []byte(`pong`),
		},
	}, nil)

	srv, err := NewReplayServer(dir)
	if err != nil {
		t.Fatalf("NewReplayServer: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/ping")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}
