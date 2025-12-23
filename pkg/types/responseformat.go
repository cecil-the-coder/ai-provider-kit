// Package types provides shared type definitions and utilities for the AI provider kit.
package types

import (
	"encoding/json"
)

// ResponseFormatParseResult represents the result of parsing a ResponseFormat value.
type ResponseFormatParseResult struct {
	// IsSchema indicates whether the ResponseFormat was a valid JSON schema object.
	IsSchema bool

	// Schema contains the parsed JSON schema object (only populated if IsSchema is true).
	Schema map[string]interface{}

	// StringValue contains the raw string value (only populated if IsSchema is false).
	StringValue string
}

// ParseResponseFormatSchema attempts to parse a ResponseFormat value as JSON schema.
//
// The ResponseFormat can be:
//   - A JSON schema object (e.g., {"type":"object","properties":{...}})
//   - A string mode (e.g., "json", "json_object")
//
// This utility function is shared across all providers to ensure consistent
// parsing behavior for structured outputs.
//
// Parameters:
//   - responseFormat: The ResponseFormat string to parse
//
// Returns:
//   - ResponseFormatParseResult: A struct containing the parse result with either
//     the parsed schema object or the raw string value
func ParseResponseFormatSchema(responseFormat string) ResponseFormatParseResult {
	result := ResponseFormatParseResult{
		StringValue: responseFormat,
	}

	if responseFormat == "" {
		return result
	}

	// Try to parse as JSON schema first
	var schemaObj map[string]interface{}
	if err := json.Unmarshal([]byte(responseFormat), &schemaObj); err == nil {
		// Only treat as schema if it's a non-nil map (null JSON becomes nil here)
		if schemaObj != nil {
			result.IsSchema = true
			result.Schema = schemaObj
			// Clear StringValue since we have a schema
			result.StringValue = ""
		}
		// If schemaObj is nil (JSON null), fall through to string handling
	}

	return result
}

// IsJSONSchema is a convenience function that checks if a ResponseFormat value
// is a valid JSON schema object.
func IsJSONSchema(responseFormat string) bool {
	result := ParseResponseFormatSchema(responseFormat)
	return result.IsSchema
}

// GetJSONSchema is a convenience function that returns the parsed JSON schema
// from a ResponseFormat value, or nil if it's not a valid schema.
func GetJSONSchema(responseFormat string) map[string]interface{} {
	result := ParseResponseFormatSchema(responseFormat)
	if result.IsSchema {
		return result.Schema
	}
	return nil
}
