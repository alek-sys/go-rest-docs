# Session Recording and Replay Mock Server

Add the ability to save HTTP interaction recordings during test runs and replay them as a standalone mock server via a CLI binary.

## Context

- The middleware already captures full request/response Interactions in the Registry
- Interactions contain method, path, query params, headers, request body, response status, response headers, response body
- Path pattern detection already exists (both explicit and auto-detect)
- No CLI binary exists today; the library is test-only

## Files

- Modify: `recorder.go` - export any needed fields for serialization
- Modify: `gorestdocs.go` - add `-gorestdocs.session` flag to enable recording to disk
- Create: `session.go` - session storage format, read/write logic
- Create: `replay.go` - replay HTTP server that matches requests to recorded interactions
- Create: `cmd/gorestdocs/main.go` - CLI binary entry point with `replay` command
- Create: `session_test.go` - tests for session read/write
- Create: `replay_test.go` - tests for replay server
- Related patterns: Interaction, Registry, groupByPattern, matchPattern from existing code
- Dependencies: none (standard library only)

## Approach

- Session format: a directory containing a `session.json` manifest (with metadata like title, version, recorded-at, registered patterns) and individual interaction files as JSON (one per recorded call, named by index like `001_GET_pets.json`)
- Recording: controlled by `-gorestdocs.session=<dir>` flag. When set, `WriteSessionIfEnabled()` dumps all interactions from the default registry to the session directory
- Replay server: reads a session directory, uses the existing pattern matching logic to group interactions by endpoint pattern, and serves matching responses. For multiple recordings on the same endpoint pattern + method, cycles through them or picks the best match based on query params / request body similarity
- CLI: `gorestdocs replay --session ./sessions --port 8080` starts the mock server
- Request matching: method + path pattern match (reusing existing `patterns.go` logic). If multiple interactions match, prefer exact path match, then match by query parameters
- **Testing approach**: Regular (code first, then tests)
- Build on existing types and patterns (Interaction, Registry, pattern matching)
- Keep the session format simple and human-readable (JSON files in a folder)
- The replay server is a standard net/http server, no external dependencies
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Tasks

### Task 1: Session storage format and serialization

**Files:**
- Create: `session.go`
- Modify: `recorder.go` (export any needed fields)

- [x] Define session manifest struct (title, version, recorded timestamp, patterns, interaction count)
- [x] Implement WriteSession(dir string, registry *Registry, patterns []string) to write manifest + interaction files
- [x] Implement ReadSession(dir string) to load a session back into memory
- [x] Interaction JSON files include: method, path, query, request headers, request body, status, response headers, response body
- [x] Write tests for round-trip serialization in `session_test.go`
- [x] Run `go test ./...` - must pass before task 2

### Task 2: Flag-based session recording during tests

**Files:**
- Modify: `gorestdocs.go`

- [x] Add `-gorestdocs.session` flag (string, directory path)
- [x] Add `WriteSessionIfEnabled()` function that checks the flag and calls WriteSession
- [x] Update examples/petstore TestMain to also call WriteSessionIfEnabled
- [x] Write test verifying session output when flag is set
- [x] Run `go test ./...` - must pass before task 3

### Task 3: Replay HTTP server

**Files:**
- Create: `replay.go`

- [x] Implement replay server that loads a session directory
- [x] Match incoming requests using method + path pattern matching (reuse groupByPattern / matchPattern logic from patterns.go)
- [x] When multiple interactions match same pattern, prefer exact path match, then first available
- [x] Return recorded status code, headers, and body
- [x] Return 404 with helpful message for unmatched requests (list available endpoints)
- [x] Write tests using httptest for the replay server in `replay_test.go`
- [x] Run `go test ./...` - must pass before task 4

### Task 4: CLI binary

**Files:**
- Create: `cmd/gorestdocs/main.go`

- [x] Implement `gorestdocs replay --session <dir> --port <port>` command
- [x] Default port 8080, session dir required
- [x] Print startup banner showing loaded endpoints and port
- [x] Handle graceful shutdown on SIGINT/SIGTERM
- [x] Test that binary builds: `go build ./cmd/gorestdocs`
- [x] Run `go test ./...` - must pass before task 5

### Task 5: Integration test with petstore example

**Files:**
- Modify: `examples/petstore/api_test.go`

- [x] Add TestMain session recording call
- [x] Manual test: run petstore tests with session flag, then replay and curl the endpoints
- [x] Verify recorded session can be replayed and returns correct responses

## Validation

- [ ] Manual test: record petstore sessions via `go test ./examples/petstore -gorestdocs.session=./tmp-session`
- [ ] Manual test: replay via `go run ./cmd/gorestdocs replay --session ./tmp-session --port 9090` and curl endpoints
- [ ] Run full test suite: `go test ./...`
- [ ] Run linter: `golangci-lint run`
- [ ] Verify test coverage meets 80%+

## Completion

- [ ] Update README.md with session recording and replay documentation
- [ ] Move this plan to `docs/plans/completed/`
