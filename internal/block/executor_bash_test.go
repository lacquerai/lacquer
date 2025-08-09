package block

import (
	"context"
	"os"
	"testing"

	"github.com/lacquerai/lacquer/internal/execcontext"
)

func TestBashExecutor(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "laq-bash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	executor, err := NewBashExecutor(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create Bash executor: %v", err)
	}

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

	err = executor.Validate(block)
	if err != nil {
		t.Fatalf("Block validation failed: %v", err)
	}

	workspace, err := os.MkdirTemp("", "laq-workspace-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer func() { _ = os.RemoveAll(workspace) }()

	ctx := context.Background()
	inputs := map[string]interface{}{
		"a": 5.0,
		"b": 3.0,
	}

	execCtx := &execcontext.ExecutionContext{
		RunID: "test-run",
		Context: execcontext.RunContext{
			Context: ctx,
		},
	}

	outputs, err := executor.Execute(execCtx, block, inputs)
	if err != nil {
		t.Fatalf("Block execution failed: %v", err)
	}

	// Verify output
	sum, ok := outputs.(map[string]interface{})
	if !ok {
		t.Error("Expected outputs to be a map")
	}

	if sum["sum"] != 8.0 {
		t.Errorf("Expected sum to be 8.0, got %v", sum)
	}
}
