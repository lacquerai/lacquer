package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// MCPClient represents a client for connecting to MCP servers
type MCPClient struct {
	serverURL  string
	httpClient *http.Client
	config     *MCPConfig
	connected  bool
	mu         sync.RWMutex
}

// MCPConfig contains configuration for MCP client
type MCPConfig struct {
	ServerURL      string            `yaml:"server_url"`
	Timeout        time.Duration     `yaml:"timeout"`
	MaxRetries     int               `yaml:"max_retries"`
	RetryDelay     time.Duration     `yaml:"retry_delay"`
	Headers        map[string]string `yaml:"headers"`
	ConnectTimeout time.Duration     `yaml:"connect_timeout"`
}

// MCPRequest represents a request to an MCP server
type MCPRequest struct {
	ID     string                 `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// MCPResponse represents a response from an MCP server
type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

// MCPError represents an error from an MCP server
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error implements the error interface
func (e *MCPError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// MCPTool represents a tool definition from an MCP server
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Schema      map[string]interface{} `json:"schema"`
}

// MCPToolResult represents the result of an MCP tool call
type MCPToolResult struct {
	Output   interface{}            `json:"output"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewMCPClient creates a new MCP client
func NewMCPClient(config *MCPConfig) (*MCPClient, error) {
	if config == nil {
		config = &MCPConfig{
			Timeout:        30 * time.Second,
			MaxRetries:     3,
			RetryDelay:     time.Second,
			ConnectTimeout: 10 * time.Second,
		}
	}

	// Validate server URL
	if config.ServerURL == "" {
		return nil, fmt.Errorf("server URL is required")
	}

	if _, err := url.Parse(config.ServerURL); err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	client := &http.Client{
		Timeout: config.ConnectTimeout,
	}

	return &MCPClient{
		serverURL:  config.ServerURL,
		httpClient: client,
		config:     config,
	}, nil
}

// Connect establishes a connection to the MCP server
func (c *MCPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// Try to ping the server
	if err := c.ping(ctx); err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	c.connected = true
	log.Info().Str("server_url", c.serverURL).Msg("Connected to MCP server")
	return nil
}

// ping sends a ping request to check server connectivity
func (c *MCPClient) ping(ctx context.Context) error {
	request := &MCPRequest{
		ID:     generateRequestID(),
		Method: "ping",
	}

	response, err := c.sendRequest(ctx, request)
	if err != nil {
		return err
	}

	if response.Error != nil {
		return response.Error
	}

	return nil
}

// ListTools returns the list of available tools from the MCP server
func (c *MCPClient) ListTools(ctx context.Context) ([]MCPTool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to MCP server")
	}

	request := &MCPRequest{
		ID:     generateRequestID(),
		Method: "tools/list",
	}

	response, err := c.sendRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	if response.Error != nil {
		return nil, response.Error
	}

	// Parse the tools from the response
	var result struct {
		Tools []MCPTool `json:"tools"`
	}

	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools: %w", err)
	}

	return result.Tools, nil
}

// CallTool calls a specific tool on the MCP server
func (c *MCPClient) CallTool(ctx context.Context, toolName string, parameters map[string]interface{}) (*MCPToolResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to MCP server")
	}

	request := &MCPRequest{
		ID:     generateRequestID(),
		Method: "tools/call",
		Params: map[string]interface{}{
			"name":       toolName,
			"parameters": parameters,
		},
	}

	response, err := c.sendRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool %s: %w", toolName, err)
	}

	if response.Error != nil {
		return &MCPToolResult{
			Error: response.Error.Message,
		}, nil
	}

	// Parse the tool result
	var result MCPToolResult
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool result: %w", err)
	}

	return &result, nil
}

// sendRequest sends a request to the MCP server with retry logic
func (c *MCPClient) sendRequest(ctx context.Context, request *MCPRequest) (*MCPResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retrying
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}
		}

		response, err := c.doRequest(ctx, request)
		if err == nil {
			return response, nil
		}

		lastErr = err
		log.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Int("max_retries", c.config.MaxRetries).
			Msg("MCP request failed, retrying")
	}

	return nil, fmt.Errorf("MCP request failed after %d attempts: %w", c.config.MaxRetries+1, lastErr)
}

// doRequest performs a single HTTP request to the MCP server
func (c *MCPClient) doRequest(ctx context.Context, request *MCPRequest) (*MCPResponse, error) {
	// Serialize request
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.serverURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range c.config.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set timeout for this specific request
	client := &http.Client{
		Timeout: c.config.Timeout,
	}

	// Send request
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, string(responseBody))
	}

	// Parse response
	var response MCPResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// IsConnected returns whether the client is connected to the server
func (c *MCPClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Disconnect closes the connection to the MCP server
func (c *MCPClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false
	log.Info().Str("server_url", c.serverURL).Msg("Disconnected from MCP server")
	return nil
}

// HealthCheck performs a health check on the MCP server
func (c *MCPClient) HealthCheck(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to MCP server")
	}

	return c.ping(ctx)
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// DefaultMCPConfig returns a default MCP configuration
func DefaultMCPConfig() *MCPConfig {
	return &MCPConfig{
		Timeout:        30 * time.Second,
		MaxRetries:     3,
		RetryDelay:     time.Second,
		ConnectTimeout: 10 * time.Second,
		Headers:        make(map[string]string),
	}
}
