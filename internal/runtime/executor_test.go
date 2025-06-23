package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
)

func TestNewExecutor(t *testing.T) {
	// Create a simple test workflow with an agent
	workflow := &ast.Workflow{
		Version: "1.0",
		Agents: map[string]*ast.Agent{
			"test_agent": {
				Provider: "local",
				Model:    "claude-code",
			},
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "test_agent", Prompt: "test"},
			},
		},
	}

	// Test with default config
	executor, err := NewExecutor(nil, workflow, nil)
	assert.NoError(t, err)
	assert.NotNil(t, executor)
	assert.NotNil(t, executor.config)
	assert.Equal(t, 3, executor.config.MaxConcurrentSteps)
	assert.True(t, executor.config.EnableRetries)

	// Test with custom config
	config := &ExecutorConfig{
		MaxConcurrentSteps: 5,
		DefaultTimeout:     10 * time.Second,
		EnableRetries:      false,
	}
	executor, err = NewExecutor(config, workflow, nil)
	assert.NoError(t, err)
	assert.Equal(t, config, executor.config)
}

func TestExecutor_ValidateInputs(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Agents: map[string]*ast.Agent{
			"test_agent": {
				Provider: "local",
				Model:    "claude-code",
			},
		},
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"required_param": {
					Type:        "string",
					Required:    true,
					Description: "A required parameter",
				},
				"optional_param": {
					Type:        "string",
					Required:    false,
					Default:     "default_value",
					Description: "An optional parameter",
				},
				"required_with_default": {
					Type:     "string",
					Required: true,
					Default:  "fallback_value",
				},
			},
		},
	}

	executor, err := NewExecutor(nil, workflow, nil)
	assert.NoError(t, err)

	// Test valid inputs
	inputs := map[string]interface{}{
		"required_param": "provided_value",
	}
	err = executor.validateInputs(workflow, inputs)
	assert.NoError(t, err)

	// Check that defaults were applied
	assert.Equal(t, "fallback_value", inputs["required_with_default"])

	// Test missing required input
	inputs = map[string]interface{}{}
	err = executor.validateInputs(workflow, inputs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required input required_param is missing")
}

func TestExecutor_EvaluateSkipCondition(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Agents: map[string]*ast.Agent{
			"agent1": {
				Provider: "local",
				Model:    "claude-code",
			},
		},
		Workflow: &ast.WorkflowDef{
			State: map[string]interface{}{
				"skip_flag": true,
				"condition": false,
			},
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	executor, err := NewExecutor(nil, workflow, nil)
	assert.NoError(t, err)

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	// Test no condition
	step := &ast.Step{ID: "test"}
	shouldSkip, err := executor.evaluateSkipCondition(execCtx, step)
	assert.NoError(t, err)
	assert.False(t, shouldSkip)

	// Test skip_if condition (true)
	step = &ast.Step{
		ID:     "test",
		SkipIf: "{{ state.skip_flag }}",
	}
	shouldSkip, err = executor.evaluateSkipCondition(execCtx, step)
	assert.NoError(t, err)
	assert.True(t, shouldSkip)

	// Test skip_if condition (false)
	step = &ast.Step{
		ID:     "test",
		SkipIf: "{{ state.condition }}",
	}
	shouldSkip, err = executor.evaluateSkipCondition(execCtx, step)
	assert.NoError(t, err)
	assert.False(t, shouldSkip)

	// Test condition (true - should not skip)
	step = &ast.Step{
		ID:        "test",
		Condition: "{{ state.skip_flag }}",
	}
	shouldSkip, err = executor.evaluateSkipCondition(execCtx, step)
	assert.NoError(t, err)
	assert.False(t, shouldSkip)

	// Test condition (false - should skip)
	step = &ast.Step{
		ID:        "test",
		Condition: "{{ state.condition }}",
	}
	shouldSkip, err = executor.evaluateSkipCondition(execCtx, step)
	assert.NoError(t, err)
	assert.True(t, shouldSkip)
}

func TestExecutor_ExecuteActionStep_UpdateState(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			State: map[string]interface{}{
				"counter": 5,
			},
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	executor, err := NewExecutor(nil, workflow, nil)
	assert.NoError(t, err)

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, map[string]interface{}{
		"name": "Alice",
	})

	step := &ast.Step{
		ID:     "update_test",
		Action: "update_state",
		Updates: map[string]interface{}{
			"new_value":     "hello",
			"dynamic_value": "{{ inputs.name }}",
			"counter":       10,
		},
	}

	output, err := executor.executeActionStep(execCtx, step)
	assert.NoError(t, err)
	assert.NotNil(t, output)

	// Check updated state
	value, exists := execCtx.GetState("new_value")
	assert.True(t, exists)
	assert.Equal(t, "hello", value)

	value, exists = execCtx.GetState("dynamic_value")
	assert.True(t, exists)
	assert.Equal(t, "Alice", value)

	value, exists = execCtx.GetState("counter")
	assert.True(t, exists)
	assert.Equal(t, 10, value)
}

func TestExecutor_ExecuteActionStep_HumanInput(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}
	executor, err := NewExecutor(nil, workflow, nil)
	assert.NoError(t, err)

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	step := &ast.Step{
		ID:     "human_input_test",
		Action: "human_input",
	}

	output, err := executor.executeActionStep(execCtx, step)
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Contains(t, output, "human_input")
}

func TestExecutor_ExecuteActionStep_UnknownAction(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	executor, err := NewExecutor(nil, workflow, nil)
	assert.NoError(t, err)

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	step := &ast.Step{
		ID:     "unknown_test",
		Action: "unknown_action",
	}

	_, err = executor.executeActionStep(execCtx, step)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
}

func TestExecutor_ExecuteBlockStep(t *testing.T) {

	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}
	executor, err := NewExecutor(nil, workflow, nil)
	assert.NoError(t, err)

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	step := &ast.Step{
		ID:   "block_test",
		Uses: "lacquer/http-request@v1",
		With: map[string]interface{}{
			"url": "https://api.example.com/test",
		},
	}

	output, err := executor.executeBlockStep(execCtx, step)
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Contains(t, output, "block_output")
}

func TestExecutor_ExecuteAgentStep(t *testing.T) {

	// Register mock provider

	workflow := &ast.Workflow{
		Version: "1.0",
		Agents: map[string]*ast.Agent{
			"test_agent": {
				Provider:     "mock",
				Model:        "test-model",
				SystemPrompt: "You are helpful",
			},
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "test_agent", Prompt: "Hello, {{ inputs.name }}!"},
			},
		},
	}
	registry := NewModelRegistry(true)
	mockProvider := NewMockModelProvider("mock", []ModelInfo{
		{
			ID:       "test-model",
			Name:     "test-model",
			Provider: "mock",
		},
	})
	mockProvider.SetResponse("Hello, Alice!", "Hello, Alice! How can I help?")
	registry.RegisterProvider(mockProvider)
	executor, err := NewExecutor(nil, workflow, registry)
	assert.NoError(t, err)

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, map[string]interface{}{
		"name": "Alice",
	})

	step := workflow.Workflow.Steps[0]

	response, usage, err := executor.executeAgentStep(execCtx, step)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, Alice! How can I help?", response)
	assert.NotNil(t, usage)
	assert.Greater(t, usage.TotalTokens, 0)
}

func TestExecutor_ExecuteAgentStep_MissingAgent(t *testing.T) {

	workflow := &ast.Workflow{
		Version: "1.0",
		Agents:  map[string]*ast.Agent{},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "missing_agent", Prompt: "test"},
			},
		},
	}

	executor, err := NewExecutor(nil, workflow, nil)
	assert.NoError(t, err)

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	step := workflow.Workflow.Steps[0]

	_, _, err = executor.executeAgentStep(execCtx, step)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent missing_agent not found")
}

func TestExecutor_ExecuteAgentStep_MissingModel(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Agents: map[string]*ast.Agent{
			"test_agent": {
				Provider: "mock",
				Model:    "nonexistent-model",
			},
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "test_agent", Prompt: "test"},
			},
		},
	}

	// Create registry and register mock provider that doesn't support the model
	registry := NewModelRegistry(true)
	mockProvider := NewMockModelProvider("mock", []ModelInfo{
		{
			ID:       "other-model",
			Name:     "other-model",
			Provider: "mock",
		},
	})
	err := registry.RegisterProvider(mockProvider)
	assert.NoError(t, err)

	executor, err := NewExecutor(nil, workflow, registry)
	assert.NoError(t, err)

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	step := workflow.Workflow.Steps[0]

	_, _, err = executor.executeAgentStep(execCtx, step)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model nonexistent-model not supported by provider mock")
}

func TestGetKeys(t *testing.T) {
	m := map[string]interface{}{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	keys := getKeys(m)
	assert.Len(t, keys, 3)
	assert.Contains(t, keys, "a")
	assert.Contains(t, keys, "b")
	assert.Contains(t, keys, "c")
}
