package expression

import (
	"context"
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
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
