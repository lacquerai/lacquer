package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lacquer/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRuntimeComprehensive_HTTPMocked tests with real HTTP calls to mock servers
func TestRuntimeComprehensive_HTTPMocked(t *testing.T) {
	// Create mock OpenAI server
	openaiServer := createMockOpenAIServer(t)
	defer openaiServer.Close()

	// Create mock Anthropic server  
	anthropicServer := createMockAnthropicServer(t)
	defer anthropicServer.Close()

	// Set environment variables to point to our mock servers
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("OPENAI_BASE_URL", openaiServer.URL)
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	t.Setenv("ANTHROPIC_BASE_URL", anthropicServer.URL)

	// Create executor (will register providers with our mock URLs)
	executor := NewExecutor(nil)

	// Test OpenAI workflow
	t.Run("OpenAI GPT-4 workflow", func(t *testing.T) {
		workflow := createTestWorkflow("gpt-4", "openai_agent")
		result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{
			"topic": "artificial intelligence",
		})

		require.NoError(t, err)
		assert.Equal(t, ExecutionStatusCompleted, result.Status)
		assert.Equal(t, 3, len(result.Steps))

		// Verify responses match what we mocked
		step1 := findStepByID(result.Steps, "step1")
		assert.Equal(t, "Mock GPT-4 response for: Process this topic: artificial intelligence", step1.Response)

		step2 := findStepByID(result.Steps, "step2")
		assert.Contains(t, step2.Response, "Mock GPT-4 response for: Build on:")

		// Verify state updates worked
		assert.Equal(t, 1, result.State["counter"])
		assert.Equal(t, "completed", result.State["status"])
	})

	// Test Anthropic workflow
	t.Run("Anthropic Claude workflow", func(t *testing.T) {
		workflow := createTestWorkflow("claude-3-sonnet", "claude_agent")
		result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{
			"topic": "machine learning",
		})

		require.NoError(t, err)
		assert.Equal(t, ExecutionStatusCompleted, result.Status)
		assert.Equal(t, 3, len(result.Steps))

		// Verify Claude responses
		step1 := findStepByID(result.Steps, "step1")
		assert.Equal(t, "Mock Claude response for: Process this topic: machine learning", step1.Response)
	})
}

// TestRuntimeComprehensive_HTTPErrorHandling tests HTTP error scenarios
func TestRuntimeComprehensive_HTTPErrorHandling(t *testing.T) {
	testCases := []struct {
		name           string
		serverHandler  func(w http.ResponseWriter, r *http.Request)
		expectedStatus ExecutionStatus
		expectedError  string
	}{
		{
			name: "OpenAI Rate Limit Error",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]interface{}{
						"type":    "rate_limit_exceeded",
						"message": "Rate limit exceeded",
					},
				})
			},
			expectedStatus: ExecutionStatusFailed,
			expectedError:  "rate_limit_exceeded",
		},
		{
			name: "OpenAI Invalid API Key",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]interface{}{
						"type":    "invalid_request_error",
						"message": "Incorrect API key provided",
					},
				})
			},
			expectedStatus: ExecutionStatusFailed,
			expectedError:  "invalid_request_error",
		},
		{
			name: "Server Internal Error",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal server error"))
			},
			expectedStatus: ExecutionStatusFailed,
			expectedError:  "500",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create server with specific error handler
			server := httptest.NewServer(http.HandlerFunc(tc.serverHandler))
			defer server.Close()

			t.Setenv("OPENAI_API_KEY", "test-key")
			t.Setenv("OPENAI_BASE_URL", server.URL)

			executor := NewExecutor(nil)
			workflow := createTestWorkflow("gpt-4", "error_agent")

			result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{
				"topic": "test",
			})

			// Should complete execution but with failed status
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, result.Status)

			// Check that error is captured in step result
			step1 := findStepByID(result.Steps, "step1")
			assert.Equal(t, StepStatusFailed, step1.Status)
			if tc.expectedError != "" {
				assert.Contains(t, step1.Error.Error(), tc.expectedError)
			}
		})
	}
}

// TestRuntimeComprehensive_HTTPRetryLogic tests retry behavior with HTTP failures
func TestRuntimeComprehensive_HTTPRetryLogic(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		
		// Fail first 2 attempts, succeed on 3rd
		if callCount < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Success response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": fmt.Sprintf("Success after %d attempts", callCount),
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 20,
				"total_tokens":      30,
			},
		})
	}))
	defer server.Close()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)

	executor := NewExecutor(nil)
	workflow := createTestWorkflow("gpt-4", "retry_agent")

	result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{
		"topic": "retry test",
	})

	require.NoError(t, err)
	assert.Equal(t, ExecutionStatusCompleted, result.Status)

	// Verify retry worked - should have made 3 calls
	assert.Equal(t, 3, callCount)

	step1 := findStepByID(result.Steps, "step1")
	assert.Equal(t, StepStatusCompleted, step1.Status)
	assert.Equal(t, "Success after 3 attempts", step1.Response)
}

// TestRuntimeComprehensive_HTTPConcurrentRequests tests concurrent HTTP requests
func TestRuntimeComprehensive_HTTPConcurrentRequests(t *testing.T) {
	requestTimes := make(map[string]time.Time)
	var requestsMutex sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record when each request started
		requestsMutex.Lock()
		requestID := fmt.Sprintf("req_%d", len(requestTimes))
		requestTimes[requestID] = time.Now()
		requestsMutex.Unlock()

		// Simulate some processing time
		time.Sleep(100 * time.Millisecond)

		// Return response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": fmt.Sprintf("Concurrent response %s", requestID),
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     5,
				"completion_tokens": 10,
				"total_tokens":      15,
			},
		})
	}))
	defer server.Close()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)

	// Configure executor for concurrent execution
	config := &ExecutorConfig{
		MaxConcurrentSteps: 3,
		DefaultTimeout:     10 * time.Second,
	}
	executor := NewExecutor(config)

	// Create workflow with independent parallel steps
	workflow := &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name: "concurrent-test-workflow",
		},
		Agents: map[string]*ast.Agent{
			"concurrent_agent": {Model: "gpt-4"},
		},
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "parallel1", Agent: "concurrent_agent", Prompt: "Task 1"},
				{ID: "parallel2", Agent: "concurrent_agent", Prompt: "Task 2"},
				{ID: "parallel3", Agent: "concurrent_agent", Prompt: "Task 3"},
			},
		},
	}

	start := time.Now()
	result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{})
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, ExecutionStatusCompleted, result.Status)

	// With concurrency, should complete faster than 3 sequential requests (300ms)
	assert.Less(t, duration, 250*time.Millisecond, "Concurrent execution should be faster than sequential")

	// Should have made 3 concurrent requests
	assert.Equal(t, 3, len(requestTimes))
}

// TestRuntimeComprehensive_HTTPVariableInterpolation tests variable substitution in HTTP requests
func TestRuntimeComprehensive_HTTPVariableInterpolation(t *testing.T) {
	var capturedPrompts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the actual prompt sent in the request
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		
		if messages, ok := reqBody["messages"].([]interface{}); ok && len(messages) > 0 {
			if msg, ok := messages[0].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					capturedPrompts = append(capturedPrompts, content)
				}
			}
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": fmt.Sprintf("Response to: %s", capturedPrompts[len(capturedPrompts)-1]),
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 20,
				"total_tokens":      30,
			},
		})
	}))
	defer server.Close()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)

	executor := NewExecutor(nil)

	workflow := &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name: "interpolation-test-workflow",
		},
		Agents: map[string]*ast.Agent{
			"interpolation_agent": {Model: "gpt-4"},
		},
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"name": {Type: "string", Required: true},
				"topic": {Type: "string", Required: true},
			},
			State: map[string]interface{}{
				"prefix": "Hello",
			},
			Steps: []*ast.Step{
				{
					ID:     "greeting",
					Agent:  "interpolation_agent",
					Prompt: "{{ state.prefix }}, {{ inputs.name }}! Let's discuss {{ inputs.topic }}.",
				},
				{
					ID:     "follow_up",
					Agent:  "interpolation_agent",
					Prompt: "Based on the greeting: {{ steps.greeting.response }}, continue the conversation about {{ inputs.topic }}.",
				},
			},
		},
	}

	result, err := executor.Execute(context.Background(), workflow, map[string]interface{}{
		"name":  "Alice",
		"topic": "machine learning",
	})

	require.NoError(t, err)
	assert.Equal(t, ExecutionStatusCompleted, result.Status)

	// Verify that variable interpolation worked in the actual HTTP requests
	require.Equal(t, 2, len(capturedPrompts))
	assert.Equal(t, "Hello, Alice! Let's discuss machine learning.", capturedPrompts[0])
	assert.Contains(t, capturedPrompts[1], "Response to: Hello, Alice! Let's discuss machine learning.")
	assert.Contains(t, capturedPrompts[1], "machine learning")
}

// Helper functions

func createMockOpenAIServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a valid OpenAI API call
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/chat/completions")
		assert.Equal(t, "Bearer test-openai-key", r.Header.Get("Authorization"))

		// Parse request to get prompt
		var reqBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		prompt := "unknown prompt"
		if messages, ok := reqBody["messages"].([]interface{}); ok && len(messages) > 0 {
			if msg, ok := messages[0].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					prompt = content
				}
			}
		}

		// Return mock OpenAI response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": fmt.Sprintf("Mock GPT-4 response for: %s", prompt),
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     len(strings.Split(prompt, " ")),
				"completion_tokens": 20,
				"total_tokens":      len(strings.Split(prompt, " ")) + 20,
			},
		})
	}))
}

func createMockAnthropicServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a valid Anthropic API call
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/messages")
		assert.Equal(t, "test-anthropic-key", r.Header.Get("x-api-key"))

		// Parse request to get prompt
		var reqBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		prompt := "unknown prompt"
		if messages, ok := reqBody["messages"].([]interface{}); ok && len(messages) > 0 {
			if msg, ok := messages[0].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					prompt = content
				}
			}
		}

		// Return mock Anthropic response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("Mock Claude response for: %s", prompt),
				},
			},
			"usage": map[string]interface{}{
				"input_tokens":  len(strings.Split(prompt, " ")),
				"output_tokens": 15,
			},
		})
	}))
}

func createTestWorkflow(model, agentName string) *ast.Workflow {
	return &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name: "http-test-workflow",
		},
		Agents: map[string]*ast.Agent{
			agentName: {
				Model:       model,
				Temperature: floatPtr(0.7),
			},
		},
		Workflow: &ast.WorkflowDef{
			Inputs: map[string]*ast.InputParam{
				"topic": {Type: "string", Required: true},
			},
			State: map[string]interface{}{
				"counter": 0,
				"status":  "initialized",
			},
			Steps: []*ast.Step{
				{
					ID:     "step1",
					Agent:  agentName,
					Prompt: "Process this topic: {{ inputs.topic }}",
				},
				{
					ID:      "step2",
					Agent:   agentName,
					Prompt:  "Build on: {{ steps.step1.response }}",
					Updates: map[string]interface{}{
						"counter": 1,
						"status":  "processing",
					},
				},
				{
					ID:     "step3",
					Agent:  agentName,
					Prompt: "Finalize: {{ steps.step2.response }}",
					Updates: map[string]interface{}{
						"status": "completed",
					},
				},
			},
		},
	}
}

func findStepByID(steps []StepResult, stepID string) *StepResult {
	for i := range steps {
		if steps[i].StepID == stepID {
			return &steps[i]
		}
	}
	return nil
}

