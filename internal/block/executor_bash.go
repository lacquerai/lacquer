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
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
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
	scriptPath, err := e.getOrPrepare(block)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare bash script: %w", err)
	}

	inputs := make(map[string]interface{})
	if err := json.Unmarshal(inputJSON, &inputs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input: %w", err)
	}

	execInput := ExecutionInput{
		Inputs: inputs,
		Env:    make(map[string]string),
	}

	execInput.Env["WORKSPACE"] = execCtx.Cwd
	execInput.Env["LOG_LEVEL"] = os.Getenv("LOG_LEVEL")
	execInput.Env["LACQUER_INPUTS"] = string(inputJSON)

	cmd := exec.CommandContext(execCtx.Context.Context, "bash", scriptPath) // #nosec G204 - scriptPath is controlled internally

	jsonInput, err := json.Marshal(execInput)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	cmd.Stdin = bytes.NewReader(jsonInput)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Dir = execCtx.Cwd
	cmd.Env = os.Environ()
	for key, value := range execInput.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

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
			_ = cmd.Process.Kill()
		}
		return nil, fmt.Errorf("block execution timeout")
	}

	var output map[string]interface{}
	out := stdout.Bytes()
	if err := json.Unmarshal(out, &output); err != nil {
		return stdout.String(), nil //nolint:nilerr // Intentional: fallback to raw string output
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

	if _, err := os.Stat(scriptPath); err == nil {
		return scriptPath, nil
	}

	if err := os.WriteFile(scriptPath, []byte(block.Script), 0600); err != nil {
		return "", fmt.Errorf("failed to write script: %w", err)
	}

	return scriptPath, nil
}
