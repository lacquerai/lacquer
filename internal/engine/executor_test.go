package engine

import (
	"context"
	"sync"
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/provider"
	"github.com/lacquerai/lacquer/internal/schema"
	pkgEvents "github.com/lacquerai/lacquer/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestWorkflow(steps []*ast.Step) *ast.Workflow {
	return &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name:        "Test Workflow",
			Description: "A workflow for testing",
		},
		Workflow: &ast.WorkflowDef{
			Steps: steps,
		},
	}
}

func createTestExecutionContext(workflow *ast.Workflow) *execcontext.ExecutionContext {
	ctx := context.Background()

	execCtx := execcontext.NewExecutionContext(
		execcontext.RunContext{Context: ctx},
		workflow,
		map[string]interface{}{}, // inputs
		"/tmp",                   // working directory
	)

	return execCtx
}

func createMockExecutor(workflow *ast.Workflow) (WorkflowExecutor, error) {
	config := DefaultExecutorConfig()
	config.MaxConcurrentSteps = 1 // Sequential execution for deterministic tests

	registry := provider.NewRegistry(false)

	mockModels := []provider.Info{
		{ID: "test-model", Name: "Test Model"},
		{ID: "gpt-4", Name: "GPT-4"},
		{ID: "claude-3", Name: "Claude 3"},
	}
	mockProvider := provider.NewMockProvider("anthropic", mockModels)
	mockProvider.SetResponse("test prompt", "test response")
	mockProvider.SetResponse("Hello, world!", "Hello from test agent!")

	err := registry.RegisterProvider(mockProvider)
	if err != nil {
		return nil, err
	}

	runner := &Runner{}

	ctx := execcontext.RunContext{
		Context: context.Background(),
	}

	return NewExecutor(ctx, config, workflow, registry, runner)
}

type safeEventCollector struct {
	events []pkgEvents.ExecutionEvent
	mu     sync.RWMutex
	done   chan struct{}
}

func (s *safeEventCollector) getEvents() []pkgEvents.ExecutionEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy to avoid race conditions
	eventsCopy := make([]pkgEvents.ExecutionEvent, len(s.events))
	copy(eventsCopy, s.events)
	return eventsCopy
}

func (s *safeEventCollector) waitForCompletion() {
	<-s.done
}

func collectProgressEvents() (chan pkgEvents.ExecutionEvent, *safeEventCollector) {
	eventsChan := make(chan pkgEvents.ExecutionEvent, 100)
	collector := &safeEventCollector{
		events: make([]pkgEvents.ExecutionEvent, 0),
		done:   make(chan struct{}),
	}

	go func() {
		for event := range eventsChan {
			collector.mu.Lock()
			collector.events = append(collector.events, event)
			collector.mu.Unlock()
		}

		collector.done <- struct{}{}
		close(collector.done)
	}()

	return eventsChan, collector
}

func TestExecuteWorkflow_BasicScriptStep(t *testing.T) {
	steps := []*ast.Step{
		{
			ID:  "test_script",
			Run: "echo 'Hello, World!'",
		},
	}

	workflow := createTestWorkflow(steps)
	execCtx := createTestExecutionContext(workflow)

	executor, err := createMockExecutor(workflow)
	require.NoError(t, err)

	eventsChan, collector := collectProgressEvents()

	err = executor.ExecuteWorkflow(execCtx, eventsChan)
	close(eventsChan)
	require.NoError(t, err)

	result, exists := execCtx.GetStepResult("test_script")
	require.True(t, exists)
	require.NotNil(t, result)
	assert.Equal(t, execcontext.StepStatusCompleted, result.Status)
	assert.Equal(t, "Hello, World!\n", result.Response)
	assert.NoError(t, result.Error)

	collector.waitForCompletion()
	events := collector.getEvents()
	assert.True(t, len(events) > 0, "Expected events to be generated")

	hasWorkflowStarted := false
	hasWorkflowCompleted := false
	hasStepStarted := false
	hasStepCompleted := false

	for _, event := range events {
		switch event.Type {
		case pkgEvents.EventWorkflowStarted:
			hasWorkflowStarted = true
		case pkgEvents.EventWorkflowCompleted:
			hasWorkflowCompleted = true
		case pkgEvents.EventStepStarted:
			hasStepStarted = true
		case pkgEvents.EventStepCompleted:
			hasStepCompleted = true
		}
	}

	assert.True(t, hasWorkflowStarted, "Expected workflow started event")
	assert.True(t, hasWorkflowCompleted, "Expected workflow completed event")
	assert.True(t, hasStepStarted, "Expected step started event")
	assert.True(t, hasStepCompleted, "Expected step completed event")
}

func TestExecuteWorkflow_AgentStep(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name:        "Agent Test Workflow",
			Description: "Testing agent step execution",
		},
		Agents: map[string]*ast.Agent{
			"test_agent": {
				Name:         "test_agent",
				Provider:     "anthropic",
				Model:        "test-model",
				SystemPrompt: "You are a helpful assistant.",
			},
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{
					ID:     "agent_step",
					Agent:  "test_agent",
					Prompt: "Hello, world!",
				},
			},
		},
	}

	execCtx := createTestExecutionContext(workflow)

	executor, err := createMockExecutor(workflow)
	require.NoError(t, err)

	eventsChan, collector := collectProgressEvents()

	err = executor.ExecuteWorkflow(execCtx, eventsChan)
	close(eventsChan)
	require.NoError(t, err)

	result, exists := execCtx.GetStepResult("agent_step")
	require.True(t, exists)
	require.NotNil(t, result)
	assert.Equal(t, execcontext.StepStatusCompleted, result.Status)
	assert.Equal(t, "Hello from test agent!", result.Response)
	assert.NoError(t, result.Error)
	assert.NotEmpty(t, result.Response)

	collector.waitForCompletion()
	events := collector.getEvents()
	assert.True(t, len(events) > 0)
}

func TestExecuteWorkflow_ContainerStep(t *testing.T) {
	steps := []*ast.Step{
		{
			ID:        "container_step",
			Container: "alpine:latest",
			Command:   []string{"echo", "Hello from container"},
		},
	}

	workflow := createTestWorkflow(steps)
	execCtx := createTestExecutionContext(workflow)

	executor, err := createMockExecutor(workflow)
	require.NoError(t, err)

	eventsChan, _ := collectProgressEvents()

	err = executor.ExecuteWorkflow(execCtx, eventsChan)
	close(eventsChan)
	require.NoError(t, err)

	result, exists := execCtx.GetStepResult("container_step")
	require.True(t, exists)
	require.NotNil(t, result)
	assert.Equal(t, execcontext.StepStatusCompleted, result.Status)
	assert.Equal(t, "Hello from container\n", result.Response)
	assert.NoError(t, result.Error)
}

func TestExecuteWorkflow_MultipleSteps(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name: "Multi-Step Test Workflow",
		},
		Agents: map[string]*ast.Agent{
			"test_agent": {
				Name:     "test_agent",
				Provider: "anthropic",
				Model:    "test-model",
			},
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{
					ID:  "script_step",
					Run: "echo 'First step'",
				},
				{
					ID:     "agent_step",
					Agent:  "test_agent",
					Prompt: "test prompt",
				},
				{
					ID:  "final_script",
					Run: "echo 'Final step'",
				},
			},
		},
	}

	execCtx := createTestExecutionContext(workflow)

	executor, err := createMockExecutor(workflow)
	require.NoError(t, err)

	eventsChan, _ := collectProgressEvents()
	err = executor.ExecuteWorkflow(execCtx, eventsChan)
	close(eventsChan)
	require.NoError(t, err)

	scriptResult, exists := execCtx.GetStepResult("script_step")
	require.True(t, exists)
	require.NotNil(t, scriptResult)
	assert.Equal(t, execcontext.StepStatusCompleted, scriptResult.Status)

	agentResult, agentExists := execCtx.GetStepResult("agent_step")
	require.True(t, agentExists)
	require.NotNil(t, agentResult)
	assert.Equal(t, execcontext.StepStatusCompleted, agentResult.Status)

	finalResult, finalExists := execCtx.GetStepResult("final_script")
	require.True(t, finalExists)
	require.NotNil(t, finalResult)
	assert.Equal(t, execcontext.StepStatusCompleted, finalResult.Status)
}

func TestExecuteWorkflow_SkipConditions(t *testing.T) {
	t.Run("SkipIf condition skips step", func(t *testing.T) {
		steps := []*ast.Step{
			{
				ID:     "skipped_step",
				Run:    "echo 'This should be skipped'",
				SkipIf: "true", // Always skip
			},
			{
				ID:  "executed_step",
				Run: "echo 'This should execute'",
			},
		}

		workflow := createTestWorkflow(steps)
		execCtx := createTestExecutionContext(workflow)

		executor, err := createMockExecutor(workflow)
		require.NoError(t, err)

		eventsChan, _ := collectProgressEvents()
		err = executor.ExecuteWorkflow(execCtx, eventsChan)
		close(eventsChan)
		require.NoError(t, err)

		skippedResult, exists := execCtx.GetStepResult("skipped_step")
		require.True(t, exists)
		require.NotNil(t, skippedResult)
		assert.Equal(t, execcontext.StepStatusSkipped, skippedResult.Status)

		executedResult, exists := execCtx.GetStepResult("executed_step")
		require.True(t, exists)
		require.NotNil(t, executedResult)
		assert.Equal(t, execcontext.StepStatusCompleted, executedResult.Status)
	})

	t.Run("Condition false skips step", func(t *testing.T) {
		steps := []*ast.Step{
			{
				ID:        "conditional_step",
				Run:       "echo 'This should be skipped'",
				Condition: "false", // Skip when false
			},
		}

		workflow := createTestWorkflow(steps)
		execCtx := createTestExecutionContext(workflow)

		executor, err := createMockExecutor(workflow)
		require.NoError(t, err)

		eventsChan, _ := collectProgressEvents()

		err = executor.ExecuteWorkflow(execCtx, eventsChan)
		close(eventsChan)
		require.NoError(t, err)

		result, exists := execCtx.GetStepResult("conditional_step")
		require.True(t, exists)
		require.NotNil(t, result)
		assert.Equal(t, execcontext.StepStatusSkipped, result.Status)
	})
}

func TestExecuteWorkflow_StateUpdates(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			State: map[string]interface{}{
				"counter": 0,
			},
			Steps: []*ast.Step{
				{
					ID:  "update_state",
					Run: "echo 'Updating state'",
					Updates: map[string]interface{}{
						"counter":   1,
						"message":   "Hello from step",
						"timestamp": "{{ now }}",
					},
				},
				{
					ID:  "check_state",
					Run: "echo 'Counter is {{ state.counter }}'",
				},
			},
		},
	}

	execCtx := createTestExecutionContext(workflow)

	executor, err := createMockExecutor(workflow)
	require.NoError(t, err)

	eventsChan, _ := collectProgressEvents()

	err = executor.ExecuteWorkflow(execCtx, eventsChan)
	close(eventsChan)
	require.NoError(t, err)

	// Verify both steps completed
	updateResult, exists := execCtx.GetStepResult("update_state")
	require.True(t, exists)
	require.NotNil(t, updateResult)
	assert.Equal(t, execcontext.StepStatusCompleted, updateResult.Status)

	checkResult, checkExists := execCtx.GetStepResult("check_state")
	require.True(t, checkExists)
	require.NotNil(t, checkResult)
	assert.Equal(t, execcontext.StepStatusCompleted, checkResult.Status)

	// Verify state was updated
	counter, counterExists := execCtx.GetState("counter")
	assert.True(t, counterExists)
	assert.Equal(t, 1, counter)
	message, messageExists := execCtx.GetState("message")
	assert.True(t, messageExists)
	assert.Equal(t, "Hello from step", message)
}

func TestExecuteWorkflow_WhileLoop(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			State: map[string]interface{}{
				"counter": 0,
			},
			Steps: []*ast.Step{
				{
					ID:    "while_loop",
					While: "${{ state.counter < 3 }}",
					Steps: []*ast.Step{
						{
							ID:  "inner_step",
							Run: "echo ${{ state.counter }}",
							Updates: map[string]interface{}{
								"counter": "${{ state.counter + 1 }}",
							},
						},
					},
				},
				{
					ID:  "after_while",
					Run: "echo 'After while loop ${{ steps.while_loop.steps.inner_step.output }}'",
				},
			},
		},
	}

	execCtx := createTestExecutionContext(workflow)

	executor, err := createMockExecutor(workflow)
	require.NoError(t, err)

	eventsChan, _ := collectProgressEvents()

	err = executor.ExecuteWorkflow(execCtx, eventsChan)
	close(eventsChan)
	require.NoError(t, err)

	result, exists := execCtx.GetStepResult("while_loop")
	require.True(t, exists)
	require.NotNil(t, result)
	assert.Equal(t, execcontext.StepStatusCompleted, result.Status)

	afterResult, exists := execCtx.GetStepResult("after_while")
	require.True(t, exists)
	require.NotNil(t, afterResult)
	assert.Equal(t, execcontext.StepStatusCompleted, afterResult.Status)
	assert.Equal(t, "After while loop 2\n\n", afterResult.Response)
}

func TestExecuteWorkflow_ErrorHandling(t *testing.T) {
	t.Run("Script step failure", func(t *testing.T) {
		steps := []*ast.Step{
			{
				ID:  "failing_step",
				Run: "exit 1", // This will fail
			},
			{
				ID:  "should_not_execute",
				Run: "echo 'This should not run'",
			},
		}

		workflow := createTestWorkflow(steps)
		execCtx := createTestExecutionContext(workflow)

		executor, err := createMockExecutor(workflow)
		require.NoError(t, err)

		eventsChan, collector := collectProgressEvents()

		err = executor.ExecuteWorkflow(execCtx, eventsChan)
		close(eventsChan)
		require.Error(t, err)

		failedResult, exists := execCtx.GetStepResult("failing_step")
		require.True(t, exists)
		require.NotNil(t, failedResult)
		assert.Equal(t, execcontext.StepStatusFailed, failedResult.Status)
		assert.Error(t, failedResult.Error)

		secondResult, _ := execCtx.GetStepResult("should_not_execute")
		assert.Equal(t, execcontext.StepStatusPending, secondResult.Status)

		collector.waitForCompletion()
		events := collector.getEvents()

		hasStepFailed := false
		hasWorkflowFailed := false

		for _, event := range events {
			switch event.Type {
			case pkgEvents.EventStepFailed:
				hasStepFailed = true
			case pkgEvents.EventWorkflowFailed:
				hasWorkflowFailed = true
			}
		}

		assert.True(t, hasStepFailed, "Expected step failed event")
		assert.True(t, hasWorkflowFailed, "Expected workflow failed event")
	})

	t.Run("Unknown step type", func(t *testing.T) {
		steps := []*ast.Step{
			{
				ID: "unknown_step",
				// No Run, Agent, Uses, Container, or While - should be unknown type
			},
		}

		workflow := createTestWorkflow(steps)
		execCtx := createTestExecutionContext(workflow)

		executor, err := createMockExecutor(workflow)
		require.NoError(t, err)

		eventsChan, _ := collectProgressEvents()
		err = executor.ExecuteWorkflow(execCtx, eventsChan)
		close(eventsChan)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown step type")
	})

	t.Run("Missing agent", func(t *testing.T) {
		steps := []*ast.Step{
			{
				ID:     "agent_step",
				Agent:  "nonexistent_agent",
				Prompt: "test",
			},
		}

		workflow := createTestWorkflow(steps)
		execCtx := createTestExecutionContext(workflow)

		executor, err := createMockExecutor(workflow)
		require.NoError(t, err)

		eventsChan, _ := collectProgressEvents()
		err = executor.ExecuteWorkflow(execCtx, eventsChan)
		close(eventsChan)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agent nonexistent_agent not found")
	})
}

func TestExecuteWorkflow_WorkflowOutputs(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			State: map[string]interface{}{
				"result": "initial",
			},
			Steps: []*ast.Step{
				{
					ID:  "set_result",
					Run: "echo 'Setting result'",
					Updates: map[string]interface{}{
						"result": "final_value",
						"count":  42,
					},
				},
			},
			Outputs: map[string]interface{}{
				"final_result": "${{ state.result }}",
				"final_count":  "${{ state.count }}",
				"static":       "This is static",
			},
		},
	}

	execCtx := createTestExecutionContext(workflow)

	executor, err := createMockExecutor(workflow)
	require.NoError(t, err)

	eventsChan, _ := collectProgressEvents()

	err = executor.ExecuteWorkflow(execCtx, eventsChan)
	close(eventsChan)
	require.NoError(t, err)

	outputs := execCtx.GetWorkflowOutputs()
	require.NotNil(t, outputs)

	assert.Equal(t, "final_value", outputs["final_result"])
	assert.Equal(t, "42", outputs["final_count"])
	assert.Equal(t, "This is static", outputs["static"])
}

func TestExecuteWorkflow_WithInputs(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Inputs: map[string]*ast.InputParam{
			"name": {
				Type:        "string",
				Description: "Name input",
				Required:    true,
			},
			"count": {
				Type:    "integer",
				Default: 5,
			},
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{
					ID:  "greet",
					Run: "echo 'Hello ${{ inputs.name }}, count is ${{ inputs.count }}'",
				},
			},
			Outputs: map[string]interface{}{
				"greeting": "${{ steps.greet.output }}",
			},
		},
	}

	inputs := map[string]interface{}{
		"name":  "World",
		"count": 10,
	}

	ctx := context.Background()

	execCtx := execcontext.NewExecutionContext(
		execcontext.RunContext{Context: ctx},
		workflow,
		inputs,
		"/tmp", // working directory
	)

	executor, err := createMockExecutor(workflow)
	require.NoError(t, err)

	eventsChan, _ := collectProgressEvents()

	err = executor.ExecuteWorkflow(execCtx, eventsChan)
	close(eventsChan)
	require.NoError(t, err)

	result, exists := execCtx.GetStepResult("greet")
	require.True(t, exists)
	require.NotNil(t, result)
	assert.Equal(t, execcontext.StepStatusCompleted, result.Status)

	outputs := execCtx.GetWorkflowOutputs()
	require.NotNil(t, outputs)
	assert.Equal(t, "Hello World, count is 10\n", outputs["greeting"])
}

func TestExecuteWorkflow_BlockStep(t *testing.T) {
	steps := []*ast.Step{
		{
			ID:   "block_step",
			Uses: "testdata/block.laq.yml",
			With: map[string]interface{}{
				"name": "World",
			},
		},
	}

	workflow := createTestWorkflow(steps)
	execCtx := createTestExecutionContext(workflow)

	executor, err := createMockExecutor(workflow)
	require.NoError(t, err)

	eventsChan, _ := collectProgressEvents()

	err = executor.ExecuteWorkflow(execCtx, eventsChan)
	close(eventsChan)
	require.NoError(t, err)

	result, exists := execCtx.GetStepResult("block_step")
	require.True(t, exists)
	require.NotNil(t, result)
	assert.Equal(t, execcontext.StepStatusCompleted, result.Status)
	assert.NoError(t, result.Error)
	assert.Equal(t, `{greeting: "Hello, World!\n"}`, result.Response)

	assert.NotEmpty(t, result.Response)
}

func TestExecuteWorkflow_EmptyWorkflow(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{}, // Empty steps
		},
	}

	execCtx := createTestExecutionContext(workflow)

	executor, err := createMockExecutor(workflow)
	require.NoError(t, err)

	eventsChan, collector := collectProgressEvents()

	err = executor.ExecuteWorkflow(execCtx, eventsChan)
	close(eventsChan)
	require.NoError(t, err)

	collector.waitForCompletion()
	events := collector.getEvents()

	hasStarted := false
	hasCompleted := false

	for _, event := range events {
		switch event.Type {
		case pkgEvents.EventWorkflowStarted:
			hasStarted = true
		case pkgEvents.EventWorkflowCompleted:
			hasCompleted = true
		}
	}

	assert.True(t, hasStarted)
	assert.True(t, hasCompleted)
}

func TestExecuteWorkflow_AgentStepWithOutputs(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Agents: map[string]*ast.Agent{
			"test_agent": {
				Name:     "test_agent",
				Provider: "anthropic",
				Model:    "test-model",
			},
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{
					ID:     "agent_with_outputs",
					Agent:  "test_agent",
					Prompt: "Generate a structured response",
					Outputs: map[string]schema.JSON{
						"result": {
							Type:        "string",
							Description: "The result",
						},
					},
				},
			},
		},
	}

	execCtx := createTestExecutionContext(workflow)

	executor, err := createMockExecutor(workflow)
	require.NoError(t, err)

	mockExecutor, ok := executor.(*Executor)
	require.True(t, ok)
	mockProvider, err := mockExecutor.modelRegistry.GetProviderByName("anthropic")
	require.NoError(t, err)
	mockProviderTyped := mockProvider.(*provider.MockProvider)
	mockProviderTyped.SetResponse("Generate a structured response", `{"result": "test_value"}`)

	eventsChan, _ := collectProgressEvents()

	err = executor.ExecuteWorkflow(execCtx, eventsChan)
	close(eventsChan)
	require.NoError(t, err)

	result, exists := execCtx.GetStepResult("agent_with_outputs")
	require.True(t, exists)
	require.NotNil(t, result)
	assert.Equal(t, execcontext.StepStatusCompleted, result.Status)
	assert.NotEmpty(t, result.Output)
}
