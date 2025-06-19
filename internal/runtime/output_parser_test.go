package runtime

import (
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputParser_ParseStepOutput(t *testing.T) {
	parser := NewOutputParser()

	tests := []struct {
		name     string
		step     *ast.Step
		response string
		expected map[string]interface{}
	}{
		{
			name: "no outputs defined returns default output",
			step: &ast.Step{
				ID: "test-step",
			},
			response: "This is the agent response",
			expected: map[string]interface{}{
				"output": "This is the agent response",
			},
		},
		{
			name: "parse JSON response with matching outputs",
			step: &ast.Step{
				ID: "test-step",
				Outputs: map[string]interface{}{
					"name":  "string",
					"score": "integer",
					"items": "array",
				},
			},
			response: `{"name": "Test Item", "score": 95, "items": ["item1", "item2", "item3"]}`,
			expected: map[string]interface{}{
				"outputs": map[string]interface{}{
					"name":  "Test Item",
					"score": 95,
					"items": []interface{}{"item1", "item2", "item3"},
				},
				"output": `{"name": "Test Item", "score": 95, "items": ["item1", "item2", "item3"]}`,
			},
		},
		{
			name: "parse JSON in code block",
			step: &ast.Step{
				ID: "test-step",
				Outputs: map[string]interface{}{
					"data": "object",
				},
			},
			response: "Here is the data:\n```json\n{\"key\": \"value\", \"count\": 42}\n```",
			expected: map[string]interface{}{
				"outputs": map[string]interface{}{
					"data": map[string]interface{}{
						"key":   "value",
						"count": float64(42),
					},
				},
				"output": "Here is the data:\n```json\n{\"key\": \"value\", \"count\": 42}\n```",
			},
		},
		{
			name: "extract structured data from natural language",
			step: &ast.Step{
				ID: "test-step",
				Outputs: map[string]interface{}{
					"score":    "integer",
					"is_valid": "boolean",
					"summary":  "string",
				},
			},
			response: "After analysis, the score: 85\nThe validation result is_valid: true\nSummary: Everything looks good!",
			expected: map[string]interface{}{
				"outputs": map[string]interface{}{
					"score":    85,
					"is_valid": true,
					"summary":  "Everything looks good!",
				},
				"output": "After analysis, the score: 85\nThe validation result is_valid: true\nSummary: Everything looks good!",
			},
		},
		{
			name: "extract array from list format",
			step: &ast.Step{
				ID: "test-step",
				Outputs: map[string]interface{}{
					"findings": "array",
				},
			},
			response: "findings:\n- First finding\n- Second finding\n- Third finding",
			expected: map[string]interface{}{
				"outputs": map[string]interface{}{
					"findings": []interface{}{
						"First finding",
						"Second finding",
						"Third finding",
					},
				},
				"output": "findings:\n- First finding\n- Second finding\n- Third finding",
			},
		},
		{
			name: "parse boolean values",
			step: &ast.Step{
				ID: "test-step",
				Outputs: map[string]interface{}{
					"approved":  "boolean",
					"completed": "boolean",
				},
			},
			response: "The request has been approved: yes\nTask completed: false",
			expected: map[string]interface{}{
				"outputs": map[string]interface{}{
					"approved":  true,
					"completed": false,
				},
				"output": "The request has been approved: yes\nTask completed: false",
			},
		},
		{
			name: "handle complex output definitions",
			step: &ast.Step{
				ID: "test-step",
				Outputs: map[string]interface{}{
					"result": map[string]interface{}{
						"type": "object",
					},
				},
			},
			response: `{"result": {"status": "success", "data": [1, 2, 3]}}`,
			expected: map[string]interface{}{
				"outputs": map[string]interface{}{
					"result": map[string]interface{}{
						"status": "success",
						"data":   []interface{}{float64(1), float64(2), float64(3)},
					},
				},
				"output": `{"result": {"status": "success", "data": [1, 2, 3]}}`,
			},
		},
		{
			name: "single output field gets entire JSON response",
			step: &ast.Step{
				ID: "test-step",
				Outputs: map[string]interface{}{
					"data": "array",
				},
			},
			response: `[{"id": 1, "name": "Item 1"}, {"id": 2, "name": "Item 2"}]`,
			expected: map[string]interface{}{
				"outputs": map[string]interface{}{
					"data": []interface{}{
						map[string]interface{}{"id": float64(1), "name": "Item 1"},
						map[string]interface{}{"id": float64(2), "name": "Item 2"},
					},
				},
				"output": `[{"id": 1, "name": "Item 1"}, {"id": 2, "name": "Item 2"}]`,
			},
		},
		{
			name: "fallback to raw response on parse failure",
			step: &ast.Step{
				ID: "test-step",
				Outputs: map[string]interface{}{
					"data": "object",
				},
			},
			response: "This is not structured data at all",
			expected: map[string]interface{}{
				"output":  "This is not structured data at all",
				"outputs": map[string]interface{}{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseStepOutput(tt.step, tt.response)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutputParser_ExtractJSON(t *testing.T) {
	parser := NewOutputParser()

	tests := []struct {
		name     string
		response string
		expected interface{}
	}{
		{
			name:     "extract plain JSON object",
			response: `{"key": "value", "number": 42}`,
			expected: map[string]interface{}{"key": "value", "number": float64(42)},
		},
		{
			name:     "extract JSON array",
			response: `["item1", "item2", "item3"]`,
			expected: []interface{}{"item1", "item2", "item3"},
		},
		{
			name:     "extract JSON from code block",
			response: "```json\n{\"nested\": {\"value\": 123}}\n```",
			expected: map[string]interface{}{"nested": map[string]interface{}{"value": float64(123)}},
		},
		{
			name:     "extract JSON from text with surrounding content",
			response: "The result is: {\"status\": \"success\"} which is good",
			expected: map[string]interface{}{"status": "success"},
		},
		{
			name:     "no JSON found",
			response: "This is just plain text without any JSON",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractJSON(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutputParser_ParseValue(t *testing.T) {
	parser := NewOutputParser()

	tests := []struct {
		name         string
		value        string
		expectedType string
		expected     interface{}
	}{
		{
			name:         "parse integer",
			value:        "42",
			expectedType: "integer",
			expected:     42,
		},
		{
			name:         "parse float",
			value:        "3.14",
			expectedType: "float",
			expected:     3.14,
		},
		{
			name:         "parse boolean true",
			value:        "yes",
			expectedType: "boolean",
			expected:     true,
		},
		{
			name:         "parse boolean false",
			value:        "no",
			expectedType: "boolean",
			expected:     false,
		},
		{
			name:         "parse array from comma-separated",
			value:        "item1, item2, item3",
			expectedType: "array",
			expected:     []interface{}{"item1, item2, item3"},
		},
		{
			name:         "parse object from JSON",
			value:        `{"key": "value"}`,
			expectedType: "object",
			expected:     map[string]interface{}{"key": "value"},
		},
		{
			name:         "default to string",
			value:        "just a string",
			expectedType: "string",
			expected:     "just a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.parseValue(tt.value, tt.expectedType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutputParser_ParseList(t *testing.T) {
	parser := NewOutputParser()

	tests := []struct {
		name     string
		text     string
		expected []interface{}
	}{
		{
			name: "parse dash list",
			text: "- First item\n- Second item\n- Third item",
			expected: []interface{}{
				"First item",
				"Second item",
				"Third item",
			},
		},
		{
			name: "parse asterisk list",
			text: "* Item A\n* Item B\n* Item C",
			expected: []interface{}{
				"Item A",
				"Item B",
				"Item C",
			},
		},
		{
			name: "parse mixed list with empty lines",
			text: "- First\n\n- Second\n  \n- Third",
			expected: []interface{}{
				"First",
				"Second",
				"Third",
			},
		},
		{
			name:     "empty list",
			text:     "",
			expected: []interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.parseList(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutputParser_SchemaGuidedParsing(t *testing.T) {
	parser := NewOutputParser()

	tests := []struct {
		name     string
		step     *ast.Step
		response string
		expected map[string]interface{}
	}{
		{
			name: "well-formed JSON response",
			step: &ast.Step{
				ID: "test-step",
				Outputs: map[string]interface{}{
					"name":  "string",
					"score": "integer",
					"tags":  "array",
				},
			},
			response: `{
				"name": "Test Analysis",
				"score": 95,
				"tags": ["important", "reviewed", "final"]
			}`,
			expected: map[string]interface{}{
				"outputs": map[string]interface{}{
					"name":  "Test Analysis",
					"score": 95,
					"tags":  []interface{}{"important", "reviewed", "final"},
				},
				"output": `{
				"name": "Test Analysis",
				"score": 95,
				"tags": ["important", "reviewed", "final"]
			}`,
			},
		},
		{
			name: "JSON with trailing commas (auto-fixed)",
			step: &ast.Step{
				ID: "test-step",
				Outputs: map[string]interface{}{
					"result": "string",
					"status": "string",
				},
			},
			response: `{
				"result": "success",
				"status": "completed",
			}`,
			expected: map[string]interface{}{
				"outputs": map[string]interface{}{
					"result": "success",
					"status": "completed",
				},
				"output": `{
				"result": "success",
				"status": "completed",
			}`,
			},
		},
		{
			name: "JSON with single quotes (auto-fixed)",
			step: &ast.Step{
				ID: "test-step",
				Outputs: map[string]interface{}{
					"message": "string",
					"code":    "integer",
				},
			},
			response: `{
				'message': 'Hello World',
				'code': 200
			}`,
			expected: map[string]interface{}{
				"outputs": map[string]interface{}{
					"message": "Hello World",
					"code":    200,
				},
				"output": `{
				'message': 'Hello World',
				'code': 200
			}`,
			},
		},
		{
			name: "schema-guided response with explanation",
			step: &ast.Step{
				ID: "test-step",
				Outputs: map[string]interface{}{
					"summary":    "string",
					"confidence": "float",
					"actionable": "boolean",
				},
			},
			response: `Based on my analysis, here's the JSON response:

` + "```json\n{\n\t\"summary\": \"Document is well-structured and comprehensive\",\n\t\"confidence\": 0.92,\n\t\"actionable\": true\n}\n```\n\nThis analysis shows high confidence in the assessment.",
			expected: map[string]interface{}{
				"outputs": map[string]interface{}{
					"summary":    "Document is well-structured and comprehensive",
					"confidence": 0.92,
					"actionable": true,
				},
				"output": `Based on my analysis, here's the JSON response:

` + "```json\n{\n\t\"summary\": \"Document is well-structured and comprehensive\",\n\t\"confidence\": 0.92,\n\t\"actionable\": true\n}\n```\n\nThis analysis shows high confidence in the assessment.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseStepOutput(tt.step, tt.response)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutputParser_JSONFixing(t *testing.T) {
	parser := NewOutputParser()

	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:  "fix trailing commas",
			input: `{"name": "test", "value": 42,}`,
			expected: map[string]interface{}{
				"name":  "test",
				"value": float64(42),
			},
		},
		{
			name:  "fix single quotes",
			input: `{'name': 'test', 'active': true}`,
			expected: map[string]interface{}{
				"name":   "test",
				"active": true,
			},
		},
		{
			name:  "multiple fixes needed",
			input: `{'result': 'success', 'count': 5,}`,
			expected: map[string]interface{}{
				"result": "success",
				"count":  float64(5),
			},
		},
		{
			name:     "unfixable JSON",
			input:    `{name: test, value: }`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.attemptJSONFix(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutputParser_SchemaGuidedDetection(t *testing.T) {
	parser := NewOutputParser()

	tests := []struct {
		name     string
		response string
		expected bool
	}{
		{
			name:     "JSON code block",
			response: "```json\n{\"test\": \"value\"}\n```",
			expected: true,
		},
		{
			name:     "schema keyword",
			response: "Here's the JSON that matches the schema: {\"test\": \"value\"}",
			expected: true,
		},
		{
			name:     "valid JSON response",
			response: "Please provide a valid JSON object with the required fields.",
			expected: true,
		},
		{
			name:     "JSON-like structure",
			response: `{"name": "test", "value": 42}`,
			expected: true,
		},
		{
			name:     "plain text response",
			response: "This is just a regular text response without any JSON indicators.",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.isSchemaGuidedResponse(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}
