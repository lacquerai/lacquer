package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/lacquer/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
)

func TestNewExecutionContext(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name:        "test-workflow",
			Description: "A test workflow",
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "Hello"},
				{ID: "step2", Agent: "agent2", Prompt: "World"},
			},
		},
	}

	inputs := map[string]interface{}{
		"name": "Alice",
		"age":  30,
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, inputs)

	// Test basic properties
	assert.Equal(t, workflow, execCtx.Workflow)
	assert.Equal(t, inputs, execCtx.Inputs)
	assert.Equal(t, 2, execCtx.TotalSteps)
	assert.Equal(t, 0, execCtx.CurrentStepIndex)
	assert.NotEmpty(t, execCtx.RunID)
	assert.True(t, time.Since(execCtx.StartTime) < time.Second)

	// Test step results initialization
	assert.Len(t, execCtx.StepResults, 2)

	result1, exists := execCtx.GetStepResult("step1")
	assert.True(t, exists)
	assert.Equal(t, "step1", result1.StepID)
	assert.Equal(t, StepStatusPending, result1.Status)

	result2, exists := execCtx.GetStepResult("step2")
	assert.True(t, exists)
	assert.Equal(t, "step2", result2.StepID)
	assert.Equal(t, StepStatusPending, result2.Status)
}

func TestExecutionContext_InputMethods(t *testing.T) {
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
		"age":  30,
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, inputs)

	// Test GetInput
	name, exists := execCtx.GetInput("name")
	assert.True(t, exists)
	assert.Equal(t, "Alice", name)

	age, exists := execCtx.GetInput("age")
	assert.True(t, exists)
	assert.Equal(t, 30, age)

	_, exists = execCtx.GetInput("nonexistent")
	assert.False(t, exists)
}

func TestExecutionContext_StateMethods(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			State: map[string]interface{}{
				"initial": "value",
			},
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	// Test initial state
	value, exists := execCtx.GetState("initial")
	assert.True(t, exists)
	assert.Equal(t, "value", value)

	// Test SetState
	execCtx.SetState("new_key", "new_value")
	value, exists = execCtx.GetState("new_key")
	assert.True(t, exists)
	assert.Equal(t, "new_value", value)

	// Test UpdateState
	updates := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}
	execCtx.UpdateState(updates)

	value1, exists := execCtx.GetState("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value1)

	value2, exists := execCtx.GetState("key2")
	assert.True(t, exists)
	assert.Equal(t, 42, value2)
}

func TestExecutionContext_StepResultMethods(t *testing.T) {
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

	// Test updating step result
	result := &StepResult{
		StepID:    "step1",
		Status:    StepStatusCompleted,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  100 * time.Millisecond,
		Output: map[string]interface{}{
			"response": "Hello, world!",
		},
	}

	execCtx.SetStepResult("step1", result)

	retrieved, exists := execCtx.GetStepResult("step1")
	assert.True(t, exists)
	assert.Equal(t, result, retrieved)
}

func TestExecutionContext_IsCompleted(t *testing.T) {
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

	// Initially not completed
	assert.False(t, execCtx.IsCompleted())

	// After first step
	execCtx.IncrementCurrentStep()
	assert.False(t, execCtx.IsCompleted())

	// After second step
	execCtx.IncrementCurrentStep()
	assert.True(t, execCtx.IsCompleted())
}

func TestExecutionSummary(t *testing.T) {
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

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, map[string]interface{}{"input": "value"})

	// Complete a step
	result := &StepResult{
		StepID:    "step1",
		Status:    StepStatusCompleted,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  100 * time.Millisecond,
		TokenUsage: &TokenUsage{
			TotalTokens:   100,
			EstimatedCost: 0.01,
		},
	}
	execCtx.SetStepResult("step1", result)
	execCtx.IncrementCurrentStep()

	summary := execCtx.GetExecutionSummary()

	assert.Equal(t, execCtx.RunID, summary.RunID)
	assert.Equal(t, ExecutionStatusCompleted, summary.Status)
	assert.Len(t, summary.Steps, 1)
	assert.Equal(t, 100, summary.TotalTokens)
	assert.Equal(t, 0.01, summary.EstimatedCost)
	assert.Equal(t, map[string]interface{}{"input": "value"}, summary.Inputs)
}
