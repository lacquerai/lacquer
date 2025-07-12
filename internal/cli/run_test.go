package cli

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var (
	// use the capture-response flag to capture the response from the model
	// and save it to the model_response.json file. This can be used to update the
	// mocked responses for the aws client calls.
	captureResponse = flag.Bool("capture-response", false, "capture the response from the model")

	re     = regexp.MustCompile(ansi)
	timeRe = regexp.MustCompile(`\(\d+\.?\d*[a-zA-Z]+\)`) // matches patterns like (6.81s), (123ms), etc.
)

type TestServer struct {
	provider          string
	captureResponse   bool
	server            *httptest.Server
	responses         map[string][]json.RawMessage // path -> list of responses
	callIndex         map[string]int               // path -> current call index
	mutex             sync.RWMutex                 // for concurrent safety
	capturedResponses map[string][]json.RawMessage // for capture mode
	capturedRequests  map[string][]json.RawMessage // for capture mode
	proxyURL          string                       // target URL for reverse proxy
	responseDir       string                       // directory to load responses from
}

// NewTestServer creates a new test server
func NewTestServer(provider string, responseDir string, captureResponse bool, proxyURL ...string) *TestServer {
	ts := &TestServer{
		provider:          provider,
		captureResponse:   captureResponse,
		responses:         make(map[string][]json.RawMessage),
		callIndex:         make(map[string]int),
		capturedResponses: make(map[string][]json.RawMessage),
		capturedRequests:  make(map[string][]json.RawMessage),
		responseDir:       responseDir,
	}

	if len(proxyURL) > 0 {
		ts.proxyURL = proxyURL[0]
	}

	// Load responses from directory if not in capture mode
	if !ts.captureResponse {
		ts.loadResponses()
	}

	ts.server = httptest.NewServer(http.HandlerFunc(ts.handler))
	return ts
}

// loadResponses loads response files from the specified directory
func (ts *TestServer) loadResponses() {
	if ts.responseDir == "" {
		return
	}

	err := filepath.Walk(ts.responseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		name := info.Name()
		if !info.IsDir() && strings.HasSuffix(name, "_responses.json") && strings.HasPrefix(name, ts.provider) {
			// Get relative path from response directory
			relPath, err := filepath.Rel(ts.responseDir, path)
			if err != nil {
				return err
			}

			// Convert file path to URL path (remove .json extension)

			relPath = strings.TrimSuffix(relPath, "_responses.json")
			relPath = strings.TrimPrefix(relPath, ts.provider+"_")
			relPath = strings.Trim(relPath, "_")

			urlPath := "/" + relPath
			urlPath = strings.ReplaceAll(urlPath, "_", "/") // normalize path separators

			// Read and parse the JSON file
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			var responses []json.RawMessage
			if err := json.Unmarshal(data, &responses); err != nil {
				// If it's not an array, treat it as a single response
				responses = []json.RawMessage{data}
			}

			ts.responses[urlPath] = responses
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Warning: failed to load responses from %s: %v\n", ts.responseDir, err)
	}
}

// handler handles HTTP requests in both normal and capture modes
func (ts *TestServer) handler(w http.ResponseWriter, r *http.Request) {
	if ts.captureResponse {
		ts.handleCaptureMode(w, r)
	} else {
		ts.handleNormalMode(w, r)
	}
}

// handleNormalMode serves pre-loaded responses
func (ts *TestServer) handleNormalMode(w http.ResponseWriter, r *http.Request) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	path := r.URL.Path
	responses, exists := ts.responses[path]

	if !exists {
		http.NotFound(w, r)
		return
	}

	index := ts.callIndex[path]
	if index >= len(responses) {
		// If we've exhausted responses, use the last one
		index = len(responses) - 1
	}

	response := responses[index]
	ts.callIndex[path]++

	// Try to determine if response is JSON
	var jsonData interface{}
	if json.Unmarshal([]byte(response), &jsonData) == nil {
		w.Header().Set("Content-Type", "application/json")
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

// handleCaptureMode acts as a reverse proxy and captures responses
func (ts *TestServer) handleCaptureMode(w http.ResponseWriter, r *http.Request) {
	if ts.proxyURL == "" {
		http.Error(w, "Proxy URL not configured for capture mode", http.StatusInternalServerError)
		return
	}

	// Parse the target URL
	targetURL, err := url.Parse(ts.proxyURL)
	if err != nil {
		http.Error(w, "Invalid proxy URL", http.StatusInternalServerError)
		return
	}
	targetURL.Path = r.URL.Path
	targetURL.RawQuery = r.URL.RawQuery

	// Read request body
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	// Create a new request to the target URL
	targetReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), bytes.NewBuffer(requestBody))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Copy headers from original request
	for key, values := range r.Header {
		for _, value := range values {
			targetReq.Header.Add(key, value)
		}
	}

	// Send the request to the target URL
	client := &http.Client{Timeout: 30 * time.Second}
	targetResp, err := client.Do(targetReq)
	if err != nil {
		http.Error(w, "Failed to proxy request: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer targetResp.Body.Close()

	// Copy response headers
	for key, values := range targetResp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Read response body
	responseBody, err := io.ReadAll(targetResp.Body)
	if err != nil {
		http.Error(w, "Failed to read response body", http.StatusInternalServerError)
		return
	}

	// Store the captured response - decode gzip if needed for storage
	var capturedResponse []byte
	if targetResp.Header.Get("Content-Encoding") == "gzip" {
		// Decompress for storage
		gzipReader, err := gzip.NewReader(bytes.NewReader(responseBody))
		if err == nil {
			decompressed, err := io.ReadAll(gzipReader)
			if err == nil {
				capturedResponse = decompressed
			} else {
				capturedResponse = responseBody // fallback to raw if decompression fails
			}
			gzipReader.Close()
		} else {
			capturedResponse = responseBody // fallback to raw if decompression fails
		}
	} else {
		capturedResponse = responseBody
	}

	// Store captured data
	ts.mutex.Lock()
	if len(requestBody) == 0 {
		requestBody = []byte(`{}`)
	}

	ts.capturedRequests[r.URL.Path] = append(ts.capturedRequests[r.URL.Path], requestBody)
	ts.capturedResponses[r.URL.Path] = append(ts.capturedResponses[r.URL.Path], capturedResponse)
	ts.mutex.Unlock()

	// Write response status and body (keeping original compression)
	w.WriteHeader(targetResp.StatusCode)
	w.Write(responseBody)
}

// Reset clears all stored responses and call indices
func (ts *TestServer) Reset() {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	ts.callIndex = make(map[string]int)
	if ts.captureResponse {
		ts.capturedResponses = make(map[string][]json.RawMessage)
		ts.capturedRequests = make(map[string][]json.RawMessage)
	}
}

// Flush writes captured responses to JSON files in the working directory
func (ts *TestServer) Flush() error {
	if !ts.captureResponse {
		return nil // Nothing to flush in normal mode
	}

	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	for path, responses := range ts.capturedResponses {
		if len(responses) == 0 {
			continue
		}

		// Convert path to safe filename
		filename := strings.ReplaceAll(path, "/", "_")
		if filename == "" || filename == "_" {
			filename = "root"
		}

		filename = filepath.Join(ts.responseDir, ts.provider+"_"+filename) + "_responses.json"

		// Marshal responses to JSON
		data, err := json.MarshalIndent(responses, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal responses for path %s: %w", path, err)
		}

		// Write to file
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return fmt.Errorf("failed to write responses to %s: %w", filename, err)
		}

		fmt.Printf("Captured %d responses for path %s -> %s\n", len(responses), path, filename)
	}

	for path, requests := range ts.capturedRequests {
		if len(requests) == 0 {
			continue
		}

		// Convert path to safe filename
		filename := strings.ReplaceAll(path, "/", "_")
		if filename == "" || filename == "_" {
			filename = "root"
		}

		filename = filepath.Join(ts.responseDir, ts.provider+"_"+filename) + "_requests.json"

		// Marshal responses to JSON
		data, err := json.MarshalIndent(requests, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal responses for path %s: %w", path, err)
		}

		// Write to file
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return fmt.Errorf("failed to write responses to %s: %w", filename, err)
		}

		fmt.Printf("Captured %d requests for path %s -> %s\n", len(requests), path, filename)
	}

	return nil
}

// URL returns the test server's URL
func (ts *TestServer) URL() string {
	return ts.server.URL
}

// Close shuts down the test server
func (ts *TestServer) Close() {
	ts.server.Close()
}

func TestCollectExecutionResultsIntegration(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1"},
				{ID: "step2"},
			},
		},
	}

	ctx := context.Background()
	runCtx := execcontext.RunContext{
		Context: ctx,
		StdOut:  io.Discard,
		StdErr:  io.Discard,
	}
	execCtx := execcontext.NewExecutionContext(runCtx, workflow, nil, "")

	// Add step results with token usage
	execCtx.SetStepResult("step1", &execcontext.StepResult{
		StepID:    "step1",
		Status:    execcontext.StepStatusCompleted,
		StartTime: time.Now().Add(-2 * time.Second),
		EndTime:   time.Now().Add(-1 * time.Second),
		Duration:  time.Second,
		Output:    map[string]interface{}{"output": "Result 1"},
		Response:  "Result 1",
		TokenUsage: &execcontext.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 15,
			TotalTokens:      25,
		},
	})

	execCtx.SetStepResult("step2", &execcontext.StepResult{
		StepID:    "step2",
		Status:    execcontext.StepStatusCompleted,
		StartTime: time.Now().Add(-1 * time.Second),
		EndTime:   time.Now(),
		Duration:  time.Second,
		Output:    map[string]interface{}{"output": "Result 2"},
		Response:  "Result 2",
		TokenUsage: &execcontext.TokenUsage{
			PromptTokens:     20,
			CompletionTokens: 30,
			TotalTokens:      50,
		},
	})

	// Create execution result
	result := ExecutionResult{
		RunID:      execCtx.RunID,
		StepsTotal: 2,
	}

	// Collect execution results
	collectExecutionResults(execCtx, &result)

	// Verify results
	require.NotNil(t, result.TokenUsage)
	assert.Equal(t, 75, result.TokenUsage.TotalTokens)
	assert.Equal(t, 30, result.TokenUsage.PromptTokens)
	assert.Equal(t, 45, result.TokenUsage.CompletionTokens)

	assert.Len(t, result.StepResults, 2)
	assert.Equal(t, "step1", result.StepResults[0].StepID)
	assert.Equal(t, "step2", result.StepResults[1].StepID)
}

func TestRunE2EWorkflow(t *testing.T) {
	_ = godotenv.Load(".env.test")
	t.Setenv("LACQUER_TEST", "true")
	// *captureResponse = true

	if os.Getenv("LACQUER_ANTHROPIC_TEST_API_KEY") == "" {
		t.Skip("Skipping e2e tests as no LACQUER_ANTHROPIC_TEST_API_KEY is set")
	}

	if os.Getenv("LACQUER_OPENAI_TEST_API_KEY") == "" {
		t.Skip("Skipping e2e tests as no LACQUER_OPENAI_TEST_API_KEY is set")
	}

	t.Setenv("ANTHROPIC_API_KEY", os.Getenv("LACQUER_ANTHROPIC_TEST_API_KEY"))
	t.Setenv("OPENAI_API_KEY", os.Getenv("LACQUER_OPENAI_TEST_API_KEY"))

	directory := "testdata/run"
	dir, err := os.ReadDir(directory)
	require.NoError(t, err)
	for _, d := range dir {
		if !d.IsDir() {
			continue
		}

		t.Run(d.Name(), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					t.Fatalf("panic in run execution: %s\n%s", r, stack)
				}
			}()
			ats := NewTestServer("anthropic", filepath.Join(directory, d.Name()), *captureResponse, "https://api.anthropic.com")
			t.Setenv("LACQUER_ANTHROPIC_BASE_URL", ats.URL())

			ots := NewTestServer("openai", filepath.Join(directory, d.Name()), *captureResponse, "https://api.openai.com/v1")
			t.Setenv("LACQUER_OPENAI_BASE_URL", ots.URL()+"/v1")
			defer func() {
				err := ats.Flush()
				require.NoError(t, err)
				ats.Close()

				err = ots.Flush()
				require.NoError(t, err)
				ots.Close()
			}()

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			runCtx := execcontext.RunContext{
				Context: context.Background(),
				StdOut:  stdout,
				StdErr:  stderr,
			}

			var inputs map[string]string
			if _, err := os.Stat(filepath.Join(directory, d.Name(), "inputs.json")); err == nil {
				b, err := os.ReadFile(filepath.Join(directory, d.Name(), "inputs.json"))
				require.NoError(t, err)

				err = json.Unmarshal(b, &inputs)
				require.NoError(t, err)
			}

			err := runWorkflow(runCtx, filepath.Join(directory, d.Name(), "workflow.laq.yml"), inputs)
			require.NoError(t, err, fmt.Sprintf("STDOUT: %s\nSTDERR: %s", stdout.String(), stderr.String()))

			// load golden file
			goldenFile := filepath.Join(directory, d.Name(), "golden.txt")
			golden, err := os.ReadFile(goldenFile)

			// Remove ANSI codes and normalize time strings
			stdout_clean := re.ReplaceAllString(stdout.String(), "")
			stderr_clean := re.ReplaceAllString(stderr.String(), "")
			stdout_normalized := timeRe.ReplaceAllString(stdout_clean, "(TIME)")
			stderr_normalized := timeRe.ReplaceAllString(stderr_clean, "(TIME)")
			actual := stdout_normalized + "\nSTDERR:\n" + stderr_normalized

			if os.IsNotExist(err) {
				golden = []byte(actual)
				err = os.WriteFile(goldenFile, golden, 0644)
				require.NoError(t, err)
			} else {
				require.NoError(t, err)
			}

			if !assert.Equal(t, string(golden), actual) {
				os.WriteFile(filepath.Join(directory, d.Name(), "actual.txt"), []byte(actual), 0644)
			}
		})

	}
}
