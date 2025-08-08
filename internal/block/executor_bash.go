package block

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lacquerai/lacquer/internal/execcontext"
)

// BashExecutor executes Bash script blocks
type BashExecutor struct {
	cacheDir string
}

// NewBashExecutor creates a new Bash script executor
func NewBashExecutor(cacheDir string) (*BashExecutor, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &BashExecutor{
		cacheDir: cacheDir,
	}, nil
}

// Validate checks if the executor can handle the given block
func (e *BashExecutor) Validate(block *Block) error {
	if block.Runtime != RuntimeBash {
		return fmt.Errorf("invalid runtime for bash executor: %s", block.Runtime)
	}
	if block.Script == "" {
		return fmt.Errorf("bash block missing script")
	}
	return nil
}

func (e *BashExecutor) ExecuteRaw(execCtx *execcontext.ExecutionContext, block *Block, inputJSON json.RawMessage) (interface{}, error) {
	// Get or prepare the script
	scriptPath, err := e.getOrPrepare(block)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare bash script: %w", err)
	}

	inputs := make(map[string]interface{})
	if err := json.Unmarshal(inputJSON, &inputs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input: %w", err)
	}

	// Prepare execution input
	execInput := ExecutionInput{
		Inputs: inputs,
		Env:    make(map[string]string),
	}

	// Add environment variables
	execInput.Env["WORKSPACE"] = execCtx.Cwd
	execInput.Env["LOG_LEVEL"] = os.Getenv("LOG_LEVEL")
	execInput.Env["LACQUER_INPUTS"] = string(inputJSON)

	// Execute the script
	cmd := exec.CommandContext(execCtx.Context.Context, "bash", scriptPath)

	jsonInput, err := json.Marshal(execInput)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	cmd.Stdin = bytes.NewReader(jsonInput)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// cmd dir needs to be the directory of the workflow file
	cmd.Dir = execCtx.Cwd

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range execInput.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

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

			return nil, fmt.Errorf("block execution failed: %w: %s", err, stdout.String())
		}
	case <-execCtx.Context.Context.Done():
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("block execution timeout")
	}

	var output map[string]interface{}
	out := stdout.Bytes()
	if err := json.Unmarshal(out, &output); err != nil {
		return stdout.String(), nil
	}

	return output, nil
}

// Execute runs a Bash script block
func (e *BashExecutor) Execute(execCtx *execcontext.ExecutionContext, block *Block, inputs map[string]interface{}) (interface{}, error) {
	jsonInput, err := json.Marshal(inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	return e.ExecuteRaw(execCtx, block, jsonInput)
}

func (e *BashExecutor) getOrPrepare(block *Block) (string, error) {
	// Generate cache key based on script content
	hash := sha256.Sum256([]byte(block.Script))
	cacheKey := hex.EncodeToString(hash[:])

	scriptName := fmt.Sprintf("block_%s_%s.sh", block.Name, cacheKey[:8])
	scriptPath := filepath.Join(e.cacheDir, scriptName)

	// Check if already cached
	if _, err := os.Stat(scriptPath); err == nil {
		// Script exists in cache
		return scriptPath, nil
	}

	// Write script to cache file
	if err := os.WriteFile(scriptPath, []byte(block.Script), 0755); err != nil {
		return "", fmt.Errorf("failed to write script: %w", err)
	}

	return scriptPath, nil
}
