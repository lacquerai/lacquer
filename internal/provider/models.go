package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/style"
	"github.com/lacquerai/lacquer/internal/tools"
	pkgEvents "github.com/lacquerai/lacquer/pkg/events"
)

type GenerateContext struct {
	StepID  string
	RunID   string
	Context context.Context
}

type LocalModelProvider interface {
	isLocal() bool
}

// Provider defines the interface for AI model providers
type Provider interface {
	// Generate generates a response from the model
	Generate(ctx GenerateContext, request *Request, progressChan chan<- pkgEvents.ExecutionEvent) ([]Message, *execcontext.TokenUsage, error)

	// GetName returns the provider name
	GetName() string

	// ListModels dynamically queries available models from the provider API
	ListModels(ctx context.Context) ([]Info, error)

	// Close cleans up resources
	Close() error
}

// ModelInfo represents information about an available model
type Info struct {
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	Provider    string   `json:"provider"`
	CreatedAt   string   `json:"created_at,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty"`
	Description string   `json:"description,omitempty"`
	Features    []string `json:"features,omitempty"`
}

type Message struct {
	Role    string                   `json:"role"`
	Content []ContentBlockParamUnion `json:"content"`
}

// Image Source Types
type Base64ImageSourceParam struct {
	Data      string `json:"data" format:"byte"`
	MediaType string `json:"media_type"`
	Type      string `json:"type"`
}

type URLImageSourceParam struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

type ImageBlockParamSourceUnion struct {
	OfBase64 *Base64ImageSourceParam `json:",omitzero,inline"`
	OfURL    *URLImageSourceParam    `json:",omitzero,inline"`
}

// Document Source Types
type Base64PDFSourceParam struct {
	Data      string `json:"data" format:"byte"`
	MediaType string `json:"media_type"`
	Type      string `json:"type"`
}

type PlainTextSourceParam struct {
	Data      string `json:"data"`
	MediaType string `json:"media_type"`
	Type      string `json:"type"`
}

type ContentBlockSourceParam struct {
	Content ContentBlockSourceContentUnionParam `json:"content,omitzero"`
	Type    string                              `json:"type"`
}

type ContentBlockSourceContentUnionParam struct {
	OfString                    *string                               `json:",omitzero,inline"`
	OfContentBlockSourceContent []ContentBlockSourceContentUnionParam `json:",omitzero,inline"`
}

type URLPDFSourceParam struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

type TextBlockParam struct {
	Text string `json:"text"`
	Type string `json:"type"` // text
}

type ImageBlockParam struct {
	Source ImageBlockParamSourceUnion `json:"source,omitzero"`
	Type   string                     `json:"type"` // image
}

type ToolUseBlockParam struct {
	ID    string          `json:"id"`
	Input json.RawMessage `json:"input,omitzero"`
	Name  string          `json:"name"`
	Type  string          `json:"type"` // tool_use
}

type ToolResultBlockParam struct {
	ToolUseID string `json:"tool_use_id"`
	IsError   *bool  `json:"is_error,omitzero"`
	Content   string `json:"content,omitzero"`
	Type      string `json:"type"` // tool_result
}

type ThinkingBlockParam struct {
	Signature string `json:"signature"`
	Thinking  string `json:"thinking"`
	Type      string `json:"type"` // thinking
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

// Request represents a request to generate text from a model
type Request struct {
	Model        string       `json:"model"`
	Messages     []Message    `json:"messages"`
	SystemPrompt string       `json:"system_prompt,omitempty"`
	Temperature  *float64     `json:"temperature,omitempty"`
	MaxTokens    *int         `json:"max_tokens,omitempty"`
	TopP         *float64     `json:"top_p,omitempty"`
	Stop         []string     `json:"stop,omitempty"`
	Tools        []tools.Tool `json:"tools,omitempty"`

	// Additional metadata
	RequestID string                 `json:"request_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// GetPrompt returns the first text prompt for the model request,
// this is used for simple agents that don't have complex messages.
// e.g. claude-code
func (mr *Request) GetPrompt() string {
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
type Registry struct {
	modelCache *ModelCache
	providers  map[string]Provider
	modelMap   map[string]map[string]bool
	mu         sync.RWMutex
}

// NewRegistry creates a new model registry
func NewRegistry(disableCache bool) *Registry {
	return &Registry{
		modelCache: NewModelCache(disableCache),
		providers:  make(map[string]Provider),
		modelMap:   make(map[string]map[string]bool),
	}
}

// RegisterProvider registers a model provider
func (mr *Registry) RegisterProvider(provider Provider) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	name := provider.GetName()
	if _, exists := mr.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	mr.providers[name] = provider

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
func (mr *Registry) GetProviderForModel(providerName, model string) (Provider, error) {
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
func (mr *Registry) GetProviderByName(name string) (Provider, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	provider, exists := mr.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

// IsModelSupported checks if a model is supported
func (mr *Registry) IsModelSupported(providerName, model string) bool {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if _, exists := mr.modelMap[providerName]; !exists {
		return false
	}

	return mr.modelMap[providerName][model]
}

// MockModelProvider is a mock implementation for testing
type MockProvider struct {
	name            string
	supportedModels []Info
	responses       map[string]string
}

// NewMockModelProvider creates a new mock model provider
func NewMockProvider(name string, models []Info) *MockProvider {
	return &MockProvider{
		name:            name,
		supportedModels: models,
		responses:       make(map[string]string),
	}
}

// ListModels returns the supported models
func (mp *MockProvider) ListModels(ctx context.Context) ([]Info, error) {
	return mp.supportedModels, nil
}

// SetResponse sets a mock response for a specific prompt
func (mp *MockProvider) SetResponse(prompt, response string) {
	mp.responses[prompt] = response
}

// Generate generates a mock response
func (mp *MockProvider) Generate(gtx GenerateContext, request *Request, progressChan chan<- pkgEvents.ExecutionEvent) ([]Message, *execcontext.TokenUsage, error) {
	// Check for specific response
	if response, exists := mp.responses[request.GetPrompt()]; exists {
		return []Message{
				{
					Role:    "assistant",
					Content: []ContentBlockParamUnion{NewTextBlock(response)},
				},
			}, &execcontext.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			}, nil
	}

	// Default mock response
	response := fmt.Sprintf("Mock response for prompt: %s", request.GetPrompt())
	return []Message{
			{
				Role:    "assistant",
				Content: []ContentBlockParamUnion{NewTextBlock(response)},
			},
		}, &execcontext.TokenUsage{
			PromptTokens:     len(request.GetPrompt()) / 4,
			CompletionTokens: len(response) / 4,
			TotalTokens:      (len(request.GetPrompt()) + len(response)) / 4,
		}, nil
}

// GetName returns the provider name
func (mp *MockProvider) GetName() string {
	return mp.name
}

// IsModelSupported checks if a model is supported
func (mp *MockProvider) IsModelSupported(model string) bool {
	for _, supported := range mp.supportedModels {
		if supported.ID == model {
			return true
		}
	}
	return false
}

// Close cleans up resources
func (mp *MockProvider) Close() error {
	return nil
}

func FormatToolCall(toolCall *ToolUseBlockParam) string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("Using tool %s ", style.InfoStyle.Render(toolCall.Name)))
	var input map[string]interface{}
	err := json.Unmarshal(toolCall.Input, &input)
	if err != nil {
		return sb.String()
	}

	sb.WriteString(formatInputs(input))

	return sb.String()
}

func FormatToolResult(toolResult *ToolResultBlockParam) string {
	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("Tool result: %s", toolResult.Content))

	return sb.String()
}

func formatInputs(inputs map[string]interface{}) string {
	sb := strings.Builder{}
	if len(inputs) > 0 {
		sortedInputs := make([]string, 0, len(inputs))
		for key := range inputs {
			sortedInputs = append(sortedInputs, key)
		}
		sort.Strings(sortedInputs)

		sb.WriteString("(")
		var i int
		for _, key := range sortedInputs {
			value := inputs[key]
			sb.WriteString(fmt.Sprintf("%s: %v", style.MutedStyle.Render(key), style.MutedStyle.Render(fmt.Sprintf("%v", value))))
			if i != len(inputs)-1 {
				sb.WriteString("; ")
			}
			i++
		}

		sb.WriteString(")")
	}

	return sb.String()
}

// MergeConfig merges a yaml config into a struct
func MergeConfig(config interface{}, yamlConfig map[string]interface{}) {
	configValue := reflect.ValueOf(config).Elem()
	configType := configValue.Type()

	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		yamlTag := field.Tag.Get("yaml")

		if yamlTag != "" {
			// Split on comma to handle options like "api_key,omitempty"
			tagParts := strings.Split(yamlTag, ",")
			yamlKey := tagParts[0]

			// Check if this yaml key exists in the yamlConfig
			if value, exists := yamlConfig[yamlKey]; exists {
				fieldValue := configValue.Field(i)
				if fieldValue.CanSet() {
					fieldValue.Set(reflect.ValueOf(value))
				}
			}
		}
	}
}
