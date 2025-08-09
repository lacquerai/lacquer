package block

import (
	"context"
	"os"
	"testing"

	"github.com/lacquerai/lacquer/internal/execcontext"
)

func TestDockerExecutor(t *testing.T) {
	executor := NewDockerExecutor()

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
				echo '{"message": "Hello from Docker", "input_received": true}'
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
		defer func() { _ = os.RemoveAll(workspace) }()

		execCtx := &execcontext.ExecutionContext{
			RunID: "test-run",
			Context: execcontext.RunContext{
				Context: ctx,
			},
		}

		inputs := map[string]interface{}{
			"test": "data",
		}

		outputs, err := executor.Execute(execCtx, block, inputs)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		message, ok := outputs.(map[string]interface{})
		if !ok {
			t.Fatal("Expected outputs to be a map")
		}

		if message["message"] != "Hello from Docker" {
			t.Errorf("Expected 'Hello from Docker', got %v", message)
		}

		received, ok := message["input_received"]
		if !ok {
			t.Fatal("Expected 'input_received' in outputs")
		}

		if received != true {
			t.Errorf("Expected true, got %v", received)
		}
	})
}
