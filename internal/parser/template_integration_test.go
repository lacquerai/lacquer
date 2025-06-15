package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateValidator_ExtractVariableReferences(t *testing.T) {
	tv := NewTemplateValidator()

	testCases := []struct {
		name     string
		template string
		expected []VariableReference
	}{
		{
			name:     "No variables",
			template: "Hello world",
			expected: nil,
		},
		{
			name:     "Single input variable",
			template: "Hello {{ inputs.name }}",
			expected: []VariableReference{
				{
					Raw:    "inputs.name",
					Scope:  "inputs",
					Path:   []string{"inputs", "name"},
					Type:   VariableTypeInput,
					Target: "name",
				},
			},
		},
		{
			name:     "Step output reference",
			template: "Previous result: {{ steps.step1.output }}",
			expected: []VariableReference{
				{
					Raw:    "steps.step1.output",
					Scope:  "steps",
					Path:   []string{"steps", "step1", "output"},
					Type:   VariableTypeStep,
					Target: "step1",
					Field:  "output",
				},
			},
		},
		{
			name:     "Multiple variables",
			template: "{{ inputs.topic }} processed by {{ steps.analyze.response }}",
			expected: []VariableReference{
				{
					Raw:    "inputs.topic",
					Scope:  "inputs",
					Path:   []string{"inputs", "topic"},
					Type:   VariableTypeInput,
					Target: "topic",
				},
				{
					Raw:    "steps.analyze.response",
					Scope:  "steps",
					Path:   []string{"steps", "analyze", "response"},
					Type:   VariableTypeStep,
					Target: "analyze",
					Field:  "response",
				},
			},
		},
		{
			name:     "Environment variable",
			template: "Environment: {{ env.NODE_ENV }}",
			expected: []VariableReference{
				{
					Raw:    "env.NODE_ENV",
					Scope:  "env",
					Path:   []string{"env", "NODE_ENV"},
					Type:   VariableTypeEnvironment,
					Target: "NODE_ENV",
				},
			},
		},
		{
			name:     "State variable",
			template: "Counter: {{ state.counter }}",
			expected: []VariableReference{
				{
					Raw:    "state.counter",
					Scope:  "state",
					Path:   []string{"state", "counter"},
					Type:   VariableTypeState,
					Target: "counter",
				},
			},
		},
		{
			name:     "Function call",
			template: "Generated at {{ now() }}",
			expected: []VariableReference{
				{
					Raw:  "now()",
					Type: VariableTypeFunction,
					Path: []string{"now()"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			refs := tv.ExtractVariableReferences(tc.template)

			require.Equal(t, len(tc.expected), len(refs), "Expected %d references, got %d", len(tc.expected), len(refs))

			for i, expected := range tc.expected {
				actual := refs[i]
				assert.Equal(t, expected.Raw, actual.Raw, "Reference %d: Raw mismatch", i)
				assert.Equal(t, expected.Scope, actual.Scope, "Reference %d: Scope mismatch", i)
				assert.Equal(t, expected.Type, actual.Type, "Reference %d: Type mismatch", i)
				assert.Equal(t, expected.Target, actual.Target, "Reference %d: Target mismatch", i)
				assert.Equal(t, expected.Field, actual.Field, "Reference %d: Field mismatch", i)
			}
		})
	}
}

func TestTemplateValidator_ValidateTemplateString(t *testing.T) {
	tv := NewTemplateValidator()

	testCases := []struct {
		name        string
		template    string
		expectValid bool
	}{
		{
			name:        "Empty template",
			template:    "",
			expectValid: true,
		},
		{
			name:        "No variables",
			template:    "Hello world",
			expectValid: true,
		},
		{
			name:        "Valid input reference",
			template:    "Hello {{ inputs.name }}",
			expectValid: true,
		},
		{
			name:        "Valid step reference",
			template:    "Result: {{ steps.step1.output }}",
			expectValid: true,
		},
		{
			name:        "Empty variable reference",
			template:    "Hello {{ }}",
			expectValid: false,
		},
		{
			name:        "Invalid scope",
			template:    "Hello {{ invalid.scope }}",
			expectValid: false,
		},
		{
			name:        "Invalid path",
			template:    "Hello {{ inputs. }}",
			expectValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tv.ValidateTemplateString(tc.template)

			if tc.expectValid {
				assert.NoError(t, err, "Expected template to be valid")
			} else {
				assert.Error(t, err, "Expected template to be invalid")
			}
		})
	}
}

func TestTemplateValidator_ParseVariableReference(t *testing.T) {
	testCases := []struct {
		name     string
		varPath  string
		expected VariableReference
	}{
		{
			name:    "Input variable",
			varPath: "inputs.topic",
			expected: VariableReference{
				Raw:    "inputs.topic",
				Scope:  "inputs",
				Path:   []string{"inputs", "topic"},
				Type:   VariableTypeInput,
				Target: "topic",
			},
		},
		{
			name:    "Step output",
			varPath: "steps.analyze.response",
			expected: VariableReference{
				Raw:    "steps.analyze.response",
				Scope:  "steps",
				Path:   []string{"steps", "analyze", "response"},
				Type:   VariableTypeStep,
				Target: "analyze",
				Field:  "response",
			},
		},
		{
			name:    "Nested state",
			varPath: "state.config.setting",
			expected: VariableReference{
				Raw:    "state.config.setting",
				Scope:  "state",
				Path:   []string{"state", "config", "setting"},
				Type:   VariableTypeState,
				Target: "config",
				Field:  "setting",
			},
		},
		{
			name:    "Environment variable",
			varPath: "env.DATABASE_URL",
			expected: VariableReference{
				Raw:    "env.DATABASE_URL",
				Scope:  "env",
				Path:   []string{"env", "DATABASE_URL"},
				Type:   VariableTypeEnvironment,
				Target: "DATABASE_URL",
			},
		},
		{
			name:    "Metadata field",
			varPath: "metadata.name",
			expected: VariableReference{
				Raw:   "metadata.name",
				Scope: "metadata",
				Path:  []string{"metadata", "name"},
				Type:  VariableTypeMetadata,
				Field: "name",
			},
		},
		{
			name:    "Workflow field",
			varPath: "workflow.run_id",
			expected: VariableReference{
				Raw:   "workflow.run_id",
				Scope: "workflow",
				Path:  []string{"workflow", "run_id"},
				Type:  VariableTypeWorkflow,
				Field: "run_id",
			},
		},
		{
			name:    "Function call",
			varPath: "now()",
			expected: VariableReference{
				Raw:  "now()",
				Path: []string{"now()"},
				Type: VariableTypeFunction,
			},
		},
		{
			name:    "Expression with operators",
			varPath: "inputs.count > 5",
			expected: VariableReference{
				Raw:  "inputs.count > 5",
				Path: []string{"inputs.count > 5"},
				Type: VariableTypeExpression,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ref := ParseVariableReference(tc.varPath)

			assert.Equal(t, tc.expected.Raw, ref.Raw)
			assert.Equal(t, tc.expected.Scope, ref.Scope)
			assert.Equal(t, tc.expected.Type, ref.Type)
			assert.Equal(t, tc.expected.Target, ref.Target)
			assert.Equal(t, tc.expected.Field, ref.Field)
			assert.Equal(t, tc.expected.Path, ref.Path)
		})
	}
}

func TestTemplateValidator_ContainsOperators(t *testing.T) {
	testCases := []struct {
		name     string
		varPath  string
		expected bool
	}{
		{
			name:     "No operators",
			varPath:  "inputs.name",
			expected: false,
		},
		{
			name:     "Equality operator",
			varPath:  "inputs.mode == 'debug'",
			expected: true,
		},
		{
			name:     "Comparison operators",
			varPath:  "state.count > 5",
			expected: true,
		},
		{
			name:     "Logical operators",
			varPath:  "inputs.a && inputs.b",
			expected: true,
		},
		{
			name:     "Ternary operator",
			varPath:  "inputs.debug ? 'on' : 'off'",
			expected: true,
		},
		{
			name:     "Pipe operator",
			varPath:  "inputs.text | upper",
			expected: true,
		},
		{
			name:     "Arithmetic operators",
			varPath:  "state.counter + 1",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := containsOperators(tc.varPath)
			assert.Equal(t, tc.expected, result)
		})
	}
}
