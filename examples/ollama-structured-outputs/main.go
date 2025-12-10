package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/ollama"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Example demonstrating Ollama structured outputs using JSON schema
func main() {
	// Create Ollama provider
	config := types.ProviderConfig{
		Type:         types.ProviderTypeOllama,
		Name:         "ollama-local",
		BaseURL:      "http://localhost:11434",
		DefaultModel: "llama3.1:8b",
	}

	provider := ollama.NewOllamaProvider(config)

	// Example 1: Basic JSON mode
	fmt.Println("=== Example 1: Basic JSON Mode ===")
	basicJSONExample(provider)

	fmt.Println("\n=== Example 2: JSON Schema with Person Object ===")
	personSchemaExample(provider)

	fmt.Println("\n=== Example 3: Complex Nested Schema ===")
	complexSchemaExample(provider)
}

// basicJSONExample demonstrates basic JSON mode
func basicJSONExample(provider *ollama.OllamaProvider) {
	ctx := context.Background()

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "Return a JSON object with the capital of France and its population",
			},
		},
		Model:          "llama3.1:8b",
		ResponseFormat: "json", // Basic JSON mode
		Temperature:    0,      // Use 0 for more deterministic output
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		log.Fatalf("Error generating completion: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var result string
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading stream: %v", err)
			return
		}
		result += chunk.Content
	}

	fmt.Printf("Response: %s\n", result)

	// Validate JSON
	var jsonObj map[string]interface{}
	if err := json.Unmarshal([]byte(result), &jsonObj); err != nil {
		log.Printf("Warning: Response is not valid JSON: %v", err)
	} else {
		fmt.Println("✓ Response is valid JSON")
	}
}

// personSchemaExample demonstrates structured output with a person schema
func personSchemaExample(provider *ollama.OllamaProvider) {
	ctx := context.Background()

	// Define JSON schema for a person
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The person's full name",
			},
			"age": map[string]interface{}{
				"type":        "number",
				"description": "The person's age in years",
			},
			"email": map[string]interface{}{
				"type":        "string",
				"description": "The person's email address",
			},
			"occupation": map[string]interface{}{
				"type":        "string",
				"description": "The person's occupation",
			},
		},
		"required": []string{"name", "age", "email"},
	}

	// Convert schema to JSON string
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		log.Fatalf("Error marshaling schema: %v", err)
	}

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "Generate information about a fictional software engineer named Alice who is 28 years old",
			},
		},
		Model:          "llama3.1:8b",
		ResponseFormat: string(schemaJSON),
		Temperature:    0,
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		log.Fatalf("Error generating completion: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var result string
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading stream: %v", err)
			return
		}
		result += chunk.Content
	}

	fmt.Printf("Response: %s\n", result)

	// Parse and validate against schema
	var person map[string]interface{}
	if err := json.Unmarshal([]byte(result), &person); err != nil {
		log.Printf("Warning: Response is not valid JSON: %v", err)
		return
	}

	fmt.Println("✓ Response is valid JSON")

	// Check required fields
	requiredFields := []string{"name", "age", "email"}
	for _, field := range requiredFields {
		if _, ok := person[field]; ok {
			fmt.Printf("✓ Required field '%s': %v\n", field, person[field])
		} else {
			fmt.Printf("✗ Missing required field '%s'\n", field)
		}
	}
}

// complexSchemaExample demonstrates a complex nested schema
func complexSchemaExample(provider *ollama.OllamaProvider) {
	ctx := context.Background()

	// Define complex nested schema
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"company": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
					"founded": map[string]interface{}{
						"type": "number",
					},
				},
				"required": []string{"name", "founded"},
			},
			"employees": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type": "string",
						},
						"role": map[string]interface{}{
							"type": "string",
						},
					},
					"required": []string{"name", "role"},
				},
			},
			"revenue": map[string]interface{}{
				"type": "number",
			},
		},
		"required": []string{"company", "employees"},
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		log.Fatalf("Error marshaling schema: %v", err)
	}

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "Generate information about a fictional tech startup with 3 employees",
			},
		},
		Model:          "llama3.1:8b",
		ResponseFormat: string(schemaJSON),
		Temperature:    0,
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		log.Fatalf("Error generating completion: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var result string
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading stream: %v", err)
			return
		}
		result += chunk.Content
	}

	fmt.Printf("Response: %s\n", result)

	// Parse and validate
	var company map[string]interface{}
	if err := json.Unmarshal([]byte(result), &company); err != nil {
		log.Printf("Warning: Response is not valid JSON: %v", err)
		return
	}

	fmt.Println("✓ Response is valid JSON")

	// Verify company object
	if companyInfo, ok := company["company"].(map[string]interface{}); ok {
		fmt.Printf("✓ Company: %s (Founded: %.0f)\n", companyInfo["name"], companyInfo["founded"])
	} else {
		fmt.Println("✗ Missing or invalid company object")
	}

	// Verify employees array
	if employees, ok := company["employees"].([]interface{}); ok {
		fmt.Printf("✓ Employees (%d):\n", len(employees))
		for i, emp := range employees {
			if empMap, ok := emp.(map[string]interface{}); ok {
				fmt.Printf("  %d. %s - %s\n", i+1, empMap["name"], empMap["role"])
			}
		}
	} else {
		fmt.Println("✗ Missing or invalid employees array")
	}
}
