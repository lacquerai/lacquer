package runtime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelRegistry_RegisterProvider(t *testing.T) {
	registry := NewModelRegistry()

	// Create mock provider
	provider := NewMockModelProvider("test", []string{"model1", "model2"})

	// Register provider
	registry.RegisterProvider(provider)

	// Check provider is registered
	providers := registry.ListProviders()
	assert.Contains(t, providers, "test")

	// Check models are registered
	models := registry.ListModels()
	assert.Contains(t, models, "model1")
	assert.Contains(t, models, "model2")
}

func TestModelRegistry_RegisterProvider_Duplicate(t *testing.T) {
	registry := NewModelRegistry()

	provider1 := NewMockModelProvider("test", []string{"model1"})
	provider2 := NewMockModelProvider("test", []string{"model2"})

	// Register first provider
	registry.RegisterProvider(provider1)

	// Try to register duplicate provider name
	registry.RegisterProvider(provider2)
}

func TestModelRegistry_GetProvider(t *testing.T) {
	registry := NewModelRegistry()
	provider := NewMockModelProvider("test", []string{"model1", "model2"})

	registry.RegisterProvider(provider)

	// Get provider by model
	retrieved, err := registry.GetProvider("model1")
	assert.NoError(t, err)
	assert.Equal(t, provider, retrieved)

	// Get provider for non-existent model
	_, err = registry.GetProvider("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no provider found")
}

func TestModelRegistry_GetProviderByName(t *testing.T) {
	registry := NewModelRegistry()
	provider := NewMockModelProvider("test", []string{"model1"})

	registry.RegisterProvider(provider)

	// Get provider by name
	retrieved, err := registry.GetProviderByName("test")
	assert.NoError(t, err)
	assert.Equal(t, provider, retrieved)

	// Get non-existent provider
	_, err = registry.GetProviderByName("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestModelRegistry_IsModelSupported(t *testing.T) {
	registry := NewModelRegistry()
	provider := NewMockModelProvider("test", []string{"model1", "model2"})

	registry.RegisterProvider(provider)

	assert.True(t, registry.IsModelSupported("test", "model1"))
	assert.True(t, registry.IsModelSupported("test", "model2"))
	assert.False(t, registry.IsModelSupported("test", "nonexistent"))
}

func TestModelRegistry_Close(t *testing.T) {
	registry := NewModelRegistry()
	provider := NewMockModelProvider("test", []string{"model1"})

	registry.RegisterProvider(provider)

	// Close should not error
	err := registry.Close()
	assert.NoError(t, err)
}

func TestMockModelProvider_Generate(t *testing.T) {
	provider := NewMockModelProvider("test", []string{"model1"})

	ctx := context.Background()
	request := &ModelRequest{
		Model:  "model1",
		Prompt: "Hello, world!",
	}

	// Test default response
	response, usage, err := provider.Generate(ctx, request)
	assert.NoError(t, err)
	assert.Contains(t, response, "Mock response")
	assert.Contains(t, response, "Hello, world!")
	assert.NotNil(t, usage)
	assert.Greater(t, usage.TotalTokens, 0)

	// Test custom response
	provider.SetResponse("Hello, world!", "Custom response")
	response, usage, err = provider.Generate(ctx, request)
	assert.NoError(t, err)
	assert.Equal(t, "Custom response", response)
	assert.NotNil(t, usage)
}

func TestModelRequest_Structure(t *testing.T) {
	request := &ModelRequest{
		Model:        "gpt-4",
		Prompt:       "Hello",
		SystemPrompt: "You are helpful",
		Temperature:  &[]float64{0.7}[0],
		MaxTokens:    &[]int{100}[0],
		TopP:         &[]float64{0.9}[0],
		Stop:         []string{"STOP"},
		RequestID:    "req-123",
		Metadata: map[string]interface{}{
			"user": "test",
		},
	}

	assert.Equal(t, "gpt-4", request.Model)
	assert.Equal(t, "Hello", request.Prompt)
	assert.Equal(t, "You are helpful", request.SystemPrompt)
	assert.Equal(t, 0.7, *request.Temperature)
	assert.Equal(t, 100, *request.MaxTokens)
	assert.Equal(t, 0.9, *request.TopP)
	assert.Equal(t, []string{"STOP"}, request.Stop)
	assert.Equal(t, "req-123", request.RequestID)
	assert.Equal(t, "test", request.Metadata["user"])
}

func TestTokenUsage_Structure(t *testing.T) {
	usage := &TokenUsage{
		PromptTokens:     50,
		CompletionTokens: 25,
		TotalTokens:      75,
		EstimatedCost:    0.001,
	}

	assert.Equal(t, 50, usage.PromptTokens)
	assert.Equal(t, 25, usage.CompletionTokens)
	assert.Equal(t, 75, usage.TotalTokens)
	assert.Equal(t, 0.001, usage.EstimatedCost)
}
