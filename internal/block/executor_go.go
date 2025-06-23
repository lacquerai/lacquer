package block

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GoExecutor executes Go script blocks
type GoExecutor struct {
	cacheDir string
}

// NewGoExecutor creates a new Go script executor
func NewGoExecutor(cacheDir string) (*GoExecutor, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	
	return &GoExecutor{
		cacheDir: cacheDir,
	}, nil
}

// Validate checks if the executor can handle the given block
func (e *GoExecutor) Validate(block *Block) error {
	if block.Runtime != RuntimeGo {
		return fmt.Errorf("invalid runtime for go executor: %s", block.Runtime)
	}
	if block.Script == "" {
		return fmt.Errorf("go block missing script")
	}
	return nil
}

// Execute runs a Go script block
func (e *GoExecutor) Execute(ctx context.Context, block *Block, inputs map[string]interface{}, execCtx *ExecutionContext) (map[string]interface{}, error) {
	// Get or compile the script
	binaryPath, err := e.getOrCompile(ctx, block)
	if err != nil {
		return nil, fmt.Errorf("failed to compile go script: %w", err)
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

	// Add environment variables
	execInput.Env["WORKSPACE"] = execCtx.Workspace
	execInput.Env["LOG_LEVEL"] = os.Getenv("LOG_LEVEL")

	// Marshal input to JSON
	inputJSON, err := json.Marshal(execInput)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Execute the binary
	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Stdin = bytes.NewReader(inputJSON)
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set working directory to workspace
	cmd.Dir = execCtx.Workspace

	// Run with timeout
	done := make(chan error)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			// Check if there's an error in stderr
			if stderr.Len() > 0 {
				var execErr ExecutionError
				if jsonErr := json.Unmarshal(stderr.Bytes(), &execErr); jsonErr == nil {
					return nil, fmt.Errorf("block execution failed: %s", execErr.Message)
				}
				return nil, fmt.Errorf("block execution failed: %s", stderr.String())
			}
			return nil, fmt.Errorf("block execution failed: %w", err)
		}
	case <-ctx.Done():
		cmd.Process.Kill()
		return nil, fmt.Errorf("block execution timeout")
	}

	// Parse output
	var output ExecutionOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("failed to parse block output: %w", err)
	}

	return output.Outputs, nil
}

func (e *GoExecutor) getOrCompile(ctx context.Context, block *Block) (string, error) {
	// Generate cache key based on script content
	hash := sha256.Sum256([]byte(block.Script))
	cacheKey := hex.EncodeToString(hash[:])
	
	binaryName := fmt.Sprintf("block_%s_%s", block.Name, cacheKey[:8])
	binaryPath := filepath.Join(e.cacheDir, binaryName)

	// Check if already compiled
	if _, err := os.Stat(binaryPath); err == nil {
		// Binary exists in cache
		return binaryPath, nil
	}

	// Create temporary directory for compilation
	tmpDir, err := os.MkdirTemp("", "laq-block-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write script to temporary file
	scriptPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(scriptPath, []byte(block.Script), 0644); err != nil {
		return "", fmt.Errorf("failed to write script: %w", err)
	}

	// Compile the script
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, scriptPath)
	cmd.Dir = tmpDir
	
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("compilation failed: %s", stderr.String())
	}

	// Make binary executable
	if err := os.Chmod(binaryPath, 0755); err != nil {
		os.Remove(binaryPath)
		return "", fmt.Errorf("failed to make binary executable: %w", err)
	}

	return binaryPath, nil
}