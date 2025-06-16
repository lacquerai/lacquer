package runtime

import (
	"context"
	"encoding/json"
	"fmt"
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
func TestAnthropicProvider_BuildRequest(t *testing.T) {
	config := &AnthropicConfig{
		APIKey: "sk-ant-test-key-12345",
	}

	provider, err := NewAnthropicProvider(config)
	require.NoError(t, err)

	// Test basic request
	modelReq := &ModelRequest{
		Model:        "claude-3-5-sonnet-20241022",
		Prompt:       "Hello, world!",
		SystemPrompt: "You are helpful",
		Temperature:  &[]float64{0.7}[0],
		MaxTokens:    &[]int{1000}[0],
		RequestID:    "test-request",
	}

	anthropicReq, err := provider.buildAnthropicRequest(modelReq)
	assert.NoError(t, err)
	assert.Equal(t, "claude-3-5-sonnet-20241022", anthropicReq.Model)
	assert.Equal(t, 1000, anthropicReq.MaxTokens)
	assert.Equal(t, "You are helpful", anthropicReq.System)
	assert.Equal(t, 0.7, *anthropicReq.Temperature)
	assert.Len(t, anthropicReq.Messages, 1)
	assert.Equal(t, "user", anthropicReq.Messages[0].Role)
	assert.Equal(t, "Hello, world!", anthropicReq.Messages[0].Content[0].Text)
	assert.NotNil(t, anthropicReq.Metadata)
	assert.Equal(t, "test-request", anthropicReq.Metadata.UserID)
}

func TestAnthropicProvider_ExtractResponseContent(t *testing.T) {
	config := &AnthropicConfig{
		APIKey: "sk-ant-test-key-12345",
	}

	provider, err := NewAnthropicProvider(config)
	require.NoError(t, err)

	// Test single text content
	response := &AnthropicResponse{
		Content: []AnthropicContent{
			{
				Type: "text",
				Text: "Hello, world!",
			},
		},
	}

	content := provider.extractResponseContent(response)
	assert.Equal(t, "Hello, world!", content)

	// Test multiple text content blocks
	response = &AnthropicResponse{
		Content: []AnthropicContent{
			{
				Type: "text",
				Text: "First part",
			},
			{
				Type: "text",
				Text: "Second part",
			},
		},
	}

	content = provider.extractResponseContent(response)
	assert.Equal(t, "First part\nSecond part", content)

	// Test mixed content with non-text
	response = &AnthropicResponse{
		Content: []AnthropicContent{
			{
				Type: "text",
				Text: "Text content",
			},
			{
				Type: "tool_use",
				Name: "calculator",
			},
		},
	}

	content = provider.extractResponseContent(response)
	assert.Equal(t, "Text content", content)
}

func TestAnthropicProvider_CalculateCost(t *testing.T) {
	config := &AnthropicConfig{
		APIKey: "sk-ant-test-key-12345",
	}

	provider, err := NewAnthropicProvider(config)
	require.NoError(t, err)

	usage := AnthropicUsage{
		InputTokens:  1000,
		OutputTokens: 500,
	}

	// Test Claude-3.5 Sonnet pricing
	cost := provider.calculateCost("claude-3-5-sonnet-20241022", usage)
	expectedCost := (1000 * 0.003 / 1000) + (500 * 0.015 / 1000)
	assert.InDelta(t, expectedCost, cost, 0.000001)

	// Test Claude-3 Opus pricing (higher cost)
	cost = provider.calculateCost("claude-3-opus-20240229", usage)
	expectedCost = (1000 * 0.015 / 1000) + (500 * 0.075 / 1000)
	assert.InDelta(t, expectedCost, cost, 0.000001)

	// Test Claude-3 Haiku pricing (lower cost)
	cost = provider.calculateCost("claude-3-haiku-20240307", usage)
	expectedCost = (1000 * 0.00025 / 1000) + (500 * 0.00125 / 1000)
	assert.InDelta(t, expectedCost, cost, 0.000001)
}

func TestAnthropicProvider_ShouldRetry(t *testing.T) {
	config := &AnthropicConfig{
		APIKey: "sk-ant-test-key-12345",
	}

	provider, err := NewAnthropicProvider(config)
	require.NoError(t, err)

	// Should retry on network errors
	assert.True(t, provider.shouldRetry(fmt.Errorf("connection timeout")))
	assert.True(t, provider.shouldRetry(fmt.Errorf("temporary failure")))
	assert.True(t, provider.shouldRetry(fmt.Errorf("connection refused")))

	// Should retry on rate limit (429)
	assert.True(t, provider.shouldRetry(fmt.Errorf("Anthropic API error (429): rate limit")))

	// Should retry on server errors (5xx)
	assert.True(t, provider.shouldRetry(fmt.Errorf("Anthropic API error (500): internal server error")))
	assert.True(t, provider.shouldRetry(fmt.Errorf("Anthropic API error (503): service unavailable")))

	// Should not retry on client errors (4xx except 429)
	assert.False(t, provider.shouldRetry(fmt.Errorf("Anthropic API error (400): bad request")))
	assert.False(t, provider.shouldRetry(fmt.Errorf("Anthropic API error (401): unauthorized")))
	assert.False(t, provider.shouldRetry(fmt.Errorf("Anthropic API error (404): not found")))
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
		Prompt:    "Say 'Hello, Lacquer!' and nothing else.",
		RequestID: "test-request",
	}

	response, usage, err := provider.Generate(ctx, request)
	require.NoError(t, err)
	assert.NotEmpty(t, response)
	assert.NotNil(t, usage)
	assert.Greater(t, usage.TotalTokens, 0)
	assert.Greater(t, usage.EstimatedCost, 0.0)

	// Test that response contains expected content
	assert.Contains(t, strings.ToLower(response), "hello")
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
		Model:  "claude-3-5-sonnet-20241022",
		Prompt: "Hello, world!",
	}

	response, usage, err := provider.Generate(ctx, request)
	require.NoError(t, err)
	assert.Equal(t, "Hello, this is a test response from Claude!", response)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 15, usage.CompletionTokens)
	assert.Equal(t, 25, usage.TotalTokens)
	assert.Greater(t, usage.EstimatedCost, 0.0)
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
		Model:  "claude-3-5-sonnet-20241022",
		Prompt: "Hello, world!",
	}

	_, _, err = provider.Generate(ctx, request)
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

	_, _, err = provider.Generate(ctx, request)
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

func TestGetSupportedAnthropicModels(t *testing.T) {
	models := getSupportedAnthropicModels()

	assert.NotEmpty(t, models)
	assert.Contains(t, models, "claude-3-5-sonnet-20241022")
	assert.Contains(t, models, "claude-3-5-sonnet-20240620")
	assert.Contains(t, models, "claude-3-opus-20240229")
	assert.Contains(t, models, "claude-3-sonnet-20240229")
	assert.Contains(t, models, "claude-3-haiku-20240307")
}

// Benchmark test for Anthropic provider
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
		Model:  "claude-3-haiku-20240307", // Use fastest model
		Prompt: "What is 2+2?",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, _, err := provider.Generate(ctx, request)
		cancel()

		if err != nil {
			b.Fatalf("Generation failed: %v", err)
		}
	}
}
