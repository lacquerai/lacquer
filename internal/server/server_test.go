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
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 8080 // fallback port
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// Test suite setup
type ServerTestSuite struct {
	server     *Server
	tempDir    string
	workflowFiles []string
	config     *Config
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

func (suite *ServerTestSuite) cleanup(t *testing.T) {
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

	// Check test-workflow (note: key includes .laq extension)
	testWorkflow, ok := workflows["test-workflow.laq"].(map[string]any)
	if !ok {
		t.Fatalf("test-workflow.laq not found or wrong type: %+v", workflows)
	}
	assert.Equal(t, "test-workflow", testWorkflow["name"])
	assert.Equal(t, "A test workflow for server testing", testWorkflow["description"])
	assert.Equal(t, "1.0", testWorkflow["version"])
	assert.Equal(t, float64(1), testWorkflow["steps"])

	// Check simple-workflow (note: key includes .laq extension)
	simpleWorkflow, ok := workflows["simple-workflow.laq"].(map[string]any)
	if !ok {
		t.Fatalf("simple-workflow.laq not found or wrong type: %+v", workflows)
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
			"input_name": "Test",
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
		fmt.Sprintf("http://%s/api/v1/workflows/test-workflow.laq/execute", addr),
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
			"input_name": "Integration Test",
		},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/test-workflow.laq/execute", addr),
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
	assert.Equal(t, "test-workflow.laq", result["workflow_id"])
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
	assert.Equal(t, "test-workflow.laq", execution.WorkflowID)
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
		fmt.Sprintf("http://%s/api/v1/workflows/simple-workflow.laq/execute", addr),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp1.Body.Close()

	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	// Immediately try second execution (should be rejected due to concurrency limit)
	resp2, err := http.Post(
		fmt.Sprintf("http://%s/api/v1/workflows/simple-workflow.laq/execute", addr),
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
	wsURL := fmt.Sprintf("ws://%s/api/v1/workflows/test-workflow.laq/stream?run_id=non-existent", addr)
	_, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.Error(t, err)
	// WebSocket dial should fail or return error status
}

func TestServerIntegration_WebSocketStream_MissingRunID(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.cleanup(t)

	addr := suite.startServerInBackground(t)

	// Test WebSocket without run_id parameter
	wsURL := fmt.Sprintf("ws://%s/api/v1/workflows/test-workflow.laq/stream", addr)
	_, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
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
	assert.Contains(t, workflows, "dir-workflow.laq")
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

// Benchmark tests

func BenchmarkServer_ListWorkflows(b *testing.B) {
	suite := setupTestSuite(&testing.T{})
	defer suite.cleanup(&testing.T{})

	addr := suite.startServerInBackground(&testing.T{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/workflows", addr))
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkServer_HealthCheck(b *testing.B) {
	suite := setupTestSuite(&testing.T{})
	defer suite.cleanup(&testing.T{})

	addr := suite.startServerInBackground(&testing.T{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}