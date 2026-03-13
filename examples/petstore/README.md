# Pet Store Example

This example is a small standalone project for trying out OpenAPI generation with `go-rest-docs`.

## Generate a spec

From this directory, run:

```bash
go test ./... -gorestdocs.output=openapi.yaml
```

That will execute the tests, record the HTTP interactions, and write an OpenAPI 3.1 document to `openapi.yaml`.

## What the example covers

- `GET /pets`
- `POST /pets`
- `GET /pets/{id}`
- `GET /search?q=...`

## Files

- `api.go` contains the sample HTTP API.
- `api_test.go` exercises the API through `gorestdocs.Handler(...)`.
- `TestMain` calls `gorestdocs.WriteSpecIfEnabled(...)` so the spec is written when you pass `-gorestdocs.output`.
