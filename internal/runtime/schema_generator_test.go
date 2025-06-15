package runtime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaGenerator_GenerateJSONSchema(t *testing.T) {
	generator := NewSchemaGenerator()

	tests := []struct {
		name     string
		outputs  map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "empty outputs returns empty schema",
			outputs:  nil,
			expected: nil,
		},
		{
			name: "simple string type",
			outputs: map[string]interface{}{
				"name": "string",
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []interface{}{"name"},
			},
		},
		{
			name: "multiple basic types",
			outputs: map[string]interface{}{
				"name":     "string",
				"age":      "integer",
				"score":    "float",
				"active":   "boolean",
				"tags":     "array",
				"metadata": "object",
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"active": map[string]interface{}{
						"type": "boolean",
					},
					"age": map[string]interface{}{
						"type": "integer",
					},
					"metadata": map[string]interface{}{
						"type": "object",
					},
					"name": map[string]interface{}{
						"type": "string",
					},
					"score": map[string]interface{}{
						"type": "number",
					},
					"tags": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []interface{}{"active", "age", "metadata", "name", "score", "tags"},
			},
		},
		{
			name: "complex object with descriptions",
			outputs: map[string]interface{}{
				"user": map[string]interface{}{
					"type":        "object",
					"description": "User information",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{"type": "string"},
						"age":  map[string]interface{}{"type": "integer"},
					},
				},
				"scores": map[string]interface{}{
					"type":        "array",
					"description": "Array of scores",
					"items":       "number",
				},
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"scores": map[string]interface{}{
						"type":        "array",
						"description": "Array of scores",
						"items": map[string]interface{}{
							"type": "number",
						},
					},
					"user": map[string]interface{}{
						"type":        "object",
						"description": "User information",
						"properties": map[string]interface{}{
							"age": map[string]interface{}{
								"type": "integer",
							},
							"name": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
				"required": []interface{}{"scores", "user"},
			},
		},
		{
			name: "optional fields",
			outputs: map[string]interface{}{
				"required_field": "string",
				"optional_field": map[string]interface{}{
					"type":     "string",
					"optional": true,
				},
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"optional_field": map[string]interface{}{
						"type": "string",
					},
					"required_field": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []interface{}{"required_field"},
			},
		},
		{
			name: "validation constraints",
			outputs: map[string]interface{}{
				"name": map[string]interface{}{
					"type":      "string",
					"minLength": float64(1),
					"maxLength": float64(100),
				},
				"age": map[string]interface{}{
					"type":    "integer",
					"minimum": 0.0,
					"maximum": 150.0,
				},
				"status": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"active", "inactive", "pending"},
				},
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"age": map[string]interface{}{
						"type":    "integer",
						"minimum": 0.0,
						"maximum": 150.0,
					},
					"name": map[string]interface{}{
						"type":      "string",
						"minLength": float64(1),
						"maxLength": float64(100),
					},
					"status": map[string]interface{}{
						"type": "string",
						"enum": []interface{}{"active", "inactive", "pending"},
					},
				},
				"required": []interface{}{"age", "name", "status"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaJSON, err := generator.GenerateJSONSchema(tt.outputs)
			require.NoError(t, err)

			if tt.expected == nil {
				assert.Empty(t, schemaJSON)
				return
			}

			// Parse the generated schema
			var actualSchema map[string]interface{}
			err = json.Unmarshal([]byte(schemaJSON), &actualSchema)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, actualSchema)
		})
	}
}

func TestSchemaGenerator_GeneratePromptInstructions(t *testing.T) {
	generator := NewSchemaGenerator()

	schema := `{
  "type": "object",
  "properties": {
    "name": {
      "type": "string"
    },
    "score": {
      "type": "integer"
    }
  },
  "required": ["name", "score"]
}`

	instructions := generator.GeneratePromptInstructions(schema)

	assert.Contains(t, instructions, "valid JSON object")
	assert.Contains(t, instructions, schema)
	assert.Contains(t, instructions, "required fields")
	assert.Contains(t, instructions, "correct data types")
}

func TestSchemaGenerator_GeneratePromptInstructions_EmptySchema(t *testing.T) {
	generator := NewSchemaGenerator()

	instructions := generator.GeneratePromptInstructions("")
	assert.Empty(t, instructions)
}

func TestSchemaGenerator_GetSchemaExampleResponse(t *testing.T) {
	generator := NewSchemaGenerator()

	tests := []struct {
		name     string
		outputs  map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "empty outputs",
			outputs:  nil,
			expected: nil,
		},
		{
			name: "basic types",
			outputs: map[string]interface{}{
				"name":    "string",
				"age":     "integer",
				"score":   "float",
				"active":  "boolean",
				"tags":    "array",
				"details": "object",
			},
			expected: map[string]interface{}{
				"name":    "example_string",
				"age":     42,
				"score":   3.14,
				"active":  true,
				"tags":    []interface{}{"item1", "item2", "item3"},
				"details": map[string]interface{}{"key": "value"},
			},
		},
		{
			name: "complex object types",
			outputs: map[string]interface{}{
				"user": map[string]interface{}{
					"type": "object",
				},
				"scores": map[string]interface{}{
					"type": "array",
				},
			},
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"key": "value",
				},
				"scores": []interface{}{"item1", "item2", "item3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.GetSchemaExampleResponse(tt.outputs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSchemaGenerator_Integration(t *testing.T) {
	generator := NewSchemaGenerator()

	outputs := map[string]interface{}{
		"analysis": map[string]interface{}{
			"type":        "object",
			"description": "Analysis results",
			"properties": map[string]interface{}{
				"score":  "integer",
				"status": "string",
			},
		},
		"recommendations": map[string]interface{}{
			"type":        "array",
			"description": "List of recommendations",
			"items":       "string",
		},
		"confidence": map[string]interface{}{
			"type":    "number",
			"minimum": 0.0,
			"maximum": 1.0,
		},
	}

	// Generate schema
	schemaJSON, err := generator.GenerateJSONSchema(outputs)
	require.NoError(t, err)
	assert.NotEmpty(t, schemaJSON)

	// Verify schema is valid JSON
	var schema map[string]interface{}
	err = json.Unmarshal([]byte(schemaJSON), &schema)
	require.NoError(t, err)

	// Generate prompt instructions
	instructions := generator.GeneratePromptInstructions(schemaJSON)
	assert.Contains(t, instructions, "valid JSON object")
	assert.Contains(t, instructions, schemaJSON)

	// Generate example response
	example := generator.GetSchemaExampleResponse(outputs)
	require.NotNil(t, example)
	assert.Contains(t, example, "analysis")
	assert.Contains(t, example, "recommendations")
	assert.Contains(t, example, "confidence")
}