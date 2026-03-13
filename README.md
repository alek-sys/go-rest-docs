# go-rest-docs

Generate OpenAPI 3.1 specs from your Go tests. Inspired by [Spring REST Docs](https://spring.io/projects/spring-restdocs).

Instead of writing OpenAPI specs by hand, `go-rest-docs` records HTTP interactions during test execution and produces a spec that reflects your actual API behavior.

## Install

```bash
go get github.com/alek-sys/go-rest-docs
```

## Quick start

```go
package api_test

import (
    "flag"
    "net/http"
    "net/http/httptest"
    "os"
    "strings"
    "testing"

    gorestdocs "github.com/alek-sys/go-rest-docs"
)

func TestListPets(t *testing.T) {
    srv := httptest.NewServer(gorestdocs.Handler(mux))
    defer srv.Close()

    resp, err := http.Get(srv.URL + "/pets")
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()

    // Document validates the response AND generates OpenAPI descriptions.
    // The test fails if:
    //  - the response contains a field not listed here (undocumented field)
    //  - a listed field is missing from the response (stale docs)
    //  - a field's type doesn't match (e.g. "string" vs actual integer)
    gorestdocs.Document(t, resp,
        gorestdocs.Summary("List all pets"),
        gorestdocs.ResponseFields(
            gorestdocs.Field("[].id", "string", "Unique pet identifier"),
            gorestdocs.Field("[].name", "string", "Pet's display name"),
            gorestdocs.Field("[].species", "string", "Animal species"), // remove this and the test fails
        ),
    )
}

func TestCreatePet(t *testing.T) {
    srv := httptest.NewServer(gorestdocs.Handler(mux))
    defer srv.Close()

    resp, err := http.Post(srv.URL+"/pets", "application/json",
        strings.NewReader(`{"name":"Whiskers","species":"cat"}`))
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()

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

func TestMain(m *testing.M) {
    flag.Parse()
    code := m.Run()

    // Write the spec after all tests complete
    gorestdocs.WriteSpecIfEnabled(gorestdocs.Info{
        Title:   "Pet Store API",
        Version: "1.0.0",
    })

    os.Exit(code)
}
```

Run your tests with the output flag:

```bash
go test ./... -gorestdocs.output=openapi.yaml
```

This produces a valid OpenAPI 3.1 YAML file from the recorded interactions.

There is also a runnable sample project in [`examples/petstore`](./examples/petstore) that you can use to try the generation flow end to end.

## How it works

1. The `Handler` middleware wraps your `http.Handler` and records every request/response pair during tests.
2. JSON Schema is automatically inferred from request and response bodies.
3. When multiple requests hit the same endpoint, schemas are merged (union of properties).
4. After tests complete, `WriteSpecIfEnabled` (or `GenerateSpec`) builds an OpenAPI 3.1 spec and writes it as YAML.

## Documenting and validating fields

Use `Document` in your tests to add descriptions to fields, parameters, and operations. Like [Spring REST Docs](https://spring.io/projects/spring-restdocs), this doubles as validation — your test will **fail** if:

- The response contains a field that isn't documented (undocumented field)
- A documented field is missing from the response (phantom documentation)
- A field's actual type doesn't match the documented type (type mismatch)

```go
func TestCreatePet(t *testing.T) {
    srv := httptest.NewServer(gorestdocs.Handler(mux))
    defer srv.Close()

    resp, err := http.Post(srv.URL+"/pets", "application/json",
        strings.NewReader(`{"name":"Whiskers","species":"cat"}`))
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()

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
```

If the API adds a new field to the response without updating the test, or removes a documented field, the test fails. This keeps your documentation in sync with your actual API.

### Nested fields and arrays

Use dot-notation for nested objects and `[]` for array items:

```go
gorestdocs.ResponseFields(
    gorestdocs.Field("address", "object", "Shipping address"),
    gorestdocs.Field("address.city", "string", "City name"),
    gorestdocs.Field("address.zip", "string", "ZIP code"),
    gorestdocs.Field("items", "array", "Ordered items"),
    gorestdocs.Field("items[].productId", "string", "Product ID"),
    gorestdocs.Field("items[].quantity", "integer", "Number of units"),
)
```

For top-level array responses (e.g., `GET /pets` returning `[...]`):

```go
gorestdocs.ResponseFields(
    gorestdocs.Field("[].id", "string", "Pet ID"),
    gorestdocs.Field("[].name", "string", "Pet name"),
)
```

### Parameters

Document path and query parameters:

```go
gorestdocs.Document(t, resp,
    gorestdocs.Summary("Get a pet by ID"),
    gorestdocs.PathParams(gorestdocs.Param("id", "The unique pet identifier")),
    gorestdocs.QueryParams(gorestdocs.Param("fields", "Comma-separated list of fields to include")),
    gorestdocs.ResponseFields(
        // ...
    ),
)
```

### How it fits with auto-generation

`Document` enriches the auto-generated spec — it adds descriptions without changing the schema structure. Endpoints without a `Document` call still appear in the spec, just without descriptions. The schema types, required fields, and nullable handling always come from the actual recorded interactions.

## Path parameters

### Explicit patterns (recommended)

Register path patterns so the builder correctly parameterizes URLs:

```go
gorestdocs.RegisterPatterns("/users/{id}", "/users/{userId}/posts/{postId}")
```

### Auto-detection

If no patterns are registered, the builder automatically detects path parameters by grouping similar paths and identifying segments that vary (e.g., `/users/123` and `/users/456` become `/users/{param0}`).

## Programmatic spec generation

You can generate the spec directly without flags:

```go
var buf bytes.Buffer
err := gorestdocs.GenerateSpec(&buf, gorestdocs.Info{
    Title:       "My API",
    Version:     "2.0.0",
    Description: "My REST API",
})
```

Or build the spec object for further manipulation:

```go
registry := gorestdocs.DefaultRegistry()
spec := gorestdocs.BuildSpec(registry, gorestdocs.Info{
    Title:   "My API",
    Version: "1.0.0",
}, gorestdocs.WithPatterns(gorestdocs.DefaultPatterns()))

yamlBytes, err := gorestdocs.MarshalYAML(spec)
```

## Custom registry

For isolated test suites or parallel test groups, use a dedicated registry:

```go
registry := gorestdocs.NewRegistry()
handler := gorestdocs.Middleware(mux, registry)
```

## CLI flags

These flags are available when running `go test`:

| Flag | Description |
|------|-------------|
| `-gorestdocs.output` | File path for the generated YAML spec (empty = disabled) |
| `-gorestdocs.title` | Override the API title |
| `-gorestdocs.version` | Override the API version |
| `-gorestdocs.description` | Override the API description |

## License

MIT
