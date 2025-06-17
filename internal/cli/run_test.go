package cli

import (
	"context"
	"testing"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommandExists(t *testing.T) {
	// Test that the run command is properly registered
	cmd := rootCmd.Commands()

	var runCmdFound bool
	for _, cmd := range cmd {
		if cmd.Name() == "run" {
			runCmdFound = true
			break
		}
	}

	assert.True(t, runCmdFound, "run command should be registered")
}

func TestRunCommandFlags(t *testing.T) {
	// Test that expected flags are available
	expectedFlags := []string{
		"input",
		"dry-run",
		"save-state",
		"max-retries",
		"timeout",
		"progress",
		"show-steps",
	}

	for _, flagName := range expectedFlags {
		flag := runCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "Flag %s should be defined", flagName)
	}
}

func TestRunCommandArguments(t *testing.T) {
	// Test that the command expects exactly one argument
	assert.Nil(t, runCmd.Args(runCmd, []string{"workflow.laq.yaml"})) // Should return nil for valid args
	assert.NotNil(t, runCmd.Args(runCmd, []string{}))                 // Should return error for no args
	assert.NotNil(t, runCmd.Args(runCmd, []string{"file1", "file2"})) // Should return error for too many args
}

func TestExecutionResultStructure(t *testing.T) {
	// Test that ExecutionResult has the expected structure
	result := ExecutionResult{
		WorkflowFile:  "test.laq.yaml",
		RunID:         "test-run-123",
		Status:        "completed",
		StartTime:     time.Now(),
		EndTime:       time.Now(),
		Duration:      time.Second,
		StepsExecuted: 1,
		StepsTotal:    1,
		Inputs:        map[string]interface{}{"test": "value"},
		FinalState:    map[string]interface{}{"result": "success"},
		StepResults:   []StepExecutionResult{},
	}

	assert.Equal(t, "test.laq.yaml", result.WorkflowFile)
	assert.Equal(t, "test-run-123", result.RunID)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, 1, result.StepsExecuted)
	assert.Equal(t, 1, result.StepsTotal)
	assert.NotNil(t, result.Inputs)
	assert.NotNil(t, result.FinalState)
}

func TestStepExecutionResultStructure(t *testing.T) {
	// Test that StepExecutionResult has the expected structure
	tokenUsage := &TokenUsage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
		EstimatedCost:    0.001,
	}

	stepResult := StepExecutionResult{
		StepID:     "step1",
		Status:     "completed",
		Duration:   time.Second,
		Output:     map[string]interface{}{"output": "Hello World"},
		Response:   "Hello World",
		Retries:    0,
		TokenUsage: tokenUsage,
	}

	assert.Equal(t, "step1", stepResult.StepID)
	assert.Equal(t, "completed", stepResult.Status)
	assert.Equal(t, time.Second, stepResult.Duration)
	assert.Equal(t, "Hello World", stepResult.Response)
	assert.Equal(t, 0, stepResult.Retries)
	assert.NotNil(t, stepResult.TokenUsage)
	assert.Equal(t, 30, stepResult.TokenUsage.TotalTokens)
}

func TestTokenUsageSummaryStructure(t *testing.T) {
	// Test that TokenUsageSummary has the expected structure
	summary := TokenUsageSummary{
		TotalTokens:      100,
		PromptTokens:     30,
		CompletionTokens: 70,
		EstimatedCost:    0.005,
	}

	assert.Equal(t, 100, summary.TotalTokens)
	assert.Equal(t, 30, summary.PromptTokens)
	assert.Equal(t, 70, summary.CompletionTokens)
	assert.Equal(t, 0.005, summary.EstimatedCost)
}

func TestGetWorkflowName(t *testing.T) {
	tests := []struct {
		name     string
		workflow *ast.Workflow
		expected string
	}{
		{
			name: "workflow with name",
			workflow: &ast.Workflow{
				Metadata: &ast.WorkflowMetadata{
					Name: "test-workflow",
				},
			},
			expected: "test-workflow",
		},
		{
			name: "workflow without metadata",
			workflow: &ast.Workflow{
				Metadata: nil,
			},
			expected: "Untitled Workflow",
		},
		{
			name: "workflow with empty name",
			workflow: &ast.Workflow{
				Metadata: &ast.WorkflowMetadata{
					Name: "",
				},
			},
			expected: "Untitled Workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getWorkflowName(tt.workflow)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCollectExecutionResultsIntegration(t *testing.T) {
	// Test the token aggregation logic

	// Create a mock execution context with step results
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1"},
				{ID: "step2"},
			},
		},
	}

	ctx := context.Background()
	execCtx := runtime.NewExecutionContext(ctx, workflow, nil)

	// Add step results with token usage
	execCtx.SetStepResult("step1", &runtime.StepResult{
		StepID:    "step1",
		Status:    runtime.StepStatusCompleted,
		StartTime: time.Now().Add(-2 * time.Second),
		EndTime:   time.Now().Add(-1 * time.Second),
		Duration:  time.Second,
		Output:    map[string]interface{}{"output": "Result 1"},
		Response:  "Result 1",
		TokenUsage: &runtime.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 15,
			TotalTokens:      25,
			EstimatedCost:    0.001,
		},
	})

	execCtx.SetStepResult("step2", &runtime.StepResult{
		StepID:    "step2",
		Status:    runtime.StepStatusCompleted,
		StartTime: time.Now().Add(-1 * time.Second),
		EndTime:   time.Now(),
		Duration:  time.Second,
		Output:    map[string]interface{}{"output": "Result 2"},
		Response:  "Result 2",
		TokenUsage: &runtime.TokenUsage{
			PromptTokens:     20,
			CompletionTokens: 30,
			TotalTokens:      50,
			EstimatedCost:    0.002,
		},
	})

	// Create execution result
	result := ExecutionResult{
		RunID:      execCtx.RunID,
		StepsTotal: 2,
	}

	// Collect execution results
	collectExecutionResults(execCtx, &result)

	// Verify results
	require.NotNil(t, result.TokenUsage)
	assert.Equal(t, 75, result.TokenUsage.TotalTokens)
	assert.Equal(t, 30, result.TokenUsage.PromptTokens)
	assert.Equal(t, 45, result.TokenUsage.CompletionTokens)
	assert.Equal(t, 0.003, result.TokenUsage.EstimatedCost)

	assert.Len(t, result.StepResults, 2)
	assert.Equal(t, "step1", result.StepResults[0].StepID)
	assert.Equal(t, "step2", result.StepResults[1].StepID)
}
