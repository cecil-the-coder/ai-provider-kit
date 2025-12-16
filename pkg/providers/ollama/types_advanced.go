package ollama

import (
	"encoding/json"
	"fmt"
)

// PropertyType represents a JSON Schema type field that can be either a string or an array of strings.
// This handles the polymorphic nature of JSON Schema's "type" field which can be:
// - A single type: "string"
// - Multiple types: ["string", "null"]
//
// This is necessary for compatibility with tools that generate JSON schemas according to
// the JSON Schema specification (draft-07), which allows both formats.
// Reference: https://github.com/ollama/ollama/issues/10328
type PropertyType struct {
	// Single holds the type when it's a single string value
	Single string
	// Multiple holds the types when it's an array of strings
	Multiple []string
	// IsArray indicates whether the type was provided as an array
	IsArray bool
}

// UnmarshalJSON implements custom JSON unmarshaling for PropertyType.
// It handles both string and array formats:
// - "string" -> PropertyType{Single: "string", IsArray: false}
// - ["string", "null"] -> PropertyType{Multiple: ["string", "null"], IsArray: true}
func (pt *PropertyType) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a string first
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		pt.Single = single
		pt.IsArray = false
		pt.Multiple = nil
		return nil
	}

	// Try to unmarshal as an array of strings
	var multiple []string
	if err := json.Unmarshal(data, &multiple); err == nil {
		pt.Multiple = multiple
		pt.IsArray = true
		pt.Single = ""
		return nil
	}

	return fmt.Errorf("type must be either a string or an array of strings")
}

// MarshalJSON implements custom JSON marshaling for PropertyType.
// It outputs the format based on how the type was originally provided:
// - If IsArray is false: outputs as a string
// - If IsArray is true: outputs as an array of strings
func (pt PropertyType) MarshalJSON() ([]byte, error) {
	if pt.IsArray {
		return json.Marshal(pt.Multiple)
	}
	return json.Marshal(pt.Single)
}

// String returns the primary type as a string.
// For array types, it returns the first non-null type.
func (pt PropertyType) String() string {
	if pt.IsArray {
		// Return the first non-null type
		for _, t := range pt.Multiple {
			if t != "null" {
				return t
			}
		}
		// If all types are null, return the first one
		if len(pt.Multiple) > 0 {
			return pt.Multiple[0]
		}
		return ""
	}
	return pt.Single
}

// IsNullable returns true if the type includes "null" as one of the options.
func (pt PropertyType) IsNullable() bool {
	if !pt.IsArray {
		return pt.Single == "null"
	}
	for _, t := range pt.Multiple {
		if t == "null" {
			return true
		}
	}
	return false
}

// ToOllamaFormat converts the PropertyType to a format that Ollama expects.
// Since Ollama only accepts string types, this returns the primary type as a string.
func (pt PropertyType) ToOllamaFormat() string {
	return pt.String()
}

// NewPropertyType creates a PropertyType from a single string type.
func NewPropertyType(typeStr string) PropertyType {
	return PropertyType{
		Single:  typeStr,
		IsArray: false,
	}
}

// NewPropertyTypeArray creates a PropertyType from an array of types.
func NewPropertyTypeArray(types []string) PropertyType {
	return PropertyType{
		Multiple: types,
		IsArray:  true,
	}
}

// NormalizeJSONSchema normalizes a JSON Schema to ensure Ollama compatibility.
// This function walks through the schema and converts any array-typed "type" fields
// to their string equivalents, which is what Ollama expects.
//
// For example:
//   - {"type": ["string", "null"]} becomes {"type": "string"}
//   - {"type": "string"} remains unchanged
//
// This handles the polymorphic nature of JSON Schema's "type" field while maintaining
// compatibility with Ollama's more restrictive implementation.
func NormalizeJSONSchema(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return nil
	}

	// Create a deep copy to avoid modifying the original
	normalized := deepCopyMap(schema)

	// Recursively normalize the schema
	normalizeSchemaRecursive(normalized)

	return normalized
}

// normalizeSchemaRecursive recursively normalizes type fields in a JSON Schema.
func normalizeSchemaRecursive(obj interface{}) {
	switch v := obj.(type) {
	case map[string]interface{}:
		// Check if this object has a "type" field
		if typeField, exists := v["type"]; exists {
			// Try to normalize the type field
			v["type"] = normalizeTypeField(typeField)
		}

		// Recursively process all nested objects
		for key, value := range v {
			// Skip the type field as we already processed it
			if key != "type" {
				normalizeSchemaRecursive(value)
			}
		}

	case []interface{}:
		// Recursively process array elements
		for _, item := range v {
			normalizeSchemaRecursive(item)
		}
	}
}

// normalizeTypeField converts a type field to Ollama-compatible format.
// If the type is an array, it returns the first non-null type as a string.
// If the type is already a string, it returns it unchanged.
func normalizeTypeField(typeField interface{}) interface{} {
	switch t := typeField.(type) {
	case []interface{}:
		// Convert array to []string
		types := make([]string, 0, len(t))
		for _, item := range t {
			if str, ok := item.(string); ok {
				types = append(types, str)
			}
		}

		// Return the first non-null type
		for _, typeStr := range types {
			if typeStr != "null" {
				return typeStr
			}
		}

		// If all types are null or array is empty, return the first one or empty string
		if len(types) > 0 {
			return types[0]
		}
		return ""

	case []string:
		// Return the first non-null type
		for _, typeStr := range t {
			if typeStr != "null" {
				return typeStr
			}
		}

		// If all types are null or array is empty, return the first one or empty string
		if len(t) > 0 {
			return t[0]
		}
		return ""

	case string:
		// Already a string, return as-is
		return t

	default:
		// Unknown type, return as-is
		return typeField
	}
}

// deepCopyMap creates a deep copy of a map[string]interface{}.
func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}

	copy := make(map[string]interface{}, len(m))
	for k, v := range m {
		copy[k] = deepCopyValue(v)
	}
	return copy
}

// deepCopyValue creates a deep copy of an interface{} value.
func deepCopyValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return deepCopyMap(val)
	case []interface{}:
		copy := make([]interface{}, len(val))
		for i, item := range val {
			copy[i] = deepCopyValue(item)
		}
		return copy
	case []string:
		result := make([]string, len(val))
		copy(result, val)
		return result
	default:
		// For primitive types, return as-is (they're copied by value)
		return v
	}
}
