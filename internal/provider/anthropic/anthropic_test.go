package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lacquerai/lacquer/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultAnthropicConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "https://api.anthropic.com", config.BaseURL)
	assert.Equal(t, 60*time.Second, config.Timeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, time.Second, config.RetryDelay)
	assert.Equal(t, "lacquer/1.0", config.UserAgent)
	assert.Equal(t, "2023-06-01", config.AnthropicVersion)
}

func TestNewAnthropicProvider(t *testing.T) {
	// Test with nil config
	_, err := NewProvider(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key is required")

	// Test with valid config
	config := &Config{
		APIKey:  "sk-ant-test-key-12345",
		BaseURL: "https://api.anthropic.com",
		Timeout: 30 * time.Second,
	}

	pr, err := NewProvider(config)
	assert.NoError(t, err)
	assert.NotNil(t, pr)
	assert.Equal(t, "anthropic", pr.name)
}

func TestAnthropicProvider_Integration(t *testing.T) {
	// Skip if no API key is available
	apiKey := GetAnthropicAPIKeyFromEnv()
	if apiKey == "" {
		t.Skip("No Anthropic API key found, skipping integration test")
	}

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &Config{
		APIKey:  apiKey,
		Timeout: 30 * time.Second,
	}

	pr, err := NewProvider(config)
	require.NoError(t, err)
	// defer pr.client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	request := &provider.Request{
		Model:     "claude-3-haiku-20240307", // Use fastest model for testing
		Messages:  []provider.Message{{Role: "user", Content: []provider.ContentBlockParamUnion{provider.NewTextBlock("Say 'Hello, Lacquer!' and nothing else.")}}},
		RequestID: "test-request",
	}

	response, usage, err := pr.Generate(provider.GenerateContext{Context: ctx}, request, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, response)
	assert.NotNil(t, usage)
	assert.Greater(t, usage.TotalTokens, 0)

	// Test that response contains expected content
	assert.Contains(t, strings.ToLower(response[0].Content[0].OfText.Text), "hello")
}

func TestAnthropicProvider_MockServer(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		assert.Equal(t, "lacquer/1.0", r.Header.Get("User-Agent"))
		assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))

		// Create mock response
		response := Response{
			ID:         "msg_test123",
			Type:       "message",
			Role:       "assistant",
			Model:      "claude-3-5-sonnet-20241022",
			StopReason: "end_turn",
			Content: []Content{
				{
					Type: "text",
					Text: "Hello, this is a test response from Claude!",
				},
			},
			Usage: Usage{
				InputTokens:  10,
				OutputTokens: 15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &Config{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Timeout: 10 * time.Second,
	}

	pr, err := NewProvider(config)
	require.NoError(t, err)
	// defer pr.client.Close()

	ctx := context.Background()
	request := &provider.Request{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []provider.Message{{Role: "user", Content: []provider.ContentBlockParamUnion{provider.NewTextBlock("Hello, world!")}}},
	}

	response, usage, err := pr.Generate(provider.GenerateContext{Context: ctx}, request, nil)
	require.NoError(t, err)
	assert.Equal(t, "Hello, this is a test response from Claude!", response[0].Content[0].OfText.Text)
	assert.Equal(t, 10, usage.InputTokens)
	assert.Equal(t, 15, usage.OutputTokens)
	assert.Equal(t, 25, usage.TotalTokens)
}

func TestAnthropicProvider_ErrorHandling(t *testing.T) {
	// Test server error (should trigger retry)
	serverErrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"type": "internal_server_error", "message": "Internal server error"}}`))
	}))
	defer serverErrorServer.Close()

	config := &Config{
		APIKey:     "test-api-key",
		BaseURL:    serverErrorServer.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1, // Limit retries for faster testing
		RetryDelay: 10 * time.Millisecond,
	}

	pr, err := NewProvider(config)
	require.NoError(t, err)
	// defer pr.client.Close()

	ctx := context.Background()
	request := &provider.Request{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []provider.Message{{Role: "user", Content: []provider.ContentBlockParamUnion{provider.NewTextBlock("Hello, world!")}}},
	}

	_, _, err = pr.Generate(provider.GenerateContext{Context: ctx}, request, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Internal server error")

	// Test client error (should not retry)
	clientErrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"type": "invalid_request_error", "message": "Invalid request"}}`))
	}))
	defer clientErrorServer.Close()

	config.BaseURL = clientErrorServer.URL
	pr, err = NewProvider(config)
	require.NoError(t, err)
	// defer pr.client.Close()

	_, _, err = pr.Generate(provider.GenerateContext{Context: ctx}, request, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid request")
}

func TestGetAnthropicAPIKeyFromEnv(t *testing.T) {
	// Save original getEnvVar function
	originalGetEnvVar := getEnvVar
	defer func() { getEnvVar = originalGetEnvVar }()

	// Test with ANTHROPIC_API_KEY
	getEnvVar = func(key string) string {
		if key == "ANTHROPIC_API_KEY" {
			return "sk-ant-test-key-123"
		}
		return ""
	}

	apiKey := GetAnthropicAPIKeyFromEnv()
	assert.Equal(t, "sk-ant-test-key-123", apiKey)

	// Test with CLAUDE_API_KEY
	getEnvVar = func(key string) string {
		if key == "CLAUDE_API_KEY" {
			return "sk-ant-test-key-456"
		}
		return ""
	}

	apiKey = GetAnthropicAPIKeyFromEnv()
	assert.Equal(t, "sk-ant-test-key-456", apiKey)

	// Test with no key
	getEnvVar = func(key string) string {
		return ""
	}

	apiKey = GetAnthropicAPIKeyFromEnv()
	assert.Empty(t, apiKey)
}

func TestValidateAnthropicAPIKey(t *testing.T) {
	// Test valid API key
	err := ValidateAnthropicAPIKey("sk-ant-api03-1234567890abcdef")
	assert.NoError(t, err)

	// Test empty API key
	err = ValidateAnthropicAPIKey("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")

	// Test invalid format
	err = ValidateAnthropicAPIKey("invalid-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "should start with 'sk-ant-'")

	// Test too short
	err = ValidateAnthropicAPIKey("sk-ant-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}
func BenchmarkAnthropicProvider_Generate(b *testing.B) {
	// Skip if no API key is available
	apiKey := GetAnthropicAPIKeyFromEnv()
	if apiKey == "" {
		b.Skip("No Anthropic API key found, skipping benchmark")
	}

	config := &Config{
		APIKey: apiKey,
	}

	pr, err := NewProvider(config)
	if err != nil {
		b.Skip("Failed to create Anthropic provider, skipping benchmark")
	}
	// defer pr.client.Close()

	request := &provider.Request{
		Model:    "claude-3-haiku-20240307", // Use fastest model
		Messages: []provider.Message{{Role: "user", Content: []provider.ContentBlockParamUnion{provider.NewTextBlock("What is 2+2?")}}},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, _, err := pr.Generate(provider.GenerateContext{Context: ctx}, request, nil)
		cancel()

		if err != nil {
			b.Fatalf("Generation failed: %v", err)
		}
	}
}
