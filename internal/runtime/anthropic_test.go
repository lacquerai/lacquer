package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultAnthropicConfig(t *testing.T) {
	config := DefaultAnthropicConfig()

	assert.Equal(t, "https://api.anthropic.com", config.BaseURL)
	assert.Equal(t, 60*time.Second, config.Timeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, time.Second, config.RetryDelay)
	assert.Equal(t, "lacquer/1.0", config.UserAgent)
	assert.Equal(t, "2023-06-01", config.AnthropicVersion)
}

func TestNewAnthropicProvider(t *testing.T) {
	// Test with nil config
	_, err := NewAnthropicProvider(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key is required")

	// Test with valid config
	config := &AnthropicConfig{
		APIKey:  "sk-ant-test-key-12345",
		BaseURL: "https://api.anthropic.com",
		Timeout: 30 * time.Second,
	}

	provider, err := NewAnthropicProvider(config)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "anthropic", provider.GetName())
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

	config := &AnthropicConfig{
		APIKey:  apiKey,
		Timeout: 30 * time.Second,
	}

	provider, err := NewAnthropicProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	request := &ModelRequest{
		Model:     "claude-3-haiku-20240307", // Use fastest model for testing
		Messages:  []ModelMessage{{Role: "user", Content: []ContentBlockParamUnion{NewTextBlock("Say 'Hello, Lacquer!' and nothing else.")}}},
		RequestID: "test-request",
	}

	response, usage, err := provider.Generate(GenerateContext{Context: ctx}, request, nil)
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
		response := AnthropicResponse{
			ID:         "msg_test123",
			Type:       "message",
			Role:       "assistant",
			Model:      "claude-3-5-sonnet-20241022",
			StopReason: "end_turn",
			Content: []AnthropicContent{
				{
					Type: "text",
					Text: "Hello, this is a test response from Claude!",
				},
			},
			Usage: AnthropicUsage{
				InputTokens:  10,
				OutputTokens: 15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &AnthropicConfig{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Timeout: 10 * time.Second,
	}

	provider, err := NewAnthropicProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx := context.Background()
	request := &ModelRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []ModelMessage{{Role: "user", Content: []ContentBlockParamUnion{NewTextBlock("Hello, world!")}}},
	}

	response, usage, err := provider.Generate(GenerateContext{Context: ctx}, request, nil)
	require.NoError(t, err)
	assert.Equal(t, "Hello, this is a test response from Claude!", response)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 15, usage.CompletionTokens)
	assert.Equal(t, 25, usage.TotalTokens)
}

func TestAnthropicProvider_ErrorHandling(t *testing.T) {
	// Test server error (should trigger retry)
	serverErrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"type": "internal_server_error", "message": "Internal server error"}}`))
	}))
	defer serverErrorServer.Close()

	config := &AnthropicConfig{
		APIKey:     "test-api-key",
		BaseURL:    serverErrorServer.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1, // Limit retries for faster testing
		RetryDelay: 10 * time.Millisecond,
	}

	provider, err := NewAnthropicProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx := context.Background()
	request := &ModelRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []ModelMessage{{Role: "user", Content: []ContentBlockParamUnion{NewTextBlock("Hello, world!")}}},
	}

	_, _, err = provider.Generate(GenerateContext{Context: ctx}, request, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Internal server error")

	// Test client error (should not retry)
	clientErrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"type": "invalid_request_error", "message": "Invalid request"}}`))
	}))
	defer clientErrorServer.Close()

	config.BaseURL = clientErrorServer.URL
	provider, err = NewAnthropicProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	_, _, err = provider.Generate(GenerateContext{Context: ctx}, request, nil)
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

func TestAnthropicProvider_Close(t *testing.T) {
	config := &AnthropicConfig{
		APIKey: "sk-ant-test-key-12345",
	}

	provider, err := NewAnthropicProvider(config)
	require.NoError(t, err)

	err = provider.Close()
	assert.NoError(t, err)
}
func BenchmarkAnthropicProvider_Generate(b *testing.B) {
	// Skip if no API key is available
	apiKey := GetAnthropicAPIKeyFromEnv()
	if apiKey == "" {
		b.Skip("No Anthropic API key found, skipping benchmark")
	}

	config := &AnthropicConfig{
		APIKey: apiKey,
	}

	provider, err := NewAnthropicProvider(config)
	if err != nil {
		b.Skip("Failed to create Anthropic provider, skipping benchmark")
	}
	defer provider.Close()

	request := &ModelRequest{
		Model:    "claude-3-haiku-20240307", // Use fastest model
		Messages: []ModelMessage{{Role: "user", Content: []ContentBlockParamUnion{NewTextBlock("What is 2+2?")}}},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, _, err := provider.Generate(GenerateContext{Context: ctx}, request, nil)
		cancel()

		if err != nil {
			b.Fatalf("Generation failed: %v", err)
		}
	}
}
