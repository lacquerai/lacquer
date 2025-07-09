package engine

import (
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateWorkflowInputs_NoInputDefinitions(t *testing.T) {
	workflow := &ast.Workflow{
		Workflow: &ast.WorkflowDef{
			Inputs: nil,
			Steps:  []*ast.Step{{ID: "test"}},
		},
	}

	inputs := map[string]any{
		"arbitrary": "value",
		"number":    42,
	}

	result := ValidateWorkflowInputs(workflow, inputs)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Equal(t, inputs, result.ProcessedInputs)
}

func TestValidateWorkflowInputs_RequiredFieldMissing(t *testing.T) {
	workflow := &ast.Workflow{
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"name": {
					Type:     "string",
					Required: true,
				},
			},
			Steps: []*ast.Step{{ID: "test"}},
		},
	}

	inputs := map[string]any{}

	result := ValidateWorkflowInputs(workflow, inputs)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "name", result.Errors[0].Field)
	assert.Contains(t, result.Errors[0].Message, "required field is missing")
}

func TestValidateWorkflowInputs_DefaultValues(t *testing.T) {
	workflow := &ast.Workflow{
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"name": {
					Type:    "string",
					Default: "World",
				},
				"count": {
					Type:    "integer",
					Default: 5,
				},
			},
			Steps: []*ast.Step{{ID: "test"}},
		},
	}

	inputs := map[string]any{
		"name": "Alice", // Override default
		// count will use default
	}

	result := ValidateWorkflowInputs(workflow, inputs)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Equal(t, "Alice", result.ProcessedInputs["name"])
	assert.Equal(t, 5, result.ProcessedInputs["count"])
}

func TestValidateWorkflowInputs_TypeValidation(t *testing.T) {
	workflow := &ast.Workflow{
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"name":     {Type: "string"},
				"age":      {Type: "integer"},
				"active":   {Type: "boolean"},
				"tags":     {Type: "array"},
				"metadata": {Type: "object"},
			},
			Steps: []*ast.Step{{ID: "test"}},
		},
	}

	inputs := map[string]any{
		"name":     "Alice",
		"age":      30,
		"active":   true,
		"tags":     []string{"user", "admin"},
		"metadata": map[string]any{"role": "admin"},
	}

	result := ValidateWorkflowInputs(workflow, inputs)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Equal(t, inputs, result.ProcessedInputs)
}

func TestValidateWorkflowInputs_TypeConversion(t *testing.T) {
	workflow := &ast.Workflow{
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"count":   {Type: "integer"},
				"enabled": {Type: "boolean"},
				"label":   {Type: "string"},
			},
			Steps: []*ast.Step{{ID: "test"}},
		},
	}

	inputs := map[string]any{
		"count":   "42",   // string to int
		"enabled": "true", // string to bool
		"label":   123,    // int to string
	}

	result := ValidateWorkflowInputs(workflow, inputs)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Equal(t, 42, result.ProcessedInputs["count"])
	assert.Equal(t, true, result.ProcessedInputs["enabled"])
	assert.Equal(t, "123", result.ProcessedInputs["label"])
}

func TestValidateWorkflowInputs_TypeValidationFailure(t *testing.T) {
	workflow := &ast.Workflow{
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"age": {Type: "integer"},
			},
			Steps: []*ast.Step{{ID: "test"}},
		},
	}

	inputs := map[string]any{
		"age": "not-a-number",
	}

	result := ValidateWorkflowInputs(workflow, inputs)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "age", result.Errors[0].Field)
	assert.Contains(t, result.Errors[0].Message, "invalid type")
}

func TestValidateWorkflowInputs_StringConstraints(t *testing.T) {
	t.Run("Pattern validation", func(t *testing.T) {
		workflow := &ast.Workflow{
			Workflow: &ast.WorkflowDef{
				Inputs: map[string]*ast.InputParam{
					"email": {
						Type:    "string",
						Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
					},
				},
				Steps: []*ast.Step{{ID: "test"}},
			},
		}

		// Valid email
		inputs := map[string]any{"email": "test@example.com"}
		result := ValidateWorkflowInputs(workflow, inputs)
		assert.True(t, result.Valid)

		// Invalid email
		inputs = map[string]any{"email": "invalid-email"}
		result = ValidateWorkflowInputs(workflow, inputs)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Message, "does not match required pattern")
	})

	t.Run("Enum validation", func(t *testing.T) {
		workflow := &ast.Workflow{
			Workflow: &ast.WorkflowDef{
				Inputs: map[string]*ast.InputParam{
					"size": {
						Type: "string",
						Enum: []string{"small", "medium", "large"},
					},
				},
				Steps: []*ast.Step{{ID: "test"}},
			},
		}

		// Valid enum value
		inputs := map[string]any{"size": "medium"}
		result := ValidateWorkflowInputs(workflow, inputs)
		assert.True(t, result.Valid)

		// Invalid enum value
		inputs = map[string]any{"size": "extra-large"}
		result = ValidateWorkflowInputs(workflow, inputs)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Message, "must be one of")
	})
}

func TestValidateWorkflowInputs_NumericConstraints(t *testing.T) {
	min := 0.0
	max := 100.0
	workflow := &ast.Workflow{
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"score": {
					Type:    "integer",
					Minimum: &min,
					Maximum: &max,
				},
			},
			Steps: []*ast.Step{{ID: "test"}},
		},
	}

	// Valid value
	inputs := map[string]any{"score": 85}
	result := ValidateWorkflowInputs(workflow, inputs)
	assert.True(t, result.Valid)

	// Below minimum
	inputs = map[string]any{"score": -5}
	result = ValidateWorkflowInputs(workflow, inputs)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Message, "less than minimum")

	// Above maximum
	inputs = map[string]any{"score": 150}
	result = ValidateWorkflowInputs(workflow, inputs)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Message, "greater than maximum")
}

func TestValidateWorkflowInputs_ArrayConstraints(t *testing.T) {
	minItems := 1
	maxItems := 5
	workflow := &ast.Workflow{
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"items": {
					Type:     "array",
					MinItems: &minItems,
					MaxItems: &maxItems,
				},
			},
			Steps: []*ast.Step{{ID: "test"}},
		},
	}

	// Valid array
	inputs := map[string]any{"items": []string{"a", "b", "c"}}
	result := ValidateWorkflowInputs(workflow, inputs)
	assert.True(t, result.Valid)

	// Too few items
	inputs = map[string]any{"items": []string{}}
	result = ValidateWorkflowInputs(workflow, inputs)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Message, "minimum required is")

	// Too many items
	inputs = map[string]any{"items": []string{"a", "b", "c", "d", "e", "f"}}
	result = ValidateWorkflowInputs(workflow, inputs)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Message, "maximum allowed is")
}

func TestValidateWorkflowInputs_UnexpectedFields(t *testing.T) {
	workflow := &ast.Workflow{
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"name": {Type: "string"},
			},
			Steps: []*ast.Step{{ID: "test"}},
		},
	}

	inputs := map[string]any{
		"name":        "Alice",
		"unexpected":  "value",
		"another_bad": 123,
	}

	result := ValidateWorkflowInputs(workflow, inputs)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 2)

	errorFields := []string{result.Errors[0].Field, result.Errors[1].Field}
	assert.Contains(t, errorFields, "unexpected")
	assert.Contains(t, errorFields, "another_bad")
}

func TestValidateWorkflowInputs_ComplexScenario(t *testing.T) {
	min := 18.0
	max := 120.0
	minItems := 1

	workflow := &ast.Workflow{
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"name": {
					Type:     "string",
					Required: true,
					Pattern:  `^[A-Za-z\s]+$`,
				},
				"age": {
					Type:    "integer",
					Minimum: &min,
					Maximum: &max,
					Default: 25,
				},
				"email": {
					Type:    "string",
					Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
				},
				"skills": {
					Type:     "array",
					MinItems: &minItems,
				},
				"role": {
					Type:    "string",
					Enum:    []string{"user", "admin", "moderator"},
					Default: "user",
				},
			},
			Steps: []*ast.Step{{ID: "test"}},
		},
	}

	inputs := map[string]any{
		"name":   "Alice Smith",
		"email":  "alice@example.com",
		"skills": []string{"golang", "testing"},
		// age and role will use defaults
	}

	result := ValidateWorkflowInputs(workflow, inputs)
	require.True(t, result.Valid)
	assert.Empty(t, result.Errors)

	expected := map[string]any{
		"name":   "Alice Smith",
		"age":    25,
		"email":  "alice@example.com",
		"skills": []string{"golang", "testing"},
		"role":   "user",
	}

	assert.Equal(t, expected, result.ProcessedInputs)
}

func TestConvertAndValidateType_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		value        any
		expectedType string
		expectError  bool
		expectedVal  any
	}{
		{"float to int - exact", 42.0, "integer", false, 42},
		{"float to int - decimal", 42.5, "integer", true, nil},
		{"bool to string", true, "string", false, "true"},
		{"int to string", 123, "string", false, "123"},
		{"invalid bool string", "maybe", "boolean", true, nil},
		{"valid bool string", "false", "boolean", false, false},
		{"nil value", nil, "string", true, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertAndValidateType(tt.value, tt.expectedType)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedVal, result)
			}
		})
	}
}
