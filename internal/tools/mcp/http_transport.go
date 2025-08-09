package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// HTTPTransport implements MCP transport over HTTP
type HTTPTransport struct {
	url        string
	client     *http.Client
	authHeader string
	mu         sync.Mutex
	closed     bool
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(url string, timeout time.Duration) *HTTPTransport {
	return &HTTPTransport{
		url: url,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// SetAuthHeader sets the authorization header for requests
func (t *HTTPTransport) SetAuthHeader(header string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.authHeader = header
}

// Send sends a message over HTTP
func (t *HTTPTransport) Send(ctx context.Context, message []byte) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return fmt.Errorf("transport is closed")
	}
	authHeader := t.authHeader
	t.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(message))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Receive receives a message from HTTP response
func (t *HTTPTransport) Receive(ctx context.Context) ([]byte, error) {
	// HTTP transport doesn't support receiving in the traditional sense
	// Each request gets its own response
	// This method should not be called for HTTP transport
	return nil, fmt.Errorf("HTTP transport does not support streaming receive")
}

// Close closes the HTTP transport
func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	t.client.CloseIdleConnections()
	return nil
}

// HTTPRequestResponseTransport implements request-response pattern for HTTP
type HTTPRequestResponseTransport struct {
	url        string
	client     *http.Client
	authHeader string
	mu         sync.Mutex
	closed     bool
}

// NewHTTPRequestResponseTransport creates a new HTTP request-response transport
func NewHTTPRequestResponseTransport(url string, timeout time.Duration) *HTTPRequestResponseTransport {
	return &HTTPRequestResponseTransport{
		url: url,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// SetAuthHeader sets the authorization header for requests
func (t *HTTPRequestResponseTransport) SetAuthHeader(header string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.authHeader = header
}

// Close closes the HTTP transport
func (t *HTTPRequestResponseTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	t.client.CloseIdleConnections()
	return nil
}

// SendAndReceive sends a request and receives the response
func (t *HTTPRequestResponseTransport) SendAndReceive(ctx context.Context, message []byte) ([]byte, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, fmt.Errorf("transport is closed")
	}
	authHeader := t.authHeader
	t.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(message))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// HTTPMCPTransport adapts HTTPRequestResponseTransport to MCPTransport interface
type HTTPMCPTransport struct {
	transport *HTTPRequestResponseTransport
	responses chan []byte
	mu        sync.Mutex
	closed    bool
}

// NewHTTPMCPTransport creates a new HTTP MCP transport
func NewHTTPMCPTransport(url string, timeout time.Duration) *HTTPMCPTransport {
	return &HTTPMCPTransport{
		transport: NewHTTPRequestResponseTransport(url, timeout),
		responses: make(chan []byte, 100),
	}
}

// SetAuthHeader sets the authorization header for requests
func (t *HTTPMCPTransport) SetAuthHeader(header string) {
	t.transport.SetAuthHeader(header)
}

// Send sends a message and queues the response
func (t *HTTPMCPTransport) Send(ctx context.Context, message []byte) error {
	var msg MCPMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return fmt.Errorf("failed to parse message: %w", err)
	}

	if msg.ID == nil {
		_, err := t.transport.SendAndReceive(ctx, message)
		return err
	}

	response, err := t.transport.SendAndReceive(ctx, message)
	if err != nil {
		return err
	}

	select {
	case t.responses <- response:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("response queue full")
	}
}

// Receive receives a queued response
func (t *HTTPMCPTransport) Receive(ctx context.Context) ([]byte, error) {
	select {
	case response := <-t.responses:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close closes the transport
func (t *HTTPMCPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	close(t.responses)
	return t.transport.Close()
}
