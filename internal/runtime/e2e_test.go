package runtime

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"net/http"
// 	"net/http/httptest"
// 	"strings"
// 	"sync"
// 	"testing"
// 	"time"

// 	"github.com/lacquerai/lacquer/internal/ast"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// )

// // TestRuntimeComprehensive_EndToEndWorkflow tests complete workflow execution
// func TestRuntimeComprehensive_EndToEndWorkflow(t *testing.T) {

// 	registry := NewModelRegistry(true)
// 	// Register mock provider
// 	mockProvider := NewMockModelProvider("mock", []ModelInfo{
// 		{
// 			ID:       "gpt-4",
// 			Name:     "gpt-4",
// 			Provider: "mock",
// 		},
// 	})
// 	mockProvider.SetResponse("Process this topic: artificial intelligence", "Mock analysis of artificial intelligence")
// 	mockProvider.SetResponse("Build on: Mock analysis of artificial intelligence", "Extended analysis with more details")
// 	mockProvider.SetResponse("Finalize with format summary: Extended analysis with more details", "Final summary completed")

// 	err := registry.RegisterProvider(mockProvider)
// 	require.NoError(t, err)

// 	// Create a simple multi-step workflow
// 	workflow := &ast.Workflow{
// 		Version: "1.0",
// 		Metadata: &ast.WorkflowMetadata{
// 			Name:        "test-workflow",
// 			Description: "Comprehensive test workflow",
// 		},
// 		Agents: map[string]*ast.Agent{
// 			"test_agent": {
// 				Provider:    "mock",
// 				Model:       "gpt-4",
// 				Temperature: floatPtr(0.7),
// 			},
// 		},
// 		Workflow: &ast.WorkflowDef{
// 			Inputs: map[string]*ast.InputParam{
// 				"topic": {
// 					Type:        "string",
// 					Description: "The topic to process",
// 					Required:    true,
// 				},
// 				"format": {
// 					Type:     "string",
// 					Default:  "summary",
// 					Required: false,
// 				},
// 			},
// 			State: map[string]interface{}{
// 				"counter": 0,
// 				"status":  "initialized",
// 			},
// 			Steps: []*ast.Step{
// 				{
// 					ID:     "step1",
// 					Agent:  "test_agent",
// 					Prompt: "Process this topic: {{ inputs.topic }}",
// 				},
// 				{
// 					ID:     "step2",
// 					Agent:  "test_agent",
// 					Prompt: "Build on: {{ steps.step1.output }}",
// 					Updates: map[string]interface{}{
// 						"counter": 1,
// 						"status":  "processing",
// 					},
// 				},
// 				{
// 					ID:     "step3",
// 					Agent:  "test_agent",
// 					Prompt: "Finalize with format {{ inputs.format }}: {{ steps.step2.output }}",
// 					Updates: map[string]interface{}{
// 						"status": "completed",
// 					},
// 				},
// 			},
// 			Outputs: map[string]interface{}{
// 				"result":       "{{ steps.step3.output }}",
// 				"step_count":   "{{ state.counter }}",
// 				"final_status": "{{ state.status }}",
// 			},
// 		},
// 	}
// 	// Create executor with mock provider
// 	executor, err := NewExecutor(nil, workflow, registry)
// 	require.NoError(t, err)

// 	// Execute workflow
// 	inputs := map[string]interface{}{
// 		"topic":  "artificial intelligence",
// 		"format": "summary",
// 	}

// 	result, err := executor.Execute(context.Background(), workflow, inputs)
// 	require.NoError(t, err)
// 	assert.NotNil(t, result)

// 	// Verify execution results
// 	assert.Equal(t, ExecutionStatusCompleted, result.Status)
// 	assert.Equal(t, 3, len(result.Steps))
// 	assert.NotNil(t, result.State)

// 	// Verify step execution order and dependencies
// 	var step1Result, step2Result, step3Result *StepResult
// 	for i := range result.Steps {
// 		switch result.Steps[i].StepID {
// 		case "step1":
// 			step1Result = &result.Steps[i]
// 		case "step2":
// 			step2Result = &result.Steps[i]
// 		case "step3":
// 			step3Result = &result.Steps[i]
// 		}
// 	}

// 	assert.NotNil(t, step1Result)
// 	assert.Equal(t, StepStatusCompleted, step1Result.Status)
// 	assert.Nil(t, step1Result.Error)

// 	assert.NotNil(t, step2Result)
// 	assert.Equal(t, StepStatusCompleted, step2Result.Status)

// 	assert.NotNil(t, step3Result)
// 	assert.Equal(t, StepStatusCompleted, step3Result.Status)

// 	// Verify state updates
// 	assert.Equal(t, 1, result.State["counter"])
// 	assert.Equal(t, "completed", result.State["status"])
// }

// // TestRuntimeComprehensive_VariableInterpolation tests template variable resolution
// func TestRuntimeComprehensive_VariableInterpolation(t *testing.T) {
// 	registry := NewModelRegistry(true)
// 	// Register mock provider for gpt-4
// 	mockProvider := NewMockModelProvider("mock", []ModelInfo{
// 		{
// 			ID:       "gpt-4",
// 			Name:     "gpt-4",
// 			Provider: "mock",
// 		},
// 	})
// 	mockProvider.SetResponse("Hello, Alice! You are 25 years old.", "Mock greeting response")
// 	mockProvider.SetResponse("Summary for Alice: Mock greeting response", "Mock summary response")

// 	err := registry.RegisterProvider(mockProvider)
// 	require.NoError(t, err)

// 	workflow := &ast.Workflow{
// 		Version: "1.0",
// 		Metadata: &ast.WorkflowMetadata{
// 			Name: "interpolation-test-workflow",
// 		},
// 		Agents: map[string]*ast.Agent{
// 			"interpolation_agent": {
// 				Provider: "mock",
// 				Model:    "gpt-4",
// 			},
// 		},
// 		Workflow: &ast.WorkflowDef{
// 			Inputs: map[string]*ast.InputParam{
// 				"name": {Type: "string", Required: true},
// 				"age":  {Type: "number", Required: true},
// 			},
// 			State: map[string]interface{}{
// 				"greeting":        "Hello",
// 				"processed_count": 0,
// 			},
// 			Steps: []*ast.Step{
// 				{
// 					ID:     "greeting_step",
// 					Agent:  "interpolation_agent",
// 					Prompt: "{{ state.greeting }}, {{ inputs.name }}! You are {{ inputs.age }} years old.",
// 					Updates: map[string]interface{}{
// 						"processed_count": 1,
// 					},
// 				},
// 				{
// 					ID:     "summary_step",
// 					Agent:  "interpolation_agent",
// 					Prompt: "Summary for {{ inputs.name }}: {{ steps.greeting_step.output }}",
// 				},
// 			},
// 			Outputs: map[string]interface{}{
// 				"greeting": "{{ steps.greeting_step.output }}",
// 				"summary":  "{{ steps.summary_step.output }}",
// 				"metadata": map[string]interface{}{
// 					"processed_by": "{{ workflow.run_id }}",
// 					"step_count":   "{{ workflow.total_steps }}",
// 				},
// 			},
// 		},
// 	}

// 	inputs := map[string]interface{}{
// 		"name": "Alice",
// 		"age":  25,
// 	}

// 	executor, err := NewExecutor(nil, workflow, registry)
// 	require.NoError(t, err)

// 	result, err := executor.Execute(context.Background(), workflow, inputs)
// 	require.NoError(t, err)

// 	// Verify variable interpolation worked
// 	assert.Equal(t, ExecutionStatusCompleted, result.Status)

// 	// State should be updated
// 	assert.Equal(t, 1, result.State["processed_count"])
// }

// // TestRuntimeComprehensive_ConditionalExecution tests step conditions and skip logic
// func TestRuntimeComprehensive_ConditionalExecution(t *testing.T) {
// 	registry := NewModelRegistry(true)
// 	// Register mock provider for gpt-4
// 	mockProvider := NewMockModelProvider("mock", []ModelInfo{
// 		{
// 			ID:       "gpt-4",
// 			Name:     "gpt-4",
// 			Provider: "mock",
// 		},
// 	})
// 	mockProvider.SetResponse("This always runs", "Mock always response")
// 	mockProvider.SetResponse("This runs conditionally", "Mock conditional response")
// 	mockProvider.SetResponse("This runs after conditional", "Mock final response")

// 	err := registry.RegisterProvider(mockProvider)
// 	require.NoError(t, err)

// 	testCases := []struct {
// 		name      string
// 		condition string
// 		shouldRun bool
// 	}{
// 		{
// 			name:      "Always true condition",
// 			condition: "true",
// 			shouldRun: true,
// 		},
// 		{
// 			name:      "Always false condition",
// 			condition: "false",
// 			shouldRun: false,
// 		},
// 		{
// 			name:      "Input-based condition (true)",
// 			condition: "{{ inputs.enabled }}",
// 			shouldRun: true,
// 		},
// 		{
// 			name:      "Input-based condition (false)",
// 			condition: "{{ inputs.enabled }}",
// 			shouldRun: false,
// 		},
// 	}

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			workflow := &ast.Workflow{
// 				Version: "1.0",
// 				Metadata: &ast.WorkflowMetadata{
// 					Name: "conditional-test-workflow",
// 				},
// 				Agents: map[string]*ast.Agent{
// 					"conditional_agent": {
// 						Provider: "mock",
// 						Model:    "gpt-4",
// 					},
// 				},
// 				Workflow: &ast.WorkflowDef{
// 					Inputs: map[string]*ast.InputParam{
// 						"enabled": {Type: "boolean", Required: true},
// 					},
// 					Steps: []*ast.Step{
// 						{
// 							ID:     "always_run",
// 							Agent:  "conditional_agent",
// 							Prompt: "This always runs",
// 						},
// 						{
// 							ID:        "conditional_step",
// 							Agent:     "conditional_agent",
// 							Prompt:    "This runs conditionally",
// 							Condition: tc.condition,
// 						},
// 						{
// 							ID:     "final_step",
// 							Agent:  "conditional_agent",
// 							Prompt: "This runs after conditional",
// 						},
// 					},
// 				},
// 			}

// 			inputs := map[string]interface{}{
// 				"enabled": tc.shouldRun,
// 			}

// 			executor, err := NewExecutor(nil, workflow, registry)
// 			require.NoError(t, err)

// 			result, err := executor.Execute(context.Background(), workflow, inputs)
// 			require.NoError(t, err)

// 			// Find step results by ID
// 			var alwaysRunResult, conditionalResult, finalResult *StepResult
// 			for i := range result.Steps {
// 				switch result.Steps[i].StepID {
// 				case "always_run":
// 					alwaysRunResult = &result.Steps[i]
// 				case "conditional_step":
// 					conditionalResult = &result.Steps[i]
// 				case "final_step":
// 					finalResult = &result.Steps[i]
// 				}
// 			}

// 			// Always run step should complete
// 			assert.NotNil(t, alwaysRunResult)
// 			assert.Equal(t, StepStatusCompleted, alwaysRunResult.Status)

// 			// Conditional step should run or be skipped based on condition
// 			assert.NotNil(t, conditionalResult)
// 			if tc.shouldRun {
// 				assert.Equal(t, StepStatusCompleted, conditionalResult.Status)
// 			} else {
// 				assert.Equal(t, StepStatusSkipped, conditionalResult.Status)
// 			}

// 			// Final step should always run
// 			assert.NotNil(t, finalResult)
// 			assert.Equal(t, StepStatusCompleted, finalResult.Status)
// 		})
// 	}
// }

// // TestRuntimeComprehensive_ErrorHandling tests error scenarios and recovery
// func TestRuntimeComprehensive_ErrorHandling(t *testing.T) {
// 	registry := NewModelRegistry(true)
// 	// Register mock provider for gpt-4
// 	mockProvider := NewMockModelProvider("mock", []ModelInfo{
// 		{
// 			ID:       "gpt-4",
// 			Name:     "gpt-4",
// 			Provider: "mock",
// 		},
// 	})
// 	mockProvider.SetResponse("test", "Mock test response")

// 	err := registry.RegisterProvider(mockProvider)
// 	require.NoError(t, err)

// 	testCases := []struct {
// 		name           string
// 		workflow       *ast.Workflow
// 		inputs         map[string]interface{}
// 		expectError    bool
// 		expectedStatus StepStatus
// 	}{
// 		{
// 			name: "Missing required input",
// 			workflow: &ast.Workflow{
// 				Version:  "1.0",
// 				Metadata: &ast.WorkflowMetadata{Name: "error-test-workflow"},
// 				Agents:   map[string]*ast.Agent{"agent": {Provider: "mock", Model: "gpt-4"}},
// 				Workflow: &ast.WorkflowDef{
// 					Inputs: map[string]*ast.InputParam{
// 						"required": {Type: "string", Required: true},
// 					},
// 					Steps: []*ast.Step{
// 						{ID: "test", Agent: "agent", Prompt: "{{ inputs.required }}"},
// 					},
// 				},
// 			},
// 			inputs:      map[string]interface{}{}, // Missing required input
// 			expectError: true,
// 		},
// 		{
// 			name: "Invalid agent reference",
// 			workflow: &ast.Workflow{
// 				Version:  "1.0",
// 				Metadata: &ast.WorkflowMetadata{Name: "error-test-workflow"},
// 				Agents:   map[string]*ast.Agent{"valid_agent": {Provider: "mock", Model: "gpt-4"}},
// 				Workflow: &ast.WorkflowDef{
// 					Steps: []*ast.Step{
// 						{ID: "test", Agent: "nonexistent_agent", Prompt: "test"},
// 					},
// 				},
// 			},
// 			inputs:         map[string]interface{}{},
// 			expectError:    true,
// 			expectedStatus: StepStatusFailed,
// 		},
// 		{
// 			name: "Missing model in agent",
// 			workflow: &ast.Workflow{
// 				Version:  "1.0",
// 				Metadata: &ast.WorkflowMetadata{Name: "error-test-workflow"},
// 				Agents:   map[string]*ast.Agent{"bad_agent": {}}, // Missing model
// 				Workflow: &ast.WorkflowDef{
// 					Steps: []*ast.Step{
// 						{ID: "test", Agent: "bad_agent", Prompt: "test"},
// 					},
// 				},
// 			},
// 			inputs:         map[string]interface{}{},
// 			expectError:    true,
// 			expectedStatus: StepStatusFailed,
// 		},
// 	}

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			executor, err := NewExecutor(nil, tc.workflow, registry)
// 			require.NoError(t, err)

// 			result, err := executor.Execute(context.Background(), tc.workflow, tc.inputs)

// 			if tc.expectError {
// 				// Should either return an error or have failed steps
// 				if err == nil {
// 					assert.NotEqual(t, ExecutionStatusCompleted, result.Status, "Workflow should not succeed")
// 					if tc.expectedStatus != "" && len(result.Steps) > 0 {
// 						// Check that at least one step has the expected failure status
// 						hasExpectedStatus := false
// 						for _, stepResult := range result.Steps {
// 							if stepResult.Status == tc.expectedStatus {
// 								hasExpectedStatus = true
// 								break
// 							}
// 						}
// 						assert.True(t, hasExpectedStatus, "Expected at least one step with status %v", tc.expectedStatus)
// 					}
// 				}
// 			} else {
// 				assert.NoError(t, err)
// 				assert.Equal(t, ExecutionStatusCompleted, result.Status)
// 			}
// 		})
// 	}
// }

// // TestRuntimeComprehensive_StateManagement tests state persistence and updates
// func TestRuntimeComprehensive_StateManagement(t *testing.T) {
// 	registry := NewModelRegistry(true)

// 	// Register mock provider for gpt-4
// 	mockProvider := NewMockModelProvider("mock", []ModelInfo{
// 		{
// 			ID:       "gpt-4",
// 			Name:     "gpt-4",
// 			Provider: "mock",
// 		},
// 	})
// 	mockProvider.SetResponse("Processing item 1", "Mock item 1 response")
// 	mockProvider.SetResponse("Adding item to list", "Mock item list response")
// 	mockProvider.SetResponse("Updating metadata", "Mock metadata response")

// 	err := registry.RegisterProvider(mockProvider)
// 	require.NoError(t, err)

// 	workflow := &ast.Workflow{
// 		Version: "1.0",
// 		Metadata: &ast.WorkflowMetadata{
// 			Name: "state-test-workflow",
// 		},
// 		Agents: map[string]*ast.Agent{
// 			"state_agent": {
// 				Provider: "mock",
// 				Model:    "gpt-4",
// 			},
// 		},
// 		Workflow: &ast.WorkflowDef{
// 			State: map[string]interface{}{
// 				"counter":    0,
// 				"items":      []string{},
// 				"metadata":   map[string]interface{}{"created": "now"},
// 				"total_cost": 0.0,
// 			},
// 			Steps: []*ast.Step{
// 				{
// 					ID:     "increment_counter",
// 					Agent:  "state_agent",
// 					Prompt: "Processing item 1",
// 					Updates: map[string]interface{}{
// 						"counter":    1,
// 						"total_cost": 0.10,
// 					},
// 				},
// 				{
// 					ID:     "add_item",
// 					Agent:  "state_agent",
// 					Prompt: "Adding item to list",
// 					Updates: map[string]interface{}{
// 						"items": []string{"item1", "item2"},
// 					},
// 				},
// 				{
// 					ID:     "update_metadata",
// 					Agent:  "state_agent",
// 					Prompt: "Updating metadata after {{ steps.increment_counter.output }}",
// 					Updates: map[string]interface{}{
// 						"metadata": map[string]interface{}{
// 							"created":    "{{ state.metadata.created }}",
// 							"updated":    "now",
// 							"step_count": "{{ state.counter }}",
// 						},
// 					},
// 				},
// 			},
// 		},
// 	}

// 	executor, err := NewExecutor(nil, workflow, registry)
// 	require.NoError(t, err)

// 	result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{})
// 	require.NoError(t, err)

// 	// Verify state updates
// 	assert.Equal(t, 1, result.State["counter"])
// 	assert.Equal(t, 0.10, result.State["total_cost"])

// 	// Verify complex state updates
// 	items, ok := result.State["items"].([]string)
// 	assert.True(t, ok)
// 	assert.Equal(t, []string{"item1", "item2"}, items)

// 	metadata, ok := result.State["metadata"].(map[string]interface{})
// 	assert.True(t, ok)
// 	assert.Equal(t, "now", metadata["created"])
// 	assert.Equal(t, "now", metadata["updated"])
// 	assert.Equal(t, "1", metadata["step_count"])
// }

// // TestRuntimeComprehensive_Performance tests runtime performance characteristics
// func TestRuntimeComprehensive_Performance(t *testing.T) {
// 	registry := NewModelRegistry(true)

// 	// Register mock provider for gpt-4
// 	mockProvider := NewMockModelProvider("mock", []ModelInfo{
// 		{
// 			ID:       "gpt-4",
// 			Name:     "gpt-4",
// 			Provider: "mock",
// 		},
// 	})
// 	for i := 0; i < 10; i++ {
// 		mockProvider.SetResponse(fmt.Sprintf("Processing step %d", i), fmt.Sprintf("Mock response for step %d", i))
// 	}

// 	err := registry.RegisterProvider(mockProvider)
// 	require.NoError(t, err)

// 	// Create a workflow with multiple steps to test performance
// 	steps := make([]*ast.Step, 0, 10)
// 	for i := 0; i < 10; i++ {
// 		steps = append(steps, &ast.Step{
// 			ID:     fmt.Sprintf("step_%d", i),
// 			Agent:  "perf_agent",
// 			Prompt: fmt.Sprintf("Processing step %d", i),
// 		})
// 	}

// 	workflow := &ast.Workflow{
// 		Version: "1.0",
// 		Metadata: &ast.WorkflowMetadata{
// 			Name: "performance-test-workflow",
// 		},
// 		Agents: map[string]*ast.Agent{
// 			"perf_agent": {
// 				Provider: "mock",
// 				Model:    "gpt-4",
// 			},
// 		},
// 		Workflow: &ast.WorkflowDef{
// 			Steps: steps,
// 		},
// 	}

// 	// Measure execution time
// 	start := time.Now()
// 	executor, err := NewExecutor(nil, workflow, registry)
// 	require.NoError(t, err)

// 	result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{})
// 	duration := time.Since(start)

// 	require.NoError(t, err)
// 	assert.Equal(t, ExecutionStatusCompleted, result.Status)

// 	// Performance assertions (these may need adjustment based on actual performance)
// 	assert.Less(t, duration, 30*time.Second, "Workflow should complete within 30 seconds")
// 	assert.Equal(t, 10, len(result.Steps), "All steps should be executed")

// 	// Verify no memory leaks by checking that all steps completed
// 	for i := 0; i < 10; i++ {
// 		stepID := fmt.Sprintf("step_%d", i)
// 		found := false
// 		for _, stepResult := range result.Steps {
// 			if stepResult.StepID == stepID {
// 				assert.Equal(t, StepStatusCompleted, stepResult.Status)
// 				found = true
// 				break
// 			}
// 		}
// 		assert.True(t, found, "Step %s should have result", stepID)
// 	}
// }

// // TestRuntimeComprehensive_ConcurrentExecution tests concurrent step execution scenarios
// func TestRuntimeComprehensive_ConcurrentExecution(t *testing.T) {
// 	registry := NewModelRegistry(true)
// 	// Configure executor for concurrent execution
// 	config := &ExecutorConfig{
// 		MaxConcurrentSteps: 3,
// 		DefaultTimeout:     30 * time.Second,
// 		EnableRetries:      true,
// 	}

// 	// Register mock provider for gpt-4
// 	mockProvider := NewMockModelProvider("mock", []ModelInfo{
// 		{
// 			ID:       "gpt-4",
// 			Name:     "gpt-4",
// 			Provider: "mock",
// 		},
// 	})
// 	mockProvider.SetResponse("Independent task 1", "Mock response 1")
// 	mockProvider.SetResponse("Independent task 2", "Mock response 2")
// 	mockProvider.SetResponse("Independent task 3", "Mock response 3")
// 	mockProvider.SetResponse("Depends on: Mock response 1, Mock response 2, Mock response 3", "Mock dependent response")

// 	err := registry.RegisterProvider(mockProvider)
// 	require.NoError(t, err)

// 	workflow := &ast.Workflow{
// 		Version: "1.0",
// 		Metadata: &ast.WorkflowMetadata{
// 			Name: "concurrent-test-workflow",
// 		},
// 		Agents: map[string]*ast.Agent{
// 			"concurrent_agent": {
// 				Provider: "mock",
// 				Model:    "gpt-4",
// 			},
// 		},
// 		Workflow: &ast.WorkflowDef{
// 			Steps: []*ast.Step{
// 				{
// 					ID:     "independent_1",
// 					Agent:  "concurrent_agent",
// 					Prompt: "Independent task 1",
// 				},
// 				{
// 					ID:     "independent_2",
// 					Agent:  "concurrent_agent",
// 					Prompt: "Independent task 2",
// 				},
// 				{
// 					ID:     "independent_3",
// 					Agent:  "concurrent_agent",
// 					Prompt: "Independent task 3",
// 				},
// 				{
// 					ID:     "dependent_step",
// 					Agent:  "concurrent_agent",
// 					Prompt: "Depends on: {{ steps.independent_1.output }}, {{ steps.independent_2.output }}, {{ steps.independent_3.output }}",
// 				},
// 			},
// 		},
// 	}

// 	start := time.Now()
// 	executor, err := NewExecutor(config, workflow, registry)
// 	require.NoError(t, err)

// 	result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{})
// 	duration := time.Since(start)

// 	require.NoError(t, err)
// 	assert.Equal(t, ExecutionStatusCompleted, result.Status)

// 	// With concurrency, execution should be faster than sequential
// 	// This is a rough test - in practice, you'd measure against a sequential baseline
// 	assert.Less(t, duration, 25*time.Second, "Concurrent execution should be reasonably fast")

// 	// All steps should complete successfully
// 	assert.Equal(t, 4, len(result.Steps))
// 	for _, stepResult := range result.Steps {
// 		assert.Equal(t, StepStatusCompleted, stepResult.Status, "Step %s should complete", stepResult.StepID)
// 	}
// }

// // TestRuntimeComprehensive_HTTPMocked tests with real HTTP calls to mock servers
// func TestRuntimeComprehensive_HTTPMocked(t *testing.T) {
// 	// Create mock OpenAI server
// 	openaiServer := createMockOpenAIServer(t, func(prompt string) {})
// 	defer openaiServer.Close()

// 	// Create mock Anthropic server
// 	anthropicServer := createMockAnthropicServer(t)
// 	defer anthropicServer.Close()

// 	// Set environment variables to point to our mock servers
// 	t.Setenv("OPENAI_API_KEY", "test-openai-key")
// 	t.Setenv("OPENAI_BASE_URL", openaiServer.URL)
// 	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
// 	t.Setenv("ANTHROPIC_BASE_URL", anthropicServer.URL)

// 	// Test OpenAI workflow
// 	t.Run("OpenAI GPT-4 workflow", func(t *testing.T) {
// 		workflow := createTestWorkflow("gpt-4", "openai_agent")
// 		// Create executor (will register providers with our mock URLs)
// 		executor, err := NewExecutor(nil, workflow, nil)
// 		require.NoError(t, err)
// 		result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{
// 			"topic": "artificial intelligence",
// 		})

// 		require.NoError(t, err)
// 		if !assert.Equal(t, ExecutionStatusCompleted, result.Status) {
// 			for _, step := range result.Steps {
// 				assert.Nil(t, step.Error)
// 			}
// 		}
// 		assert.Equal(t, 3, len(result.Steps))

// 		// Verify responses match what we mocked
// 		step1 := findStepByID(result.Steps, "step1")
// 		assert.Equal(t, "Mock GPT-4 response for: Process this topic: artificial intelligence", step1.Response)

// 		step2 := findStepByID(result.Steps, "step2")
// 		assert.Contains(t, step2.Response, "Mock GPT-4 response for: Build on:")

// 		// Verify state updates worked
// 		assert.Equal(t, 1, result.State["counter"])
// 		assert.Equal(t, "completed", result.State["status"])
// 	})

// 	// Test Anthropic workflow
// 	t.Run("Anthropic Claude workflow", func(t *testing.T) {
// 		registry := NewModelRegistry(true)

// 		// Register mock Anthropic provider since real provider requires API key
// 		mockAnthropic := NewMockModelProvider("anthropic", []ModelInfo{
// 			{
// 				ID:       "claude-3-sonnet",
// 				Name:     "claude-3-sonnet",
// 				Provider: "anthropic",
// 			},
// 		})
// 		mockAnthropic.SetResponse("Process this topic: machine learning", "Mock Claude response for: Process this topic: machine learning")
// 		mockAnthropic.SetResponse("Build on: Mock Claude response for: Process this topic: machine learning", "Mock Claude extended response")
// 		mockAnthropic.SetResponse("Finalize: Mock Claude extended response", "Mock Claude final response")

// 		err := registry.RegisterProvider(mockAnthropic)
// 		require.NoError(t, err)

// 		workflow := createTestWorkflow("claude-3-sonnet", "claude_agent")
// 		testExecutor, err := NewExecutor(nil, workflow, registry)
// 		require.NoError(t, err)

// 		result, err := testExecutor.Execute(context.Background(), workflow, map[string]interface{}{
// 			"topic": "machine learning",
// 		})

// 		require.NoError(t, err)
// 		assert.Equal(t, ExecutionStatusCompleted, result.Status)
// 		assert.Equal(t, 3, len(result.Steps))

// 		// Verify Claude responses
// 		step1 := findStepByID(result.Steps, "step1")
// 		assert.Equal(t, "Mock Claude response for: Process this topic: machine learning", step1.Response)
// 	})
// }

// // TestRuntimeComprehensive_HTTPErrorHandling tests HTTP error scenarios
// func TestRuntimeComprehensive_HTTPErrorHandling(t *testing.T) {
// 	testCases := []struct {
// 		name           string
// 		serverHandler  func(w http.ResponseWriter, r *http.Request)
// 		expectedStatus ExecutionStatus
// 		expectedError  string
// 	}{
// 		{
// 			name: "OpenAI Rate Limit Error",
// 			serverHandler: func(w http.ResponseWriter, r *http.Request) {
// 				w.Header().Set("Content-Type", "application/json")
// 				w.WriteHeader(http.StatusTooManyRequests)
// 				json.NewEncoder(w).Encode(map[string]interface{}{
// 					"error": map[string]interface{}{
// 						"type":    "rate_limit_exceeded",
// 						"message": "Rate limit exceeded",
// 					},
// 				})
// 			},
// 			expectedStatus: ExecutionStatusFailed,
// 			expectedError:  "rate_limit_exceeded",
// 		},
// 		{
// 			name: "OpenAI Invalid API Key",
// 			serverHandler: func(w http.ResponseWriter, r *http.Request) {
// 				w.Header().Set("Content-Type", "application/json")
// 				w.WriteHeader(http.StatusUnauthorized)
// 				json.NewEncoder(w).Encode(map[string]interface{}{
// 					"error": map[string]interface{}{
// 						"type":    "invalid_request_error",
// 						"message": "Incorrect API key provided",
// 					},
// 				})
// 			},
// 			expectedStatus: ExecutionStatusFailed,
// 			expectedError:  "invalid_request_error",
// 		},
// 		{
// 			name: "Server Internal Error",
// 			serverHandler: func(w http.ResponseWriter, r *http.Request) {
// 				w.WriteHeader(http.StatusInternalServerError)
// 				w.Write([]byte("Internal server error"))
// 			},
// 			expectedStatus: ExecutionStatusFailed,
// 			expectedError:  "500",
// 		},
// 	}

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			// Create server with specific error handler
// 			server := httptest.NewServer(http.HandlerFunc(tc.serverHandler))
// 			defer server.Close()

// 			// Set environment variables BEFORE creating executor
// 			t.Setenv("OPENAI_API_KEY", "test-key")
// 			t.Setenv("OPENAI_BASE_URL", server.URL)

// 			// Create executor after setting environment variables
// 			workflow := createTestWorkflow("gpt-4", "error_agent")
// 			executor, err := NewExecutor(nil, workflow, nil)
// 			require.NoError(t, err)

// 			result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{
// 				"topic": "test",
// 			})

// 			// Should complete execution but with failed status
// 			require.NoError(t, err)
// 			assert.Equal(t, tc.expectedStatus, result.Status)

// 			// Check that error is captured in step result
// 			step1 := findStepByID(result.Steps, "step1")
// 			assert.Equal(t, StepStatusFailed, step1.Status)
// 			if tc.expectedError != "" {
// 				assert.Contains(t, step1.Error.Error(), tc.expectedError)
// 			}
// 		})
// 	}
// }

// // TestRuntimeComprehensive_HTTPRetryLogic tests retry behavior with HTTP failures
// func TestRuntimeComprehensive_HTTPRetryLogic(t *testing.T) {
// 	callCount := 0
// 	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		callCount++

// 		// Fail first 2 attempts, succeed on 3rd
// 		if callCount < 3 {
// 			w.WriteHeader(http.StatusServiceUnavailable)
// 			return
// 		}

// 		// Success response
// 		w.Header().Set("Content-Type", "application/json")
// 		json.NewEncoder(w).Encode(map[string]interface{}{
// 			"choices": []map[string]interface{}{
// 				{
// 					"message": map[string]interface{}{
// 						"content": fmt.Sprintf("Success after %d attempts", callCount),
// 					},
// 					"finish_reason": "stop",
// 				},
// 			},
// 			"usage": map[string]interface{}{
// 				"prompt_tokens":     10,
// 				"completion_tokens": 20,
// 				"total_tokens":      30,
// 			},
// 		})
// 	}))
// 	defer server.Close()

// 	t.Setenv("OPENAI_API_KEY", "test-key")
// 	t.Setenv("OPENAI_BASE_URL", server.URL)

// 	// Create a single-step workflow for retry testing
// 	workflow := &ast.Workflow{
// 		Version: "1.0",
// 		Metadata: &ast.WorkflowMetadata{
// 			Name: "retry-test-workflow",
// 		},
// 		Agents: map[string]*ast.Agent{
// 			"retry_agent": {
// 				Provider:    "openai",
// 				Model:       "gpt-4",
// 				Temperature: floatPtr(0.7),
// 			},
// 		},
// 		Workflow: &ast.WorkflowDef{
// 			Inputs: map[string]*ast.InputParam{
// 				"topic": {Type: "string", Required: true},
// 			},
// 			Steps: []*ast.Step{
// 				{
// 					ID:     "retry_step",
// 					Agent:  "retry_agent",
// 					Prompt: "Process: {{ inputs.topic }}",
// 				},
// 			},
// 		},
// 	}

// 	executor, err := NewExecutor(nil, workflow, nil)
// 	require.NoError(t, err)

// 	result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{
// 		"topic": "retry test",
// 	})

// 	require.NoError(t, err)
// 	assert.Equal(t, ExecutionStatusCompleted, result.Status)

// 	// Verify retry worked - should have made 3 calls
// 	assert.Equal(t, 3, callCount)

// 	retryStep := findStepByID(result.Steps, "retry_step")
// 	assert.Equal(t, StepStatusCompleted, retryStep.Status)
// 	assert.Equal(t, "Success after 3 attempts", retryStep.Response)
// }

// // TestRuntimeComprehensive_HTTPConcurrentRequests tests concurrent HTTP requests
// func TestRuntimeComprehensive_HTTPConcurrentRequests(t *testing.T) {
// 	requestTimes := make(map[string]time.Time)
// 	var requestsMutex sync.Mutex

// 	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		switch r.URL.Path {
// 		case "/v1/models":
// 			w.Header().Set("Content-Type", "application/json")
// 			json.NewEncoder(w).Encode(map[string]interface{}{
// 				"object": "list",
// 				"data": []map[string]interface{}{
// 					{
// 						"id":       "gpt-4",
// 						"object":   "model",
// 						"created":  1677610602,
// 						"owned_by": "openai",
// 					},
// 					{
// 						"id":       "gpt-3.5-turbo",
// 						"object":   "model",
// 						"created":  1677610602,
// 						"owned_by": "openai",
// 					},
// 				},
// 			})
// 		case "/chat/completions":
// 			// Record when each request started
// 			requestsMutex.Lock()
// 			requestID := fmt.Sprintf("req_%d", len(requestTimes))
// 			requestTimes[requestID] = time.Now()
// 			requestsMutex.Unlock()
// 			// Simulate some processing time
// 			time.Sleep(100 * time.Millisecond)

// 			// Return response
// 			w.Header().Set("Content-Type", "application/json")
// 			json.NewEncoder(w).Encode(map[string]interface{}{
// 				"choices": []map[string]interface{}{
// 					{
// 						"message": map[string]interface{}{
// 							"content": fmt.Sprintf("Concurrent response %s", requestID),
// 						},
// 						"finish_reason": "stop",
// 					},
// 				},
// 				"usage": map[string]interface{}{
// 					"prompt_tokens":     5,
// 					"completion_tokens": 10,
// 					"total_tokens":      15,
// 				},
// 			})
// 		}
// 	}))
// 	defer server.Close()

// 	t.Setenv("OPENAI_API_KEY", "test-key")
// 	t.Setenv("OPENAI_BASE_URL", server.URL)

// 	// Configure executor for concurrent execution
// 	config := &ExecutorConfig{
// 		MaxConcurrentSteps: 3,
// 		DefaultTimeout:     10 * time.Second,
// 	}

// 	// Create workflow with independent parallel steps
// 	workflow := &ast.Workflow{
// 		Version: "1.0",
// 		Metadata: &ast.WorkflowMetadata{
// 			Name: "concurrent-test-workflow",
// 		},
// 		Agents: map[string]*ast.Agent{
// 			"concurrent_agent": {
// 				Provider: "openai",
// 				Model:    "gpt-4",
// 			},
// 		},
// 		Workflow: &ast.WorkflowDef{
// 			Steps: []*ast.Step{
// 				{ID: "parallel1", Agent: "concurrent_agent", Prompt: "Task 1"},
// 				{ID: "parallel2", Agent: "concurrent_agent", Prompt: "Task 2"},
// 				{ID: "parallel3", Agent: "concurrent_agent", Prompt: "Task 3"},
// 			},
// 		},
// 	}

// 	executor, err := NewExecutor(config, workflow, nil)
// 	require.NoError(t, err)

// 	start := time.Now()
// 	result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{})
// 	duration := time.Since(start)

// 	require.NoError(t, err)
// 	if !assert.Equal(t, ExecutionStatusCompleted, result.Status) {
// 		for _, step := range result.Steps {
// 			assert.Nil(t, step.Error)
// 		}
// 	}

// 	// With concurrency, should complete faster than 3 sequential requests (300ms)
// 	assert.Less(t, duration, 250*time.Millisecond, "Concurrent execution should be faster than sequential")

// 	// Should have made 3 concurrent requests
// 	assert.Equal(t, 3, len(requestTimes))
// }

// // TestRuntimeComprehensive_HTTPVariableInterpolation tests variable substitution in HTTP requests
// func TestRuntimeComprehensive_HTTPVariableInterpolation(t *testing.T) {
// 	var capturedPrompts []string
// 	server := createMockOpenAIServer(t, func(prompt string) {
// 		capturedPrompts = append(capturedPrompts, prompt)
// 	})
// 	defer server.Close()
// 	// Set environment variables to point to our mock servers
// 	t.Setenv("OPENAI_API_KEY", "test-openai-key")
// 	t.Setenv("OPENAI_BASE_URL", server.URL)

// 	workflow := &ast.Workflow{
// 		Version: "1.0",
// 		Metadata: &ast.WorkflowMetadata{
// 			Name: "interpolation-test-workflow",
// 		},
// 		Agents: map[string]*ast.Agent{
// 			"interpolation_agent": {
// 				Provider: "openai",
// 				Model:    "gpt-4",
// 			},
// 		},
// 		Workflow: &ast.WorkflowDef{
// 			Inputs: map[string]*ast.InputParam{
// 				"name":  {Type: "string", Required: true},
// 				"topic": {Type: "string", Required: true},
// 			},
// 			State: map[string]interface{}{
// 				"prefix": "Hello",
// 			},
// 			Steps: []*ast.Step{
// 				{
// 					ID:     "greeting",
// 					Agent:  "interpolation_agent",
// 					Prompt: "{{ state.prefix }}, {{ inputs.name }}! Let's discuss {{ inputs.topic }}.",
// 				},
// 				{
// 					ID:     "follow_up",
// 					Agent:  "interpolation_agent",
// 					Prompt: "Based on the greeting: {{ steps.greeting.output }}, continue the conversation about {{ inputs.topic }}.",
// 				},
// 			},
// 		},
// 	}

// 	executor, err := NewExecutor(nil, workflow, nil)
// 	require.NoError(t, err)

// 	result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{
// 		"name":  "Alice",
// 		"topic": "machine learning",
// 	})

// 	require.NoError(t, err)
// 	if !assert.Equal(t, ExecutionStatusCompleted, result.Status) {
// 		for _, step := range result.Steps {
// 			assert.Nil(t, step.Error)
// 		}
// 	}

// 	// Verify that variable interpolation worked in the actual HTTP requests
// 	require.Equal(t, 2, len(capturedPrompts))
// 	assert.Equal(t, "Hello, Alice! Let's discuss machine learning.", capturedPrompts[0])
// 	assert.Contains(t, capturedPrompts[1], "Based on the greeting: Mock GPT-4 response for: Hello, Alice! Let's discuss machine learning., continue the conversation about machine learning.")
// }

// // Helper functions for HTTP tests

// func createMockOpenAIServer(t *testing.T, callback func(prompt string)) *httptest.Server {
// 	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Verify authorization header is present
// 		assert.Equal(t, "Bearer test-openai-key", r.Header.Get("Authorization"))

// 		// Handle different endpoints
// 		switch {
// 		case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/models"):
// 			// Mock the models endpoint
// 			w.Header().Set("Content-Type", "application/json")
// 			json.NewEncoder(w).Encode(map[string]interface{}{
// 				"object": "list",
// 				"data": []map[string]interface{}{
// 					{
// 						"id":       "gpt-4",
// 						"object":   "model",
// 						"created":  1677610602,
// 						"owned_by": "openai",
// 					},
// 					{
// 						"id":       "gpt-3.5-turbo",
// 						"object":   "model",
// 						"created":  1677610602,
// 						"owned_by": "openai",
// 					},
// 				},
// 			})

// 		case r.Method == "POST" && strings.Contains(r.URL.Path, "/chat/completions"):
// 			// Handle chat completions
// 			// Parse request to get prompt
// 			var reqBody map[string]interface{}
// 			err := json.NewDecoder(r.Body).Decode(&reqBody)
// 			require.NoError(t, err)

// 			prompt := "unknown prompt"
// 			if messages, ok := reqBody["messages"].([]interface{}); ok && len(messages) > 0 {
// 				if msg, ok := messages[0].(map[string]interface{}); ok {
// 					if content, ok := msg["content"].(string); ok {
// 						prompt = content
// 					}
// 				}
// 			}

// 			callback(prompt)

// 			// Return mock OpenAI response
// 			w.Header().Set("Content-Type", "application/json")
// 			json.NewEncoder(w).Encode(map[string]interface{}{
// 				"choices": []map[string]interface{}{
// 					{
// 						"message": map[string]interface{}{
// 							"content": fmt.Sprintf("Mock GPT-4 response for: %s", prompt),
// 						},
// 						"finish_reason": "stop",
// 					},
// 				},
// 				"usage": map[string]interface{}{
// 					"prompt_tokens":     len(strings.Split(prompt, " ")),
// 					"completion_tokens": 20,
// 					"total_tokens":      len(strings.Split(prompt, " ")) + 20,
// 				},
// 			})

// 		default:
// 			// Unhandled endpoint
// 			w.WriteHeader(http.StatusNotFound)
// 			w.Write([]byte(fmt.Sprintf("Unhandled endpoint: %s %s", r.Method, r.URL.Path)))
// 		}
// 	}))
// }

// func createMockAnthropicServer(t *testing.T) *httptest.Server {
// 	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Verify authorization header is present
// 		assert.Equal(t, "test-anthropic-key", r.Header.Get("x-api-key"))

// 		// Handle different endpoints
// 		switch {
// 		case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/models"):
// 			// Mock the models endpoint
// 			w.Header().Set("Content-Type", "application/json")
// 			json.NewEncoder(w).Encode(map[string]interface{}{
// 				"data": []map[string]interface{}{
// 					{
// 						"id":           "claude-3-5-sonnet-20241022",
// 						"display_name": "Claude 3.5 Sonnet",
// 						"created_at":   "2024-10-22T00:00:00Z",
// 						"type":         "message",
// 					},
// 					{
// 						"id":           "claude-3-opus-20240229",
// 						"display_name": "Claude 3 Opus",
// 						"created_at":   "2024-02-29T00:00:00Z",
// 						"type":         "message",
// 					},
// 					{
// 						"id":           "claude-3-sonnet-20240229",
// 						"display_name": "Claude 3 Sonnet",
// 						"created_at":   "2024-02-29T00:00:00Z",
// 						"type":         "message",
// 					},
// 					{
// 						"id":           "claude-3-haiku-20240307",
// 						"display_name": "Claude 3 Haiku",
// 						"created_at":   "2024-03-07T00:00:00Z",
// 						"type":         "message",
// 					},
// 				},
// 				"has_more": false,
// 			})

// 		case r.Method == "POST" && strings.Contains(r.URL.Path, "/v1/messages"):
// 			// Handle messages endpoint
// 			// Parse request to get prompt
// 			var reqBody map[string]interface{}
// 			err := json.NewDecoder(r.Body).Decode(&reqBody)
// 			require.NoError(t, err)

// 			prompt := "unknown prompt"
// 			if messages, ok := reqBody["messages"].([]interface{}); ok && len(messages) > 0 {
// 				if msg, ok := messages[0].(map[string]interface{}); ok {
// 					if content, ok := msg["content"].([]interface{}); ok && len(content) > 0 {
// 						if contentItem, ok := content[0].(map[string]interface{}); ok {
// 							if text, ok := contentItem["text"].(string); ok {
// 								prompt = text
// 							}
// 						}
// 					}
// 				}
// 			}

// 			// Return mock Anthropic response
// 			w.Header().Set("Content-Type", "application/json")
// 			json.NewEncoder(w).Encode(map[string]interface{}{
// 				"id":   "msg_test123",
// 				"type": "message",
// 				"role": "assistant",
// 				"content": []map[string]interface{}{
// 					{
// 						"type": "text",
// 						"text": fmt.Sprintf("Mock Claude response for: %s", prompt),
// 					},
// 				},
// 				"model":       "claude-3-5-sonnet-20241022",
// 				"stop_reason": "end_turn",
// 				"usage": map[string]interface{}{
// 					"input_tokens":  len(strings.Split(prompt, " ")),
// 					"output_tokens": 15,
// 				},
// 			})

// 		default:
// 			// Unhandled endpoint
// 			w.WriteHeader(http.StatusNotFound)
// 			w.Write([]byte(fmt.Sprintf("Unhandled endpoint: %s %s", r.Method, r.URL.Path)))
// 		}
// 	}))
// }

// func createTestWorkflow(model, agentName string) *ast.Workflow {
// 	// Determine provider based on model
// 	provider := "openai"
// 	if strings.Contains(model, "claude") {
// 		provider = "anthropic"
// 	}

// 	return &ast.Workflow{
// 		Version: "1.0",
// 		Metadata: &ast.WorkflowMetadata{
// 			Name: "http-test-workflow",
// 		},
// 		Agents: map[string]*ast.Agent{
// 			agentName: {
// 				Provider:    provider,
// 				Model:       model,
// 				Temperature: floatPtr(0.7),
// 			},
// 		},
// 		Workflow: &ast.WorkflowDef{
// 			Inputs: map[string]*ast.InputParam{
// 				"topic": {Type: "string", Required: true},
// 			},
// 			State: map[string]interface{}{
// 				"counter": 0,
// 				"status":  "initialized",
// 			},
// 			Steps: []*ast.Step{
// 				{
// 					ID:     "step1",
// 					Agent:  agentName,
// 					Prompt: "Process this topic: {{ inputs.topic }}",
// 				},
// 				{
// 					ID:     "step2",
// 					Agent:  agentName,
// 					Prompt: "Build on: {{ steps.step1.output }}",
// 					Updates: map[string]interface{}{
// 						"counter": 1,
// 						"status":  "processing",
// 					},
// 				},
// 				{
// 					ID:     "step3",
// 					Agent:  agentName,
// 					Prompt: "Finalize: {{ steps.step2.output }}",
// 					Updates: map[string]interface{}{
// 						"status": "completed",
// 					},
// 				},
// 			},
// 		},
// 	}
// }

// func findStepByID(steps []StepResult, stepID string) *StepResult {
// 	for i := range steps {
// 		if steps[i].StepID == stepID {
// 			return &steps[i]
// 		}
// 	}
// 	return nil
// }

// // Helper functions
// func floatPtr(f float64) *float64 {
// 	return &f
// }

// func boolPtr(b bool) *bool {
// 	return &b
// }
