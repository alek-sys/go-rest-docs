# Go REST Docs - OpenAPI Generation from Tests

Generate OpenAPI 3.1 specs from test-recorded HTTP interactions using middleware-based capture.

## Context

- **Module**: `github.com/alek-sys/go-rest-docs`
- **Files involved**: see tasks below (all new files in a fresh project)
- **Related patterns**: Spring REST Docs (Java), but adapted for Go idioms
- **Dependencies**: `gopkg.in/yaml.v3` (YAML output), no OpenAPI-specific libraries - build spec structs directly

### Architecture overview

- Recording middleware wraps any `http.Handler` to capture request/response pairs during tests
- Each recorded interaction stores method, path, headers, query params, request body, response status, response body
- Schema inference derives JSON Schema from observed request/response bodies
- OpenAPI builder converts all recorded interactions into an OpenAPI 3.1 spec
- A global registry collects interactions across test functions; TestMain or a test helper writes the final YAML
- Users enable generation via a go test flag (e.g. `-gorestdocs.output=openapi.yaml`)

## Approach

- **Testing approach**: TDD
- Each task produces a focused, testable package or component
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Tasks

### Task 1: Project scaffolding and interaction model

**Files:**
- Create: `go.mod` (module `github.com/alek-sys/go-rest-docs`)
- Create: `recorder.go` (interaction types)
- Create: `recorder_test.go`

- [x] Initialize go module as `github.com/alek-sys/go-rest-docs`
- [x] Define `Interaction` struct: Method, Path, QueryParams, RequestHeaders, RequestBody, ResponseStatus, ResponseHeaders, ResponseBody
- [x] Define `Registry` (thread-safe collection of interactions, using `sync.Mutex`)
- [x] Write tests for Registry: add interactions, retrieve by path, concurrent access
- [x] Run `go test ./...` - must pass before task 2

### Task 2: Recording middleware

**Files:**
- Create: `middleware.go`
- Create: `middleware_test.go`

- [ ] Implement `Middleware(handler http.Handler, registry *Registry) http.Handler`
- [ ] Middleware captures full request (method, path, query, headers, body) and response (status, headers, body) using a ResponseRecorder wrapper
- [ ] Write tests: use `httptest.NewServer` with middleware, make requests, verify interactions are recorded accurately
- [ ] Test with various content types (JSON, form-encoded, empty body)
- [ ] Run `go test ./...` - must pass before task 3

### Task 3: JSON Schema inference

**Files:**
- Create: `schema.go`
- Create: `schema_test.go`

- [ ] Implement `InferSchema(data []byte)` that produces a JSON Schema map from a JSON body
- [ ] Support: string, number, integer, boolean, null, object (with properties), array (with item schema)
- [ ] Handle nested objects and arrays of objects
- [ ] When multiple interactions hit the same endpoint, merge schemas (union of properties, nullable if sometimes missing)
- [ ] Write tests: primitives, nested objects, arrays, merging of two schemas, empty body returns nil
- [ ] Run `go test ./...` - must pass before task 4

### Task 4: OpenAPI spec builder

**Files:**
- Create: `openapi.go`
- Create: `openapi_test.go`

- [ ] Define OpenAPI 3.1 structs: OpenAPI, Info, PathItem, Operation, Parameter, RequestBody, Response, MediaType, Schema
- [ ] Implement `BuildSpec(registry *Registry, info Info) OpenAPI` - groups interactions by path+method, infers parameters from path and query, builds request/response schemas
- [ ] Detect path parameters from URL patterns (e.g. `/users/123` vs `/users/456` -> `/users/{id}`) using a configurable path pattern or auto-detection
- [ ] Implement MarshalYAML output using `yaml.v3`
- [ ] Write tests: single endpoint, multiple endpoints, endpoint with path params, endpoint with query params, request and response bodies
- [ ] Run `go test ./...` - must pass before task 5

### Task 5: Test integration and flag-based generation

**Files:**
- Create: `gorestdocs.go` (public API and flag registration)
- Create: `gorestdocs_test.go`
- Create: `example_test.go` (usage example that also serves as integration test)

- [ ] Provide a package-level default registry and middleware helper: `gorestdocs.Handler(h http.Handler) http.Handler`
- [ ] Register go test flag: `-gorestdocs.output` (file path for YAML output, empty = disabled)
- [ ] Provide `GenerateSpec(w io.Writer, info Info)` that writes the YAML spec from the default registry
- [ ] Provide a TestMain helper or test cleanup hook that triggers generation when the flag is set
- [ ] Write integration test: start test server with middleware, make requests, call GenerateSpec, parse output YAML and verify structure
- [ ] Write `example_test.go` showing typical usage pattern
- [ ] Run `go test ./...` - must pass before task 6

### Task 6: Path pattern configuration and polish

**Files:**
- Modify: `openapi.go`
- Modify: `gorestdocs.go`
- Create: `patterns.go`
- Create: `patterns_test.go`

- [ ] Allow users to register path patterns (e.g. `/users/{id}`) so the builder can correctly parameterize paths instead of guessing
- [ ] Implement automatic path parameter detection as fallback (group similar paths, detect varying segments)
- [ ] Add option to set spec title, version, description via flags or API
- [ ] Write tests for pattern matching and auto-detection
- [ ] Run `go test ./...` - must pass

## Validation

- [ ] Manual test: create a sample REST API with 3-4 endpoints, write tests using the middleware, run with `-gorestdocs.output=api.yaml`, verify the YAML is a valid OpenAPI 3.1 spec
- [ ] Run full test suite: `go test ./...`
- [ ] Run linter: `golangci-lint run`
- [ ] Verify test coverage meets 80%+

## Post-completion

- [ ] Update README.md with usage instructions and example
- [ ] Move this plan to `docs/plans/completed/`
