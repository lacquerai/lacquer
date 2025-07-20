package anthropic

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/provider"
	"github.com/lacquerai/lacquer/internal/utils"
	pkgEvents "github.com/lacquerai/lacquer/pkg/events"
	"github.com/rs/zerolog/log"
)

var (
	maxTokenMap = map[string]int{
		"claude-opus-4":     64000,
		"claude-sonnet-4":   64000,
		"claude-3-7-sonne":  64000,
		"claude-3-5-sonnet": 8192,
		"claude-3-5-haiku":  8192,
		"claude-3-opus":     4096,
		"claude-3-haiku":    4096,
	}
)

// Provider implements the ModelProvider interface using Anthropic's API
type Provider struct {
	name   string
	client *anthropic.Client
	config *Config
}

// Config contains configuration for the Anthropic provider
type Config struct {
	APIKey           string        `yaml:"api_key"`
	BaseURL          string        `yaml:"base_url"`
	Timeout          time.Duration `yaml:"timeout"`
	MaxRetries       int           `yaml:"max_retries"`
	RetryDelay       time.Duration `yaml:"retry_delay"`
	UserAgent        string        `yaml:"user_agent"`
	AnthropicVersion string        `yaml:"anthropic_version"`
}

// Request represents a request to the Anthropic API
type Request struct {
	Model         string      `json:"model"`
	MaxTokens     int         `json:"max_tokens"`
	Messages      []Message   `json:"messages"`
	System        string      `json:"system,omitempty"`
	Temperature   *float64    `json:"temperature,omitempty"`
	TopP          *float64    `json:"top_p,omitempty"`
	TopK          *int        `json:"top_k,omitempty"`
	StopSequences []string    `json:"stop_sequences,omitempty"`
	Stream        bool        `json:"stream,omitempty"`
	Metadata      *Metadata   `json:"metadata,omitempty"`
	Tools         []Tool      `json:"tools,omitempty"`
	ToolChoice    *ToolChoice `json:"tool_choice,omitempty"`
}

// Message represents a message in the conversation
type Message struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

// Content represents content within a message
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// For tool use and tool result content types
	ID      string                 `json:"id,omitempty"`
	Name    string                 `json:"name,omitempty"`
	Input   map[string]interface{} `json:"input,omitempty"`
	Content string                 `json:"content,omitempty"`
	IsError bool                   `json:"is_error,omitempty"`
}

// Tool represents a tool that can be used by the model
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolChoice controls tool usage
type ToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// Metadata contains metadata for the request
type Metadata struct {
	UserID string `json:"user_id,omitempty"`
}

// Response represents a response from the Anthropic API
type Response struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Role         string    `json:"role"`
	Content      []Content `json:"content"`
	Model        string    `json:"model"`
	StopReason   string    `json:"stop_reason"`
	StopSequence string    `json:"stop_sequence,omitempty"`
	Usage        Usage     `json:"usage"`
}

// AnthropicUsage represents token usage information
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Error represents an error response from the Anthropic API
type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ErrorResponse represents the full error response structure
type ErrorResponse struct {
	Error Error `json:"error"`
}

// ModelsResponse represents the response from the /v1/models endpoint
type ModelsResponse struct {
	Data    []ModelInfo `json:"data"`
	FirstID string      `json:"first_id,omitempty"`
	LastID  string      `json:"last_id,omitempty"`
	HasMore bool        `json:"has_more"`
}

// ModelInfo represents a single model from the Anthropic API
type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	Type        string `json:"type"`
}

// Default configuration for Anthropic
func DefaultConfig() *Config {
	config := &Config{
		BaseURL:          "https://api.anthropic.com",
		Timeout:          60 * time.Second,
		MaxRetries:       3,
		RetryDelay:       time.Second,
		UserAgent:        "lacquer/1.0",
		AnthropicVersion: "2023-06-01",
	}
	if baseURL := os.Getenv("LACQUER_ANTHROPIC_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
	}

	return config
}

// NewAnthropicProvider creates a new Anthropic model provider
func NewProvider(config *Config) (*Provider, error) {
	if config == nil {
		config = DefaultConfig()
	} else {
		// Merge with defaults for missing fields
		defaults := DefaultConfig()
		if config.Timeout == 0 {
			config.Timeout = defaults.Timeout
		}
		if config.MaxRetries == 0 {
			config.MaxRetries = defaults.MaxRetries
		}
		if config.RetryDelay == 0 {
			config.RetryDelay = defaults.RetryDelay
		}
		if config.UserAgent == "" {
			config.UserAgent = defaults.UserAgent
		}
		if config.AnthropicVersion == "" {
			config.AnthropicVersion = defaults.AnthropicVersion
		}
	}

	// Validate API key
	if config.APIKey == "" {
		config.APIKey = GetAnthropicAPIKeyFromEnv()
		if config.APIKey == "" {
			return nil, fmt.Errorf("anthropic API key is required")
		}
	}

	client := anthropic.NewClient(
		option.WithAPIKey(config.APIKey),
		option.WithBaseURL(config.BaseURL),
		option.WithHTTPClient(&http.Client{
			Timeout: config.Timeout,
		}),
	)

	provider := &Provider{
		name:   "anthropic",
		client: &client,
		config: config,
	}

	log.Info().
		Str("base_url", config.BaseURL).
		Str("anthropic_version", config.AnthropicVersion).
		Msg("Anthropic provider initialized")

	return provider, nil
}

// Generate generates a response using the Anthropic API
func (p *Provider) Generate(gtx provider.GenerateContext, request *provider.Request, progressChan chan<- pkgEvents.ExecutionEvent) ([]provider.Message, *execcontext.TokenUsage, error) {
	// Build the Anthropic request
	anthropicReq, err := p.buildAnthropicRequest(request)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build Anthropic request: %w", err)
	}

	// Make the API call with retries
	response, err := p.client.Messages.New(gtx.Context, anthropicReq, option.WithRequestTimeout(time.Minute*10))
	if err != nil {
		return nil, nil, fmt.Errorf("Anthropic API call failed: %w", err)
	}

	// Convert usage information
	tokenUsage := &execcontext.TokenUsage{
		PromptTokens:     int(response.Usage.InputTokens),
		CompletionTokens: int(response.Usage.OutputTokens),
		TotalTokens:      int(response.Usage.InputTokens + response.Usage.OutputTokens),
	}

	content := make([]provider.Message, len(response.Content))
	for i, contentBlock := range response.Content {
		message := p.anthropicContentToModelMessage(contentBlock)
		if message != nil {
			content[i] = *message
		}
	}

	return content, tokenUsage, nil
}

func (p *Provider) anthropicContentToModelMessage(contentBlock anthropic.ContentBlockUnion) *provider.Message {
	switch contentBlock.AsAny().(type) {
	case anthropic.TextBlock:
		return &provider.Message{
			Role:    "assistant",
			Content: []provider.ContentBlockParamUnion{provider.NewTextBlock(contentBlock.Text)},
		}
	case anthropic.ToolUseBlock:
		return &provider.Message{
			Role:    "assistant",
			Content: []provider.ContentBlockParamUnion{provider.NewToolUseBlock(contentBlock.ID, contentBlock.Input, contentBlock.Name)},
		}
	case anthropic.ThinkingBlock:
		return &provider.Message{
			Role:    "assistant",
			Content: []provider.ContentBlockParamUnion{provider.NewThinkingBlock(contentBlock.Signature, contentBlock.Thinking)},
		}
	}

	log.Warn().
		Interface("content_block", contentBlock).
		Msg("Unknown content block type")

	return nil
}

// GetName returns the provider name
func (p *Provider) GetName() string {
	return p.name
}

// ListModels dynamically fetches available models from the Anthropic API
func (p *Provider) ListModels(ctx context.Context) ([]provider.Info, error) {
	models, err := p.client.Models.List(ctx, anthropic.ModelListParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	modelInfos := make([]provider.Info, len(models.Data))
	for i, model := range models.Data {
		modelInfos[i] = provider.Info{
			ID:        model.ID,
			Name:      model.DisplayName,
			Provider:  p.name,
			CreatedAt: model.CreatedAt.Format(time.RFC3339),
			Features:  []string{"text-generation", "chat"},
		}
	}

	log.Debug().
		Int("model_count", len(models.Data)).
		Str("provider", p.name).
		Msg("Successfully fetched models from Anthropic API")

	return modelInfos, nil
}

// Close cleans up resources
func (p *Provider) Close() error {
	return nil
}

// buildAnthropicRequest converts a ModelRequest to an AnthropicRequest
func (p *Provider) buildAnthropicRequest(request *provider.Request) (anthropic.MessageNewParams, error) {
	maxTokens := 8192

	for model, tokens := range maxTokenMap {
		if strings.HasPrefix(request.Model, model) {
			maxTokens = tokens
			break
		}
	}

	if request.MaxTokens != nil {
		maxTokens = *request.MaxTokens
	}

	messages := make([]anthropic.MessageParam, len(request.Messages))
	for i, message := range request.Messages {
		messages[i] = anthropic.MessageParam{
			Content: p.convertContentToAnthropicContent(message.Content),
			Role:    anthropic.MessageParamRole(message.Role),
		}
	}

	temperature := anthropic.Float(0)
	if request.Temperature != nil {
		temperature = anthropic.Float(*request.Temperature)
	}

	topP := anthropic.Float(0)
	if request.TopP != nil {
		topP = anthropic.Float(*request.TopP)
	}

	tools := make([]anthropic.ToolUnionParam, len(request.Tools))
	for i, tool := range request.Tools {
		tools[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Type:       "object",
					Properties: tool.Parameters.Properties,
					Required:   tool.Parameters.Required,
				},
			},
		}
	}
	mp := anthropic.MessageNewParams{
		StopSequences: request.Stop,
		MaxTokens:     int64(maxTokens),
		Temperature:   temperature,
		TopP:          topP,
		Messages:      messages,
		Model:         anthropic.Model(request.Model),
		Tools:         tools,
	}

	if request.SystemPrompt != "" {
		mp.System = []anthropic.TextBlockParam{{Text: request.SystemPrompt}}
	}

	return mp, nil
}

// convertContentToAnthropicContent converts a content block to an Anthropic content block
func (p *Provider) convertContentToAnthropicContent(content []provider.ContentBlockParamUnion) []anthropic.ContentBlockParamUnion {
	anthropicContent := make([]anthropic.ContentBlockParamUnion, len(content))

	for i, contentBlock := range content {
		switch contentBlock.Type() {
		case provider.ContentBlockTypeText:
			anthropicContent[i] = anthropic.NewTextBlock(contentBlock.OfText.Text)
		case provider.ContentBlockTypeToolUse:
			anthropicContent[i] = anthropic.NewToolUseBlock(contentBlock.OfToolUse.ID, contentBlock.OfToolUse.Input, contentBlock.OfToolUse.Name)
		case provider.ContentBlockTypeToolResult:
			anthropicContent[i] = anthropic.NewToolResultBlock(contentBlock.OfToolResult.ToolUseID, contentBlock.OfToolResult.Content, *contentBlock.OfToolResult.IsError)
		case provider.ContentBlockTypeThinking:
			anthropicContent[i] = anthropic.NewThinkingBlock(contentBlock.OfThinking.Signature, contentBlock.OfThinking.Thinking)
			// TODO: Add image support
			// case ContentBlockTypeImage:
		}
	}

	return anthropicContent
}

func GetAnthropicAPIKeyFromEnv() string {
	// Try common environment variable names
	envVars := []string{
		"ANTHROPIC_API_KEY",
		"CLAUDE_API_KEY",
		"ANTHROPIC_KEY",
	}

	for _, envVar := range envVars {
		if key := strings.TrimSpace(getEnvVar(envVar)); key != "" {
			return key
		}
	}

	return ""
}

// getEnvVar is a helper to get environment variables (can be mocked for testing)
var getEnvVar = func(key string) string {
	env := utils.GetEnvironmentVars()
	return env[key]
}

// ValidateAnthropicAPIKey validates that an API key has the correct format
func ValidateAnthropicAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// Anthropic API keys typically start with "sk-ant-"
	if !strings.HasPrefix(apiKey, "sk-ant-") {
		return fmt.Errorf("invalid API key format: should start with 'sk-ant-'")
	}

	// Should be reasonably long
	if len(apiKey) < 20 {
		return fmt.Errorf("API key appears to be too short")
	}

	return nil
}
