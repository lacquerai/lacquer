package block

import (
	"context"
	"os"
	"testing"
)

func TestBashExecutor(t *testing.T) {
	// Create temporary cache directory
	tmpDir, err := os.MkdirTemp("", "laq-bash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Bash executor
	executor, err := NewBashExecutor(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create Bash executor: %v", err)
	}

	// Create a simple Bash script block that reads JSON input and produces JSON output
	block := &Block{
		Name:    "test-bash-block",
		Runtime: RuntimeBash,
		Script: `#!/bin/bash

# Read JSON input from stdin
input=$(cat)

# Parse input using jq or basic bash
a=$(echo "$input" | grep -o '"a":[^,}]*' | cut -d':' -f2 | tr -d ' ')
b=$(echo "$input" | grep -o '"b":[^,}]*' | cut -d':' -f2 | tr -d ' ')

# Calculate sum
sum=$((a + b))

# Output JSON result
echo "{\"sum\": $sum}"
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

func TestBashExecutorWithJq(t *testing.T) {
	// Create temporary cache directory
	tmpDir, err := os.MkdirTemp("", "laq-bash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Bash executor
	executor, err := NewBashExecutor(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create Bash executor: %v", err)
	}

	// Create a Bash script block that uses jq for JSON processing (if available)
	block := &Block{
		Name:    "test-bash-jq-block",
		Runtime: RuntimeBash,
		Script: `#!/bin/bash

# Read JSON input from stdin
input=$(cat)

# Check if jq is available, otherwise fall back to basic parsing
if command -v jq >/dev/null 2>&1; then
    # Use jq for proper JSON parsing
    a=$(echo "$input" | jq -r '.inputs.a // .a')
    b=$(echo "$input" | jq -r '.inputs.b // .b')
    name=$(echo "$input" | jq -r '.inputs.name // .name // "World"')
    
    sum=$(echo "$a + $b" | bc 2>/dev/null || echo $((${a%.*} + ${b%.*})))
    
    echo "{\"sum\": $sum, \"greeting\": \"Hello, $name!\"}"
else
    # Fallback to basic parsing
    a=$(echo "$input" | grep -o '"a":[^,}]*' | cut -d':' -f2 | tr -d ' ')
    b=$(echo "$input" | grep -o '"b":[^,}]*' | cut -d':' -f2 | tr -d ' ')
    sum=$((${a%.*} + ${b%.*}))
    echo "{\"sum\": $sum, \"greeting\": \"Hello, World!\"}"
fi
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
		"a":    7.0,
		"b":    3.0,
		"name": "Lacquer",
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

	// Verify outputs
	sum, ok := outputs["sum"]
	if !ok {
		t.Error("Expected 'sum' in outputs")
	}

	greeting, ok := outputs["greeting"]
	if !ok {
		t.Error("Expected 'greeting' in outputs")
	}

	t.Logf("Execution successful, outputs: %+v", outputs)
	t.Logf("Sum: %v, Greeting: %v", sum, greeting)
}

func TestBashExecutorScriptError(t *testing.T) {
	// Create temporary cache directory
	tmpDir, err := os.MkdirTemp("", "laq-bash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Bash executor
	executor, err := NewBashExecutor(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create Bash executor: %v", err)
	}

	// Create a block with a script that exits with error
	block := &Block{
		Name:    "failing-bash-block",
		Runtime: RuntimeBash,
		Script: `#!/bin/bash
echo "This script will fail" >&2
exit 1
`,
	}

	// Test execution should fail
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
		t.Error("Expected execution error for failing bash script")
	}

	if err != nil {
		t.Logf("Got expected error: %v", err)
	}
}

func TestBashExecutorValidation(t *testing.T) {
	// Create temporary cache directory
	tmpDir, err := os.MkdirTemp("", "laq-bash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Bash executor
	executor, err := NewBashExecutor(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create Bash executor: %v", err)
	}

	// Test validation with wrong runtime
	block := &Block{
		Name:    "wrong-runtime-block",
		Runtime: RuntimeGo, // Wrong runtime
		Script:  "echo 'test'",
	}

	err = executor.Validate(block)
	if err == nil {
		t.Error("Expected validation error for wrong runtime")
	}

	// Test validation with missing script
	block = &Block{
		Name:    "no-script-block",
		Runtime: RuntimeBash,
		Script:  "", // Missing script
	}

	err = executor.Validate(block)
	if err == nil {
		t.Error("Expected validation error for missing script")
	}

	// Test validation with valid block
	block = &Block{
		Name:    "valid-block",
		Runtime: RuntimeBash,
		Script:  "echo 'valid'",
	}

	err = executor.Validate(block)
	if err != nil {
		t.Errorf("Valid block should pass validation: %v", err)
	}
}
