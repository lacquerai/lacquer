package engine

import (
	"context"
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutor_CollectWorkflowOutputs(t *testing.T) {
	tests := []struct {
		name            string
		workflowDef     *ast.WorkflowDef
		stepResults     map[string]*StepResult
		inputs          map[string]interface{}
		expectedOutputs map[string]interface{}
		expectError     bool
	}{
		{
			name: "simple string template output",
			workflowDef: &ast.WorkflowDef{
				Outputs: map[string]interface{}{
					"greeting": "Hello {{ inputs.name }}",
				},
			},
			inputs: map[string]interface{}{
				"name": "World",
			},
			expectedOutputs: map[string]interface{}{
				"greeting": "Hello World",
			},
		},
		{
			name: "multiple outputs with step references",
			workflowDef: &ast.WorkflowDef{
				Outputs: map[string]interface{}{
					"result":    "{{ steps.test_step.output }}",
					"input_ref": "{{ inputs.test_input }}",
					"literal":   "static value",
				},
			},
			stepResults: map[string]*StepResult{
				"test_step": {
					StepID: "test_step",
					Output: map[string]interface{}{
						"output": "test response",
					},
					Response: "test response",
				},
			},
			inputs: map[string]interface{}{
				"test_input": "test value",
			},
			expectedOutputs: map[string]interface{}{
				"result":    "test response",
				"input_ref": "test value",
				"literal":   "static value",
			},
		},
		{
			name: "no outputs defined",
			workflowDef: &ast.WorkflowDef{
				Outputs: nil,
			},
			expectedOutputs: map[string]interface{}{},
		},
		{
			name: "empty outputs",
			workflowDef: &ast.WorkflowDef{
				Outputs: map[string]interface{}{},
			},
			expectedOutputs: map[string]interface{}{},
		},
		{
			name: "non-string template value",
			workflowDef: &ast.WorkflowDef{
				Outputs: map[string]interface{}{
					"number": 42,
					"bool":   true,
				},
			},
			expectedOutputs: map[string]interface{}{
				"number": 42,
				"bool":   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create workflow
			workflow := &ast.Workflow{
				Version:  "1.0",
				Workflow: tt.workflowDef,
			}

			// Create execution context
			ctx := context.Background()
			execCtx := NewExecutionContext(ctx, workflow, tt.inputs)

			// Set step results if provided
			if tt.stepResults != nil {
				for stepID, result := range tt.stepResults {
					execCtx.SetStepResult(stepID, result)
				}
			}

			// Create executor
			config := DefaultExecutorConfig()
			registry := NewModelRegistry(true)
			executor, err := NewExecutor(config, workflow, registry)
			require.NoError(t, err)

			// Test collectWorkflowOutputs
			err = executor.collectWorkflowOutputs(execCtx)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify outputs
			actualOutputs := execCtx.GetWorkflowOutputs()

			if len(tt.expectedOutputs) == 0 {
				assert.Empty(t, actualOutputs)
			} else {
				assert.Equal(t, tt.expectedOutputs, actualOutputs)
			}

			// Verify outputs are included in execution summary
			summary := execCtx.GetExecutionSummary()
			assert.Equal(t, actualOutputs, summary.Outputs)
		})
	}
}

func TestExecutor_WorkflowOutputsInExecutionSummary(t *testing.T) {
	// Create a simple workflow with outputs
	workflow := &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name: "test-outputs",
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{
					ID: "dummy_step",
				},
			},
			Outputs: map[string]interface{}{
				"result": "test result",
				"count":  123,
			},
		},
	}

	// Create execution context
	ctx := context.Background()
	inputs := map[string]interface{}{"test": "value"}
	execCtx := NewExecutionContext(ctx, workflow, inputs)

	// Create executor
	config := DefaultExecutorConfig()
	registry := NewModelRegistry(true)
	executor, err := NewExecutor(config, workflow, registry)
	require.NoError(t, err)

	// Collect outputs
	err = executor.collectWorkflowOutputs(execCtx)
	require.NoError(t, err)

	// Get execution summary
	summary := execCtx.GetExecutionSummary()

	// Verify outputs are included
	expectedOutputs := map[string]interface{}{
		"result": "test result",
		"count":  123,
	}
	assert.Equal(t, expectedOutputs, summary.Outputs)
}
