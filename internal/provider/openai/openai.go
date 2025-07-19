package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lacquerai/lacquer/internal/events"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/provider"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/rs/zerolog/log"
)

// OpenAIProvider implements the ModelProvider interface using OpenAI's API
type OpenAIProvider struct {
	name   string
	client *openai.Client
	config *OpenAIConfig
}

// OpenAIConfig contains configuration for the OpenAI provider
type OpenAIConfig struct {
	APIKey     string        `yaml:"api_key"`
	BaseURL    string        `yaml:"base_url"`
	Timeout    time.Duration `yaml:"timeout"`
	MaxRetries int           `yaml:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay"`
	UserAgent  string        `yaml:"user_agent"`
	OrgID      string        `yaml:"organization_id"`
}

// NewProvider creates a new OpenAI provider
func NewProvider(config *OpenAIConfig) (*OpenAIProvider, error) {
	if config == nil {
		config = &OpenAIConfig{}
	}

	// Set defaults
	defaults := getDefaultOpenAIConfig()
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
	if config.BaseURL == "" {
		config.BaseURL = defaults.BaseURL
	}

	// Get API key from config or environment
	if config.APIKey == "" {
		config.APIKey = GetOpenAIAPIKeyFromEnv()
	}

	// Validate API key
	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	client := openai.NewClient(
		option.WithAPIKey(config.APIKey),
		option.WithBaseURL(config.BaseURL),
		option.WithMaxRetries(config.MaxRetries),
	)
	// Create HTTP client with timeout

	provider := &OpenAIProvider{
		name:   "openai",
		client: &client,
		config: config,
	}

	return provider, nil
}

// Generate generates a response using the OpenAI API
func (p *OpenAIProvider) Generate(ctx provider.GenerateContext, request *provider.Request, progressChan chan<- events.ExecutionEvent) ([]provider.Message, *execcontext.TokenUsage, error) {
	tools := make([]openai.ChatCompletionToolParam, len(request.Tools), 0)
	for _, tool := range request.Tools {
		parameters, err := json.Marshal(tool.Parameters)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal tool parameters: %w", err)
		}

		var functionParameters openai.FunctionParameters
		if err := json.Unmarshal(parameters, &functionParameters); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal tool parameters: %w", err)
		}

		tools = append(tools, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  functionParameters,
			},
		})
	}

	maxTokens := int64(4096)
	if request.MaxTokens != nil {
		maxTokens = int64(*request.MaxTokens)
	}

	var temperature float64
	if request.Temperature != nil {
		temperature = *request.Temperature
	}

	var topP float64
	if request.TopP != nil {
		topP = *request.TopP
	}

	response, err := p.client.Chat.Completions.New(ctx.Context, openai.ChatCompletionNewParams{
		Model:               request.Model,
		Messages:            p.buildOpenAIRequest(request),
		Temperature:         openai.Float(temperature),
		MaxCompletionTokens: openai.Int(maxTokens),
		TopP:                openai.Float(topP),
		N:                   openai.Int(1),
		Tools:               tools,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create OpenAI completion: %w", err)
	}

	// Calculate token usage and cost
	tokenUsage := &execcontext.TokenUsage{
		PromptTokens:     int(response.Usage.PromptTokens),
		CompletionTokens: int(response.Usage.CompletionTokens),
		TotalTokens:      int(response.Usage.TotalTokens),
	}

	log.Debug().
		Str("model", request.Model).
		Int("prompt_tokens", tokenUsage.PromptTokens).
		Int("completion_tokens", tokenUsage.CompletionTokens).
		Msg("OpenAI API call completed")

	messages := p.extractResponseContent(response)

	return messages, tokenUsage, nil
}

// GetName returns the provider name
func (p *OpenAIProvider) GetName() string {
	return p.name
}

// ListModels dynamically fetches available models from the OpenAI API
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]provider.Info, error) {
	response, err := p.client.Models.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	models := make([]provider.Info, len(response.Data))

	for i, model := range response.Data {
		features := []string{"text-generation"}
		if strings.Contains(model.ID, "gpt") {
			features = append(features, "chat")
		}
		if strings.Contains(model.ID, "embedding") {
			features = []string{"embeddings"}
		}

		models[i] = provider.Info{
			ID:          model.ID,
			Name:        model.ID, // OpenAI uses ID as display name
			Provider:    p.name,
			CreatedAt:   fmt.Sprintf("%d", model.Created),
			Description: fmt.Sprintf("Model owned by %s", model.OwnedBy),
			Features:    features,
		}
	}

	log.Debug().
		Int("model_count", len(models)).
		Str("provider", p.name).
		Msg("Successfully fetched models from OpenAI API")

	return models, nil
}

// Close cleans up resources
func (p *OpenAIProvider) Close() error {
	// No persistent connections to close for HTTP client
	return nil
}

// buildOpenAIRequest converts a ModelRequest to an OpenAI API request
func (p *OpenAIProvider) buildOpenAIRequest(request *provider.Request) []openai.ChatCompletionMessageParamUnion {
	var messages []openai.ChatCompletionMessageParamUnion
	if request.SystemPrompt != "" {
		messages = append(messages, openai.SystemMessage(request.SystemPrompt))
	}

	for _, message := range request.Messages {
		for _, content := range message.Content {
			switch content.Type() {
			case provider.ContentBlockTypeText:
				messages = append(messages, openai.UserMessage(content.OfText.Text))
			case provider.ContentBlockTypeToolUse:
				messages = append(messages, openai.ChatCompletionMessageParamOfFunction(string(content.OfToolUse.Input), content.OfToolUse.ID))
			case provider.ContentBlockTypeToolResult:
				messages = append(messages, openai.ToolMessage(content.OfToolResult.Content, content.OfToolResult.ToolUseID))
			}
		}
	}

	return messages
}

// extractResponseContent extracts the text content from the API response
func (p *OpenAIProvider) extractResponseContent(response *openai.ChatCompletion) []provider.Message {
	var messages []provider.Message

	choice := response.Choices[0]

	if choice.Message.Content != "" {
		messages = append(messages, provider.Message{
			Role:    "assistant",
			Content: []provider.ContentBlockParamUnion{provider.NewTextBlock(choice.Message.Content)},
		})
	}

	if choice.Message.ToolCalls != nil {
		for _, toolCall := range choice.Message.ToolCalls {
			messages = append(messages, provider.Message{
				Role:    "assistant",
				Content: []provider.ContentBlockParamUnion{provider.NewToolUseBlock(toolCall.Function.Name, json.RawMessage(toolCall.Function.Arguments), toolCall.Function.Name)},
			})
		}
	}

	return messages
}

// getDefaultOpenAIConfig returns default configuration values
func getDefaultOpenAIConfig() *OpenAIConfig {
	config := &OpenAIConfig{
		BaseURL:    "https://api.openai.com/v1",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
		UserAgent:  "lacquer/1.0",
	}

	if baseURL := os.Getenv("LACQUER_OPENAI_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
	}

	return config
}

// GetOpenAIAPIKeyFromEnv retrieves the OpenAI API key from environment variables
func GetOpenAIAPIKeyFromEnv() string {
	// Try multiple environment variable names
	envVars := []string{
		"OPENAI_API_KEY",
		"OPENAI_KEY",
		"OPENAI_TOKEN",
	}

	for _, envVar := range envVars {
		if apiKey := os.Getenv(envVar); apiKey != "" {
			return apiKey
		}
	}

	return ""
}
