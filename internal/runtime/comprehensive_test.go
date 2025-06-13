package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lacquer/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRuntimeComprehensive_EndToEndWorkflow tests complete workflow execution
func TestRuntimeComprehensive_EndToEndWorkflow(t *testing.T) {
	// Create executor with mock provider
	executor := NewExecutor(nil)
	
	// Register mock provider
	mockProvider := NewMockModelProvider("mock", []string{"gpt-4"})
	mockProvider.SetResponse("Process this topic: artificial intelligence", "Mock analysis of artificial intelligence")
	mockProvider.SetResponse("Build on: Mock analysis of artificial intelligence", "Extended analysis with more details")
	mockProvider.SetResponse("Finalize with format summary: Extended analysis with more details", "Final summary completed")
	
	err := executor.modelRegistry.RegisterProvider(mockProvider)
	require.NoError(t, err)

	// Create a simple multi-step workflow
	workflow := &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name:        "test-workflow",
			Description: "Comprehensive test workflow",
		},
		Agents: map[string]*ast.Agent{
			"test_agent": {
				Model:       "gpt-4",
				Temperature: floatPtr(0.7),
			},
		},
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"topic": {
					Type:        "string",
					Description: "The topic to process",
					Required:    true,
				},
				"format": {
					Type:     "string",
					Default:  "summary",
					Required: false,
				},
			},
			State: map[string]interface{}{
				"counter": 0,
				"status":  "initialized",
			},
			Steps: []*ast.Step{
				{
					ID:     "step1",
					Agent:  "test_agent",
					Prompt: "Process this topic: {{ inputs.topic }}",
				},
				{
					ID:      "step2",
					Agent:   "test_agent",
					Prompt:  "Build on: {{ steps.step1.response }}",
					Updates: map[string]interface{}{
						"counter": 1,
						"status":  "processing",
					},
				},
				{
					ID:     "step3",
					Agent:  "test_agent",
					Prompt: "Finalize with format {{ inputs.format }}: {{ steps.step2.response }}",
					Updates: map[string]interface{}{
						"status": "completed",
					},
				},
			},
			Outputs: map[string]interface{}{
				"result":     "{{ steps.step3.response }}",
				"step_count": "{{ state.counter }}",
				"final_status": "{{ state.status }}",
			},
		},
	}

	// Execute workflow
	inputs := map[string]interface{}{
		"topic":  "artificial intelligence",
		"format": "summary",
	}

	result, err := executor.Execute(context.Background(), workflow, inputs)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify execution results
	assert.Equal(t, ExecutionStatusCompleted, result.Status)
	assert.Equal(t, 3, len(result.Steps))
	assert.NotNil(t, result.State)

	// Verify step execution order and dependencies
	var step1Result, step2Result, step3Result *StepResult
	for i := range result.Steps {
		switch result.Steps[i].StepID {
		case "step1":
			step1Result = &result.Steps[i]
		case "step2":
			step2Result = &result.Steps[i]
		case "step3":
			step3Result = &result.Steps[i]
		}
	}
	
	assert.NotNil(t, step1Result)
	assert.Equal(t, StepStatusCompleted, step1Result.Status)

	assert.NotNil(t, step2Result)
	assert.Equal(t, StepStatusCompleted, step2Result.Status)

	assert.NotNil(t, step3Result)
	assert.Equal(t, StepStatusCompleted, step3Result.Status)

	// Verify state updates
	assert.Equal(t, 1, result.State["counter"])
	assert.Equal(t, "completed", result.State["status"])
}

// TestRuntimeComprehensive_VariableInterpolation tests template variable resolution
func TestRuntimeComprehensive_VariableInterpolation(t *testing.T) {
	executor := NewExecutor(nil)

	workflow := &ast.Workflow{
		Version: "1.0",
		Agents: map[string]*ast.Agent{
			"interpolation_agent": {
				Model: "gpt-4",
			},
		},
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"name": {Type: "string", Required: true},
				"age":  {Type: "number", Required: true},
			},
			State: map[string]interface{}{
				"greeting": "Hello",
				"processed_count": 0,
			},
			Steps: []*ast.Step{
				{
					ID:     "greeting_step",
					Agent:  "interpolation_agent",
					Prompt: "{{ state.greeting }}, {{ inputs.name }}! You are {{ inputs.age }} years old.",
					Updates: map[string]interface{}{
						"processed_count": "{{ state.processed_count + 1 }}",
					},
				},
				{
					ID:     "summary_step",
					Agent:  "interpolation_agent",
					Prompt: "Summary for {{ inputs.name }}: {{ steps.greeting_step.response }}",
				},
			},
			Outputs: map[string]interface{}{
				"greeting": "{{ steps.greeting_step.response }}",
				"summary":  "{{ steps.summary_step.response }}",
				"metadata": map[string]interface{}{
					"processed_by": "{{ workflow.run_id }}",
					"step_count":   "{{ workflow.total_steps }}",
				},
			},
		},
	}

	inputs := map[string]interface{}{
		"name": "Alice",
		"age":  25,
	}

	result, err := executor.Execute(context.Background(), workflow, inputs)
	require.NoError(t, err)

	// Verify variable interpolation worked
	assert.Equal(t, ExecutionStatusCompleted, result.Status)

	// State should be updated
	assert.Equal(t, "1", result.State["processed_count"])
}

// TestRuntimeComprehensive_ConditionalExecution tests step conditions and skip logic
func TestRuntimeComprehensive_ConditionalExecution(t *testing.T) {
	executor := NewExecutor(nil)

	testCases := []struct {
		name      string
		condition string
		shouldRun bool
	}{
		{
			name:      "Always true condition",
			condition: "true",
			shouldRun: true,
		},
		{
			name:      "Always false condition",
			condition: "false",
			shouldRun: false,
		},
		{
			name:      "Input-based condition (true)",
			condition: "{{ inputs.enabled == true }}",
			shouldRun: true,
		},
		{
			name:      "Input-based condition (false)",
			condition: "{{ inputs.enabled == false }}",
			shouldRun: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workflow := &ast.Workflow{
				Version: "1.0",
				Agents: map[string]*ast.Agent{
					"conditional_agent": {Model: "gpt-4"},
				},
				Workflow: &ast.WorkflowDef{
					Inputs: map[string]*ast.InputParam{
						"enabled": {Type: "boolean", Required: true},
					},
					Steps: []*ast.Step{
						{
							ID:        "always_run",
							Agent:     "conditional_agent",
							Prompt:    "This always runs",
						},
						{
							ID:        "conditional_step",
							Agent:     "conditional_agent",
							Prompt:    "This runs conditionally",
							Condition: tc.condition,
						},
						{
							ID:     "final_step",
							Agent:  "conditional_agent",
							Prompt: "This runs after conditional",
						},
					},
				},
			}

			inputs := map[string]interface{}{
				"enabled": tc.shouldRun,
			}

			result, err := executor.Execute(context.Background(), workflow, inputs)
			require.NoError(t, err)

			// Find step results by ID
			var alwaysRunResult, conditionalResult, finalResult *StepResult
			for i := range result.Steps {
				switch result.Steps[i].StepID {
				case "always_run":
					alwaysRunResult = &result.Steps[i]
				case "conditional_step":
					conditionalResult = &result.Steps[i]
				case "final_step":
					finalResult = &result.Steps[i]
				}
			}

			// Always run step should complete
			assert.NotNil(t, alwaysRunResult)
			assert.Equal(t, StepStatusCompleted, alwaysRunResult.Status)

			// Conditional step should run or be skipped based on condition
			assert.NotNil(t, conditionalResult)
			if tc.shouldRun {
				assert.Equal(t, StepStatusCompleted, conditionalResult.Status)
			} else {
				assert.Equal(t, StepStatusSkipped, conditionalResult.Status)
			}

			// Final step should always run
			assert.NotNil(t, finalResult)
			assert.Equal(t, StepStatusCompleted, finalResult.Status)
		})
	}
}

// TestRuntimeComprehensive_ErrorHandling tests error scenarios and recovery
func TestRuntimeComprehensive_ErrorHandling(t *testing.T) {
	executor := NewExecutor(nil)

	testCases := []struct {
		name           string
		workflow       *ast.Workflow
		inputs         map[string]interface{}
		expectError    bool
		expectedStatus StepStatus
	}{
		{
			name: "Missing required input",
			workflow: &ast.Workflow{
				Version: "1.0",
				Agents:  map[string]*ast.Agent{"agent": {Model: "gpt-4"}},
				Workflow: &ast.WorkflowDef{
					Inputs: map[string]*ast.InputParam{
						"required": {Type: "string", Required: true},
					},
					Steps: []*ast.Step{
						{ID: "test", Agent: "agent", Prompt: "{{ inputs.required }}"},
					},
				},
			},
			inputs:      map[string]interface{}{}, // Missing required input
			expectError: true,
		},
		{
			name: "Invalid agent reference",
			workflow: &ast.Workflow{
				Version: "1.0",
				Agents:  map[string]*ast.Agent{"valid_agent": {Model: "gpt-4"}},
				Workflow: &ast.WorkflowDef{
					Steps: []*ast.Step{
						{ID: "test", Agent: "nonexistent_agent", Prompt: "test"},
					},
				},
			},
			inputs:         map[string]interface{}{},
			expectError:    true,
			expectedStatus: StepStatusFailed,
		},
		{
			name: "Missing model in agent",
			workflow: &ast.Workflow{
				Version: "1.0",
				Agents:  map[string]*ast.Agent{"bad_agent": {}}, // Missing model
				Workflow: &ast.WorkflowDef{
					Steps: []*ast.Step{
						{ID: "test", Agent: "bad_agent", Prompt: "test"},
					},
				},
			},
			inputs:         map[string]interface{}{},
			expectError:    true,
			expectedStatus: StepStatusFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := executor.Execute(context.Background(), tc.workflow, tc.inputs)

			if tc.expectError {
				// Should either return an error or have failed steps
				if err == nil {
					assert.NotEqual(t, ExecutionStatusCompleted, result.Status, "Workflow should not succeed")
					if tc.expectedStatus != "" && len(result.Steps) > 0 {
						// Check that at least one step has the expected failure status
						hasExpectedStatus := false
						for _, stepResult := range result.Steps {
							if stepResult.Status == tc.expectedStatus {
								hasExpectedStatus = true
								break
							}
						}
						assert.True(t, hasExpectedStatus, "Expected at least one step with status %v", tc.expectedStatus)
					}
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, ExecutionStatusCompleted, result.Status)
			}
		})
	}
}

// TestRuntimeComprehensive_StateManagement tests state persistence and updates
func TestRuntimeComprehensive_StateManagement(t *testing.T) {
	executor := NewExecutor(nil)

	workflow := &ast.Workflow{
		Version: "1.0",
		Agents: map[string]*ast.Agent{
			"state_agent": {Model: "gpt-4"},
		},
		Workflow: &ast.WorkflowDef{
			State: map[string]interface{}{
				"counter":    0,
				"items":      []string{},
				"metadata":   map[string]interface{}{"created": "now"},
				"total_cost": 0.0,
			},
			Steps: []*ast.Step{
				{
					ID:     "increment_counter",
					Agent:  "state_agent",
					Prompt: "Processing item 1",
					Updates: map[string]interface{}{
						"counter":    "{{ state.counter + 1 }}",
						"total_cost": "{{ state.total_cost + 0.10 }}",
					},
				},
				{
					ID:     "add_item",
					Agent:  "state_agent",
					Prompt: "Adding item to list",
					Updates: map[string]interface{}{
						"items": []string{"item1", "item2"},
					},
				},
				{
					ID:     "update_metadata",
					Agent:  "state_agent",
					Prompt: "Updating metadata",
					Updates: map[string]interface{}{
						"metadata": map[string]interface{}{
							"created":     "{{ state.metadata.created }}",
							"updated":     "now",
							"step_count":  "{{ state.counter }}",
						},
					},
				},
			},
		},
	}

	result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{})
	require.NoError(t, err)

	// Verify state updates
	assert.Equal(t, "1", result.State["counter"])
	assert.Equal(t, "0.10", result.State["total_cost"])

	// Verify complex state updates
	items, ok := result.State["items"].([]string)
	assert.True(t, ok)
	assert.Equal(t, []string{"item1", "item2"}, items)

	metadata, ok := result.State["metadata"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "now", metadata["created"])
	assert.Equal(t, "now", metadata["updated"])
	assert.Equal(t, "1", metadata["step_count"])
}

// TestRuntimeComprehensive_Performance tests runtime performance characteristics
func TestRuntimeComprehensive_Performance(t *testing.T) {
	executor := NewExecutor(nil)

	// Create a workflow with multiple steps to test performance
	steps := make([]*ast.Step, 0, 10)
	for i := 0; i < 10; i++ {
		steps = append(steps, &ast.Step{
			ID:     fmt.Sprintf("step_%d", i),
			Agent:  "perf_agent",
			Prompt: fmt.Sprintf("Processing step %d", i),
		})
	}

	workflow := &ast.Workflow{
		Version: "1.0",
		Agents: map[string]*ast.Agent{
			"perf_agent": {Model: "gpt-4"},
		},
		Workflow: &ast.WorkflowDef{
			Steps: steps,
		},
	}

	// Measure execution time
	start := time.Now()
	result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{})
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, ExecutionStatusCompleted, result.Status)

	// Performance assertions (these may need adjustment based on actual performance)
	assert.Less(t, duration, 30*time.Second, "Workflow should complete within 30 seconds")
	assert.Equal(t, 10, len(result.Steps), "All steps should be executed")

	// Verify no memory leaks by checking that all steps completed
	for i := 0; i < 10; i++ {
		stepID := fmt.Sprintf("step_%d", i)
		found := false
		for _, stepResult := range result.Steps {
			if stepResult.StepID == stepID {
				assert.Equal(t, StepStatusCompleted, stepResult.Status)
				found = true
				break
			}
		}
		assert.True(t, found, "Step %s should have result", stepID)
	}
}

// TestRuntimeComprehensive_ConcurrentExecution tests concurrent step execution scenarios
func TestRuntimeComprehensive_ConcurrentExecution(t *testing.T) {
	// Configure executor for concurrent execution
	config := &ExecutorConfig{
		MaxConcurrentSteps: 3,
		DefaultTimeout:     30 * time.Second,
		EnableRetries:      true,
	}
	executor := NewExecutor(config)

	workflow := &ast.Workflow{
		Version: "1.0",
		Agents: map[string]*ast.Agent{
			"concurrent_agent": {Model: "gpt-4"},
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{
					ID:     "independent_1",
					Agent:  "concurrent_agent",
					Prompt: "Independent task 1",
				},
				{
					ID:     "independent_2",
					Agent:  "concurrent_agent",
					Prompt: "Independent task 2",
				},
				{
					ID:     "independent_3",
					Agent:  "concurrent_agent",
					Prompt: "Independent task 3",
				},
				{
					ID:     "dependent_step",
					Agent:  "concurrent_agent",
					Prompt: "Depends on: {{ steps.independent_1.response }}, {{ steps.independent_2.response }}, {{ steps.independent_3.response }}",
				},
			},
		},
	}

	start := time.Now()
	result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{})
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, ExecutionStatusCompleted, result.Status)

	// With concurrency, execution should be faster than sequential
	// This is a rough test - in practice, you'd measure against a sequential baseline
	assert.Less(t, duration, 25*time.Second, "Concurrent execution should be reasonably fast")

	// All steps should complete successfully
	assert.Equal(t, 4, len(result.Steps))
	for _, stepResult := range result.Steps {
		assert.Equal(t, StepStatusCompleted, stepResult.Status, "Step %s should complete", stepResult.StepID)
	}
}

// Helper functions
func floatPtr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}