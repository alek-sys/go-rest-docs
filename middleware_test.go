package gorestdocs

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareRecordsGETRequest(t *testing.T) {
	registry := NewRegistry()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"users":["alice","bob"]}`))
	})

	srv := httptest.NewServer(Middleware(handler, registry))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/users?page=1&limit=10")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// Verify response was passed through correctly
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if string(body) != `{"users":["alice","bob"]}` {
		t.Errorf("unexpected response body: %s", body)
	}

	// Verify interaction was recorded
	all := registry.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 interaction, got %d", len(all))
	}

	i := all[0]
	if i.Method != "GET" {
		t.Errorf("expected method GET, got %s", i.Method)
	}
	if i.Path != "/users" {
		t.Errorf("expected path /users, got %s", i.Path)
	}
	if v := i.QueryParams["page"]; len(v) == 0 || v[0] != "1" {
		t.Errorf("expected query param page=1, got %v", v)
	}
	if v := i.QueryParams["limit"]; len(v) == 0 || v[0] != "10" {
		t.Errorf("expected query param limit=10, got %v", v)
	}
	if i.ResponseStatus != 200 {
		t.Errorf("expected response status 200, got %d", i.ResponseStatus)
	}
	if string(i.ResponseBody) != `{"users":["alice","bob"]}` {
		t.Errorf("unexpected recorded response body: %s", i.ResponseBody)
	}
	if len(i.RequestBody) != 0 {
		t.Errorf("expected empty request body for GET, got %s", i.RequestBody)
	}
}

func TestMiddlewareRecordsJSONPostRequest(t *testing.T) {
	registry := NewRegistry()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(body)
	})

	srv := httptest.NewServer(Middleware(handler, registry))
	defer srv.Close()

	reqBody := `{"name":"Alice","email":"alice@example.com"}`
	resp, err := http.Post(srv.URL+"/users", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	all := registry.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 interaction, got %d", len(all))
	}

	i := all[0]
	if i.Method != "POST" {
		t.Errorf("expected method POST, got %s", i.Method)
	}
	if i.Path != "/users" {
		t.Errorf("expected path /users, got %s", i.Path)
	}
	if string(i.RequestBody) != reqBody {
		t.Errorf("expected request body %s, got %s", reqBody, i.RequestBody)
	}
	if i.RequestHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", i.RequestHeaders.Get("Content-Type"))
	}
	if i.ResponseStatus != 201 {
		t.Errorf("expected response status 201, got %d", i.ResponseStatus)
	}
}

func TestMiddlewareRecordsFormEncodedRequest(t *testing.T) {
	registry := NewRegistry()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	srv := httptest.NewServer(Middleware(handler, registry))
	defer srv.Close()

	formData := "username=alice&password=secret"
	resp, err := http.Post(srv.URL+"/login", "application/x-www-form-urlencoded", strings.NewReader(formData))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	all := registry.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 interaction, got %d", len(all))
	}

	i := all[0]
	if string(i.RequestBody) != formData {
		t.Errorf("expected request body %s, got %s", formData, i.RequestBody)
	}
	if i.RequestHeaders.Get("Content-Type") != "application/x-www-form-urlencoded" {
		t.Errorf("expected form content type, got %s", i.RequestHeaders.Get("Content-Type"))
	}
}

func TestMiddlewareRecordsEmptyBodyRequest(t *testing.T) {
	registry := NewRegistry()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(Middleware(handler, registry))
	defer srv.Close()

	req, _ := http.NewRequest("DELETE", srv.URL+"/users/1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", resp.StatusCode)
	}

	all := registry.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 interaction, got %d", len(all))
	}

	i := all[0]
	if i.Method != "DELETE" {
		t.Errorf("expected method DELETE, got %s", i.Method)
	}
	if i.ResponseStatus != 204 {
		t.Errorf("expected response status 204, got %d", i.ResponseStatus)
	}
	if len(i.RequestBody) != 0 {
		t.Errorf("expected empty request body, got %s", i.RequestBody)
	}
	if len(i.ResponseBody) != 0 {
		t.Errorf("expected empty response body, got %s", i.ResponseBody)
	}
}

func TestMiddlewareMultipleRequests(t *testing.T) {
	registry := NewRegistry()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	srv := httptest.NewServer(Middleware(handler, registry))
	defer srv.Close()

	http.Get(srv.URL + "/a")
	http.Get(srv.URL + "/b")
	http.Get(srv.URL + "/c")

	all := registry.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 interactions, got %d", len(all))
	}
	if all[0].Path != "/a" || all[1].Path != "/b" || all[2].Path != "/c" {
		t.Error("interactions not recorded in order")
	}
}

func TestMiddlewarePreservesHandlerBehavior(t *testing.T) {
	registry := NewRegistry()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handler reads body and uses it
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Echo", string(body))
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("handled"))
	})

	srv := httptest.NewServer(Middleware(handler, registry))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/echo", "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify the handler could read the body (middleware restored it)
	if resp.Header.Get("X-Echo") != "hello" {
		t.Errorf("handler did not receive body, X-Echo=%s", resp.Header.Get("X-Echo"))
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "handled" {
		t.Errorf("unexpected response: %s", body)
	}
}
