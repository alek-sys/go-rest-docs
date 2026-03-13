package petstore_test

import (
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	gorestdocs "github.com/alek-sys/go-rest-docs"
	"github.com/alek-sys/go-rest-docs/examples/petstore"
)

func TestMain(m *testing.M) {
	flag.Parse()
	gorestdocs.ResetDefaultRegistry()
	gorestdocs.ResetDefaultPatterns()
	gorestdocs.RegisterPatterns("/pets/{id}")

	code := m.Run()

	if err := gorestdocs.WriteSpecIfEnabled(gorestdocs.Info{
		Title:       "Pet Store Example API",
		Version:     "1.0.0",
		Description: "Sample API used to test go-rest-docs OpenAPI generation.",
	}); err != nil {
		panic(err)
	}

	os.Exit(code)
}

func TestListPets(t *testing.T) {
	srv := httptest.NewServer(gorestdocs.Handler(petstore.NewHandler()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/pets")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCreatePet(t *testing.T) {
	srv := httptest.NewServer(gorestdocs.Handler(petstore.NewHandler()))
	defer srv.Close()

	resp, err := http.Post(
		srv.URL+"/pets",
		"application/json",
		strings.NewReader(`{"name":"Whiskers","species":"cat"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestGetPetByID(t *testing.T) {
	srv := httptest.NewServer(gorestdocs.Handler(petstore.NewHandler()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/pets/42")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSearchPets(t *testing.T) {
	srv := httptest.NewServer(gorestdocs.Handler(petstore.NewHandler()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/search?q=cat")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
