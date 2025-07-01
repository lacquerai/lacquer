package runtime

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/rs/zerolog/log"
)

// AnthropicProvider implements the ModelProvider interface using Anthropic's API
type AnthropicProvider struct {
	name   string
	client *anthropic.Client
	config *AnthropicConfig
}

// AnthropicConfig contains configuration for the Anthropic provider
type AnthropicConfig struct {
	APIKey           string        `yaml:"api_key"`
	BaseURL          string        `yaml:"base_url"`
	Timeout          time.Duration `yaml:"timeout"`
	MaxRetries       int           `yaml:"max_retries"`
	RetryDelay       time.Duration `yaml:"retry_delay"`
	UserAgent        string        `yaml:"user_agent"`
	AnthropicVersion string        `yaml:"anthropic_version"`
}

// AnthropicRequest represents a request to the Anthropic API
type AnthropicRequest struct {
	Model         string               `json:"model"`
	MaxTokens     int                  `json:"max_tokens"`
	Messages      []AnthropicMessage   `json:"messages"`
	System        string               `json:"system,omitempty"`
	Temperature   *float64             `json:"temperature,omitempty"`
	TopP          *float64             `json:"top_p,omitempty"`
	TopK          *int                 `json:"top_k,omitempty"`
	StopSequences []string             `json:"stop_sequences,omitempty"`
	Stream        bool                 `json:"stream,omitempty"`
	Metadata      *AnthropicMetadata   `json:"metadata,omitempty"`
	Tools         []AnthropicTool      `json:"tools,omitempty"`
	ToolChoice    *AnthropicToolChoice `json:"tool_choice,omitempty"`
}

// AnthropicMessage represents a message in the conversation
type AnthropicMessage struct {
	Role    string             `json:"role"`
	Content []AnthropicContent `json:"content"`
}

// AnthropicContent represents content within a message
type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// For tool use and tool result content types
	ID      string                 `json:"id,omitempty"`
	Name    string                 `json:"name,omitempty"`
	Input   map[string]interface{} `json:"input,omitempty"`
	Content string                 `json:"content,omitempty"`
	IsError bool                   `json:"is_error,omitempty"`
}

// AnthropicTool represents a tool that can be used by the model
type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// AnthropicToolChoice controls tool usage
type AnthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// AnthropicMetadata contains metadata for the request
type AnthropicMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

// AnthropicResponse represents a response from the Anthropic API
type AnthropicResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []AnthropicContent `json:"content"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason"`
	StopSequence string             `json:"stop_sequence,omitempty"`
	Usage        AnthropicUsage     `json:"usage"`
}

// AnthropicUsage represents token usage information
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicError represents an error response from the Anthropic API
type AnthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// AnthropicErrorResponse represents the full error response structure
type AnthropicErrorResponse struct {
	Error AnthropicError `json:"error"`
}

// AnthropicModelsResponse represents the response from the /v1/models endpoint
type AnthropicModelsResponse struct {
	Data    []AnthropicModelInfo `json:"data"`
	FirstID string               `json:"first_id,omitempty"`
	LastID  string               `json:"last_id,omitempty"`
	HasMore bool                 `json:"has_more"`
}

// AnthropicModelInfo represents a single model from the Anthropic API
type AnthropicModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	Type        string `json:"type"`
}

// Default configuration for Anthropic
func DefaultAnthropicConfig() *AnthropicConfig {
	return &AnthropicConfig{
		BaseURL:          "https://api.anthropic.com",
		Timeout:          60 * time.Second,
		MaxRetries:       3,
		RetryDelay:       time.Second,
		UserAgent:        "lacquer/1.0",
		AnthropicVersion: "2023-06-01",
	}
}

// NewAnthropicProvider creates a new Anthropic model provider
func NewAnthropicProvider(config *AnthropicConfig) (*AnthropicProvider, error) {
	if config == nil {
		config = DefaultAnthropicConfig()
	} else {
		// Merge with defaults for missing fields
		defaults := DefaultAnthropicConfig()
		if config.BaseURL == "" {
			if baseURL := os.Getenv("ANTHROPIC_BASE_URL"); baseURL != "" {
				config.BaseURL = baseURL
			} else {
				config.BaseURL = defaults.BaseURL
			}
		}
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
			return nil, fmt.Errorf("Anthropic API key is required")
		}
	}

	client := anthropic.NewClient(
		option.WithAPIKey(config.APIKey),
		option.WithBaseURL(config.BaseURL),
		option.WithHTTPClient(&http.Client{
			Timeout: config.Timeout,
		}),
	)

	provider := &AnthropicProvider{
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
func (p *AnthropicProvider) Generate(gtx GenerateContext, request *ModelRequest, progressChan chan<- ExecutionEvent) ([]ModelMessage, *TokenUsage, error) {
	// Build the Anthropic request
	anthropicReq, err := p.buildAnthropicRequest(request)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build Anthropic request: %w", err)
	}

	// Make the API call with retries
	response, err := p.client.Messages.New(gtx.Context, anthropicReq)
	if err != nil {
		return nil, nil, fmt.Errorf("Anthropic API call failed: %w", err)
	}

	// Convert usage information
	tokenUsage := &TokenUsage{
		PromptTokens:     int(response.Usage.InputTokens),
		CompletionTokens: int(response.Usage.OutputTokens),
		TotalTokens:      int(response.Usage.InputTokens + response.Usage.OutputTokens),
	}

	content := make([]ModelMessage, len(response.Content))
	for i, contentBlock := range response.Content {
		message := p.anthropicContentToModelMessage(contentBlock)
		if message != nil {
			content[i] = *message
		}
	}

	return content, tokenUsage, nil
}

func (p *AnthropicProvider) anthropicContentToModelMessage(contentBlock anthropic.ContentBlockUnion) *ModelMessage {
	switch contentBlock.AsAny().(type) {
	case anthropic.TextBlock:
		return &ModelMessage{
			Role:    "assistant",
			Content: []ContentBlockParamUnion{NewTextBlock(contentBlock.Text)},
		}
	case anthropic.ToolUseBlock:
		return &ModelMessage{
			Role:    "assistant",
			Content: []ContentBlockParamUnion{NewToolUseBlock(contentBlock.ID, contentBlock.Input, contentBlock.Name)},
		}
	case anthropic.ThinkingBlock:
		return &ModelMessage{
			Role:    "assistant",
			Content: []ContentBlockParamUnion{NewThinkingBlock(contentBlock.Signature, contentBlock.Thinking)},
		}
	}

	log.Warn().
		Interface("content_block", contentBlock).
		Msg("Unknown content block type")

	return nil
}

// GetName returns the provider name
func (p *AnthropicProvider) GetName() string {
	return p.name
}

// ListModels dynamically fetches available models from the Anthropic API
func (p *AnthropicProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	models, err := p.client.Models.List(ctx, anthropic.ModelListParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	modelInfos := make([]ModelInfo, len(models.Data))
	for i, model := range models.Data {
		modelInfos[i] = ModelInfo{
			ID:          model.ID,
			Name:        model.DisplayName,
			Provider:    p.name,
			CreatedAt:   model.CreatedAt.Format(time.RFC3339),
			Deprecated:  false, // Anthropic doesn't provide this field directly
			Description: "",    // Not available in basic response
			Features:    []string{"text-generation", "chat"},
		}
	}

	log.Debug().
		Int("model_count", len(models.Data)).
		Str("provider", p.name).
		Msg("Successfully fetched models from Anthropic API")

	return modelInfos, nil
}

// Close cleans up resources
func (p *AnthropicProvider) Close() error {
	return nil
}

// buildAnthropicRequest converts a ModelRequest to an AnthropicRequest
func (p *AnthropicProvider) buildAnthropicRequest(request *ModelRequest) (anthropic.MessageNewParams, error) {
	maxTokens := 4096
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
func (p *AnthropicProvider) convertContentToAnthropicContent(content []ContentBlockParamUnion) []anthropic.ContentBlockParamUnion {
	anthropicContent := make([]anthropic.ContentBlockParamUnion, len(content))

	for i, contentBlock := range content {
		switch contentBlock.Type() {
		case ContentBlockTypeText:
			anthropicContent[i] = anthropic.NewTextBlock(contentBlock.OfText.Text)
		case ContentBlockTypeToolUse:
			anthropicContent[i] = anthropic.NewToolUseBlock(contentBlock.OfToolUse.ID, contentBlock.OfToolUse.Input, contentBlock.OfToolUse.Name)
		case ContentBlockTypeToolResult:
			anthropicContent[i] = anthropic.NewToolResultBlock(contentBlock.OfToolResult.ToolUseID, contentBlock.OfToolResult.Content, *contentBlock.OfToolResult.IsError)
		case ContentBlockTypeThinking:
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
	env := getEnvironmentVars()
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
