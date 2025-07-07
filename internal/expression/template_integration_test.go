package expression

import (
	"context"
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateEngine_Integration(t *testing.T) {
	te := NewTemplateEngine()

	testCases := []struct {
		name     string
		template string
		setup    func() *ExecutionContext
		expected string
	}{
		{
			name:     "Simple expression in template",
			template: "Result: {{ inputs.count > 5 }}",
			setup: func() *ExecutionContext {
				workflow := &ast.Workflow{
					Version: "1.0",
					Workflow: &ast.WorkflowDef{
						Steps: []*ast.Step{
							{ID: "step1", Agent: "agent1", Prompt: "test"},
						},
					},
				}
				inputs := map[string]interface{}{"count": 10}
				return NewExecutionContext(context.Background(), workflow, inputs)
			},
			expected: "Result: true",
		},
		{
			name:     "Ternary expression in template",
			template: "Status: {{ inputs.enabled ? 'active' : 'inactive' }}",
			setup: func() *ExecutionContext {
				workflow := &ast.Workflow{
					Version: "1.0",
					Workflow: &ast.WorkflowDef{
						Steps: []*ast.Step{
							{ID: "step1", Agent: "agent1", Prompt: "test"},
						},
					},
				}
				inputs := map[string]interface{}{"enabled": true}
				return NewExecutionContext(context.Background(), workflow, inputs)
			},
			expected: "Status: active",
		},
		{
			name:     "Function call in template",
			template: "Message: {{ format('Hello {0}!', inputs.name) }}",
			setup: func() *ExecutionContext {
				workflow := &ast.Workflow{
					Version: "1.0",
					Workflow: &ast.WorkflowDef{
						Steps: []*ast.Step{
							{ID: "step1", Agent: "agent1", Prompt: "test"},
						},
					},
				}
				inputs := map[string]interface{}{"name": "world"}
				return NewExecutionContext(context.Background(), workflow, inputs)
			},
			expected: "Message: Hello world!",
		},
		{
			name:     "Complex expression with multiple operators",
			template: "Valid: {{ inputs.count > 5 && state.enabled == true }}",
			setup: func() *ExecutionContext {
				workflow := &ast.Workflow{
					Version: "1.0",
					Workflow: &ast.WorkflowDef{
						State: map[string]interface{}{
							"enabled": true,
						},
						Steps: []*ast.Step{
							{ID: "step1", Agent: "agent1", Prompt: "test"},
						},
					},
				}
				inputs := map[string]interface{}{"count": 10}
				return NewExecutionContext(context.Background(), workflow, inputs)
			},
			expected: "Valid: true",
		},
		{
			name:     "String manipulation functions",
			template: "Check: {{ contains(inputs.text, 'test') && startsWith(inputs.text, 'This') }}",
			setup: func() *ExecutionContext {
				workflow := &ast.Workflow{
					Version: "1.0",
					Workflow: &ast.WorkflowDef{
						Steps: []*ast.Step{
							{ID: "step1", Agent: "agent1", Prompt: "test"},
						},
					},
				}
				inputs := map[string]interface{}{"text": "This is a test"}
				return NewExecutionContext(context.Background(), workflow, inputs)
			},
			expected: "Check: true",
		},
		{
			name:     "Workflow status functions",
			template: "Success: {{ success() && always() }}",
			setup: func() *ExecutionContext {
				workflow := &ast.Workflow{
					Version: "1.0",
					Workflow: &ast.WorkflowDef{
						Steps: []*ast.Step{
							{ID: "step1", Agent: "agent1", Prompt: "test"},
						},
					},
				}
				return NewExecutionContext(context.Background(), workflow, nil)
			},
			expected: "Success: true",
		},
		{
			name:     "Mixed variable types and functions",
			template: "Output: {{ inputs.count + 5 > 10 ? format('High: {0}', inputs.count) : 'Low' }}",
			setup: func() *ExecutionContext {
				workflow := &ast.Workflow{
					Version: "1.0",
					Workflow: &ast.WorkflowDef{
						Steps: []*ast.Step{
							{ID: "step1", Agent: "agent1", Prompt: "test"},
						},
					},
				}
				inputs := map[string]interface{}{"count": 8}
				return NewExecutionContext(context.Background(), workflow, inputs)
			},
			expected: "Output: High: 8",
		},
		{
			name:     "Regular variable alongside expression",
			template: "Name: {{ inputs.name }}, Age Check: {{ inputs.age >= 18 }}",
			setup: func() *ExecutionContext {
				workflow := &ast.Workflow{
					Version: "1.0",
					Workflow: &ast.WorkflowDef{
						Steps: []*ast.Step{
							{ID: "step1", Agent: "agent1", Prompt: "test"},
						},
					},
				}
				inputs := map[string]interface{}{
					"name": "Alice",
					"age":  25,
				}
				return NewExecutionContext(context.Background(), workflow, inputs)
			},
			expected: "Name: Alice, Age Check: true",
		},
		{
			name:     "JSON manipulation",
			template: "Data: {{ toJSON(inputs.data) }}",
			setup: func() *ExecutionContext {
				workflow := &ast.Workflow{
					Version: "1.0",
					Workflow: &ast.WorkflowDef{
						Steps: []*ast.Step{
							{ID: "step1", Agent: "agent1", Prompt: "test"},
						},
					},
				}
				inputs := map[string]interface{}{
					"data": map[string]interface{}{
						"key": "value",
					},
				}
				return NewExecutionContext(context.Background(), workflow, inputs)
			},
			expected: "Data: {\"key\":\"value\"}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execCtx := tc.setup()
			result, err := te.Render(tc.template, execCtx)
			require.NoError(t, err, "Template rendering failed")
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTemplateEngine_ComplexWorkflowScenarios(t *testing.T) {
	te := NewTemplateEngine()

	t.Run("Conditional step execution based on expressions", func(t *testing.T) {
		workflow := &ast.Workflow{
			Version: "1.0",
			Workflow: &ast.WorkflowDef{
				Steps: []*ast.Step{
					{ID: "setup", Agent: "agent1", Prompt: "Setup"},
					{ID: "process", Agent: "agent2", Prompt: "Process"},
				},
			},
		}

		inputs := map[string]interface{}{
			"environment": "production",
			"debug":       false,
		}

		execCtx := NewExecutionContext(context.Background(), workflow, inputs)

		// Test conditional execution templates
		testCases := []struct {
			template string
			expected string
		}{
			{
				template: "{{ inputs.environment == 'production' ? 'Deploy to prod' : 'Deploy to staging' }}",
				expected: "Deploy to prod",
			},
			{
				template: "Debug mode: {{ inputs.debug ? 'enabled' : 'disabled' }}",
				expected: "Debug mode: disabled",
			},
			{
				template: "{{ contains(inputs.environment, 'prod') && !inputs.debug ? 'Production deployment' : 'Development deployment' }}",
				expected: "Production deployment",
			},
		}

		for _, tc := range testCases {
			result, err := te.Render(tc.template, execCtx)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		}
	})

	t.Run("Step result processing with expressions", func(t *testing.T) {
		workflow := &ast.Workflow{
			Version: "1.0",
			Workflow: &ast.WorkflowDef{
				Steps: []*ast.Step{
					{ID: "analyze", Agent: "agent1", Prompt: "Analyze"},
					{ID: "process", Agent: "agent2", Prompt: "Process"},
				},
			},
		}

		execCtx := NewExecutionContext(context.Background(), workflow, nil)

		// Add a step result
		stepResult := &StepResult{
			StepID:   "analyze",
			Status:   StepStatusCompleted,
			Response: "Analysis complete",
			Output: map[string]interface{}{
				"score":      85,
				"confidence": 0.9,
				"category":   "positive",
			},
		}
		execCtx.SetStepResult("analyze", stepResult)

		testCases := []struct {
			template string
			expected string
		}{
			{
				template: "Score check: {{ steps.analyze.score > 80 }}",
				expected: "Score check: true",
			},
			{
				template: "Category: {{ steps.analyze.category == 'positive' ? 'Good result' : 'Needs review' }}",
				expected: "Category: Good result",
			},
			{
				template: "Confidence: {{ steps.analyze.confidence >= 0.8 && steps.analyze.score > 75 ? 'High' : 'Low' }}",
				expected: "Confidence: High",
			},
			{
				template: "Status: {{ success() ? format('Step {0} completed successfully', 'analyze') : 'Step failed' }}",
				expected: "Status: Step analyze completed successfully",
			},
		}

		for _, tc := range testCases {
			result, err := te.Render(tc.template, execCtx)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		}
	})

	t.Run("Matrix strategy simulation", func(t *testing.T) {
		workflow := &ast.Workflow{
			Version: "1.0",
			Workflow: &ast.WorkflowDef{
				Steps: []*ast.Step{
					{ID: "test", Agent: "agent1", Prompt: "Test"},
				},
			},
		}

		execCtx := NewExecutionContext(context.Background(), workflow, nil)

		// Simulate matrix strategy
		execCtx.Matrix = map[string]interface{}{
			"os":      "ubuntu-latest",
			"version": "3.9",
			"arch":    "x64",
		}

		testCases := []struct {
			template string
			expected string
		}{
			{
				template: "Running on: {{ matrix().os }}",
				expected: "Running on: ubuntu-latest",
			},
			{
				template: "{{ startsWith(matrix().os, 'ubuntu') ? 'Linux environment' : 'Other environment' }}",
				expected: "Linux environment",
			},
			{
				template: "Config: {{ format('{0}-{1}-{2}', matrix().os, matrix().version, matrix().arch) }}",
				expected: "Config: ubuntu-latest-3.9-x64",
			},
		}

		for _, tc := range testCases {
			result, err := te.Render(tc.template, execCtx)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		}
	})
}

func TestTemplateEngine_ErrorHandlingIntegration(t *testing.T) {
	te := NewTemplateEngine()

	t.Run("Expression errors are properly handled", func(t *testing.T) {
		workflow := &ast.Workflow{
			Version: "1.0",
			Workflow: &ast.WorkflowDef{
				Steps: []*ast.Step{
					{ID: "step1", Agent: "agent1", Prompt: "test"},
				},
			},
		}

		execCtx := NewExecutionContext(context.Background(), workflow, nil)

		errorCases := []string{
			"{{ 10 / 0 }}",               // Division by zero
			"{{ unknownFunction() }}",    // Unknown function
			"{{ inputs.undefined > 5 }}", // Undefined variable
			"{{ 5 + }}",                  // Invalid syntax
			"{{ (5 + 3 }}",               // Mismatched parentheses
		}

		for _, template := range errorCases {
			_, err := te.Render(template, execCtx)
			assert.Error(t, err, "Expected error for template: %s", template)
		}
	})

	t.Run("Mixed valid and invalid expressions", func(t *testing.T) {
		workflow := &ast.Workflow{
			Version: "1.0",
			Workflow: &ast.WorkflowDef{
				Steps: []*ast.Step{
					{ID: "step1", Agent: "agent1", Prompt: "test"},
				},
			},
		}

		inputs := map[string]interface{}{
			"valid": "test",
		}

		execCtx := NewExecutionContext(context.Background(), workflow, inputs)

		// This should work for the valid part but fail for the invalid part
		template := "Valid: {{ inputs.valid }}, Invalid: {{ inputs.undefined > 5 }}"
		_, err := te.Render(template, execCtx)
		assert.Error(t, err)
	})
}

func TestTemplateEngine_PerformanceScenarios(t *testing.T) {
	te := NewTemplateEngine()

	t.Run("Complex nested expressions", func(t *testing.T) {
		workflow := &ast.Workflow{
			Version: "1.0",
			Workflow: &ast.WorkflowDef{
				Steps: []*ast.Step{
					{ID: "step1", Agent: "agent1", Prompt: "test"},
				},
			},
		}

		inputs := map[string]interface{}{
			"a": 1,
			"b": 2,
			"c": 3,
			"d": 4,
			"e": 5,
		}

		execCtx := NewExecutionContext(context.Background(), workflow, inputs)

		// Complex expression with multiple levels of nesting
		template := "{{ (inputs.a + inputs.b) * (inputs.c + inputs.d) == inputs.e * 6 ? format('Math works: {0}', (inputs.a + inputs.b) * (inputs.c + inputs.d)) : 'Math failed' }}"

		result, err := te.Render(template, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "Math works: 21", result)
	})

	t.Run("Multiple expressions in one template", func(t *testing.T) {
		workflow := &ast.Workflow{
			Version: "1.0",
			Workflow: &ast.WorkflowDef{
				Steps: []*ast.Step{
					{ID: "step1", Agent: "agent1", Prompt: "test"},
				},
			},
		}

		inputs := map[string]interface{}{
			"name":  "test",
			"count": 10,
			"flag":  true,
		}

		execCtx := NewExecutionContext(context.Background(), workflow, inputs)

		template := `
Results:
- Name valid: {{ contains(inputs.name, 'test') }}
- Count high: {{ inputs.count > 5 }}
- Flag status: {{ inputs.flag ? 'enabled' : 'disabled' }}
- Combined: {{ contains(inputs.name, 'test') && inputs.count > 5 && inputs.flag }}
- Message: {{ format('Processing {0} items', inputs.count) }}
`

		result, err := te.Render(template, execCtx)
		require.NoError(t, err)

		assert.Contains(t, result, "Name valid: true")
		assert.Contains(t, result, "Count high: true")
		assert.Contains(t, result, "Flag status: enabled")
		assert.Contains(t, result, "Combined: true")
		assert.Contains(t, result, "Message: Processing 10 items")
	})
}
