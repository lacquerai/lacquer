package block

import (
	"context"
	"os"
	"testing"
)

func TestGoExecutor(t *testing.T) {
	// Create temporary cache directory
	tmpDir, err := os.MkdirTemp("", "laq-go-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Go executor
	executor, err := NewGoExecutor(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create Go executor: %v", err)
	}

	// Create a simple Go script block
	block := &Block{
		Name:    "test-go-block",
		Runtime: RuntimeGo,
		Script: `
package main

import (
	"encoding/json"
	"os"
)

type ExecutionInput struct {
	Inputs map[string]interface{} ` + "`json:\"inputs\"`" + `
}

func main() {
	var input ExecutionInput
	json.NewDecoder(os.Stdin).Decode(&input)
	
	a := input.Inputs["a"].(float64)
	b := input.Inputs["b"].(float64)
	
	result := a + b
	
	output := map[string]interface{}{
		"outputs": map[string]interface{}{
			"sum": result,
		},
	}
	
	json.NewEncoder(os.Stdout).Encode(output)
}
`,
	}

	// Test validation
	err = executor.Validate(block)
	if err != nil {
		t.Fatalf("Block validation failed: %v", err)
	}

	// Create workspace
	workspace, err := os.MkdirTemp("", "laq-workspace-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer os.RemoveAll(workspace)

	// Test execution
	ctx := context.Background()
	inputs := map[string]interface{}{
		"a": 5.0,
		"b": 3.0,
	}

	execCtx := &ExecutionContext{
		WorkflowID: "test-workflow",
		StepID:     "test-step",
		Workspace:  workspace,
		Context:    ctx,
	}

	outputs, err := executor.Execute(ctx, block, inputs, execCtx)
	if err != nil {
		t.Fatalf("Block execution failed: %v", err)
	}

	// Verify output
	sum, ok := outputs["sum"]
	if !ok {
		t.Error("Expected 'sum' in outputs")
	}

	if sum != 8.0 {
		t.Errorf("Expected sum to be 8.0, got %v", sum)
	}

	t.Logf("Execution successful, outputs: %+v", outputs)
}

func TestGoExecutorCompilationError(t *testing.T) {
	// Create temporary cache directory
	tmpDir, err := os.MkdirTemp("", "laq-go-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Go executor
	executor, err := NewGoExecutor(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create Go executor: %v", err)
	}

	// Create a block with invalid Go code
	block := &Block{
		Name:    "invalid-go-block",
		Runtime: RuntimeGo,
		Script:  "invalid go code",
	}

	// Test execution should fail with compilation error
	ctx := context.Background()
	inputs := map[string]interface{}{}

	workspace, err := os.MkdirTemp("", "laq-workspace-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer os.RemoveAll(workspace)

	execCtx := &ExecutionContext{
		WorkflowID: "test-workflow",
		StepID:     "test-step",
		Workspace:  workspace,
		Context:    ctx,
	}

	_, err = executor.Execute(ctx, block, inputs, execCtx)
	if err == nil {
		t.Error("Expected compilation error for invalid Go code")
	}

	if err != nil {
		t.Logf("Got expected error: %v", err)
	}
}
