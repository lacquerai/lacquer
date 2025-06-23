package block

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockWorkflowEngine implements WorkflowEngine for testing
type MockWorkflowEngine struct {
	mock.Mock
}

func (m *MockWorkflowEngine) Execute(ctx context.Context, workflow interface{}, inputs map[string]interface{}) (map[string]interface{}, error) {
	args := m.Called(ctx, workflow, inputs)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func TestNativeExecutor_Validation(t *testing.T) {
	engine := &MockWorkflowEngine{}
	executor := NewNativeExecutor(engine)

	tests := []struct {
		name    string
		block   *Block
		wantErr bool
		errMsg  string
	}{
		{
			name: "ValidNativeBlock",
			block: &Block{
				Name:    "test-native",
				Runtime: RuntimeNative,
				Workflow: map[string]interface{}{
					"agents": map[string]interface{}{
						"test_agent": map[string]interface{}{
							"model": "test-model",
						},
					},
					"steps": []interface{}{
						map[string]interface{}{
							"id":     "test_step",
							"agent":  "test_agent",
							"prompt": "test prompt",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "WrongRuntime",
			block: &Block{
				Name:    "test-go",
				Runtime: RuntimeGo,
				Script:  "test script",
			},
			wantErr: true,
			errMsg:  "invalid runtime for native executor",
		},
		{
			name: "MissingWorkflow",
			block: &Block{
				Name:     "test-native",
				Runtime:  RuntimeNative,
				Workflow: nil,
			},
			wantErr: true,
			errMsg:  "native block missing workflow definition",
		},
		{
			name: "EmptyWorkflow",
			block: &Block{
				Name:     "test-native",
				Runtime:  RuntimeNative,
				Workflow: map[string]interface{}{},
			},
			wantErr: false, // Empty workflow should be valid, execution will handle it
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.Validate(tt.block)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNativeExecutor_BasicExecution(t *testing.T) {
	engine := &MockWorkflowEngine{}
	executor := NewNativeExecutor(engine)

	block := &Block{
		Name:    "simple-native",
		Runtime: RuntimeNative,
		Inputs: map[string]InputSchema{
			"text": {
				Type:     "string",
				Required: true,
			},
			"mode": {
				Type:    "string",
				Default: "analyze",
			},
		},
		Outputs: map[string]OutputSchema{
			"result": {
				Type: "string",
			},
			"score": {
				Type: "number",
			},
		},
		Workflow: map[string]interface{}{
			"agents": map[string]interface{}{
				"analyzer": map[string]interface{}{
					"model": "test-model",
				},
			},
			"steps": []interface{}{
				map[string]interface{}{
					"id":     "process",
					"agent":  "analyzer",
					"prompt": "Process: {{ inputs.text }}",
				},
			},
			"outputs": map[string]interface{}{
				"result": "{{ steps.process.outputs.result }}",
				"score":  "{{ steps.process.outputs.score }}",
			},
		},
	}

	// Set up mock expectations
	expectedInputs := map[string]interface{}{
		"text": "hello world",
		"mode": "analyze",
	}
	expectedOutputs := map[string]interface{}{
		"result": "processed text",
		"score":  0.85,
	}

	engine.On("Execute", mock.Anything, block.Workflow, expectedInputs).Return(expectedOutputs, nil)

	// Create execution context
	workspace, err := os.MkdirTemp("", "native-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(workspace)

	execCtx := &ExecutionContext{
		WorkflowID: "test-workflow",
		StepID:     "test-step",
		Workspace:  workspace,
		Context:    context.Background(),
	}

	// Execute the block
	inputs := map[string]interface{}{
		"text": "hello world",
		"mode": "analyze",
	}

	outputs, err := executor.Execute(context.Background(), block, inputs, execCtx)
	
	assert.NoError(t, err)
	assert.Equal(t, expectedOutputs, outputs)
	
	// Verify mock was called with correct parameters
	engine.AssertExpectations(t)
}

func TestNativeExecutor_InputMapping(t *testing.T) {
	engine := &MockWorkflowEngine{}
	executor := NewNativeExecutor(engine)

	block := &Block{
		Name:    "input-mapping-test",
		Runtime: RuntimeNative,
		Inputs: map[string]InputSchema{
			"required_param": {
				Type:     "string",
				Required: true,
			},
			"optional_param": {
				Type:    "string",
				Default: "default_value",
			},
			"numeric_param": {
				Type: "number",
			},
		},
		Workflow: map[string]interface{}{
			"steps": []interface{}{
				map[string]interface{}{
					"id":     "test",
					"action": "update_state",
					"updates": map[string]interface{}{
						"result": "ok",
					},
				},
			},
		},
	}

	tests := []struct {
		name           string
		inputs         map[string]interface{}
		expectedInputs map[string]interface{}
		expectError    bool
	}{
		{
			name: "AllInputsProvided",
			inputs: map[string]interface{}{
				"required_param": "test_value",
				"optional_param": "custom_value",
				"numeric_param":  42.0,
			},
			expectedInputs: map[string]interface{}{
				"required_param": "test_value",
				"optional_param": "custom_value",
				"numeric_param":  42.0,
			},
			expectError: false,
		},
		{
			name: "OnlyRequiredInputs",
			inputs: map[string]interface{}{
				"required_param": "test_value",
			},
			expectedInputs: map[string]interface{}{
				"required_param": "test_value",
				"optional_param": "default_value", // Should apply default
			},
			expectError: false,
		},
		{
			name: "MissingRequiredInput",
			inputs: map[string]interface{}{
				"optional_param": "custom_value",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace, err := os.MkdirTemp("", "native-input-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(workspace)

			execCtx := &ExecutionContext{
				WorkflowID: "test-workflow",
				StepID:     "test-step",
				Workspace:  workspace,
				Context:    context.Background(),
			}

			if !tt.expectError {
				// Set up mock to expect the mapped inputs
				engine.On("Execute", mock.Anything, block.Workflow, tt.expectedInputs).Return(
					map[string]interface{}{"success": true}, nil).Once()
			}

			_, err = executor.Execute(context.Background(), block, tt.inputs, execCtx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	engine.AssertExpectations(t)
}

func TestNativeExecutor_WorkflowFailure(t *testing.T) {
	engine := &MockWorkflowEngine{}
	executor := NewNativeExecutor(engine)

	block := &Block{
		Name:    "failing-native",
		Runtime: RuntimeNative,
		Workflow: map[string]interface{}{
			"steps": []interface{}{
				map[string]interface{}{
					"id":     "failing_step",
					"action": "unknown_action",
				},
			},
		},
	}

	// Set up mock to return an error
	expectedError := assert.AnError
	engine.On("Execute", mock.Anything, block.Workflow, mock.Anything).Return(
		map[string]interface{}{}, expectedError)

	workspace, err := os.MkdirTemp("", "native-failure-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(workspace)

	execCtx := &ExecutionContext{
		WorkflowID: "test-workflow",
		StepID:     "test-step",
		Workspace:  workspace,
		Context:    context.Background(),
	}

	_, err = executor.Execute(context.Background(), block, map[string]interface{}{}, execCtx)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow execution failed")
	
	engine.AssertExpectations(t)
}

func TestNativeExecutor_ContextIsolation(t *testing.T) {
	engine := &MockWorkflowEngine{}
	executor := NewNativeExecutor(engine)

	block := &Block{
		Name:    "isolation-test",
		Runtime: RuntimeNative,
		Inputs: map[string]InputSchema{
			"block_input": {
				Type:     "string",
				Required: true,
			},
		},
		Workflow: map[string]interface{}{
			"steps": []interface{}{
				map[string]interface{}{
					"id":     "isolated_step",
					"action": "update_state",
					"updates": map[string]interface{}{
						"isolated_result": "{{ inputs.block_input }}",
					},
				},
			},
		},
	}

	// The key test: the child workflow should only have access to block inputs,
	// not any state from the parent workflow
	parentInputs := map[string]interface{}{
		"block_input":  "accessible",
		"parent_state": "should_not_be_accessible", // This should be filtered out
	}

	expectedChildInputs := map[string]interface{}{
		"block_input": "accessible", // Only this should be passed to child
	}

	engine.On("Execute", mock.Anything, block.Workflow, expectedChildInputs).Return(
		map[string]interface{}{"isolated_result": "processed"}, nil)

	workspace, err := os.MkdirTemp("", "native-isolation-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(workspace)

	execCtx := &ExecutionContext{
		WorkflowID: "test-workflow", 
		StepID:     "test-step",
		Workspace:  workspace,
		Context:    context.Background(),
	}

	_, err = executor.Execute(context.Background(), block, parentInputs, execCtx)
	
	assert.NoError(t, err)
	engine.AssertExpectations(t)
}

func TestNativeExecutor_Timeout(t *testing.T) {
	engine := &MockWorkflowEngine{}
	executor := NewNativeExecutor(engine)

	block := &Block{
		Name:    "timeout-test",
		Runtime: RuntimeNative,
		Workflow: map[string]interface{}{
			"steps": []interface{}{
				map[string]interface{}{
					"id":     "slow_step",
					"action": "update_state",
				},
			},
		},
	}

	// Create a context that times out quickly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Set up mock to simulate a slow operation that would exceed timeout
	engine.On("Execute", mock.Anything, block.Workflow, mock.Anything).Return(
		map[string]interface{}{}, context.DeadlineExceeded)

	workspace, err := os.MkdirTemp("", "native-timeout-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(workspace)

	execCtx := &ExecutionContext{
		WorkflowID: "test-workflow",
		StepID:     "test-step",
		Workspace:  workspace,
		Context:    ctx,
	}

	_, err = executor.Execute(ctx, block, map[string]interface{}{}, execCtx)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
	
	engine.AssertExpectations(t)
}

func TestNativeExecutor_ComplexWorkflow(t *testing.T) {
	engine := &MockWorkflowEngine{}
	executor := NewNativeExecutor(engine)

	// Test a more complex native block with multiple agents, steps, and outputs
	block := &Block{
		Name:    "complex-analyzer",
		Runtime: RuntimeNative,
		Inputs: map[string]InputSchema{
			"text": {
				Type:     "string",
				Required: true,
			},
			"analysis_type": {
				Type:    "string",
				Default: "full",
			},
		},
		Outputs: map[string]OutputSchema{
			"sentiment": {
				Type: "number",
			},
			"summary": {
				Type: "string",
			},
			"keywords": {
				Type: "array",
			},
		},
		Workflow: map[string]interface{}{
			"agents": map[string]interface{}{
				"sentiment_analyzer": map[string]interface{}{
					"model":       "claude-3-sonnet",
					"temperature": 0.3,
				},
				"summarizer": map[string]interface{}{
					"model":       "claude-3-sonnet",
					"temperature": 0.5,
				},
			},
			"steps": []interface{}{
				map[string]interface{}{
					"id":     "analyze_sentiment",
					"agent":  "sentiment_analyzer",
					"prompt": "Analyze sentiment of: {{ inputs.text }}",
				},
				map[string]interface{}{
					"id":        "summarize",
					"agent":     "summarizer", 
					"prompt":    "Summarize: {{ inputs.text }}",
					"condition": "{{ inputs.analysis_type == 'full' }}",
				},
				map[string]interface{}{
					"id":     "extract_keywords",
					"action": "update_state",
					"updates": map[string]interface{}{
						"keywords": "['key1', 'key2']",
					},
				},
			},
			"outputs": map[string]interface{}{
				"sentiment": "{{ steps.analyze_sentiment.outputs.sentiment }}",
				"summary":   "{{ steps.summarize.outputs.summary }}",
				"keywords":  "{{ steps.extract_keywords.outputs.keywords }}",
			},
		},
	}

	expectedInputs := map[string]interface{}{
		"text":          "This is a complex test text for analysis",
		"analysis_type": "full",
	}
	
	expectedOutputs := map[string]interface{}{
		"sentiment": 0.75,
		"summary":   "Complex analysis summary",
		"keywords":  []interface{}{"test", "analysis", "complex"},
	}

	engine.On("Execute", mock.Anything, block.Workflow, expectedInputs).Return(expectedOutputs, nil)

	workspace, err := os.MkdirTemp("", "native-complex-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(workspace)

	execCtx := &ExecutionContext{
		WorkflowID: "test-workflow",
		StepID:     "test-step",
		Workspace:  workspace,
		Context:    context.Background(),
	}

	outputs, err := executor.Execute(context.Background(), block, expectedInputs, execCtx)
	
	assert.NoError(t, err)
	assert.Equal(t, expectedOutputs, outputs)
	
	// Verify the outputs match expected schema types
	assert.IsType(t, float64(0), outputs["sentiment"])
	assert.IsType(t, "", outputs["summary"])
	assert.IsType(t, []interface{}{}, outputs["keywords"])
	
	engine.AssertExpectations(t)
}