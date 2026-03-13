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
	gorestdocs.ResetDefaultDocs()
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

	gorestdocs.Document(t, resp,
		gorestdocs.Summary("List all pets"),
		gorestdocs.ResponseFields(
			gorestdocs.Field("[].id", "string", "Unique pet identifier"),
			gorestdocs.Field("[].name", "string", "Pet's display name"),
			gorestdocs.Field("[].species", "string", "Animal species"),
		),
	)
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

	gorestdocs.Document(t, resp,
		gorestdocs.Summary("Create a pet"),
		gorestdocs.RequestFields(
			gorestdocs.Field("name", "string", "The pet's name"),
			gorestdocs.Field("species", "string", "The animal species"),
		),
		gorestdocs.ResponseFields(
			gorestdocs.Field("id", "string", "The assigned pet ID"),
			gorestdocs.Field("name", "string", "The pet's name"),
			gorestdocs.Field("species", "string", "The animal species"),
		),
	)
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

	gorestdocs.Document(t, resp,
		gorestdocs.Summary("Get a pet by ID"),
		gorestdocs.PathParams(gorestdocs.Param("id", "The unique pet identifier")),
		gorestdocs.ResponseFields(
			gorestdocs.Field("id", "string", "Unique pet identifier"),
			gorestdocs.Field("name", "string", "Pet's display name"),
			gorestdocs.Field("species", "string", "Animal species"),
		),
	)
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

	gorestdocs.Document(t, resp,
		gorestdocs.Summary("Search pets"),
		gorestdocs.QueryParams(gorestdocs.Param("q", "Search query string")),
		gorestdocs.ResponseFields(
			gorestdocs.Field("query", "string", "The search query that was executed"),
			gorestdocs.Field("results", "array", "Matching pets"),
		),
	)
}
