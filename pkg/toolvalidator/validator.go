// Package toolvalidator provides validation utilities for tool definitions and tool calls.
// It includes functionality to validate tool schemas, arguments, and ensure compliance
// with expected formats and constraints.
package toolvalidator

import (
	"encoding/json"
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Validator validates tool definitions and tool calls
type Validator struct {
	strictMode bool
}

// New creates a new Validator
// strictMode: if true, extra fields in tool call arguments are rejected
func New(strictMode bool) *Validator {
	return &Validator{strictMode: strictMode}
}

// ValidateToolDefinition validates a tool definition
func (v *Validator) ValidateToolDefinition(tool types.Tool) error {
	// Check required fields
	if tool.Name == "" {
		return fmt.Errorf("tool name is required")
	}
	if tool.Description == "" {
		return fmt.Errorf("tool description is required")
	}

	// Validate JSON schema
	if tool.InputSchema == nil {
		return fmt.Errorf("tool input schema is required")
	}

	// Ensure schema is valid JSON
	_, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return fmt.Errorf("invalid input schema: %w", err)
	}

	// Validate that schema has a type
	if schemaType, ok := tool.InputSchema["type"]; !ok || schemaType == "" {
		return fmt.Errorf("input schema must have a type field")
	}

	return nil
}

// ValidateToolCall validates a tool call against its definition
func (v *Validator) ValidateToolCall(tool types.Tool, call types.ToolCall) error {
	// Check tool name matches
	if call.Function.Name != tool.Name {
		return fmt.Errorf("tool call name %s doesn't match tool %s", call.Function.Name, tool.Name)
	}

	// Check that ID is present
	if call.ID == "" {
		return fmt.Errorf("tool call ID is required")
	}

	// Check that type is present
	if call.Type == "" {
		return fmt.Errorf("tool call type is required")
	}

	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return fmt.Errorf("invalid tool call arguments JSON: %w", err)
	}

	// Validate against schema
	if err := v.validateAgainstSchema(args, tool.InputSchema); err != nil {
		return fmt.Errorf("arguments don't match schema: %w", err)
	}

	return nil
}

// validateAgainstSchema validates data against a JSON schema
func (v *Validator) validateAgainstSchema(data map[string]interface{}, schema map[string]interface{}) error {
	if err := v.validateRequiredFields(data, schema); err != nil {
		return err
	}

	return v.validateProperties(data, schema)
}

// validateRequiredFields checks that all required fields are present
func (v *Validator) validateRequiredFields(data map[string]interface{}, schema map[string]interface{}) error {
	if required, ok := schema["required"].([]interface{}); ok {
		for _, req := range required {
			reqField, ok := req.(string)
			if !ok {
				continue
			}
			if _, exists := data[reqField]; !exists {
				return fmt.Errorf("required field %s is missing", reqField)
			}
		}
	}
	return nil
}

// validateProperties validates all properties against their schemas
func (v *Validator) validateProperties(data map[string]interface{}, schema map[string]interface{}) error {
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return nil
	}

	if err := v.checkUnexpectedFields(data, properties); err != nil {
		return err
	}

	return v.validateFieldProperties(data, properties)
}

// checkUnexpectedFields validates that no unexpected fields are present in strict mode
func (v *Validator) checkUnexpectedFields(data map[string]interface{}, properties map[string]interface{}) error {
	if !v.strictMode {
		return nil
	}

	for field := range data {
		if _, exists := properties[field]; !exists {
			return fmt.Errorf("unexpected field %s (strict mode)", field)
		}
	}
	return nil
}

// validateFieldProperties validates each field against its property schema
func (v *Validator) validateFieldProperties(data map[string]interface{}, properties map[string]interface{}) error {
	for field, value := range data {
		propSchema, exists := properties[field]
		if !exists {
			if v.strictMode {
				return fmt.Errorf("unexpected field %s (strict mode)", field)
			}
			continue
		}

		if err := v.validateFieldSchema(field, value, propSchema); err != nil {
			return err
		}
	}
	return nil
}

// validateFieldSchema validates a single field against its schema
func (v *Validator) validateFieldSchema(field string, value interface{}, propSchema interface{}) error {
	propSchemaMap, ok := propSchema.(map[string]interface{})
	if !ok {
		return nil
	}

	if propType, ok := propSchemaMap["type"].(string); ok {
		if err := v.validateType(field, value, propType); err != nil {
			return err
		}
	}

	if enum, ok := propSchemaMap["enum"].([]interface{}); ok {
		if err := v.validateEnum(field, value, enum); err != nil {
			return err
		}
	}

	return nil
}

// validateType validates that a value matches the expected JSON schema type
func (v *Validator) validateType(field string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field %s must be a string", field)
		}
	case "number":
		switch value.(type) {
		case float64, float32, int, int32, int64:
			// Valid number
		default:
			return fmt.Errorf("field %s must be a number", field)
		}
	case "integer":
		switch v := value.(type) {
		case int, int32, int64:
			// Valid integer
		case float64:
			// Check if it's a whole number
			if v != float64(int64(v)) {
				return fmt.Errorf("field %s must be an integer", field)
			}
		default:
			return fmt.Errorf("field %s must be an integer", field)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field %s must be a boolean", field)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("field %s must be an array", field)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("field %s must be an object", field)
		}
	}
	return nil
}

// validateEnum validates that a value is in the allowed enum values
func (v *Validator) validateEnum(field string, value interface{}, enum []interface{}) error {
	for _, enumValue := range enum {
		if value == enumValue {
			return nil
		}
	}
	return fmt.Errorf("field %s value %v is not in allowed values %v", field, value, enum)
}
