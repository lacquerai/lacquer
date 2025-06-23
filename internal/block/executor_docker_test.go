package block

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestDockerExecutor(t *testing.T) {
	executor := NewDockerExecutor()

	// Check if Docker is available
	if err := executor.checkDockerAvailable(); err != nil {
		t.Skip("Docker not available, skipping Docker executor tests")
	}

	ctx := context.Background()

	t.Run("BasicExecution", func(t *testing.T) {
		// Create a simple Docker block that echoes input
		block := &Block{
			Name:    "echo-block",
			Runtime: RuntimeDocker,
			Image:   "alpine:latest",
			Command: []string{"sh", "-c", `
				cat > /tmp/input.json
				echo '{"outputs": {"message": "Hello from Docker", "input_received": true}}'
			`},
		}

		err := executor.Validate(block)
		if err != nil {
			t.Fatalf("Block validation failed: %v", err)
		}

		workspace, err := os.MkdirTemp("", "docker-test-*")
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

		inputs := map[string]interface{}{
			"test": "data",
		}

		outputs, err := executor.Execute(ctx, block, inputs, execCtx)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		message, ok := outputs["message"]
		if !ok {
			t.Fatal("Expected 'message' in outputs")
		}

		if message != "Hello from Docker" {
			t.Errorf("Expected 'Hello from Docker', got %v", message)
		}

		received, ok := outputs["input_received"]
		if !ok {
			t.Fatal("Expected 'input_received' in outputs")
		}

		if received != true {
			t.Errorf("Expected true, got %v", received)
		}
	})

	t.Run("PythonScript", func(t *testing.T) {
		// Test Python script execution
		block := &Block{
			Name:    "python-calculator",
			Runtime: RuntimeDocker,
			Image:   "python:3.11-alpine",
			Command: []string{"python3", "-c", `
import json
import sys

# Read input
input_data = json.load(sys.stdin)
inputs = input_data['inputs']

# Simple calculation
result = inputs.get('a', 0) + inputs.get('b', 0)

# Output
output = {
    'outputs': {
        'sum': result,
        'language': 'python'
    }
}
json.dump(output, sys.stdout)
			`},
		}

		err := executor.Validate(block)
		if err != nil {
			t.Fatalf("Block validation failed: %v", err)
		}

		workspace, err := os.MkdirTemp("", "docker-python-test-*")
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

		inputs := map[string]interface{}{
			"a": 15.0,
			"b": 27.0,
		}

		outputs, err := executor.Execute(ctx, block, inputs, execCtx)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		sum, ok := outputs["sum"]
		if !ok {
			t.Fatal("Expected 'sum' in outputs")
		}

		if sum != 42.0 {
			t.Errorf("Expected sum 42.0, got %v", sum)
		}

		language, ok := outputs["language"]
		if !ok {
			t.Fatal("Expected 'language' in outputs")
		}

		if language != "python" {
			t.Errorf("Expected 'python', got %v", language)
		}
	})

	t.Run("EnvironmentVariables", func(t *testing.T) {
		// Test environment variable passing
		block := &Block{
			Name:    "env-test",
			Runtime: RuntimeDocker,
			Image:   "alpine:latest",
			Command: []string{"sh", "-c", `
				echo "{\"outputs\": {\"env_var\": \"$TEST_VAR\", \"workspace\": \"$WORKSPACE\"}}"
			`},
			Env: map[string]string{
				"TEST_VAR": "test_value",
			},
		}

		workspace, err := os.MkdirTemp("", "docker-env-test-*")
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

		outputs, err := executor.Execute(ctx, block, map[string]interface{}{}, execCtx)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		envVar, ok := outputs["env_var"]
		if !ok {
			t.Fatal("Expected 'env_var' in outputs")
		}

		if envVar != "test_value" {
			t.Errorf("Expected 'test_value', got %v", envVar)
		}

		workspaceOut, ok := outputs["workspace"]
		if !ok {
			t.Fatal("Expected 'workspace' in outputs")
		}

		if workspaceOut != workspace {
			t.Errorf("Expected workspace path %s, got %v", workspace, workspaceOut)
		}
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		// Test error handling
		block := &Block{
			Name:    "error-block",
			Runtime: RuntimeDocker,
			Image:   "alpine:latest",
			Command: []string{"sh", "-c", `
				echo '{"error": {"message": "Intentional error", "code": "TEST_ERROR"}}' >&2
				exit 1
			`},
		}

		workspace, err := os.MkdirTemp("", "docker-error-test-*")
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

		_, err = executor.Execute(ctx, block, map[string]interface{}{}, execCtx)
		if err == nil {
			t.Error("Expected error from failing container")
		}

		if err != nil {
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("Timeout", func(t *testing.T) {
		// Test execution timeout
		block := &Block{
			Name:    "timeout-block",
			Runtime: RuntimeDocker,
			Image:   "alpine:latest",
			Command: []string{"sh", "-c", "sleep 10; echo '{\"outputs\": {\"result\": \"should not reach here\"}}'"},
		}

		// Create a context with a short timeout
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		workspace, err := os.MkdirTemp("", "docker-timeout-test-*")
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer os.RemoveAll(workspace)

		execCtx := &ExecutionContext{
			WorkflowID: "test-workflow",
			StepID:     "test-step",
			Workspace:  workspace,
			Context:    timeoutCtx,
		}

		_, err = executor.Execute(timeoutCtx, block, map[string]interface{}{}, execCtx)
		if err == nil {
			t.Error("Expected timeout error")
		}

		if err != nil {
			t.Logf("Got expected timeout error: %v", err)
		}
	})

	t.Run("Validation", func(t *testing.T) {
		// Test validation errors
		tests := []struct {
			name    string
			block   *Block
			wantErr bool
		}{
			{
				name: "ValidBlock",
				block: &Block{
					Name:    "valid",
					Runtime: RuntimeDocker,
					Image:   "alpine:latest",
				},
				wantErr: false,
			},
			{
				name: "WrongRuntime",
				block: &Block{
					Name:    "wrong-runtime",
					Runtime: RuntimeGo,
					Image:   "alpine:latest",
				},
				wantErr: true,
			},
			{
				name: "MissingImage",
				block: &Block{
					Name:    "no-image",
					Runtime: RuntimeDocker,
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := executor.Validate(tt.block)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestDockerExecutorImagePull(t *testing.T) {
	executor := NewDockerExecutor()

	// Check if Docker is available
	if err := executor.checkDockerAvailable(); err != nil {
		t.Skip("Docker not available, skipping Docker image pull tests")
	}

	ctx := context.Background()

	t.Run("ExistingImage", func(t *testing.T) {
		// Test with a commonly available image
		err := executor.pullImageIfNeeded(ctx, "alpine:latest")
		if err != nil {
			t.Errorf("Failed to pull alpine:latest: %v", err)
		}
	})

	t.Run("NonExistentImage", func(t *testing.T) {
		// Test with an image that doesn't exist
		err := executor.pullImageIfNeeded(ctx, "this-image-does-not-exist:never")
		if err == nil {
			t.Error("Expected error for non-existent image")
		}
	})
}

func BenchmarkDockerExecutor(b *testing.B) {
	executor := NewDockerExecutor()

	// Check if Docker is available
	if err := executor.checkDockerAvailable(); err != nil {
		b.Skip("Docker not available, skipping Docker executor benchmarks")
	}

	ctx := context.Background()

	block := &Block{
		Name:    "benchmark-block",
		Runtime: RuntimeDocker,
		Image:   "alpine:latest",
		Command: []string{"echo", `{"outputs": {"result": "benchmark"}}`},
	}

	workspace, err := os.MkdirTemp("", "docker-benchmark-*")
	if err != nil {
		b.Fatalf("Failed to create workspace: %v", err)
	}
	defer os.RemoveAll(workspace)

	execCtx := &ExecutionContext{
		WorkflowID: "benchmark-workflow",
		StepID:     "benchmark-step",
		Workspace:  workspace,
		Context:    ctx,
	}

	// Ensure image is pulled before benchmark
	executor.pullImageIfNeeded(ctx, block.Image)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := executor.Execute(ctx, block, map[string]interface{}{}, execCtx)
		if err != nil {
			b.Fatalf("Execution failed: %v", err)
		}
	}
}