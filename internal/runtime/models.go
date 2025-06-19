package runtime

import (
	"context"
	"fmt"
	"sync"
)

// ModelProvider defines the interface for AI model providers
type ModelProvider interface {
	// Generate generates a response from the model
	Generate(ctx context.Context, request *ModelRequest, progressChan chan<- ExecutionEvent) (string, *TokenUsage, error)

	// GetName returns the provider name
	GetName() string

	// ListModels dynamically queries available models from the provider API
	ListModels(ctx context.Context) ([]ModelInfo, error)

	// Close cleans up resources
	Close() error
}

// ModelInfo represents information about an available model
type ModelInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	Provider    string   `json:"provider"`
	CreatedAt   string   `json:"created_at,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty"`
	Description string   `json:"description,omitempty"`
	Features    []string `json:"features,omitempty"`
}

// ModelRequest represents a request to generate text from a model
type ModelRequest struct {
	Model        string   `json:"model"`
	Prompt       string   `json:"prompt"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
	Temperature  *float64 `json:"temperature,omitempty"`
	MaxTokens    *int     `json:"max_tokens,omitempty"`
	TopP         *float64 `json:"top_p,omitempty"`
	Stop         []string `json:"stop,omitempty"`

	// Additional metadata
	RequestID string                 `json:"request_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ModelRegistry manages available model providers
type ModelRegistry struct {
	modelCache *ModelCache
	providers  map[string]ModelProvider
	modelMap   map[string]map[string]bool
	mu         sync.RWMutex
}

// NewModelRegistry creates a new model registry
func NewModelRegistry(disableCache bool) *ModelRegistry {
	return &ModelRegistry{
		modelCache: NewModelCache(disableCache),
		providers:  make(map[string]ModelProvider),
		modelMap:   make(map[string]map[string]bool),
	}
}

// RegisterProvider registers a model provider
func (mr *ModelRegistry) RegisterProvider(provider ModelProvider) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	name := provider.GetName()
	if _, exists := mr.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	mr.providers[name] = provider

	// Register supported models
	models, err := mr.modelCache.GetModels(context.Background(), provider)
	if err != nil {
		return fmt.Errorf("failed to get models: %w", err)
	}

	for _, model := range models {
		if _, exists := mr.modelMap[name]; !exists {
			mr.modelMap[name] = make(map[string]bool)
		}
		mr.modelMap[name][model.ID] = true
	}

	return nil
}

// GetProviderForModel returns the provider for a specific model from a specific provider
func (mr *ModelRegistry) GetProviderForModel(providerName, model string) (ModelProvider, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	provider, exists := mr.providers[providerName]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", providerName)
	}

	if !mr.IsModelSupported(providerName, model) {
		return nil, fmt.Errorf("model %s not supported by provider %s", model, providerName)
	}

	return provider, nil
}

// GetProviderByName returns a provider by name
func (mr *ModelRegistry) GetProviderByName(name string) (ModelProvider, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	provider, exists := mr.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

// ListProviders returns all registered provider names
func (mr *ModelRegistry) ListProviders() []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	names := make([]string, 0, len(mr.providers))
	for name := range mr.providers {
		names = append(names, name)
	}
	return names
}

// IsModelSupported checks if a model is supported
func (mr *ModelRegistry) IsModelSupported(providerName, model string) bool {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if _, exists := mr.modelMap[providerName]; !exists {
		return false
	}

	return mr.modelMap[providerName][model]
}

// Close closes all providers
func (mr *ModelRegistry) Close() error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	var lastErr error
	for _, provider := range mr.providers {
		if err := provider.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// MockModelProvider is a mock implementation for testing
type MockModelProvider struct {
	name            string
	supportedModels []ModelInfo
	responses       map[string]string
}

// NewMockModelProvider creates a new mock model provider
func NewMockModelProvider(name string, models []ModelInfo) *MockModelProvider {
	return &MockModelProvider{
		name:            name,
		supportedModels: models,
		responses:       make(map[string]string),
	}
}

// ListModels returns the supported models
func (mp *MockModelProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return mp.supportedModels, nil
}

// SetResponse sets a mock response for a specific prompt
func (mp *MockModelProvider) SetResponse(prompt, response string) {
	mp.responses[prompt] = response
}

// Generate generates a mock response
func (mp *MockModelProvider) Generate(ctx context.Context, request *ModelRequest, progressChan chan<- ExecutionEvent) (string, *TokenUsage, error) {
	// Check for specific response
	if response, exists := mp.responses[request.Prompt]; exists {
		return response, &TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
			EstimatedCost:    0.001,
		}, nil
	}

	// Default mock response
	response := fmt.Sprintf("Mock response for prompt: %s", request.Prompt)
	return response, &TokenUsage{
		PromptTokens:     len(request.Prompt) / 4, // Rough estimate
		CompletionTokens: len(response) / 4,
		TotalTokens:      (len(request.Prompt) + len(response)) / 4,
		EstimatedCost:    0.001,
	}, nil
}

// GetName returns the provider name
func (mp *MockModelProvider) GetName() string {
	return mp.name
}

// IsModelSupported checks if a model is supported
func (mp *MockModelProvider) IsModelSupported(model string) bool {
	for _, supported := range mp.supportedModels {
		if supported.ID == model {
			return true
		}
	}
	return false
}

// Close cleans up resources
func (mp *MockModelProvider) Close() error {
	return nil
}
