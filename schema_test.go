package gorestdocs

import (
	"testing"
)

func TestInferSchema_EmptyBody(t *testing.T) {
	s := InferSchema([]byte{})
	if s != nil {
		t.Errorf("expected nil for empty body, got %v", s)
	}
}

func TestInferSchema_InvalidJSON(t *testing.T) {
	s := InferSchema([]byte("not json"))
	if s != nil {
		t.Errorf("expected nil for invalid JSON, got %v", s)
	}
}

func TestInferSchema_String(t *testing.T) {
	s := InferSchema([]byte(`"hello"`))
	assertType(t, s, "string")
}

func TestInferSchema_Integer(t *testing.T) {
	s := InferSchema([]byte(`42`))
	assertType(t, s, "integer")
}

func TestInferSchema_Number(t *testing.T) {
	s := InferSchema([]byte(`3.14`))
	assertType(t, s, "number")
}

func TestInferSchema_Boolean(t *testing.T) {
	s := InferSchema([]byte(`true`))
	assertType(t, s, "boolean")
}

func TestInferSchema_Null(t *testing.T) {
	s := InferSchema([]byte(`null`))
	assertType(t, s, "null")
}

func TestInferSchema_SimpleObject(t *testing.T) {
	s := InferSchema([]byte(`{"name": "alice", "age": 30}`))
	assertType(t, s, "object")

	props := s["properties"].(map[string]interface{})
	nameSchema := props["name"].(Schema)
	assertType(t, nameSchema, "string")

	ageSchema := props["age"].(Schema)
	assertType(t, ageSchema, "integer")

	required := toStringSlice(s["required"])
	if !containsAll(required, "age", "name") {
		t.Errorf("expected required to contain age and name, got %v", required)
	}
}

func TestInferSchema_NestedObject(t *testing.T) {
	s := InferSchema([]byte(`{"user": {"name": "alice", "address": {"city": "NYC"}}}`))
	assertType(t, s, "object")

	props := s["properties"].(map[string]interface{})
	userSchema := props["user"].(Schema)
	assertType(t, userSchema, "object")

	userProps := userSchema["properties"].(map[string]interface{})
	addrSchema := userProps["address"].(Schema)
	assertType(t, addrSchema, "object")

	addrProps := addrSchema["properties"].(map[string]interface{})
	citySchema := addrProps["city"].(Schema)
	assertType(t, citySchema, "string")
}

func TestInferSchema_Array(t *testing.T) {
	s := InferSchema([]byte(`[1, 2, 3]`))
	assertType(t, s, "array")

	items := s["items"].(Schema)
	assertType(t, items, "integer")
}

func TestInferSchema_EmptyArray(t *testing.T) {
	s := InferSchema([]byte(`[]`))
	assertType(t, s, "array")

	if _, ok := s["items"]; ok {
		t.Error("expected no items for empty array")
	}
}

func TestInferSchema_ArrayOfObjects(t *testing.T) {
	s := InferSchema([]byte(`[{"name": "alice"}, {"name": "bob", "age": 30}]`))
	assertType(t, s, "array")

	items := s["items"].(Schema)
	assertType(t, items, "object")

	props := items["properties"].(map[string]interface{})

	nameSchema := toSchema(props["name"])
	assertType(t, nameSchema, "string")

	// age is only in second element, should be nullable
	ageSchema := toSchema(props["age"])
	assertType(t, ageSchema, "integer")
	if !isNullable(ageSchema) {
		t.Error("expected age to be nullable since it's missing from first array element")
	}
}

func TestMergeSchemas_NilInputs(t *testing.T) {
	a := Schema{"type": "string"}

	if result := MergeSchemas(nil, a); result["type"] != "string" {
		t.Error("merge(nil, a) should return a")
	}
	if result := MergeSchemas(a, nil); result["type"] != "string" {
		t.Error("merge(a, nil) should return a")
	}
}

func TestMergeSchemas_WithNull(t *testing.T) {
	a := Schema{"type": "string"}
	b := Schema{"type": "null"}

	result := MergeSchemas(a, b)
	assertType(t, result, "string")
	if !isNullable(result) {
		t.Error("expected nullable when merging with null")
	}

	// Reverse order
	result2 := MergeSchemas(b, a)
	assertType(t, result2, "string")
	if !isNullable(result2) {
		t.Error("expected nullable when merging null with string")
	}
}

func TestMergeSchemas_Objects(t *testing.T) {
	a := InferSchema([]byte(`{"name": "alice", "age": 30}`))
	b := InferSchema([]byte(`{"name": "bob", "email": "bob@example.com"}`))

	merged := MergeSchemas(a, b)
	assertType(t, merged, "object")

	props := merged["properties"].(map[string]interface{})

	// name is in both - should remain as-is
	nameSchema := toSchema(props["name"])
	assertType(t, nameSchema, "string")

	// age is only in a - should be nullable
	ageSchema := toSchema(props["age"])
	assertType(t, ageSchema, "integer")
	if !isNullable(ageSchema) {
		t.Error("expected age to be nullable (only in first schema)")
	}

	// email is only in b - should be nullable
	emailSchema := toSchema(props["email"])
	assertType(t, emailSchema, "string")
	if !isNullable(emailSchema) {
		t.Error("expected email to be nullable (only in second schema)")
	}

	// required should be intersection: only "name"
	required := toStringSlice(merged["required"])
	if !containsAll(required, "name") || containsAny(required, "age", "email") {
		t.Errorf("expected required to be [name], got %v", required)
	}
}

func TestMergeSchemas_Arrays(t *testing.T) {
	a := InferSchema([]byte(`[{"x": 1}]`))
	b := InferSchema([]byte(`[{"x": 2, "y": "hello"}]`))

	merged := MergeSchemas(a, b)
	assertType(t, merged, "array")

	items := getItems(merged)
	assertType(t, items, "object")

	props := items["properties"].(map[string]interface{})
	ySchema := toSchema(props["y"])
	assertType(t, ySchema, "string")
	if !isNullable(ySchema) {
		t.Error("expected y to be nullable")
	}
}

// Helpers

func assertType(t *testing.T, s Schema, expected string) {
	t.Helper()
	if s == nil {
		t.Fatalf("schema is nil, expected type %s", expected)
	}
	got, _ := s["type"].(string)
	if got != expected {
		t.Errorf("expected type %q, got %q (schema: %v)", expected, got, s)
	}
}

func toSchema(v interface{}) Schema {
	switch s := v.(type) {
	case Schema:
		return s
	case map[string]interface{}:
		return Schema(s)
	}
	return nil
}

func toStringSlice(v interface{}) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []interface{}:
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

func containsAll(slice []string, items ...string) bool {
	set := make(map[string]bool)
	for _, s := range slice {
		set[s] = true
	}
	for _, item := range items {
		if !set[item] {
			return false
		}
	}
	return true
}

func containsAny(slice []string, items ...string) bool {
	set := make(map[string]bool)
	for _, s := range slice {
		set[s] = true
	}
	for _, item := range items {
		if set[item] {
			return true
		}
	}
	return false
}
