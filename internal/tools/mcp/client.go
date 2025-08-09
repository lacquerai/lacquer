package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

// MCPTransport defines the interface for MCP communication transports
type MCPTransport interface {
	Send(ctx context.Context, message []byte) error
	Receive(ctx context.Context) ([]byte, error)
	Close() error
}

// MCPClient handles MCP protocol communication
type MCPClient struct {
	transport MCPTransport
	requestID atomic.Int64
	pending   map[int64]chan *MCPResponse
	mu        sync.Mutex
	closed    bool
	closeChan chan struct{}
}

// MCPMessage represents an MCP protocol message
type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents an MCP protocol error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPResponse represents a response from the MCP server
type MCPResponse struct {
	Result json.RawMessage
	Error  *MCPError
}

// NewMCPClient creates a new MCP client
func NewMCPClient(transport MCPTransport) *MCPClient {
	c := &MCPClient{
		transport: transport,
		pending:   make(map[int64]chan *MCPResponse),
		closeChan: make(chan struct{}),
	}

	// Start the message receiver
	go c.receiveLoop()

	return c
}

// Initialize initializes the MCP connection
func (c *MCPClient) Initialize(ctx context.Context) error {
	params := map[string]interface{}{
		"protocolVersion": "0.1.0",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"clientInfo": map[string]interface{}{
			"name":    "lacquer",
			"version": "1.0.0",
		},
	}

	var result map[string]interface{}
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	if err := c.notify(ctx, "notifications/initialized", nil); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	return nil
}

// ListTools lists available tools from the MCP server
func (c *MCPClient) ListTools(ctx context.Context) ([]Tool, error) {
	var result struct {
		Tools []Tool `json:"tools"`
	}

	if err := c.call(ctx, "tools/list", map[string]interface{}{}, &result); err != nil {
		return nil, err
	}

	return result.Tools, nil
}

// CallTool calls a tool on the MCP server
func (c *MCPClient) CallTool(ctx context.Context, name string, arguments json.RawMessage) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"name": name,
	}

	if len(arguments) > 0 {
		var args interface{}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid tool arguments: %w", err)
		}
		params["arguments"] = args
	}

	var result map[string]interface{}
	if err := c.call(ctx, "tools/call", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Close closes the MCP client
func (c *MCPClient) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	close(c.closeChan)
	c.mu.Unlock()

	c.mu.Lock()
	for _, ch := range c.pending {
		close(ch)
	}
	c.pending = nil
	c.mu.Unlock()

	return c.transport.Close()
}

// call makes an RPC call and waits for the response
func (c *MCPClient) call(ctx context.Context, method string, params interface{}, result interface{}) error {
	id := c.requestID.Add(1)

	respChan := make(chan *MCPResponse, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("client is closed")
	}
	c.pending[id] = respChan
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	msg := MCPMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  paramsJSON,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := c.transport.Send(ctx, msgBytes); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	select {
	case resp := <-respChan:
		if resp == nil {
			return fmt.Errorf("connection closed")
		}
		if resp.Error != nil {
			return fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		if result != nil && len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("failed to unmarshal result: %w", err)
			}
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closeChan:
		return fmt.Errorf("client closed")
	}
}

// notify sends a notification (no response expected)
func (c *MCPClient) notify(ctx context.Context, method string, params interface{}) error {
	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	msg := MCPMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return c.transport.Send(ctx, msgBytes)
}

// receiveLoop continuously receives messages from the transport
func (c *MCPClient) receiveLoop() {
	for {
		select {
		case <-c.closeChan:
			return
		default:
		}

		msgBytes, err := c.transport.Receive(context.Background())
		if err != nil {
			c.Close()
			return
		}

		var msg MCPMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			continue
		}

		if msg.ID != nil {
			c.mu.Lock()
			ch, ok := c.pending[*msg.ID]
			c.mu.Unlock()

			if ok {
				resp := &MCPResponse{
					Result: msg.Result,
					Error:  msg.Error,
				}
				select {
				case ch <- resp:
				default:
				}
			}
		}

		// @TODO: Handle server-initiated requests/notifications
	}
}
