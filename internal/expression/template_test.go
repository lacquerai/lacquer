package expression

import (
	"context"
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateEngine_BasicRendering(t *testing.T) {
	te := NewTemplateEngine()

	workflow := &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name: "test-workflow",
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	inputs := map[string]interface{}{
		"name": "Alice",
		"age":  30,
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, inputs)

	// Test input variable
	result, err := te.Render("Hello, {{ inputs.name }}!", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, Alice!", result)

	// Test multiple variables
	result, err = te.Render("Name: {{ inputs.name }}, Age: {{ inputs.age }}", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Name: Alice, Age: 30", result)
}

func TestTemplateEngine_StateVariables(t *testing.T) {
	te := NewTemplateEngine()

	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			State: map[string]interface{}{
				"counter": 5,
				"status":  "active",
			},
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	// Test state variables
	result, err := te.Render("Counter: {{ state.counter }}, Status: {{ state.status }}", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Counter: 5, Status: active", result)

	// Test updated state
	execCtx.SetState("counter", 10)
	result, err = te.Render("Counter: {{ state.counter }}", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Counter: 10", result)
}

func TestTemplateEngine_StepVariables(t *testing.T) {
	te := NewTemplateEngine()

	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	// Add a completed step result
	stepResult := &StepResult{
		StepID:   "step1",
		Status:   StepStatusCompleted,
		Response: "Hello, world!",
		Output: map[string]interface{}{
			"response":  "Hello, world!",
			"sentiment": "positive",
		},
	}
	execCtx.SetStepResult("step1", stepResult)

	// Test step response
	result, err := te.Render("Response: {{ steps.step1.output }}", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Response: Hello, world!", result)

	// Test step output
	result, err = te.Render("Sentiment: {{ steps.step1.sentiment }}", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Sentiment: positive", result)

	// Test step status
	result, err = te.Render("Status: {{ steps.step1.status }}", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Status: completed", result)

	// Test step success flag
	result, err = te.Render("Success: {{ steps.step1.success }}", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Success: true", result)
}

func TestTemplateEngine_MetadataVariables(t *testing.T) {
	te := NewTemplateEngine()

	workflow := &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name:        "test-workflow",
			Description: "A test workflow",
			Author:      "Alice",
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	// Test metadata variables
	result, err := te.Render("Workflow: {{ metadata.name }} by {{ metadata.author }}", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Workflow: test-workflow by Alice", result)
}

func TestTemplateEngine_WorkflowVariables(t *testing.T) {
	te := NewTemplateEngine()

	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
				{ID: "step2", Agent: "agent2", Prompt: "test"},
			},
		},
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	// Test workflow variables
	result, err := te.Render("Step {{ workflow.step_index }} of {{ workflow.total_steps }}", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Step 1 of 2", result)

	// Test run ID
	result, err = te.Render("Run ID: {{ workflow.run_id }}", execCtx)
	assert.NoError(t, err)
	assert.Contains(t, result, "Run ID: run_")
}

func TestTemplateEngine_EnvironmentVariables(t *testing.T) {
	te := NewTemplateEngine()

	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	// Add mock environment variable
	execCtx.Environment["TEST_VAR"] = "test_value"

	// Test environment variable
	result, err := te.Render("Env var: {{ env.TEST_VAR }}", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Env var: test_value", result)

	// Test missing environment variable (should return empty string)
	result, err = te.Render("Missing: '{{ env.MISSING_VAR }}'", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Missing: ''", result)
}

func TestTemplateEngine_NoVariables(t *testing.T) {
	te := NewTemplateEngine()

	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	// Test template with no variables
	result, err := te.Render("Hello, world!", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, world!", result)

	// Test empty template
	result, err = te.Render("", execCtx)
	assert.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestTemplateEngine_Errors(t *testing.T) {
	te := NewTemplateEngine()

	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	// Test missing input
	_, err := te.Render("{{ inputs.missing }}", execCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input parameter missing not found")

	// Test missing state
	_, err = te.Render("{{ state.missing }}", execCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state variable missing not found")

	// Test missing step
	_, err = te.Render("{{ steps.missing.output }}", execCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "step missing not found")

	// Test invalid scope
	_, err = te.Render("{{ invalid.scope }}", execCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown variable scope: invalid")
}

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
