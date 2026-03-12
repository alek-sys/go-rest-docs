package gorestdocs

import (
	"sort"
	"testing"
)

func TestPathPatterns_Register(t *testing.T) {
	pp := NewPathPatterns()
	pp.Register("/users/{id}")
	pp.Register("/users/{userId}/posts/{postId}")

	all := pp.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(all))
	}
	if all[0] != "/users/{id}" {
		t.Errorf("expected /users/{id}, got %s", all[0])
	}
	if all[1] != "/users/{userId}/posts/{postId}" {
		t.Errorf("expected /users/{userId}/posts/{postId}, got %s", all[1])
	}
}

func TestPathPatterns_RegisterMultiple(t *testing.T) {
	pp := NewPathPatterns()
	pp.Register("/users/{id}", "/posts/{id}")

	all := pp.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(all))
	}
}

func TestPathPatterns_Reset(t *testing.T) {
	pp := NewPathPatterns()
	pp.Register("/users/{id}")
	pp.Reset()

	all := pp.All()
	if len(all) != 0 {
		t.Fatalf("expected 0 patterns after reset, got %d", len(all))
	}
}

func TestPathPatterns_Match(t *testing.T) {
	pp := NewPathPatterns()
	pp.Register("/users/{id}", "/users/{userId}/posts/{postId}")

	tests := []struct {
		path      string
		wantMatch string
		wantOK    bool
	}{
		{"/users/123", "/users/{id}", true},
		{"/users/abc", "/users/{id}", true},
		{"/users/42/posts/7", "/users/{userId}/posts/{postId}", true},
		{"/users", "", false},
		{"/posts/1", "", false},
		{"/users/1/comments/2", "", false},
	}

	for _, tt := range tests {
		match, ok := pp.Match(tt.path)
		if ok != tt.wantOK {
			t.Errorf("Match(%q): got ok=%v, want %v", tt.path, ok, tt.wantOK)
		}
		if match != tt.wantMatch {
			t.Errorf("Match(%q): got match=%q, want %q", tt.path, match, tt.wantMatch)
		}
	}
}

func TestAutoDetectPatterns_SinglePath(t *testing.T) {
	result := AutoDetectPatterns([]string{"/users"})
	if len(result) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(result))
	}
	if _, ok := result["/users"]; !ok {
		t.Error("expected /users pattern")
	}
}

func TestAutoDetectPatterns_NumericIDs(t *testing.T) {
	result := AutoDetectPatterns([]string{"/users/123", "/users/456"})

	// Should detect /users/{param0}
	var patterns []string
	for p := range result {
		patterns = append(patterns, p)
	}

	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d: %v", len(patterns), patterns)
	}
	if patterns[0] != "/users/{param0}" {
		t.Errorf("expected /users/{param0}, got %s", patterns[0])
	}
	if len(result[patterns[0]]) != 2 {
		t.Errorf("expected 2 paths in group, got %d", len(result[patterns[0]]))
	}
}

func TestAutoDetectPatterns_DifferentResources(t *testing.T) {
	result := AutoDetectPatterns([]string{"/users/123", "/posts/456"})

	// These have different static prefixes, should NOT merge
	if len(result) != 2 {
		t.Fatalf("expected 2 patterns (different resources), got %d: %v", len(result), keys(result))
	}
}

func TestAutoDetectPatterns_NestedPaths(t *testing.T) {
	result := AutoDetectPatterns([]string{
		"/users/1/posts/10",
		"/users/2/posts/20",
	})

	var patterns []string
	for p := range result {
		patterns = append(patterns, p)
	}

	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d: %v", len(patterns), patterns)
	}
	if patterns[0] != "/users/{param0}/posts/{param1}" {
		t.Errorf("expected /users/{param0}/posts/{param1}, got %s", patterns[0])
	}
}

func TestBuildSpec_WithRegisteredPatterns(t *testing.T) {
	reg := NewRegistry()
	reg.Record(Interaction{
		Method:         "GET",
		Path:           "/users/123",
		ResponseStatus: 200,
		ResponseBody:   []byte(`{"id": 123, "name": "alice"}`),
	})
	reg.Record(Interaction{
		Method:         "GET",
		Path:           "/users/456",
		ResponseStatus: 200,
		ResponseBody:   []byte(`{"id": 456, "name": "bob"}`),
	})

	pp := NewPathPatterns()
	pp.Register("/users/{id}")

	spec := BuildSpec(reg, Info{Title: "Test", Version: "1.0"}, WithPatterns(pp))

	// Should use the registered pattern, not auto-detected {param0}
	pathItem, ok := spec.Paths["/users/{id}"]
	if !ok {
		t.Fatalf("expected /users/{id} path, got paths: %v", specPaths(spec))
	}
	if pathItem.Get == nil {
		t.Fatal("expected GET operation")
	}

	// Should have path parameter named "id"
	foundID := false
	for _, p := range pathItem.Get.Parameters {
		if p.In == "path" && p.Name == "id" {
			foundID = true
			if !p.Required {
				t.Error("path parameter 'id' should be required")
			}
		}
	}
	if !foundID {
		t.Error("expected path parameter named 'id'")
	}
}

func TestBuildSpec_PatternsWithFallback(t *testing.T) {
	reg := NewRegistry()
	// This one matches a registered pattern
	reg.Record(Interaction{
		Method:         "GET",
		Path:           "/users/123",
		ResponseStatus: 200,
		ResponseBody:   []byte(`{"id": 123}`),
	})
	// This one does NOT match any registered pattern - should use auto-detection
	reg.Record(Interaction{
		Method:         "GET",
		Path:           "/posts",
		ResponseStatus: 200,
		ResponseBody:   []byte(`[]`),
	})

	pp := NewPathPatterns()
	pp.Register("/users/{id}")

	spec := BuildSpec(reg, Info{Title: "Test", Version: "1.0"}, WithPatterns(pp))

	if _, ok := spec.Paths["/users/{id}"]; !ok {
		t.Errorf("expected /users/{id}, got paths: %v", specPaths(spec))
	}
	if _, ok := spec.Paths["/posts"]; !ok {
		t.Errorf("expected /posts, got paths: %v", specPaths(spec))
	}
}

func TestLooksStatic(t *testing.T) {
	tests := []struct {
		segment string
		want    bool
	}{
		{"users", true},
		{"posts", true},
		{"api", true},
		{"123", false},
		{"456", false},
		{"550e8400-e29b-41d4-a716-446655440000", false},
		{"v1", true},
	}

	for _, tt := range tests {
		got := looksStatic(tt.segment)
		if got != tt.want {
			t.Errorf("looksStatic(%q) = %v, want %v", tt.segment, got, tt.want)
		}
	}
}

func TestDefaultPatternsIntegration(t *testing.T) {
	ResetDefaultRegistry()
	ResetDefaultPatterns()

	RegisterPatterns("/items/{itemId}")

	patterns := DefaultPatterns().All()
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	if patterns[0] != "/items/{itemId}" {
		t.Errorf("expected /items/{itemId}, got %s", patterns[0])
	}

	// Clean up
	ResetDefaultPatterns()
}

// helpers

func keys(m map[string][]string) []string {
	var result []string
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}
