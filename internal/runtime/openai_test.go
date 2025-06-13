package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenAIProvider(t *testing.T) {
	t.Run("with valid config", func(t *testing.T) {
		config := &OpenAIConfig{
			APIKey: "test-api-key",
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, "openai", provider.GetName())
		assert.True(t, len(provider.SupportedModels()) > 0)
	})

	t.Run("with nil config and env var", func(t *testing.T) {
		// Set environment variable
		os.Setenv("OPENAI_API_KEY", "env-api-key")
		defer os.Unsetenv("OPENAI_API_KEY")

		provider, err := NewOpenAIProvider(nil)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, "env-api-key", provider.apiKey)
	})

	t.Run("without api key", func(t *testing.T) {
		// Clear environment variables
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENAI_KEY")
		os.Unsetenv("OPENAI_TOKEN")

		config := &OpenAIConfig{}
		provider, err := NewOpenAIProvider(config)
		assert.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "API key is required")
	})
}

func TestOpenAIProvider_ModelSupport(t *testing.T) {
	provider := &OpenAIProvider{
		models: getSupportedOpenAIModels(),
	}

	t.Run("supported models", func(t *testing.T) {
		supportedModels := []string{
			"gpt-4o",
			"gpt-4-turbo",
			"gpt-4",
			"gpt-3.5-turbo",
		}

		for _, model := range supportedModels {
			assert.True(t, provider.IsModelSupported(model), "model %s should be supported", model)
		}
	})

	t.Run("unsupported models", func(t *testing.T) {
		unsupportedModels := []string{
			"invalid-model",
			"claude-3-opus",
			"gemini-pro",
		}

		for _, model := range unsupportedModels {
			assert.False(t, provider.IsModelSupported(model), "model %s should not be supported", model)
		}
	})

	t.Run("model list not empty", func(t *testing.T) {
		models := provider.SupportedModels()
		assert.NotEmpty(t, models)
		
		// Check some expected models are present
		modelSet := make(map[string]bool)
		for _, model := range models {
			modelSet[model] = true
		}
		
		assert.True(t, modelSet["gpt-4o"], "should include gpt-4o")
		assert.True(t, modelSet["gpt-4"], "should include gpt-4")
		assert.True(t, modelSet["gpt-3.5-turbo"], "should include gpt-3.5-turbo")
	})
}

func TestOpenAIProvider_BuildRequest(t *testing.T) {
	provider := &OpenAIProvider{}

	t.Run("basic request", func(t *testing.T) {
		modelReq := &ModelRequest{
			Model:  "gpt-4",
			Prompt: "Hello, world!",
		}

		openaiReq, err := provider.buildOpenAIRequest(modelReq)
		require.NoError(t, err)
		assert.Equal(t, "gpt-4", openaiReq.Model)
		assert.Len(t, openaiReq.Messages, 1)
		assert.Equal(t, "user", openaiReq.Messages[0].Role)
		assert.Equal(t, "Hello, world!", openaiReq.Messages[0].Content)
	})

	t.Run("request with system prompt", func(t *testing.T) {
		modelReq := &ModelRequest{
			Model:        "gpt-4",
			Prompt:       "Hello, world!",
			SystemPrompt: "You are a helpful assistant.",
		}

		openaiReq, err := provider.buildOpenAIRequest(modelReq)
		require.NoError(t, err)
		assert.Len(t, openaiReq.Messages, 2)
		assert.Equal(t, "system", openaiReq.Messages[0].Role)
		assert.Equal(t, "You are a helpful assistant.", openaiReq.Messages[0].Content)
		assert.Equal(t, "user", openaiReq.Messages[1].Role)
		assert.Equal(t, "Hello, world!", openaiReq.Messages[1].Content)
	})

	t.Run("request with parameters", func(t *testing.T) {
		temperature := 0.7
		maxTokens := 100
		topP := 0.9

		modelReq := &ModelRequest{
			Model:       "gpt-4",
			Prompt:      "Hello, world!",
			Temperature: &temperature,
			MaxTokens:   &maxTokens,
			TopP:        &topP,
			Stop:        []string{"STOP"},
			RequestID:   "test-request-123",
		}

		openaiReq, err := provider.buildOpenAIRequest(modelReq)
		require.NoError(t, err)
		assert.Equal(t, &temperature, openaiReq.Temperature)
		assert.Equal(t, &maxTokens, openaiReq.MaxTokens)
		assert.Equal(t, &topP, openaiReq.TopP)
		assert.Equal(t, []string{"STOP"}, openaiReq.Stop)
		assert.Equal(t, "test-request-123", openaiReq.User)
	})
}

func TestOpenAIProvider_Generate(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Bearer "))

		// Mock response
		response := OpenAIResponse{
			ID:      "chatcmpl-test",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: "Hello! How can I help you today?",
					},
					FinishReason: "stop",
				},
			},
			Usage: OpenAIUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &OpenAIConfig{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	}

	provider, err := NewOpenAIProvider(config)
	require.NoError(t, err)

	request := &ModelRequest{
		Model:  "gpt-4",
		Prompt: "Hello, how are you?",
	}

	response, usage, err := provider.Generate(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "Hello! How can I help you today?", response)
	assert.NotNil(t, usage)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 20, usage.CompletionTokens)
	assert.Equal(t, 30, usage.TotalTokens)
	assert.Greater(t, usage.EstimatedCost, 0.0)
}

func TestOpenAIProvider_Generate_Error(t *testing.T) {
	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			errorResponse := OpenAIError{
				ErrorInfo: struct {
					Message string      `json:"message"`
					Type    string      `json:"type"`
					Param   interface{} `json:"param"`
					Code    interface{} `json:"code"`
				}{
					Message: "Invalid request",
					Type:    "invalid_request_error",
					Param:   nil,
					Code:    nil,
				},
			}
			json.NewEncoder(w).Encode(errorResponse)
		}))
		defer server.Close()

		config := &OpenAIConfig{
			APIKey:     "test-api-key",
			BaseURL:    server.URL,
			MaxRetries: 0, // No retries for this test
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		request := &ModelRequest{
			Model:  "gpt-4",
			Prompt: "Hello",
		}

		_, _, err = provider.Generate(context.Background(), request)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid request")
	})

	t.Run("network error with retries", func(t *testing.T) {
		config := &OpenAIConfig{
			APIKey:     "test-api-key",
			BaseURL:    "http://nonexistent-server.local",
			MaxRetries: 2,
			RetryDelay: 10 * time.Millisecond,
			Timeout:    100 * time.Millisecond,
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		request := &ModelRequest{
			Model:  "gpt-4",
			Prompt: "Hello",
		}

		start := time.Now()
		_, _, err = provider.Generate(context.Background(), request)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed after 2 retries")
		// Should have taken at least some time due to retries
		assert.Greater(t, duration, 20*time.Millisecond)
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &OpenAIConfig{
			APIKey:  "test-api-key",
			BaseURL: server.URL,
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		request := &ModelRequest{
			Model:  "gpt-4",
			Prompt: "Hello",
		}

		_, _, err = provider.Generate(ctx, request)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestOpenAIProvider_CalculateCost(t *testing.T) {
	provider := &OpenAIProvider{}

	tests := []struct {
		name       string
		model      string
		usage      OpenAIUsage
		expectCost bool
	}{
		{
			name:  "gpt-4",
			model: "gpt-4",
			usage: OpenAIUsage{
				PromptTokens:     1000,
				CompletionTokens: 500,
			},
			expectCost: true,
		},
		{
			name:  "gpt-3.5-turbo",
			model: "gpt-3.5-turbo",
			usage: OpenAIUsage{
				PromptTokens:     1000,
				CompletionTokens: 500,
			},
			expectCost: true,
		},
		{
			name:  "unknown model defaults to gpt-4 pricing",
			model: "unknown-model",
			usage: OpenAIUsage{
				PromptTokens:     1000,
				CompletionTokens: 500,
			},
			expectCost: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := provider.calculateCost(tt.model, tt.usage)
			if tt.expectCost {
				assert.Greater(t, cost, 0.0, "cost should be greater than 0 for model %s", tt.model)
			} else {
				assert.Equal(t, 0.0, cost, "cost should be 0 for model %s", tt.model)
			}
		})
	}
}

func TestOpenAIProvider_ExtractResponse(t *testing.T) {
	provider := &OpenAIProvider{}

	t.Run("extract content from valid response", func(t *testing.T) {
		response := &OpenAIResponse{
			Choices: []OpenAIChoice{
				{
					Message: OpenAIMessage{
						Content: "  Hello, world!  ",
					},
				},
			},
		}

		content := provider.extractResponseContent(response)
		assert.Equal(t, "Hello, world!", content)
	})

	t.Run("empty response", func(t *testing.T) {
		response := &OpenAIResponse{
			Choices: []OpenAIChoice{},
		}

		content := provider.extractResponseContent(response)
		assert.Equal(t, "", content)
	})
}

func TestGetOpenAIAPIKeyFromEnv(t *testing.T) {
	// Clear all environment variables first
	envVars := []string{"OPENAI_API_KEY", "OPENAI_KEY", "OPENAI_TOKEN"}
	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}

	t.Run("no environment variable set", func(t *testing.T) {
		apiKey := GetOpenAIAPIKeyFromEnv()
		assert.Equal(t, "", apiKey)
	})

	t.Run("OPENAI_API_KEY set", func(t *testing.T) {
		os.Setenv("OPENAI_API_KEY", "test-key-1")
		defer os.Unsetenv("OPENAI_API_KEY")

		apiKey := GetOpenAIAPIKeyFromEnv()
		assert.Equal(t, "test-key-1", apiKey)
	})

	t.Run("OPENAI_KEY set", func(t *testing.T) {
		os.Setenv("OPENAI_KEY", "test-key-2")
		defer os.Unsetenv("OPENAI_KEY")

		apiKey := GetOpenAIAPIKeyFromEnv()
		assert.Equal(t, "test-key-2", apiKey)
	})

	t.Run("OPENAI_TOKEN set", func(t *testing.T) {
		os.Setenv("OPENAI_TOKEN", "test-key-3")
		defer os.Unsetenv("OPENAI_TOKEN")

		apiKey := GetOpenAIAPIKeyFromEnv()
		assert.Equal(t, "test-key-3", apiKey)
	})

	t.Run("multiple environment variables set - first one wins", func(t *testing.T) {
		os.Setenv("OPENAI_API_KEY", "first-key")
		os.Setenv("OPENAI_KEY", "second-key")
		defer func() {
			os.Unsetenv("OPENAI_API_KEY")
			os.Unsetenv("OPENAI_KEY")
		}()

		apiKey := GetOpenAIAPIKeyFromEnv()
		assert.Equal(t, "first-key", apiKey)
	})
}

func TestOpenAIProvider_Close(t *testing.T) {
	provider := &OpenAIProvider{}
	err := provider.Close()
	assert.NoError(t, err)
}

func TestGetSupportedOpenAIModels(t *testing.T) {
	models := getSupportedOpenAIModels()
	
	assert.NotEmpty(t, models)
	assert.Contains(t, models, "gpt-4o")
	assert.Contains(t, models, "gpt-4")
	assert.Contains(t, models, "gpt-3.5-turbo")
	
	// Ensure no duplicates
	modelSet := make(map[string]bool)
	for _, model := range models {
		assert.False(t, modelSet[model], "duplicate model found: %s", model)
		modelSet[model] = true
	}
}

func TestGetDefaultOpenAIConfig(t *testing.T) {
	config := getDefaultOpenAIConfig()
	
	assert.NotNil(t, config)
	assert.Equal(t, "https://api.openai.com/v1", config.BaseURL)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
	assert.Equal(t, "lacquer/1.0", config.UserAgent)
}