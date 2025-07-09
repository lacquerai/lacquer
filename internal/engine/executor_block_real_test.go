package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutor_realBlockExecution_Native(t *testing.T) {
	// Create temporary test files
	tempDir := createRealTestFiles(t)

	testCases := []struct {
		name          string
		step          *ast.Step
		expectError   bool
		expectedError string
		expectedKeys  []string
	}{
		{
			name: "Valid native block execution",
			step: &ast.Step{
				ID:   "test_native_block",
				Uses: filepath.Join(tempDir, "text-analyzer"),
				With: map[string]interface{}{
					"text":          "Hello world",
					"analysis_type": "sentiment",
				},
			},
			expectError:  false,
			expectedKeys: []string{"sentiment", "explanation"},
		},
		{
			name: "Block directory not found",
			step: &ast.Step{
				ID:   "missing_block",
				Uses: filepath.Join(tempDir, "nonexistent"),
				With: map[string]interface{}{
					"text": "Hello world",
				},
			},
			expectError:   true,
			expectedError: "block directory not found",
		},
		{
			name: "Block with missing required input",
			step: &ast.Step{
				ID:   "missing_input",
				Uses: filepath.Join(tempDir, "text-analyzer"),
				With: map[string]interface{}{
					// Missing required 'text' input
					"analysis_type": "sentiment",
				},
			},
			expectError:   true,
			expectedError: "required input",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create workflow with real block manager
			workflow := &ast.Workflow{
				Version: "1.0",
				Workflow: &ast.WorkflowDef{
					Steps: []*ast.Step{tc.step},
				},
			}

			// Create executor with real block manager
			executor, err := NewExecutorWithRealBlocks(nil, workflow, nil)
			require.NoError(t, err)

			// Create execution context
			ctx := context.Background()
			execCtx := NewExecutionContext(ctx, workflow, map[string]interface{}{})

			// Execute the step
			err = executor.executeStep(execCtx, tc.step)

			if tc.expectError {
				assert.Error(t, err)
				if tc.expectedError != "" {
					assert.Contains(t, err.Error(), tc.expectedError)
				}
			} else {
				assert.NoError(t, err)

				// Check step result
				result, exists := execCtx.GetStepResult(tc.step.ID)
				assert.True(t, exists)
				assert.Equal(t, StepStatusCompleted, result.Status)

				// Check expected output keys
				for _, key := range tc.expectedKeys {
					assert.Contains(t, result.Output, key)
				}
			}
		})
	}
}

func TestExecutor_realBlockExecution_Script(t *testing.T) {
	// Create temporary test files
	tempDir := createRealTestFiles(t)

	// Read processor file content for the file-based test
	processorPath := filepath.Join(tempDir, "processor.go")
	processorFileContent, err := os.ReadFile(processorPath)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		step          *ast.Step
		expectError   bool
		expectedError string
		expectedKeys  []string
	}{
		{
			name: "Valid Go script from file content",
			step: &ast.Step{
				ID:     "test_go_script",
				Script: string(processorFileContent),
				With: map[string]interface{}{
					"input": "test data",
					"mode":  "uppercase",
				},
			},
			expectError:  false,
			expectedKeys: []string{"result"},
		},
		{
			name: "Valid inline Go script",
			step: &ast.Step{
				ID: "inline_go_script",
				Script: `package main

import (
	"encoding/json"
	"os"
	"strings"
)

type ExecutionInput struct {
	Inputs  map[string]interface{} ` + "`json:\"inputs\"`" + `
	Env     map[string]string      ` + "`json:\"env\"`" + `
	Context map[string]interface{} ` + "`json:\"context\"`" + `
}

type ExecutionOutput struct {
	Outputs map[string]interface{} ` + "`json:\"outputs\"`" + `
}

func main() {
	var execInput ExecutionInput
	json.NewDecoder(os.Stdin).Decode(&execInput)
	
	text, _ := execInput.Inputs["text"].(string)
	
	output := ExecutionOutput{
		Outputs: map[string]interface{}{
			"result": strings.ToUpper(text),
		},
	}
	json.NewEncoder(os.Stdout).Encode(output)
}`,
				With: map[string]interface{}{
					"text": "hello world",
				},
			},
			expectError:  false,
			expectedKeys: []string{"result"},
		},
		{
			name: "Empty script",
			step: &ast.Step{
				ID:     "empty_script",
				Script: " ", // Non-empty but whitespace only to trigger compilation error
				With: map[string]interface{}{
					"input": "test",
				},
			},
			expectError:   true,
			expectedError: "compilation failed",
		},
		{
			name: "Invalid Go script",
			step: &ast.Step{
				ID:     "invalid_go_script",
				Script: "invalid go syntax here",
				With: map[string]interface{}{
					"input": "test",
				},
			},
			expectError:   true,
			expectedError: "compilation failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create workflow
			workflow := &ast.Workflow{
				Version: "1.0",
				Workflow: &ast.WorkflowDef{
					Steps: []*ast.Step{tc.step},
				},
			}

			// Create executor with real block manager
			executor, err := NewExecutorWithRealBlocks(nil, workflow, nil)
			require.NoError(t, err)

			// Create execution context
			ctx := context.Background()
			execCtx := NewExecutionContext(ctx, workflow, map[string]interface{}{})

			// Execute the step
			err = executor.executeStep(execCtx, tc.step)

			if tc.expectError {
				assert.Error(t, err)
				if tc.expectedError != "" {
					assert.Contains(t, err.Error(), tc.expectedError)
				}
			} else {
				assert.NoError(t, err)

				// Check step result
				result, exists := execCtx.GetStepResult(tc.step.ID)
				assert.True(t, exists)
				assert.Equal(t, StepStatusCompleted, result.Status)

				// Check expected output keys
				for _, key := range tc.expectedKeys {
					assert.Contains(t, result.Output, key)
				}
			}
		})
	}
}

func TestExecutor_realBlockExecution_Container(t *testing.T) {
	// Skip if Docker is not available
	if !isDockerAvailable(t) {
		t.Skip("Docker not available, skipping container tests")
	}

	testCases := []struct {
		name          string
		step          *ast.Step
		expectError   bool
		expectedError string
		expectedKeys  []string
	}{
		{
			name: "Valid container execution with alpine echo",
			step: &ast.Step{
				ID:        "test_alpine_container",
				Container: "alpine:latest",
				With: map[string]interface{}{
					"message": "hello from container",
				},
			},
			expectError:  false,
			expectedKeys: []string{"message"},
		},
		{
			name: "Container with simple processing",
			step: &ast.Step{
				ID:        "test_processing_container",
				Container: "alpine:latest",
				With: map[string]interface{}{
					"text": "hello world",
				},
			},
			expectError:  false,
			expectedKeys: []string{"result"},
		},
		{
			name: "Nonexistent container image",
			step: &ast.Step{
				ID:        "nonexistent_image",
				Container: "nonexistent-image:latest",
				With: map[string]interface{}{
					"input": "test",
				},
			},
			expectError:   true,
			expectedError: "failed to pull image",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create workflow
			workflow := &ast.Workflow{
				Version: "1.0",
				Workflow: &ast.WorkflowDef{
					Steps: []*ast.Step{tc.step},
				},
			}

			// Create executor with real block manager
			executor, err := NewExecutorWithRealBlocks(nil, workflow, nil)
			require.NoError(t, err)

			// Create execution context
			ctx := context.Background()
			execCtx := NewExecutionContext(ctx, workflow, map[string]interface{}{})

			// Execute the step
			err = executor.executeStep(execCtx, tc.step)

			if tc.expectError {
				assert.Error(t, err)
				if tc.expectedError != "" {
					assert.Contains(t, err.Error(), tc.expectedError)
				}
			} else {
				assert.NoError(t, err)

				// Check step result
				result, exists := execCtx.GetStepResult(tc.step.ID)
				assert.True(t, exists)
				assert.Equal(t, StepStatusCompleted, result.Status)

				// Check expected output keys
				for _, key := range tc.expectedKeys {
					assert.Contains(t, result.Output, key)
				}
			}
		})
	}
}

// Helper function to create a test executor with real block manager
func NewExecutorWithRealBlocks(config *ExecutorConfig, workflow *ast.Workflow, registry *ModelRegistry) (*Executor, error) {
	// Use the standard NewExecutor which now includes block manager integration
	return NewExecutor(config, workflow, registry)
}

// Helper function to create real test files for block execution
func createRealTestFiles(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "lacquer-real-test-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Create a real native block directory (text analyzer)
	textAnalyzerDir := filepath.Join(tempDir, "text-analyzer")
	err = os.MkdirAll(textAnalyzerDir, 0755)
	require.NoError(t, err)

	textAnalyzerContent := `name: text-analyzer
runtime: native
description: Analyzes text sentiment

inputs:
  text:
    type: string
    required: true
    description: The text to analyze
  analysis_type:
    type: string
    required: false
    default: "sentiment"
    description: Type of analysis to perform

outputs:
  sentiment:
    type: number
    description: Sentiment score from -1 to 1
  explanation:
    type: string
    description: Explanation of the sentiment

workflow:
  agents:
    analyzer:
      provider: local
      model: claude-code
      system_prompt: "You are a text analysis expert."

  steps:
    - id: analyze
      agent: analyzer
      prompt: |
        Analyze the sentiment of this text: "{{ inputs.text }}"
        Return a JSON object with sentiment score (-1 to 1) and explanation.
        Format: {"sentiment": 0.5, "explanation": "Positive sentiment due to..."}
      outputs:
        sentiment:
          type: number
        explanation:
          type: string

  outputs:
    sentiment: "{{ steps.analyze.outputs.sentiment }}"
    explanation: "{{ steps.analyze.outputs.explanation }}"
`

	textAnalyzerPath := filepath.Join(textAnalyzerDir, "block.laq.yaml")
	err = os.WriteFile(textAnalyzerPath, []byte(textAnalyzerContent), 0644)
	require.NoError(t, err)

	// Create a Go script processor
	processorContent := `package main

import (
	"encoding/json"
	"os"
	"strings"
)

type ExecutionInput struct {
	Inputs  map[string]interface{} ` + "`json:\"inputs\"`" + `
	Env     map[string]string      ` + "`json:\"env\"`" + `
	Context map[string]interface{} ` + "`json:\"context\"`" + `
}

type ExecutionOutput struct {
	Outputs map[string]interface{} ` + "`json:\"outputs\"`" + `
}

func main() {
	var execInput ExecutionInput
	json.NewDecoder(os.Stdin).Decode(&execInput)
	
	// Extract inputs
	inputStr, _ := execInput.Inputs["input"].(string)
	mode, _ := execInput.Inputs["mode"].(string)
	
	var result string
	switch mode {
	case "uppercase":
		result = strings.ToUpper(inputStr)
	case "lowercase":
		result = strings.ToLower(inputStr)
	default:
		result = inputStr
	}
	
	output := ExecutionOutput{
		Outputs: map[string]interface{}{
			"result": result,
		},
	}
	json.NewEncoder(os.Stdout).Encode(output)
}
`

	processorPath := filepath.Join(tempDir, "processor.go")
	err = os.WriteFile(processorPath, []byte(processorContent), 0644)
	require.NoError(t, err)

	return tempDir
}

// Helper function to check if Docker is available
func isDockerAvailable(t *testing.T) bool {
	// Check if docker command is available and daemon is running
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	err := cmd.Run()
	if err != nil {
		t.Logf("Docker not available: %v", err)
		return false
	}
	return true
}
