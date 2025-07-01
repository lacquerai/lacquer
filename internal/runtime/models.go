package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type GenerateContext struct {
	StepID  string
	RunID   string
	Context context.Context
}

type LocalModelProvider interface {
	isLocal() bool
}

// ModelProvider defines the interface for AI model providers
type ModelProvider interface {
	// Generate generates a response from the model
	Generate(ctx GenerateContext, request *ModelRequest, progressChan chan<- ExecutionEvent) ([]ModelMessage, *TokenUsage, error)

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

type ModelMessage struct {
	Role    string                   `json:"role"`
	Content []ContentBlockParamUnion `json:"content"`
}

// Image Source Types
type Base64ImageSourceParam struct {
	Data      string `json:"data,required" format:"byte"`
	MediaType string `json:"media_type,omitzero,required"`
	Type      string `json:"type,required"`
}

type URLImageSourceParam struct {
	URL  string `json:"url,required"`
	Type string `json:"type,required"`
}

type ImageBlockParamSourceUnion struct {
	OfBase64 *Base64ImageSourceParam `json:",omitzero,inline"`
	OfURL    *URLImageSourceParam    `json:",omitzero,inline"`
}

// Document Source Types
type Base64PDFSourceParam struct {
	Data      string `json:"data,required" format:"byte"`
	MediaType string `json:"media_type,required"`
	Type      string `json:"type,required"`
}

type PlainTextSourceParam struct {
	Data      string `json:"data,required"`
	MediaType string `json:"media_type,required"`
	Type      string `json:"type,required"`
}

type ContentBlockSourceParam struct {
	Content ContentBlockSourceContentUnionParam `json:"content,omitzero,required"`
	Type    string                              `json:"type,required"`
}

type ContentBlockSourceContentUnionParam struct {
	OfString                    *string                               `json:",omitzero,inline"`
	OfContentBlockSourceContent []ContentBlockSourceContentUnionParam `json:",omitzero,inline"`
}

type URLPDFSourceParam struct {
	URL  string `json:"url,required"`
	Type string `json:"type,required"`
}

type TextBlockParam struct {
	Text string `json:"text,required"`
	Type string `json:"type,required"` // text
}

type ImageBlockParam struct {
	Source ImageBlockParamSourceUnion `json:"source,omitzero,required"`
	Type   string                     `json:"type,required"` // image
}

type ToolUseBlockParam struct {
	ID    string          `json:"id,required"`
	Input json.RawMessage `json:"input,omitzero,required"`
	Name  string          `json:"name,required"`
	Type  string          `json:"type,required"` // tool_use
}

type ToolResultBlockParam struct {
	ToolUseID string `json:"tool_use_id,required"`
	IsError   *bool  `json:"is_error,omitzero"`
	Content   string `json:"content,omitzero"`
	Type      string `json:"type,required"` // tool_result
}

type ThinkingBlockParam struct {
	Signature string `json:"signature,required"`
	Thinking  string `json:"thinking,required"`
	Type      string `json:"type,required"` // thinking
}

type ContentBlockType string

const (
	ContentBlockTypeText       ContentBlockType = "text"
	ContentBlockTypeImage      ContentBlockType = "image"
	ContentBlockTypeToolUse    ContentBlockType = "tool_use"
	ContentBlockTypeToolResult ContentBlockType = "tool_result"
	ContentBlockTypeThinking   ContentBlockType = "thinking"
)

// Main Union Type
type ContentBlockParamUnion struct {
	OfText       *TextBlockParam       `json:",omitzero,inline"`
	OfImage      *ImageBlockParam      `json:",omitzero,inline"`
	OfToolUse    *ToolUseBlockParam    `json:",omitzero,inline"`
	OfToolResult *ToolResultBlockParam `json:",omitzero,inline"`
	OfThinking   *ThinkingBlockParam   `json:",omitzero,inline"`
}

func (c *ContentBlockParamUnion) Type() ContentBlockType {
	if c.OfImage != nil {
		return ContentBlockTypeImage
	}
	if c.OfToolUse != nil {
		return ContentBlockTypeToolUse
	}
	if c.OfToolResult != nil {
		return ContentBlockTypeToolResult
	}
	if c.OfThinking != nil {
		return ContentBlockTypeThinking
	}

	return ContentBlockTypeText
}

func NewTextBlock(text string) ContentBlockParamUnion {
	return ContentBlockParamUnion{OfText: &TextBlockParam{Text: text, Type: "text"}}
}

func NewImageBlock[T Base64ImageSourceParam | URLImageSourceParam](source T) ContentBlockParamUnion {
	var image ImageBlockParam
	switch v := any(source).(type) {
	case Base64ImageSourceParam:
		image.Source.OfBase64 = &v
	case URLImageSourceParam:
		image.Source.OfURL = &v
	}

	image.Type = "image"
	return ContentBlockParamUnion{OfImage: &image}
}

func NewToolUseBlock(id string, input json.RawMessage, name string) ContentBlockParamUnion {
	var toolUse ToolUseBlockParam
	toolUse.ID = id
	toolUse.Input = input
	toolUse.Name = name
	toolUse.Type = "tool_use"

	return ContentBlockParamUnion{OfToolUse: &toolUse}
}

func NewToolResultBlock(toolUseID string, content string, isError *bool) ContentBlockParamUnion {
	toolBlock := ToolResultBlockParam{
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
		Type:      "tool_result",
	}

	return ContentBlockParamUnion{OfToolResult: &toolBlock}
}

func NewThinkingBlock(signature string, thinking string) ContentBlockParamUnion {
	var variant ThinkingBlockParam
	variant.Signature = signature
	variant.Thinking = thinking
	variant.Type = "thinking"
	return ContentBlockParamUnion{OfThinking: &variant}
}

// ModelRequest represents a request to generate text from a model
type ModelRequest struct {
	Model        string         `json:"model"`
	Messages     []ModelMessage `json:"messages"`
	SystemPrompt string         `json:"system_prompt,omitempty"`
	Temperature  *float64       `json:"temperature,omitempty"`
	MaxTokens    *int           `json:"max_tokens,omitempty"`
	TopP         *float64       `json:"top_p,omitempty"`
	Stop         []string       `json:"stop,omitempty"`
	Tools        []Tool         `json:"tools,omitempty"`

	// Additional metadata
	RequestID string                 `json:"request_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// GetPrompt returns the first text prompt for the model request,
// this is used for simple agents that don't have complex messages.
// e.g. claude-code
func (mr *ModelRequest) GetPrompt() string {
	if len(mr.Messages) == 0 {
		return ""
	}

	if len(mr.Messages[0].Content) == 0 {
		return ""
	}

	if mr.Messages[0].Content[0].OfText != nil {
		return mr.Messages[0].Content[0].OfText.Text
	}

	return ""
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
func (mp *MockModelProvider) Generate(ctx context.Context, request *ModelRequest, progressChan chan<- ExecutionEvent) ([]ModelMessage, *TokenUsage, error) {
	// Check for specific response
	if response, exists := mp.responses[request.GetPrompt()]; exists {
		return []ModelMessage{
				{
					Role:    "assistant",
					Content: []ContentBlockParamUnion{NewTextBlock(response)},
				},
			}, &TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			}, nil
	}

	// Default mock response
	response := fmt.Sprintf("Mock response for prompt: %s", request.GetPrompt())
	return []ModelMessage{
			{
				Role:    "assistant",
				Content: []ContentBlockParamUnion{NewTextBlock(response)},
			},
		}, &TokenUsage{
			PromptTokens:     len(request.GetPrompt()) / 4,
			CompletionTokens: len(response) / 4,
			TotalTokens:      (len(request.GetPrompt()) + len(response)) / 4,
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
