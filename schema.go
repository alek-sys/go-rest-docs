package gorestdocs

import (
	"encoding/json"
	"sort"
)

// Schema represents a JSON Schema definition.
type Schema map[string]interface{}

// InferSchema produces a JSON Schema from a JSON body.
// Returns nil if data is empty or not valid JSON.
func InferSchema(data []byte) Schema {
	if len(data) == 0 {
		return nil
	}

	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil
	}

	return inferValue(v)
}

func inferValue(v interface{}) Schema {
	if v == nil {
		return Schema{"type": "null"}
	}

	switch val := v.(type) {
	case bool:
		return Schema{"type": "boolean"}
	case float64:
		if val == float64(int64(val)) {
			return Schema{"type": "integer"}
		}
		return Schema{"type": "number"}
	case string:
		return Schema{"type": "string"}
	case map[string]interface{}:
		return inferObject(val)
	case []interface{}:
		return inferArray(val)
	default:
		return Schema{"type": "string"}
	}
}

func inferObject(obj map[string]interface{}) Schema {
	properties := make(map[string]interface{})
	required := make([]string, 0)

	for k, v := range obj {
		properties[k] = inferValue(v)
		if v != nil {
			required = append(required, k)
		}
	}

	sort.Strings(required)

	s := Schema{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func inferArray(arr []interface{}) Schema {
	s := Schema{"type": "array"}

	if len(arr) == 0 {
		return s
	}

	// Infer item schema from the first element
	itemSchema := inferValue(arr[0])

	// Merge with remaining elements for a more complete schema
	for i := 1; i < len(arr); i++ {
		itemSchema = MergeSchemas(itemSchema, inferValue(arr[i]))
	}

	s["items"] = itemSchema
	return s
}

// primaryType extracts the main (non-"null") type string from a Schema,
// handling both string and []string/[]interface{} type values.
func primaryType(s Schema) string {
	switch t := s["type"].(type) {
	case string:
		return t
	case []string:
		for _, v := range t {
			if v != "null" {
				return v
			}
		}
	case []interface{}:
		for _, v := range t {
			if str, ok := v.(string); ok && str != "null" {
				return str
			}
		}
	}
	return ""
}

// MergeSchemas merges two schemas into one that accepts values valid under either.
// This is used when multiple interactions hit the same endpoint to create a union schema.
func MergeSchemas(a, b Schema) Schema {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	typeA := primaryType(a)
	typeB := primaryType(b)
	nullable := isNullable(a) || isNullable(b)

	// If one is null, make the other nullable
	if typeA == "null" && typeB != "null" {
		return makeNullable(b)
	}
	if typeB == "null" && typeA != "null" {
		return makeNullable(a)
	}

	// Integer and number are compatible: number is the wider type
	if (typeA == "integer" && typeB == "number") || (typeA == "number" && typeB == "integer") {
		result := Schema{"type": "number"}
		if nullable {
			result["type"] = []string{"number", "null"}
		}
		return result
	}

	// Different non-null types: can't merge meaningfully, return a
	if typeA != typeB {
		return a
	}

	// Same type: merge details
	switch typeA {
	case "object":
		merged := mergeObjects(a, b)
		if nullable && !isNullable(merged) {
			return makeNullable(merged)
		}
		return merged
	case "array":
		merged := mergeArrays(a, b)
		if nullable && !isNullable(merged) {
			return makeNullable(merged)
		}
		return merged
	default:
		// Primitive same type - preserve nullable if either is nullable
		result := Schema{"type": typeA}
		if nullable {
			result["type"] = []string{typeA, "null"}
		}
		return result
	}
}

func mergeObjects(a, b Schema) Schema {
	propsA := getProperties(a)
	propsB := getProperties(b)
	reqA := getRequired(a)
	reqB := getRequired(b)

	merged := make(map[string]interface{})

	// All keys from both
	allKeys := make(map[string]bool)
	for k := range propsA {
		allKeys[k] = true
	}
	for k := range propsB {
		allKeys[k] = true
	}

	for k := range allKeys {
		sa, inA := propsA[k]
		sb, inB := propsB[k]

		switch {
		case inA && inB:
			schemaA, _ := sa.(Schema)
			schemaB, _ := sb.(Schema)
			if schemaA == nil {
				if m, ok := sa.(map[string]interface{}); ok {
					schemaA = Schema(m)
				}
			}
			if schemaB == nil {
				if m, ok := sb.(map[string]interface{}); ok {
					schemaB = Schema(m)
				}
			}
			merged[k] = MergeSchemas(schemaA, schemaB)
		case inA:
			// Only in A, make nullable since B doesn't have it
			schemaA, _ := sa.(Schema)
			if schemaA == nil {
				if m, ok := sa.(map[string]interface{}); ok {
					schemaA = Schema(m)
				}
			}
			merged[k] = makeNullable(schemaA)
		case inB:
			// Only in B, make nullable since A doesn't have it
			schemaB, _ := sb.(Schema)
			if schemaB == nil {
				if m, ok := sb.(map[string]interface{}); ok {
					schemaB = Schema(m)
				}
			}
			merged[k] = makeNullable(schemaB)
		}
	}

	// Required = intersection of required fields
	required := intersect(reqA, reqB)
	sort.Strings(required)

	result := Schema{
		"type":       "object",
		"properties": merged,
	}
	if isNullable(a) || isNullable(b) {
		result["type"] = []string{"object", "null"}
	}
	if len(required) > 0 {
		result["required"] = required
	}
	return result
}

func mergeArrays(a, b Schema) Schema {
	result := Schema{"type": "array"}
	if isNullable(a) || isNullable(b) {
		result["type"] = []string{"array", "null"}
	}

	itemsA := getItems(a)
	itemsB := getItems(b)

	if itemsA != nil && itemsB != nil {
		result["items"] = MergeSchemas(itemsA, itemsB)
	} else if itemsA != nil {
		result["items"] = itemsA
	} else if itemsB != nil {
		result["items"] = itemsB
	}

	return result
}

func makeNullable(s Schema) Schema {
	if s == nil {
		return Schema{"type": "null"}
	}
	if isNullable(s) {
		return s
	}
	result := make(Schema)
	for k, v := range s {
		result[k] = v
	}
	// OpenAPI 3.1 uses JSON Schema 2020-12 type arrays for nullable
	currentType := primaryType(s)
	if currentType != "" {
		result["type"] = []string{currentType, "null"}
	} else {
		result["type"] = []string{"null"}
	}
	return result
}

func isNullable(s Schema) bool {
	switch t := s["type"].(type) {
	case []string:
		for _, v := range t {
			if v == "null" {
				return true
			}
		}
	case []interface{}:
		for _, v := range t {
			if v == "null" {
				return true
			}
		}
	}
	return false
}

func getProperties(s Schema) map[string]interface{} {
	v, ok := s["properties"]
	if !ok {
		return nil
	}
	m, ok := v.(map[string]interface{})
	if ok {
		return m
	}
	return nil
}

func getRequired(s Schema) []string {
	v, ok := s["required"]
	if !ok {
		return nil
	}
	switch r := v.(type) {
	case []string:
		return r
	case []interface{}:
		result := make([]string, 0, len(r))
		for _, item := range r {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func getItems(s Schema) Schema {
	v, ok := s["items"]
	if !ok {
		return nil
	}
	switch items := v.(type) {
	case Schema:
		return items
	case map[string]interface{}:
		return Schema(items)
	}
	return nil
}

func intersect(a, b []string) []string {
	set := make(map[string]bool)
	for _, s := range a {
		set[s] = true
	}
	var result []string
	for _, s := range b {
		if set[s] {
			result = append(result, s)
		}
	}
	return result
}
