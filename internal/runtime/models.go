package runtime

import (
	"context"
	"fmt"
	"sync"
)

// ModelProvider defines the interface for AI model providers
type ModelProvider interface {
	// Generate generates a response from the model
	Generate(ctx context.Context, request *ModelRequest) (string, *TokenUsage, error)
	
	// GetName returns the provider name
	GetName() string
	
	// SupportedModels returns a list of supported model names
	SupportedModels() []string
	
	// IsModelSupported checks if a model is supported by this provider
	IsModelSupported(model string) bool
	
	// Close cleans up resources
	Close() error
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
	providers map[string]ModelProvider
	modelMap  map[string]string // model -> provider name
	mu        sync.RWMutex
}

// NewModelRegistry creates a new model registry
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		providers: make(map[string]ModelProvider),
		modelMap:  make(map[string]string),
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
	for _, model := range provider.SupportedModels() {
		if existingProvider, exists := mr.modelMap[model]; exists {
			return fmt.Errorf("model %s already supported by provider %s", model, existingProvider)
		}
		mr.modelMap[model] = name
	}
	
	return nil
}

// GetProvider returns the provider for a given model
func (mr *ModelRegistry) GetProvider(model string) (ModelProvider, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	providerName, exists := mr.modelMap[model]
	if !exists {
		return nil, fmt.Errorf("no provider found for model %s", model)
	}
	
	provider, exists := mr.providers[providerName]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", providerName)
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

// ListModels returns all supported models
func (mr *ModelRegistry) ListModels() []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	models := make([]string, 0, len(mr.modelMap))
	for model := range mr.modelMap {
		models = append(models, model)
	}
	return models
}

// IsModelSupported checks if a model is supported
func (mr *ModelRegistry) IsModelSupported(model string) bool {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	_, exists := mr.modelMap[model]
	return exists
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
	name           string
	supportedModels []string
	responses      map[string]string
}

// NewMockModelProvider creates a new mock model provider
func NewMockModelProvider(name string, models []string) *MockModelProvider {
	return &MockModelProvider{
		name:            name,
		supportedModels: models,
		responses:       make(map[string]string),
	}
}

// SetResponse sets a mock response for a specific prompt
func (mp *MockModelProvider) SetResponse(prompt, response string) {
	mp.responses[prompt] = response
}

// Generate generates a mock response
func (mp *MockModelProvider) Generate(ctx context.Context, request *ModelRequest) (string, *TokenUsage, error) {
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

// SupportedModels returns supported models
func (mp *MockModelProvider) SupportedModels() []string {
	return mp.supportedModels
}

// IsModelSupported checks if a model is supported
func (mp *MockModelProvider) IsModelSupported(model string) bool {
	for _, supported := range mp.supportedModels {
		if supported == model {
			return true
		}
	}
	return false
}

// Close cleans up resources
func (mp *MockModelProvider) Close() error {
	return nil
}