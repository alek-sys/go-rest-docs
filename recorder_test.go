package gorestdocs

import (
	"net/http"
	"sync"
	"testing"
)

func TestRegistryRecord(t *testing.T) {
	r := NewRegistry()

	i := Interaction{
		Method:         "GET",
		Path:           "/users",
		ResponseStatus: 200,
	}
	r.Record(i)

	all := r.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 interaction, got %d", len(all))
	}
	if all[0].Method != "GET" {
		t.Errorf("expected method GET, got %s", all[0].Method)
	}
	if all[0].Path != "/users" {
		t.Errorf("expected path /users, got %s", all[0].Path)
	}
}

func TestRegistryByPath(t *testing.T) {
	r := NewRegistry()

	r.Record(Interaction{Method: "GET", Path: "/users", ResponseStatus: 200})
	r.Record(Interaction{Method: "POST", Path: "/users", ResponseStatus: 201})
	r.Record(Interaction{Method: "GET", Path: "/posts", ResponseStatus: 200})

	users := r.ByPath("/users")
	if len(users) != 2 {
		t.Fatalf("expected 2 interactions for /users, got %d", len(users))
	}

	posts := r.ByPath("/posts")
	if len(posts) != 1 {
		t.Fatalf("expected 1 interaction for /posts, got %d", len(posts))
	}

	none := r.ByPath("/nonexistent")
	if len(none) != 0 {
		t.Fatalf("expected 0 interactions for /nonexistent, got %d", len(none))
	}
}

func TestRegistryReset(t *testing.T) {
	r := NewRegistry()
	r.Record(Interaction{Method: "GET", Path: "/users", ResponseStatus: 200})
	r.Reset()

	all := r.All()
	if len(all) != 0 {
		t.Fatalf("expected 0 interactions after reset, got %d", len(all))
	}
}

func TestRegistryAllReturnsCopy(t *testing.T) {
	r := NewRegistry()
	r.Record(Interaction{Method: "GET", Path: "/users", ResponseStatus: 200})

	all := r.All()
	all[0].Method = "POST"

	original := r.All()
	if original[0].Method != "GET" {
		t.Error("All() should return a copy, not a reference to internal data")
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Record(Interaction{
				Method:         "GET",
				Path:           "/concurrent",
				ResponseStatus: 200,
			})
		}(i)
	}

	wg.Wait()

	all := r.All()
	if len(all) != 100 {
		t.Fatalf("expected 100 interactions, got %d", len(all))
	}
}

func TestInteractionFields(t *testing.T) {
	i := Interaction{
		Method:      "POST",
		Path:        "/users",
		QueryParams: map[string][]string{"page": {"1"}, "limit": {"10"}},
		RequestHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		RequestBody:    []byte(`{"name":"Alice"}`),
		ResponseStatus: 201,
		ResponseHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		ResponseBody: []byte(`{"id":1,"name":"Alice"}`),
	}

	if i.Method != "POST" {
		t.Errorf("expected POST, got %s", i.Method)
	}
	if len(i.QueryParams) != 2 {
		t.Errorf("expected 2 query params, got %d", len(i.QueryParams))
	}
	if i.RequestHeaders.Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type header")
	}
	if string(i.RequestBody) != `{"name":"Alice"}` {
		t.Error("unexpected request body")
	}
	if i.ResponseStatus != 201 {
		t.Errorf("expected status 201, got %d", i.ResponseStatus)
	}
	if string(i.ResponseBody) != `{"id":1,"name":"Alice"}` {
		t.Error("unexpected response body")
	}
}
