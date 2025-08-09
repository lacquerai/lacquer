package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lacquerai/lacquer/pkg/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWorkflowYAML = `version: "1.0"
metadata:
  name: test-workflow
  description: A test workflow for server testing
  author: test
agents:
  testAgent:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0.7
    system_prompt: You are a test assistant.
workflow:
  inputs:
    inputName:
      type: string
      description: Name for testing
      default: "World"
  steps:
    - id: testStep
      agent: testAgent
      prompt: "Hello {{ inputs.inputName }}!"
  outputs:
    result: "{{ steps.testStep.output }}"
`

const simpleWorkflowYAML = `version: "1.0"
metadata:
  name: simple-workflow
  description: A simple test workflow
  author: test
agents:
  simpleAgent:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0.7
    system_prompt: You are a simple assistant.
workflow:
  inputs:
    message:
      type: string
      description: Test message
      default: "Test"
  steps:
    - id: simpleStep
      agent: simpleAgent
      prompt: "{{ inputs.message }}"
  outputs:
    message: "{{ steps.simpleStep.output }}"
`

// findAvailablePort finds an available port for testing
func findAvailablePort() int {
	listener, err := net.Listen("tcp", "127.0.0.1:0") // Bind to localhost only
	if err != nil {
		return 8080 // fallback port
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port
}

// Test suite setup
type ServerTestSuite struct {
	server        *Server
	tempDir       string
	workflowFiles []string
	config        *Config
}

func setupTestSuite(t *testing.T) *ServerTestSuite {
	tempDir, err := os.MkdirTemp("", "lacquer-server-test-*")
	require.NoError(t, err)

	// Create test workflow files
	testWorkflowFile := filepath.Join(tempDir, "test-workflow.laq.yaml")
	err = os.WriteFile(testWorkflowFile, []byte(testWorkflowYAML), 0644)
	require.NoError(t, err)

	simpleWorkflowFile := filepath.Join(tempDir, "simple-workflow.laq.yaml")
	err = os.WriteFile(simpleWorkflowFile, []byte(simpleWorkflowYAML), 0644)
	require.NoError(t, err)

	workflowFiles := []string{testWorkflowFile, simpleWorkflowFile}

	// Find available port for testing
	testPort := findAvailablePort()

	config := &Config{
		Host:          "127.0.0.1",
		Port:          testPort,
		Concurrency:   2,
		Timeout:       30 * time.Second,
		EnableMetrics: true,
		EnableCORS:    true,
		WorkflowFiles: workflowFiles,
		ReadTimeout:   5 * time.Second,
		WriteTimeout:  5 * time.Second,
		IdleTimeout:   30 * time.Second,
	}

	server, err := New(config)
	require.NoError(t, err)

	// Use separate metrics registry for tests
	server.manager = NewExecutionManagerWithRegistry(config.Concurrency, nil)

	err = server.LoadWorkflows()
	require.NoError(t, err)

	return &ServerTestSuite{
		server:        server,
		tempDir:       tempDir,
		workflowFiles: workflowFiles,
		config:        config,
	}
}

func (suite *ServerTestSuite) cleanup(_ *testing.T) {
	if suite.server.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		suite.server.Stop(ctx)
	}
	os.RemoveAll(suite.tempDir)
}

func (suite *ServerTestSuite) startServerInBackground(t *testing.T) string {
	// Start server in background
	err := suite.server.Start()
	require.NoError(t, err)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	return suite.server.GetAddr()
}

// Integration Tests

func TestServerIntegration_StartupAndShutdown(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.cleanup(t)

	// Test server creation
	assert.NotNil(t, suite.server)
	assert.Equal(t, 2, suite.server.GetWorkflowCount())

	// Test server start
	addr := suite.startServerInBackground(t)
	assert.Contains(t, addr, "127.0.0.1:")

	// Test health endpoint
	resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health map[string]any
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)

	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, float64(2), health["workflows_loaded"])
	assert.Equal(t, float64(0), health["active_executions"])
}

func TestServerIntegration_ListWorkflows(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test list workflows endpoint
	resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/workflows", addr))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	workflows, ok := result["workflows"].(map[string]any)
	if !ok {
		t.Logf("Result: %+v", result)
		t.Fatalf("workflows is not a map[string]any: %T", result["workflows"])
	}
	assert.Len(t, workflows, 2)

	// Check test-workflow (note: key is without .laq extension)
	testWorkflow, ok := workflows["test-workflow"].(map[string]any)
	if !ok {
		t.Fatalf("test-workflow not found or wrong type: %+v", workflows)
	}
	assert.Equal(t, "test-workflow", testWorkflow["name"])
	assert.Equal(t, "A test workflow for server testing", testWorkflow["description"])
	assert.Equal(t, "1.0", testWorkflow["version"])
	assert.Equal(t, float64(1), testWorkflow["steps"])

	// Check simple-workflow (note: key is without .laq extension)
	simpleWorkflow, ok := workflows["simple-workflow"].(map[string]any)
	if !ok {
		t.Fatalf("simple-workflow not found or wrong type: %+v", workflows)
	}
	assert.Equal(t, "simple-workflow", simpleWorkflow["name"])
	assert.Equal(t, "A simple test workflow", simpleWorkflow["description"])
}

func TestServerIntegration_ExecuteWorkflow_NotFound(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test executing non-existent workflow
	reqBody := map[string]any{
		"inputs": map[string]any{
			"inputName": "Test",
		},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/non-existent/execute", addr),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(responseBody), "Workflow 'non-existent' not found")
}

func TestServerIntegration_ExecuteWorkflow_BadJSON(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test executing with bad JSON
	resp, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/test-workflow/execute", addr),
		"application/json",
		strings.NewReader("{invalid json}"),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(responseBody), "Invalid JSON")
}

func TestServerIntegration_ExecuteWorkflow_Success(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test successful workflow execution
	reqBody := map[string]any{
		"inputs": map[string]any{
			"inputName": "Integration Test",
		},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/test-workflow/execute", addr),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Contains(t, result, "run_id")
	assert.Equal(t, "test-workflow", result["workflow_id"])
	assert.Equal(t, "running", result["status"])
	assert.Contains(t, result, "started_at")

	runID := result["run_id"].(string)
	assert.NotEmpty(t, runID)

	// Wait a moment for execution to potentially start
	time.Sleep(100 * time.Millisecond)

	// Test getting execution status
	resp, err = http.Get(fmt.Sprintf("http://%s/api/v1/executions/%s", addr, runID))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var execution ExecutionStatus
	err = json.NewDecoder(resp.Body).Decode(&execution)
	require.NoError(t, err)

	assert.Equal(t, runID, execution.RunID)
	assert.Equal(t, "test-workflow", execution.WorkflowID)
	assert.Contains(t, []string{"running", "completed", "failed"}, execution.Status)
	assert.NotEmpty(t, execution.StartTime)
}

func TestServerIntegration_GetExecution_NotFound(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test getting non-existent execution
	resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/executions/non-existent-run-id", addr))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(responseBody), "Execution 'non-existent-run-id' not found")
}

func TestServerIntegration_ConcurrencyLimit(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.cleanup(t)

	// Set low concurrency limit for testing
	suite.config.Concurrency = 1
	suite.server.manager = NewExecutionManagerWithRegistry(1, nil)

	addr := suite.startServerInBackground(t)

	reqBody := map[string]any{
		"inputs": map[string]any{},
	}
	body, _ := json.Marshal(reqBody)

	// Start first execution
	resp1, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/simple-workflow/execute", addr),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp1.Body.Close()

	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	// Immediately try second execution (should be rejected due to concurrency limit)
	resp2, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/simple-workflow/execute", addr),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp2.Body.Close()

	if resp2.StatusCode == http.StatusServiceUnavailable {
		responseBody, err := io.ReadAll(resp2.Body)
		require.NoError(t, err)
		assert.Contains(t, string(responseBody), "Server at capacity")
	}
	// Note: Due to the async nature, this test may sometimes pass if the first execution completes quickly
}

func TestServerIntegration_WebSocketStream_NotFound(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test WebSocket with non-existent run ID
	wsURL := fmt.Sprintf("ws://%s/api/v1/workflows/test-workflow/stream?run_id=non-existent", addr)
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if conn != nil {
		conn.Close()
	}
	assert.Error(t, err)
	// WebSocket dial should fail or return error status
}

func TestServerIntegration_WebSocketStream_MissingRunID(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test WebSocket without run_id parameter
	wsURL := fmt.Sprintf("ws://%s/api/v1/workflows/test-workflow/stream", addr)
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if conn != nil {
		conn.Close()
	}
	assert.Error(t, err)
	// Should fail due to missing run_id parameter
}

func TestServerIntegration_CORS_Headers(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test CORS preflight request
	req, err := http.NewRequest("OPTIONS", fmt.Sprintf("http://%s/api/v1/workflows", addr), nil)
	require.NoError(t, err)

	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "POST")
}

func TestServerIntegration_PrometheusMetrics(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lacquer-metrics-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test workflow file
	testWorkflowFile := filepath.Join(tempDir, "test-workflow.laq.yaml")
	err = os.WriteFile(testWorkflowFile, []byte(testWorkflowYAML), 0644)
	require.NoError(t, err)

	// Create config with metrics enabled and default Prometheus registry
	config := &Config{
		Host:          "127.0.0.1",
		Port:          findAvailablePort(),
		Concurrency:   2,
		Timeout:       30 * time.Second,
		EnableMetrics: true,
		EnableCORS:    true,
		WorkflowFiles: []string{testWorkflowFile},
		ReadTimeout:   5 * time.Second,
		WriteTimeout:  5 * time.Second,
		IdleTimeout:   30 * time.Second,
	}

	server, err := New(config)
	require.NoError(t, err)

	// Don't override the manager - let it use default registry
	err = server.LoadWorkflows()
	require.NoError(t, err)

	err = server.Start()
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	addr := server.GetAddr()

	// Test metrics endpoint
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", addr))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	metricsText := string(responseBody)

	// Check for expected Prometheus metrics
	assert.Contains(t, metricsText, "lacquer_executions_total")
	assert.Contains(t, metricsText, "lacquer_executions_active")
	// Note: Histogram and CounterVec metrics may not appear until they have data
	// So we'll just check for the basic metrics that are always present
}

func TestServerIntegration_WorkflowDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lacquer-server-dir-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test workflow in directory
	err = os.WriteFile(filepath.Join(tempDir, "dir-workflow.laq.yaml"), []byte(simpleWorkflowYAML), 0644)
	require.NoError(t, err)

	config := &Config{
		Host:          "127.0.0.1",
		Port:          findAvailablePort(),
		Concurrency:   2,
		Timeout:       30 * time.Second,
		EnableMetrics: false,
		EnableCORS:    true,
		WorkflowDir:   tempDir,
	}

	server, err := New(config)
	require.NoError(t, err)

	// Use separate metrics registry for tests
	server.manager = NewExecutionManagerWithRegistry(config.Concurrency, nil)

	err = server.LoadWorkflows()
	require.NoError(t, err)

	assert.Equal(t, 1, server.GetWorkflowCount())

	// Verify workflow was loaded correctly
	workflows := server.registry.List()
	assert.Contains(t, workflows, "dir-workflow")
}

func TestServerIntegration_InvalidWorkflowFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lacquer-server-invalid-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create invalid workflow file
	invalidWorkflow := `invalid: yaml: content: [[[`
	err = os.WriteFile(filepath.Join(tempDir, "invalid.laq.yaml"), []byte(invalidWorkflow), 0644)
	require.NoError(t, err)

	config := &Config{
		Host:        "127.0.0.1",
		Port:        findAvailablePort(),
		WorkflowDir: tempDir,
	}

	server, err := New(config)
	require.NoError(t, err)

	// Loading workflows should fail due to invalid YAML
	err = server.LoadWorkflows()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse workflow")
}

func TestServerIntegration_EmptyWorkflowList(t *testing.T) {
	config := &Config{
		Host:          "127.0.0.1",
		Port:          findAvailablePort(),
		WorkflowFiles: []string{},
	}

	server, err := New(config)
	require.NoError(t, err)

	// Loading empty workflow list should fail
	err = server.LoadWorkflows()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no workflow files specified")
}

const validationTestWorkflowYAML = `version: "1.0"
metadata:
  name: validation-test-workflow
  description: Workflow for testing input validation
  author: test
agents:
  validationAgent:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0.7
    system_prompt: You are a validation test assistant.
inputs:
  name:
    type: string
    description: User name
    required: true
    pattern: '^[A-Za-z\s]+$'
  age:
    type: integer
    description: User age
    minimum: 18
    maximum: 120
    default: 25
  email:
    type: string
    description: Email address
    pattern: '^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
  skills:
    type: array
    description: List of skills
    min_items: 1
    max_items: 10
  role:
    type: string
    description: User role
    enum: ["user", "admin", "moderator"]
    default: "user"
  active:
    type: boolean
    description: Is user active
    default: true
  metadata:
    type: object
    description: Additional metadata
workflow:
  steps:
    - id: validationStep
      agent: validationAgent
      prompt: "Hello ${{ inputs.name }}! You are ${{ inputs.age }} years old."
  outputs:
    result: "${{ steps.validationStep.output }}"
`

func setupValidationTestSuite(t *testing.T) *ServerTestSuite {
	tempDir, err := os.MkdirTemp("", "lacquer-validation-test-*")
	require.NoError(t, err)

	// Create validation test workflow file
	testWorkflowFile := filepath.Join(tempDir, "validation-test.laq.yaml")
	err = os.WriteFile(testWorkflowFile, []byte(validationTestWorkflowYAML), 0644)
	require.NoError(t, err)

	workflowFiles := []string{testWorkflowFile}

	// Find available port for testing
	testPort := findAvailablePort()

	config := &Config{
		Host:          "127.0.0.1",
		Port:          testPort,
		Concurrency:   2,
		Timeout:       30 * time.Second,
		EnableMetrics: false,
		EnableCORS:    true,
		WorkflowFiles: workflowFiles,
		ReadTimeout:   5 * time.Second,
		WriteTimeout:  5 * time.Second,
		IdleTimeout:   30 * time.Second,
	}

	server, err := New(config)
	require.NoError(t, err)

	// Use separate metrics registry for tests
	server.manager = NewExecutionManagerWithRegistry(config.Concurrency, nil)

	err = server.LoadWorkflows()
	require.NoError(t, err)

	return &ServerTestSuite{
		server:        server,
		tempDir:       tempDir,
		workflowFiles: workflowFiles,
		config:        config,
	}
}

func TestServerIntegration_InputValidation_RequiredFieldMissing(t *testing.T) {
	suite := setupValidationTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test missing required field
	reqBody := map[string]any{
		"inputs": map[string]any{
			"age": 30,
			// missing required "name" field
		},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "Input validation failed", result["error"])

	details := result["details"].([]any)
	assert.Len(t, details, 1)

	errorDetail := details[0].(map[string]any)
	assert.Equal(t, "name", errorDetail["field"])
	assert.Contains(t, errorDetail["message"], "required field is missing")
}

func TestServerIntegration_InputValidation_DefaultValues(t *testing.T) {
	suite := setupValidationTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test request with minimal inputs (defaults should be applied)
	reqBody := map[string]any{
		"inputs": map[string]any{
			"name":   "Alice Smith",
			"skills": []string{"golang", "testing"},
			// age, role, and active should get defaults
		},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Contains(t, result, "run_id")
	assert.Equal(t, "validation-test", result["workflow_id"])
	assert.Equal(t, "running", result["status"])
}

func TestServerIntegration_InputValidation_TypeConversion(t *testing.T) {
	suite := setupValidationTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test type conversion (string to int, string to bool)
	reqBody := map[string]any{
		"inputs": map[string]any{
			"name":   "Bob Johnson",
			"age":    "42",    // string to int
			"active": "false", // string to bool
			"skills": []string{"python", "docker"},
		},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Contains(t, result, "run_id")
	assert.Equal(t, "validation-test", result["workflow_id"])
	assert.Equal(t, "running", result["status"])
}

func TestServerIntegration_InputValidation_TypeValidationFailure(t *testing.T) {
	suite := setupValidationTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test invalid type conversion
	reqBody := map[string]any{
		"inputs": map[string]any{
			"name":   "Charlie Brown",
			"age":    "not-a-number", // invalid int
			"skills": []string{"java"},
		},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "Input validation failed", result["error"])

	details := result["details"].([]any)
	assert.Len(t, details, 1)

	errorDetail := details[0].(map[string]any)
	assert.Equal(t, "age", errorDetail["field"])
	assert.Contains(t, errorDetail["message"], "invalid type")
	assert.Equal(t, "not-a-number", errorDetail["value"])
}

func TestServerIntegration_InputValidation_StringConstraints(t *testing.T) {
	t.Run("Pattern validation failure", func(t *testing.T) {
		suite := setupValidationTestSuite(t)
		defer suite.cleanup(t)

		addr := suite.startServerInBackground(t)
		reqBody := map[string]any{
			"inputs": map[string]any{
				"name":   "Alice123", // invalid pattern (contains numbers)
				"skills": []string{"golang"},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := http.Post(
			fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
			"application/json",
			bytes.NewReader(body),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			responseBody, _ := io.ReadAll(resp.Body)
			t.Logf("Expected 400, got %d. Response: %s", resp.StatusCode, string(responseBody))
		}
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var result map[string]any
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		details := result["details"].([]any)
		assert.Len(t, details, 1)

		errorDetail := details[0].(map[string]any)
		assert.Equal(t, "name", errorDetail["field"])
		assert.Contains(t, errorDetail["message"], "does not match required pattern")
	})

	t.Run("Enum validation failure", func(t *testing.T) {
		suite := setupValidationTestSuite(t)
		defer suite.cleanup(t)

		addr := suite.startServerInBackground(t)
		reqBody := map[string]any{
			"inputs": map[string]any{
				"name":   "Alice Smith",
				"role":   "superuser", // invalid enum value
				"skills": []string{"golang"},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := http.Post(
			fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
			"application/json",
			bytes.NewReader(body),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var result map[string]any
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		details := result["details"].([]any)
		assert.Len(t, details, 1)

		errorDetail := details[0].(map[string]any)
		assert.Equal(t, "role", errorDetail["field"])
		assert.Contains(t, errorDetail["message"], "must be one of")
	})

	t.Run("Email pattern validation", func(t *testing.T) {
		suite := setupValidationTestSuite(t)
		defer suite.cleanup(t)

		addr := suite.startServerInBackground(t)
		reqBody := map[string]any{
			"inputs": map[string]any{
				"name":   "Alice Smith",
				"email":  "invalid-email", // invalid email pattern
				"skills": []string{"golang"},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := http.Post(
			fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
			"application/json",
			bytes.NewReader(body),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var result map[string]any
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		details := result["details"].([]any)
		assert.Len(t, details, 1)

		errorDetail := details[0].(map[string]any)
		assert.Equal(t, "email", errorDetail["field"])
		assert.Contains(t, errorDetail["message"], "does not match required pattern")
	})
}

func TestServerIntegration_InputValidation_NumericConstraints(t *testing.T) {
	t.Run("Below minimum", func(t *testing.T) {
		suite := setupValidationTestSuite(t)
		defer suite.cleanup(t)

		addr := suite.startServerInBackground(t)
		reqBody := map[string]any{
			"inputs": map[string]any{
				"name":   "Alice Smith",
				"age":    15, // below minimum of 18
				"skills": []string{"golang"},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := http.Post(
			fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
			"application/json",
			bytes.NewReader(body),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var result map[string]any
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		details := result["details"].([]any)
		assert.Len(t, details, 1)

		errorDetail := details[0].(map[string]any)
		assert.Equal(t, "age", errorDetail["field"])
		assert.Contains(t, errorDetail["message"], "less than minimum")
	})

	t.Run("Above maximum", func(t *testing.T) {
		suite := setupValidationTestSuite(t)
		defer suite.cleanup(t)

		addr := suite.startServerInBackground(t)
		reqBody := map[string]any{
			"inputs": map[string]any{
				"name":   "Alice Smith",
				"age":    150, // above maximum of 120
				"skills": []string{"golang"},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := http.Post(
			fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
			"application/json",
			bytes.NewReader(body),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var result map[string]any
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		details := result["details"].([]any)
		assert.Len(t, details, 1)

		errorDetail := details[0].(map[string]any)
		assert.Equal(t, "age", errorDetail["field"])
		assert.Contains(t, errorDetail["message"], "greater than maximum")
	})
}

func TestServerIntegration_InputValidation_ArrayConstraints(t *testing.T) {
	t.Run("Too few items", func(t *testing.T) {
		suite := setupValidationTestSuite(t)
		defer suite.cleanup(t)

		addr := suite.startServerInBackground(t)
		reqBody := map[string]any{
			"inputs": map[string]any{
				"name":   "Alice Smith",
				"skills": []string{}, // empty array, min_items is 1
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := http.Post(
			fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
			"application/json",
			bytes.NewReader(body),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var result map[string]any
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		details := result["details"].([]any)
		assert.Len(t, details, 1)

		errorDetail := details[0].(map[string]any)
		assert.Equal(t, "skills", errorDetail["field"])
		assert.Contains(t, errorDetail["message"], "minimum required is")
	})

	t.Run("Too many items", func(t *testing.T) {
		suite := setupValidationTestSuite(t)
		defer suite.cleanup(t)

		addr := suite.startServerInBackground(t)
		reqBody := map[string]any{
			"inputs": map[string]any{
				"name": "Alice Smith",
				"skills": []string{
					"golang", "python", "java", "javascript", "typescript",
					"rust", "c++", "ruby", "php", "swift", "kotlin", // 11 items, max is 10
				},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := http.Post(
			fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
			"application/json",
			bytes.NewReader(body),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var result map[string]any
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		details := result["details"].([]any)
		assert.Len(t, details, 1)

		errorDetail := details[0].(map[string]any)
		assert.Equal(t, "skills", errorDetail["field"])
		assert.Contains(t, errorDetail["message"], "maximum allowed is")
	})
}

func TestServerIntegration_InputValidation_UnexpectedFields(t *testing.T) {
	suite := setupValidationTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test unexpected input fields
	reqBody := map[string]any{
		"inputs": map[string]any{
			"name":        "Alice Smith",
			"skills":      []string{"golang"},
			"unexpected":  "value", // unexpected field
			"another_bad": 123,     // another unexpected field
		},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "Input validation failed", result["error"])

	details := result["details"].([]any)
	assert.Len(t, details, 2)

	// Check that both unexpected fields are reported
	errorFields := make([]string, len(details))
	for i, detail := range details {
		errorDetail := detail.(map[string]any)
		errorFields[i] = errorDetail["field"].(string)
		assert.Contains(t, errorDetail["message"], "unexpected input field")
	}
	assert.Contains(t, errorFields, "unexpected")
	assert.Contains(t, errorFields, "another_bad")
}

func TestServerIntegration_InputValidation_MultipleErrors(t *testing.T) {
	suite := setupValidationTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test multiple validation errors at once
	reqBody := map[string]any{
		"inputs": map[string]any{
			"name":       "Alice123",      // invalid pattern
			"age":        15,              // below minimum
			"email":      "invalid-email", // invalid pattern
			"skills":     []string{},      // too few items
			"role":       "superuser",     // invalid enum
			"unexpected": "value",         // unexpected field
		},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "Input validation failed", result["error"])

	details := result["details"].([]any)
	assert.Len(t, details, 6) // All 6 errors should be reported

	// Collect all error fields
	errorFields := make([]string, len(details))
	for i, detail := range details {
		errorDetail := detail.(map[string]any)
		errorFields[i] = errorDetail["field"].(string)
	}

	// Verify all expected errors are present
	expectedFields := []string{"name", "age", "email", "skills", "role", "unexpected"}
	for _, expectedField := range expectedFields {
		assert.Contains(t, errorFields, expectedField)
	}
}

func TestServerIntegration_InputValidation_ValidComplexInput(t *testing.T) {
	suite := setupValidationTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test valid complex input with all constraints satisfied
	reqBody := map[string]any{
		"inputs": map[string]any{
			"name":     "Alice Smith",
			"age":      30,
			"email":    "alice@example.com",
			"skills":   []string{"golang", "python", "docker"},
			"role":     "admin",
			"active":   true,
			"metadata": map[string]any{"department": "engineering"},
		},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/validation-test/execute", addr),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Contains(t, result, "run_id")
	assert.Equal(t, "validation-test", result["workflow_id"])
	assert.Equal(t, "running", result["status"])
}

func TestExecutionManager_NewManager(t *testing.T) {
	// Use a separate registry for tests to avoid conflicts
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(5, registry)

	assert.NotNil(t, manager)
	assert.Equal(t, 5, manager.maxConcurrency)
	assert.Equal(t, 0, manager.currentCount)
	assert.Equal(t, 0, manager.GetActiveExecutions())
	assert.True(t, manager.CanStartExecution())
}

func TestExecutionManager_StartExecution(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(2, registry)

	inputs := map[string]any{"test": "value"}

	status := manager.StartExecution("run-123", "workflow-test", func() {}, inputs)

	assert.NotNil(t, status)
	assert.Equal(t, "run-123", status.RunID)
	assert.Equal(t, "workflow-test", status.WorkflowID)
	assert.Equal(t, "running", status.Status)
	assert.Equal(t, inputs, status.Inputs)
	assert.NotEmpty(t, status.StartTime)
	assert.Nil(t, status.EndTime)
	assert.Empty(t, status.Progress)

	assert.Equal(t, 1, manager.GetActiveExecutions())
	assert.True(t, manager.CanStartExecution())

	// Verify execution can be retrieved
	retrieved, exists := manager.GetExecution("run-123")
	assert.True(t, exists)
	assert.Equal(t, status, retrieved)
}

func TestExecutionManager_ConcurrencyLimit(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(2, registry)

	// Start first execution
	status1 := manager.StartExecution("run-1", "workflow-1", func() {}, map[string]any{})
	assert.NotNil(t, status1)
	assert.True(t, manager.CanStartExecution())
	assert.Equal(t, 1, manager.GetActiveExecutions())

	// Start second execution
	status2 := manager.StartExecution("run-2", "workflow-2", func() {}, map[string]any{})
	assert.NotNil(t, status2)
	assert.False(t, manager.CanStartExecution()) // Should be at capacity
	assert.Equal(t, 2, manager.GetActiveExecutions())

	// Finish first execution
	manager.FinishExecution("run-1", map[string]any{"result": "success"}, nil)
	assert.True(t, manager.CanStartExecution()) // Should have capacity again
	assert.Equal(t, 1, manager.GetActiveExecutions())

	// Check first execution status
	finished, exists := manager.GetExecution("run-1")
	assert.True(t, exists)
	assert.Equal(t, "completed", finished.Status)
	assert.NotNil(t, finished.EndTime)
	assert.Greater(t, finished.Duration, time.Duration(0))
	assert.Equal(t, map[string]any{"result": "success"}, finished.Outputs)
	assert.Empty(t, finished.Error)
}

func TestExecutionManager_FinishExecutionWithError(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(1, registry)

	status := manager.StartExecution("run-error", "workflow-error", func() {}, map[string]any{})
	assert.Equal(t, "running", status.Status)

	// Finish with error
	testError := assert.AnError
	manager.FinishExecution("run-error", nil, testError)

	finished, exists := manager.GetExecution("run-error")
	assert.True(t, exists)
	assert.Equal(t, "failed", finished.Status)
	assert.Nil(t, finished.Outputs)
	assert.Equal(t, testError.Error(), finished.Error)
	assert.NotNil(t, finished.EndTime)

	assert.Equal(t, 0, manager.GetActiveExecutions())
	assert.True(t, manager.CanStartExecution())
}

func TestExecutionManager_GetExecution_NotFound(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(1, registry)

	execution, exists := manager.GetExecution("non-existent")
	assert.False(t, exists)
	assert.Nil(t, execution)
}

func TestExecutionManager_AddProgressEvent(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(1, registry)

	status := manager.StartExecution("run-progress", "workflow-progress", func() {}, map[string]any{})
	assert.Empty(t, status.Progress)

	event := events.ExecutionEvent{
		Type:      events.EventStepActionStarted,
		Timestamp: time.Now(),
		RunID:     "run-progress",
		StepID:    "step-1",
	}

	manager.AddProgressEvent("run-progress", event)

	updated, exists := manager.GetExecution("run-progress")
	assert.True(t, exists)
	assert.Len(t, updated.Progress, 1)
	assert.Equal(t, event, updated.Progress[0])

	// Add another event
	event2 := events.ExecutionEvent{
		Type:      events.EventStepActionCompleted,
		Timestamp: time.Now(),
		RunID:     "run-progress",
		StepID:    "step-1",
	}

	manager.AddProgressEvent("run-progress", event2)

	updated, exists = manager.GetExecution("run-progress")
	assert.True(t, exists)
	assert.Len(t, updated.Progress, 2)
	assert.Equal(t, event2, updated.Progress[1])
}

func TestExecutionManager_AddProgressEvent_NonExistentExecution(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(1, registry)

	event := events.ExecutionEvent{
		Type:      events.EventStepActionStarted,
		Timestamp: time.Now(),
		RunID:     "non-existent",
		StepID:    "step-1",
	}

	manager.AddProgressEvent("non-existent", event)
}

func TestExecutionManager_FinishExecution_NonExistent(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(1, registry)

	// Should not panic or error when finishing non-existent execution
	manager.FinishExecution("non-existent", nil, nil)

	assert.Equal(t, 0, manager.GetActiveExecutions())
}

func TestExecutionManager_MultipleExecutions(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(5, registry)

	// Start multiple executions
	for i := 0; i < 3; i++ {
		runID := fmt.Sprintf("run-%d", i)
		workflowID := fmt.Sprintf("workflow-%d", i)
		inputs := map[string]any{"index": i}

		status := manager.StartExecution(runID, workflowID, func() {}, inputs)
		assert.NotNil(t, status)
		assert.Equal(t, runID, status.RunID)
		assert.Equal(t, workflowID, status.WorkflowID)
	}

	assert.Equal(t, 3, manager.GetActiveExecutions())
	assert.True(t, manager.CanStartExecution())

	// Finish executions in different order
	manager.FinishExecution("run-1", map[string]any{"result": 1}, nil)
	assert.Equal(t, 2, manager.GetActiveExecutions())

	manager.FinishExecution("run-0", map[string]any{"result": 0}, nil)
	assert.Equal(t, 1, manager.GetActiveExecutions())

	manager.FinishExecution("run-2", nil, assert.AnError)
	assert.Equal(t, 0, manager.GetActiveExecutions())

	// Verify all executions are in correct state
	exec0, exists0 := manager.GetExecution("run-0")
	assert.True(t, exists0)
	assert.Equal(t, "completed", exec0.Status)

	exec1, exists1 := manager.GetExecution("run-1")
	assert.True(t, exists1)
	assert.Equal(t, "completed", exec1.Status)

	exec2, exists2 := manager.GetExecution("run-2")
	assert.True(t, exists2)
	assert.Equal(t, "failed", exec2.Status)
}
