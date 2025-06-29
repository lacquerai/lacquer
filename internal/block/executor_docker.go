package block

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// DockerExecutor executes Docker blocks
type DockerExecutor struct {
	pullTimeout time.Duration
}

// NewDockerExecutor creates a new Docker block executor
func NewDockerExecutor() *DockerExecutor {
	return &DockerExecutor{
		pullTimeout: 5 * time.Minute,
	}
}

// Validate checks if the executor can handle the given block
func (e *DockerExecutor) Validate(block *Block) error {
	if block.Runtime != RuntimeDocker {
		return fmt.Errorf("invalid runtime for docker executor: %s", block.Runtime)
	}
	if block.Image == "" {
		return fmt.Errorf("docker block missing image")
	}

	// Check if Docker is available
	if err := e.checkDockerAvailable(); err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}

	return nil
}

// Execute runs a Docker block
func (e *DockerExecutor) Execute(ctx context.Context, block *Block, inputs map[string]interface{}, execCtx *ExecutionContext) (map[string]interface{}, error) {
	// Pull image if not present
	if err := e.pullImageIfNeeded(ctx, block.Image); err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	// Prepare execution input
	execInput := ExecutionInput{
		Inputs: inputs,
		Env:    make(map[string]string),
		Context: ExecutionContextJSON{
			WorkflowID: execCtx.WorkflowID,
			StepID:     execCtx.StepID,
			Workspace:  execCtx.Workspace,
		},
	}

	// Add environment variables from block config
	for key, value := range block.Env {
		execInput.Env[key] = value
	}

	// Add workspace environment
	execInput.Env["WORKSPACE"] = execCtx.Workspace

	// Marshal input to JSON
	inputJSON, err := json.Marshal(execInput)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Build Docker run command
	args := []string{"run", "--rm"}

	// Add LACQUER_INPUTS environment variable with the execution input
	args = append(args, "-e", fmt.Sprintf("LACQUER_INPUTS=%s", string(inputJSON)))

	// Add environment variables from block config
	for key, value := range execInput.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Mount workspace if specified
	if execCtx.Workspace != "" {
		args = append(args, "-v", fmt.Sprintf("%s:/workspace", execCtx.Workspace))
	}

	// Add image
	args = append(args, block.Image)

	// Add command if specified
	if len(block.Command) > 0 {
		args = append(args, block.Command...)
	}

	// Execute Docker container
	cmd := exec.CommandContext(ctx, "docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		// Check for error in stderr
		if stderr.Len() > 0 {
			var execErr ExecutionError
			if jsonErr := json.Unmarshal(stderr.Bytes(), &execErr); jsonErr == nil {
				return nil, fmt.Errorf("container execution failed: %s", execErr.Message)
			}
			return nil, fmt.Errorf("container execution failed: %s", stderr.String())
		}
		return nil, fmt.Errorf("container execution failed: %w", err)
	}

	// Parse output
	var output map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("failed to parse container output: %w", err)
	}

	return output, nil
}

func (e *DockerExecutor) checkDockerAvailable() error {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon not available or not running")
	}
	return nil
}

func (e *DockerExecutor) pullImageIfNeeded(ctx context.Context, image string) error {
	// Check if image exists locally
	cmd := exec.CommandContext(ctx, "docker", "image", "inspect", image)
	if err := cmd.Run(); err == nil {
		// Image exists locally
		return nil
	}

	// Pull the image
	pullCtx, cancel := context.WithTimeout(ctx, e.pullTimeout)
	defer cancel()

	cmd = exec.CommandContext(pullCtx, "docker", "pull", image)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull image %s: %s", image, stderr.String())
	}

	return nil
}

func (e *DockerExecutor) streamLogs(ctx context.Context, containerID string) {
	// Stream container logs for debugging
	// This is a simplified version - in production, you'd want better log management
	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", containerID)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	go func() {
		defer stdout.Close()
		io.Copy(io.Discard, stdout) // In production, send to proper logger
	}()

	cmd.Start()
}
