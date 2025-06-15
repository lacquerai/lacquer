package runtime

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SchemaGenerator handles JSON schema generation for output definitions
type SchemaGenerator struct{}

// NewSchemaGenerator creates a new schema generator instance
func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{}
}

// GenerateJSONSchema generates a JSON schema from step output definitions
func (sg *SchemaGenerator) GenerateJSONSchema(outputs map[string]interface{}) (string, error) {
	if outputs == nil || len(outputs) == 0 {
		return "", nil
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
		"required":   make([]string, 0),
	}

	properties := schema["properties"].(map[string]interface{})
	var required []string

	// Sort keys for consistent output
	keys := make([]string, 0, len(outputs))
	for key := range outputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		outputDef := outputs[key]
		propSchema := sg.generatePropertySchema(outputDef)
		properties[key] = propSchema

		// Check if field is required (default to required unless explicitly optional)
		if sg.isRequired(outputDef) {
			required = append(required, key)
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	} else {
		delete(schema, "required")
	}

	// Convert to JSON string
	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema: %w", err)
	}

	return string(schemaBytes), nil
}

// generatePropertySchema generates schema for a single property
func (sg *SchemaGenerator) generatePropertySchema(outputDef interface{}) map[string]interface{} {
	switch def := outputDef.(type) {
	case string:
		return sg.generateSchemaFromType(def)
	case map[string]interface{}:
		return sg.generateSchemaFromObject(def)
	default:
		// Default to string if type is unclear
		return map[string]interface{}{"type": "string"}
	}
}

// generateSchemaFromType generates schema from a simple type string
func (sg *SchemaGenerator) generateSchemaFromType(typeStr string) map[string]interface{} {
	switch strings.ToLower(typeStr) {
	case "string":
		return map[string]interface{}{
			"type": "string",
		}
	case "integer", "int":
		return map[string]interface{}{
			"type": "integer",
		}
	case "float", "number":
		return map[string]interface{}{
			"type": "number",
		}
	case "boolean", "bool":
		return map[string]interface{}{
			"type": "boolean",
		}
	case "array":
		return map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "string", // Default array item type
			},
		}
	case "object":
		return map[string]interface{}{
			"type": "object",
		}
	default:
		return map[string]interface{}{
			"type": "string",
		}
	}
}

// generateSchemaFromObject generates schema from a complex object definition
func (sg *SchemaGenerator) generateSchemaFromObject(def map[string]interface{}) map[string]interface{} {
	schema := make(map[string]interface{})

	// Extract type
	if typeVal, ok := def["type"]; ok {
		if typeStr, ok := typeVal.(string); ok {
			baseSchema := sg.generateSchemaFromType(typeStr)
			// Copy base schema
			for k, v := range baseSchema {
				schema[k] = v
			}
		}
	} else {
		schema["type"] = "string" // default
	}

	// Add description if provided
	if desc, ok := def["description"].(string); ok && desc != "" {
		schema["description"] = desc
	}

	// Handle array item types
	if schema["type"] == "array" {
		if itemType, ok := def["items"]; ok {
			if itemTypeStr, ok := itemType.(string); ok {
				schema["items"] = sg.generateSchemaFromType(itemTypeStr)
			} else if itemTypeDef, ok := itemType.(map[string]interface{}); ok {
				schema["items"] = sg.generateSchemaFromObject(itemTypeDef)
			}
		}
	}

	// Handle object properties
	if schema["type"] == "object" {
		if props, ok := def["properties"].(map[string]interface{}); ok {
			objProperties := make(map[string]interface{})
			for propKey, propDef := range props {
				objProperties[propKey] = sg.generatePropertySchema(propDef)
			}
			schema["properties"] = objProperties
		}
	}

	// Add validation constraints
	sg.addValidationConstraints(schema, def)

	return schema
}

// addValidationConstraints adds validation constraints to the schema
func (sg *SchemaGenerator) addValidationConstraints(schema map[string]interface{}, def map[string]interface{}) {
	// String constraints
	if schema["type"] == "string" {
		if minLen, ok := def["minLength"].(float64); ok {
			schema["minLength"] = minLen
		}
		if maxLen, ok := def["maxLength"].(float64); ok {
			schema["maxLength"] = maxLen
		}
		if pattern, ok := def["pattern"].(string); ok {
			schema["pattern"] = pattern
		}
		if enum, ok := def["enum"].([]interface{}); ok {
			schema["enum"] = enum
		}
	}

	// Number constraints
	if schema["type"] == "integer" || schema["type"] == "number" {
		if min, ok := def["minimum"].(float64); ok {
			schema["minimum"] = min
		}
		if max, ok := def["maximum"].(float64); ok {
			schema["maximum"] = max
		}
	}

	// Array constraints
	if schema["type"] == "array" {
		if minItems, ok := def["minItems"].(int); ok {
			schema["minItems"] = minItems
		}
		if maxItems, ok := def["maxItems"].(int); ok {
			schema["maxItems"] = maxItems
		}
	}
}

// isRequired determines if a field is required
func (sg *SchemaGenerator) isRequired(outputDef interface{}) bool {
	switch def := outputDef.(type) {
	case map[string]interface{}:
		if required, ok := def["required"].(bool); ok {
			return required
		}
		// Default to required unless explicitly marked optional
		if optional, ok := def["optional"].(bool); ok {
			return !optional
		}
	}
	// Simple string types are required by default
	return true
}

// GeneratePromptInstructions generates prompt instructions for JSON schema compliance
func (sg *SchemaGenerator) GeneratePromptInstructions(schema string) string {
	if schema == "" {
		return ""
	}

	return fmt.Sprintf(`

IMPORTANT: You must respond with a valid JSON object that matches this exact schema:

%s

Requirements:
- Your response must be valid JSON
- Include all required fields
- Use the correct data types for each field
- Follow any validation constraints specified
- You may include the JSON in a code block, but ensure the JSON itself is valid

Example response format:
` + "```json\n{\n  \"field1\": \"value1\",\n  \"field2\": 42,\n  \"field3\": [\"item1\", \"item2\"]\n}\n```", schema)
}

// GetSchemaExampleResponse generates an example response based on the schema
func (sg *SchemaGenerator) GetSchemaExampleResponse(outputs map[string]interface{}) map[string]interface{} {
	if outputs == nil || len(outputs) == 0 {
		return nil
	}

	example := make(map[string]interface{})

	for key, outputDef := range outputs {
		example[key] = sg.getExampleValue(outputDef)
	}

	return example
}

// getExampleValue generates an example value for a given output definition
func (sg *SchemaGenerator) getExampleValue(outputDef interface{}) interface{} {
	switch def := outputDef.(type) {
	case string:
		return sg.getExampleValueFromType(def)
	case map[string]interface{}:
		if typeVal, ok := def["type"].(string); ok {
			return sg.getExampleValueFromType(typeVal)
		}
		return "example_value"
	default:
		return "example_value"
	}
}

// getExampleValueFromType generates example values based on type
func (sg *SchemaGenerator) getExampleValueFromType(typeStr string) interface{} {
	switch strings.ToLower(typeStr) {
	case "string":
		return "example_string"
	case "integer", "int":
		return 42
	case "float", "number":
		return 3.14
	case "boolean", "bool":
		return true
	case "array":
		return []interface{}{"item1", "item2", "item3"}
	case "object":
		return map[string]interface{}{
			"key": "value",
		}
	default:
		return "example_value"
	}
}