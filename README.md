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
    "bytes"
    "net/http"
    "net/http/httptest"
    "os"
    "strings"
    "testing"

    gorestdocs "github.com/alek-sys/go-rest-docs"
)

func TestListPets(t *testing.T) {
    mux := http.NewServeMux()
    mux.HandleFunc("/pets", handlePets)

    // Wrap your handler with the recording middleware
    srv := httptest.NewServer(gorestdocs.Handler(mux))
    defer srv.Close()

    // Every request is recorded automatically
    resp, err := http.Get(srv.URL + "/pets")
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()
    // ... assertions
}

func TestMain(m *testing.M) {
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

## How it works

1. The `Handler` middleware wraps your `http.Handler` and records every request/response pair during tests.
2. JSON Schema is automatically inferred from request and response bodies.
3. When multiple requests hit the same endpoint, schemas are merged (union of properties).
4. After tests complete, `WriteSpecIfEnabled` (or `GenerateSpec`) builds an OpenAPI 3.1 spec and writes it as YAML.

## Path parameters

### Explicit patterns (recommended)

Register path patterns so the builder correctly parameterizes URLs:

```go
gorestdocs.RegisterPatterns("/users/{id}", "/users/{userId}/posts/{postId}")
```

### Auto-detection

If no patterns are registered, the builder automatically detects path parameters by grouping similar paths and identifying segments that vary (e.g., `/users/123` and `/users/456` become `/users/{id}`).

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
