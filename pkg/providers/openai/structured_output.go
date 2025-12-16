package openai

import (
	"encoding/json"
	"fmt"
)

// JSONSchemaType represents the type of a JSON schema property
type JSONSchemaType string

const (
	// JSONSchemaTypeObject represents an object type
	JSONSchemaTypeObject JSONSchemaType = "object"
	// JSONSchemaTypeArray represents an array type
	JSONSchemaTypeArray JSONSchemaType = "array"
	// JSONSchemaTypeString represents a string type
	JSONSchemaTypeString JSONSchemaType = "string"
	// JSONSchemaTypeNumber represents a number type
	JSONSchemaTypeNumber JSONSchemaType = "number"
	// JSONSchemaTypeInteger represents an integer type
	JSONSchemaTypeInteger JSONSchemaType = "integer"
	// JSONSchemaTypeBoolean represents a boolean type
	JSONSchemaTypeBoolean JSONSchemaType = "boolean"
	// JSONSchemaTypeNull represents a null type
	JSONSchemaTypeNull JSONSchemaType = "null"
)

// JSONSchema represents a JSON schema for structured outputs
type JSONSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Strict      bool                   `json:"strict"`
	Schema      map[string]interface{} `json:"schema"`
}

// ResponseFormatJSONSchema represents OpenAI's structured output format
type ResponseFormatJSONSchema struct {
	Type       string     `json:"type"`
	JSONSchema JSONSchema `json:"json_schema"`
}

// SchemaProperty represents a property in a JSON schema
type SchemaProperty struct {
	Type        JSONSchemaType         `json:"type,omitempty"`
	Description string                 `json:"description,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Items       map[string]interface{} `json:"items,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Default     interface{}            `json:"default,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty"`
	MinLength   *int                   `json:"minLength,omitempty"`
	MaxLength   *int                   `json:"maxLength,omitempty"`
	Pattern     string                 `json:"pattern,omitempty"`
	Format      string                 `json:"format,omitempty"`
}

// SchemaBuilder provides a fluent API for building JSON schemas
type SchemaBuilder struct {
	properties map[string]interface{}
	required   []string
	name       string
	desc       string
	strict     bool
}

// NewSchemaBuilder creates a new schema builder
func NewSchemaBuilder(name string) *SchemaBuilder {
	return &SchemaBuilder{
		name:       name,
		properties: make(map[string]interface{}),
		required:   make([]string, 0),
		strict:     true, // Default to strict mode for OpenAI
	}
}

// Description sets the schema description
func (b *SchemaBuilder) Description(desc string) *SchemaBuilder {
	b.desc = desc
	return b
}

// Strict sets the strict mode flag
func (b *SchemaBuilder) Strict(strict bool) *SchemaBuilder {
	b.strict = strict
	return b
}

// AddProperty adds a property to the schema
func (b *SchemaBuilder) AddProperty(name string, prop map[string]interface{}) *SchemaBuilder {
	b.properties[name] = prop
	return b
}

// AddStringProperty adds a string property
func (b *SchemaBuilder) AddStringProperty(name, description string, required bool) *SchemaBuilder {
	prop := map[string]interface{}{
		"type": "string",
	}
	if description != "" {
		prop["description"] = description
	}
	b.properties[name] = prop
	if required {
		b.required = append(b.required, name)
	}
	return b
}

// AddIntegerProperty adds an integer property
func (b *SchemaBuilder) AddIntegerProperty(name, description string, required bool) *SchemaBuilder {
	prop := map[string]interface{}{
		"type": "integer",
	}
	if description != "" {
		prop["description"] = description
	}
	b.properties[name] = prop
	if required {
		b.required = append(b.required, name)
	}
	return b
}

// AddNumberProperty adds a number property
func (b *SchemaBuilder) AddNumberProperty(name, description string, required bool) *SchemaBuilder {
	prop := map[string]interface{}{
		"type": "number",
	}
	if description != "" {
		prop["description"] = description
	}
	b.properties[name] = prop
	if required {
		b.required = append(b.required, name)
	}
	return b
}

// AddBooleanProperty adds a boolean property
func (b *SchemaBuilder) AddBooleanProperty(name, description string, required bool) *SchemaBuilder {
	prop := map[string]interface{}{
		"type": "boolean",
	}
	if description != "" {
		prop["description"] = description
	}
	b.properties[name] = prop
	if required {
		b.required = append(b.required, name)
	}
	return b
}

// AddObjectProperty adds an object property
func (b *SchemaBuilder) AddObjectProperty(name, description string, properties map[string]interface{}, requiredFields []string, required bool) *SchemaBuilder {
	prop := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if description != "" {
		prop["description"] = description
	}
	if len(requiredFields) > 0 {
		prop["required"] = requiredFields
	}
	b.properties[name] = prop
	if required {
		b.required = append(b.required, name)
	}
	return b
}

// AddArrayProperty adds an array property
func (b *SchemaBuilder) AddArrayProperty(name, description string, items map[string]interface{}, required bool) *SchemaBuilder {
	prop := map[string]interface{}{
		"type":  "array",
		"items": items,
	}
	if description != "" {
		prop["description"] = description
	}
	b.properties[name] = prop
	if required {
		b.required = append(b.required, name)
	}
	return b
}

// AddEnumProperty adds an enum property
func (b *SchemaBuilder) AddEnumProperty(name, description string, enumValues []interface{}, required bool) *SchemaBuilder {
	prop := map[string]interface{}{
		"type": "string",
		"enum": enumValues,
	}
	if description != "" {
		prop["description"] = description
	}
	b.properties[name] = prop
	if required {
		b.required = append(b.required, name)
	}
	return b
}

// AddRequiredField marks a field as required
func (b *SchemaBuilder) AddRequiredField(fieldName string) *SchemaBuilder {
	b.required = append(b.required, fieldName)
	return b
}

// Build builds the final JSON schema
func (b *SchemaBuilder) Build() JSONSchema {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": b.properties,
	}

	if len(b.required) > 0 {
		schema["required"] = b.required
	}

	// In strict mode, additionalProperties must be false
	if b.strict {
		schema["additionalProperties"] = false
	}

	return JSONSchema{
		Name:        b.name,
		Description: b.desc,
		Strict:      b.strict,
		Schema:      schema,
	}
}

// BuildJSON builds and returns the schema as a JSON string
func (b *SchemaBuilder) BuildJSON() (string, error) {
	schema := b.Build()
	data, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema: %w", err)
	}
	return string(data), nil
}

// BuildResponseFormat builds the complete response format for OpenAI
func (b *SchemaBuilder) BuildResponseFormat() (map[string]interface{}, error) {
	schema := b.Build()
	return map[string]interface{}{
		"type":        "json_schema",
		"json_schema": schema,
	}, nil
}

// ValidateJSONSchema validates a JSON schema structure
func ValidateJSONSchema(schema map[string]interface{}) error {
	// Check if it has required fields
	if _, hasType := schema["type"]; !hasType {
		return fmt.Errorf("schema must have a 'type' field")
	}

	schemaType, ok := schema["type"].(string)
	if !ok {
		return fmt.Errorf("schema 'type' must be a string")
	}

	// Validate based on type
	switch schemaType {
	case "object":
		return validateObjectSchema(schema)
	case "array":
		return validateArraySchema(schema)
	case "string", "number", "integer", "boolean", "null":
		// Basic types are valid
		return nil
	default:
		return fmt.Errorf("invalid schema type: %s", schemaType)
	}
}

// validateObjectSchema validates an object schema
func validateObjectSchema(schema map[string]interface{}) error {
	properties, hasProperties := schema["properties"]
	if !hasProperties {
		return fmt.Errorf("object schema must have 'properties' field")
	}

	propsMap, ok := properties.(map[string]interface{})
	if !ok {
		return fmt.Errorf("'properties' must be an object")
	}

	// Validate each property
	for name, prop := range propsMap {
		propMap, ok := prop.(map[string]interface{})
		if !ok {
			return fmt.Errorf("property '%s' must be an object", name)
		}

		if err := ValidateJSONSchema(propMap); err != nil {
			return fmt.Errorf("property '%s' is invalid: %w", name, err)
		}
	}

	// Validate required array if present
	if required, hasRequired := schema["required"]; hasRequired {
		// Try to handle both []interface{} and []string
		var requiredFields []string

		switch req := required.(type) {
		case []interface{}:
			requiredFields = make([]string, 0, len(req))
			for _, r := range req {
				if reqStr, ok := r.(string); ok {
					requiredFields = append(requiredFields, reqStr)
				} else {
					return fmt.Errorf("required field must be a string")
				}
			}
		case []string:
			requiredFields = req
		default:
			return fmt.Errorf("'required' must be an array")
		}

		// Check that all required fields exist in properties
		for _, reqStr := range requiredFields {
			if _, exists := propsMap[reqStr]; !exists {
				return fmt.Errorf("required field '%s' not found in properties", reqStr)
			}
		}
	}

	return nil
}

// validateArraySchema validates an array schema
func validateArraySchema(schema map[string]interface{}) error {
	items, hasItems := schema["items"]
	if !hasItems {
		return fmt.Errorf("array schema must have 'items' field")
	}

	itemsMap, ok := items.(map[string]interface{})
	if !ok {
		return fmt.Errorf("'items' must be an object")
	}

	return ValidateJSONSchema(itemsMap)
}

// ParseResponseFormat parses a response format string and validates it
func ParseResponseFormat(responseFormat string) (map[string]interface{}, error) {
	// Try to parse as JSON
	var schemaObj map[string]interface{}
	if err := json.Unmarshal([]byte(responseFormat), &schemaObj); err != nil {
		// Not JSON, assume it's a simple format like "json_object"
		return map[string]interface{}{
			"type": responseFormat,
		}, nil
	}

	// Check if it's already in OpenAI format with "type" and "json_schema"
	if formatType, hasType := schemaObj["type"]; hasType {
		if formatType == "json_schema" {
			// Already in correct format
			if jsonSchema, hasSchema := schemaObj["json_schema"]; hasSchema {
				// Validate the schema
				if schemaMap, ok := jsonSchema.(map[string]interface{}); ok {
					if innerSchema, hasInnerSchema := schemaMap["schema"]; hasInnerSchema {
						if innerSchemaMap, ok := innerSchema.(map[string]interface{}); ok {
							if err := ValidateJSONSchema(innerSchemaMap); err != nil {
								return nil, fmt.Errorf("invalid JSON schema: %w", err)
							}
						}
					}
				}
			}
			return schemaObj, nil
		}

		// Check if it's a simple format like "json_object" or a raw schema
		// Simple formats don't have properties field
		if _, hasProperties := schemaObj["properties"]; !hasProperties && formatType == "json_object" {
			// It's a simple format like "json_object"
			return schemaObj, nil
		}

		// Otherwise, it's a raw schema with type="object" and properties
		// Fall through to handle it as a raw schema
	}

	// It's a raw schema object, wrap it in OpenAI format
	// Check if it's a JSONSchema object with name, strict, schema
	if _, hasName := schemaObj["name"]; hasName {
		if schema, hasSchema := schemaObj["schema"]; hasSchema {
			if schemaMap, ok := schema.(map[string]interface{}); ok {
				// Validate the schema
				if err := ValidateJSONSchema(schemaMap); err != nil {
					return nil, fmt.Errorf("invalid JSON schema: %w", err)
				}
			}

			// It's a complete JSONSchema object
			return map[string]interface{}{
				"type":        "json_schema",
				"json_schema": schemaObj,
			}, nil
		}

		// It has a name but no schema field, might be malformed
		return nil, fmt.Errorf("schema has 'name' field but no 'schema' field")
	}

	// It's a raw schema without name, create a default one
	if err := ValidateJSONSchema(schemaObj); err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %w", err)
	}

	return map[string]interface{}{
		"type": "json_schema",
		"json_schema": map[string]interface{}{
			"name":   "response",
			"strict": true,
			"schema": schemaObj,
		},
	}, nil
}

// IsRefusal checks if a response contains a refusal
// OpenAI may refuse to generate structured output in certain cases
func IsRefusal(message string) bool {
	// OpenAI-specific refusal patterns
	refusalPatterns := []string{
		"I cannot",
		"I can't",
		"I'm unable to",
		"I am unable to",
		"I'm not able to",
		"I am not able to",
		"I apologize",
		"I'm sorry",
		"I am sorry",
	}

	for _, pattern := range refusalPatterns {
		if len(message) >= len(pattern) && message[:len(pattern)] == pattern {
			return true
		}
	}

	return false
}

// ValidateStrictSchema validates that a schema is compatible with strict mode
// In strict mode, OpenAI requires:
// - additionalProperties: false for all objects
// - All properties must be defined
// - No patternProperties or unevaluatedProperties
func ValidateStrictSchema(schema map[string]interface{}) error {
	schemaType, ok := schema["type"].(string)
	if !ok {
		return fmt.Errorf("schema must have a string 'type' field")
	}

	if schemaType == "object" {
		// Check additionalProperties
		if additionalProps, hasAdditional := schema["additionalProperties"]; hasAdditional {
			if additionalBool, ok := additionalProps.(bool); !ok || additionalBool {
				return fmt.Errorf("strict mode requires additionalProperties to be false")
			}
		}

		// Check for unsupported fields
		unsupportedFields := []string{"patternProperties", "unevaluatedProperties"}
		for _, field := range unsupportedFields {
			if _, has := schema[field]; has {
				return fmt.Errorf("strict mode does not support '%s'", field)
			}
		}

		// Recursively validate nested objects
		if properties, hasProps := schema["properties"].(map[string]interface{}); hasProps {
			for name, prop := range properties {
				if propMap, ok := prop.(map[string]interface{}); ok {
					if err := ValidateStrictSchema(propMap); err != nil {
						return fmt.Errorf("property '%s': %w", name, err)
					}
				}
			}
		}
	}

	if schemaType == "array" {
		if items, hasItems := schema["items"].(map[string]interface{}); hasItems {
			if err := ValidateStrictSchema(items); err != nil {
				return fmt.Errorf("array items: %w", err)
			}
		}
	}

	return nil
}

// EnsureStrictCompliance ensures a schema is compliant with strict mode
// Modifies the schema in-place to add required fields
func EnsureStrictCompliance(schema map[string]interface{}) {
	schemaType, ok := schema["type"].(string)
	if !ok {
		return
	}

	if schemaType == "object" {
		// Add additionalProperties: false
		schema["additionalProperties"] = false

		// Recursively process nested objects
		if properties, hasProps := schema["properties"].(map[string]interface{}); hasProps {
			for _, prop := range properties {
				if propMap, ok := prop.(map[string]interface{}); ok {
					EnsureStrictCompliance(propMap)
				}
			}
		}
	}

	if schemaType == "array" {
		if items, hasItems := schema["items"].(map[string]interface{}); hasItems {
			EnsureStrictCompliance(items)
		}
	}
}
