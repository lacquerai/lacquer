package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

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

// OpenAIRequest represents a request to the OpenAI API
type OpenAIRequest struct {
	Model            string                `json:"model"`
	Messages         []OpenAIMessage       `json:"messages"`
	MaxTokens        *int                  `json:"max_tokens,omitempty"`
	Temperature      *float64              `json:"temperature,omitempty"`
	TopP             *float64              `json:"top_p,omitempty"`
	N                int                   `json:"n,omitempty"`
	Stream           bool                  `json:"stream,omitempty"`
	Stop             []string              `json:"stop,omitempty"`
	PresencePenalty  *float64              `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64              `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]float64    `json:"logit_bias,omitempty"`
	User             string                `json:"user,omitempty"`
	Tools            []OpenAITool          `json:"tools,omitempty"`
	ToolChoice       interface{}           `json:"tool_choice,omitempty"`
	ResponseFormat   *OpenAIResponseFormat `json:"response_format,omitempty"`
	Seed             *int                  `json:"seed,omitempty"`
}

// OpenAIMessage represents a message in the OpenAI API format
type OpenAIMessage struct {
	Role         string              `json:"role"`
	Content      string              `json:"content"`
	Name         string              `json:"name,omitempty"`
	ToolCalls    []OpenAIToolCall    `json:"tool_calls,omitempty"`
	ToolCallID   string              `json:"tool_call_id,omitempty"`
	FunctionCall *OpenAIFunctionCall `json:"function_call,omitempty"`
}

// OpenAITool represents a tool/function definition
type OpenAITool struct {
	Type     string             `json:"type"`
	Function *OpenAIFunctionDef `json:"function,omitempty"`
}

// OpenAIFunctionDef represents a function definition
type OpenAIFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// OpenAIToolCall represents a tool call
type OpenAIToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function *OpenAIFunctionCall `json:"function,omitempty"`
}

// OpenAIFunctionCall represents a function call
type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// OpenAIResponseFormat controls the format of the response
type OpenAIResponseFormat struct {
	Type string `json:"type"`
}

// OpenAIResponse represents the response from OpenAI API
type OpenAIResponse struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []OpenAIChoice `json:"choices"`
	Usage             OpenAIUsage    `json:"usage"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
}

// OpenAIChoice represents a choice in the response
type OpenAIChoice struct {
	Index        int             `json:"index"`
	Message      OpenAIMessage   `json:"message"`
	FinishReason string          `json:"finish_reason"`
	LogProbs     *OpenAILogProbs `json:"logprobs,omitempty"`
}

// OpenAILogProbs represents log probabilities
type OpenAILogProbs struct {
	Content []OpenAITokenLogProb `json:"content"`
}

// OpenAITokenLogProb represents token log probabilities
type OpenAITokenLogProb struct {
	Token   string  `json:"token"`
	LogProb float64 `json:"logprob"`
	Bytes   []int   `json:"bytes"`
}

// OpenAIUsage represents token usage information
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIError represents an error from the OpenAI API
type OpenAIError struct {
	ErrorInfo struct {
		Message string      `json:"message"`
		Type    string      `json:"type"`
		Param   interface{} `json:"param"`
		Code    interface{} `json:"code"`
	} `json:"error"`
}

// Error implements the error interface
func (e *OpenAIError) Error() string {
	return fmt.Sprintf("OpenAI API error: %s (type: %s)", e.ErrorInfo.Message, e.ErrorInfo.Type)
}

// OpenAIModelsResponse represents the response from the /v1/models endpoint
type OpenAIModelsResponse struct {
	Object string            `json:"object"`
	Data   []OpenAIModelInfo `json:"data"`
}

// OpenAIModelInfo represents a single model from the OpenAI API
type OpenAIModelInfo struct {
	ID         string `json:"id"`
	Object     string `json:"object"`
	Created    int64  `json:"created"`
	OwnedBy    string `json:"owned_by"`
	Status     string `json:"status,omitempty"`
	Deprecated bool   `json:"deprecated,omitempty"`
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(config *OpenAIConfig) (*OpenAIProvider, error) {
	if config == nil {
		config = &OpenAIConfig{}
	}

	// Set defaults
	defaults := getDefaultOpenAIConfig()
	if config.BaseURL == "" {
		if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
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
func (p *OpenAIProvider) Generate(ctx GenerateContext, request *ModelRequest, progressChan chan<- ExecutionEvent) ([]ModelMessage, *TokenUsage, error) {
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

	var maxTokens int64
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
		Model:       request.Model,
		Messages:    p.buildOpenAIRequest(request),
		Temperature: openai.Float(temperature),
		MaxTokens:   openai.Int(maxTokens),
		TopP:        openai.Float(topP),
		N:           openai.Int(1),
		Tools:       tools,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create OpenAI completion: %w", err)
	}

	// Calculate token usage and cost
	tokenUsage := &TokenUsage{
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
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	response, err := p.client.Models.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	models := make([]ModelInfo, len(response.Data))

	for i, model := range response.Data {
		features := []string{"text-generation"}
		if strings.Contains(model.ID, "gpt") {
			features = append(features, "chat")
		}
		if strings.Contains(model.ID, "embedding") {
			features = []string{"embeddings"}
		}

		models[i] = ModelInfo{
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
func (p *OpenAIProvider) buildOpenAIRequest(request *ModelRequest) []openai.ChatCompletionMessageParamUnion {
	var messages []openai.ChatCompletionMessageParamUnion
	if request.SystemPrompt != "" {
		messages = append(messages, openai.SystemMessage(request.SystemPrompt))
	}

	for _, message := range request.Messages {
		for _, content := range message.Content {
			switch content.Type() {
			case ContentBlockTypeText:
				messages = append(messages, openai.UserMessage(content.OfText.Text))
			case ContentBlockTypeToolUse:
				messages = append(messages, openai.ChatCompletionMessageParamOfFunction(string(content.OfToolUse.Input), content.OfToolUse.ID))
			case ContentBlockTypeToolResult:
				messages = append(messages, openai.ToolMessage(content.OfToolResult.Content, content.OfToolResult.ToolUseID))
			}
		}
	}

	return messages
}

// extractResponseContent extracts the text content from the API response
func (p *OpenAIProvider) extractResponseContent(response *openai.ChatCompletion) []ModelMessage {
	var messages []ModelMessage

	choice := response.Choices[0]

	if choice.Message.Content != "" {
		messages = append(messages, ModelMessage{
			Role:    "assistant",
			Content: []ContentBlockParamUnion{NewTextBlock(choice.Message.Content)},
		})
	}

	if choice.Message.ToolCalls != nil {
		for _, toolCall := range choice.Message.ToolCalls {
			messages = append(messages, ModelMessage{
				Role:    "assistant",
				Content: []ContentBlockParamUnion{NewToolUseBlock(toolCall.Function.Name, json.RawMessage(toolCall.Function.Arguments), toolCall.Function.Name)},
			})
		}
	}

	return messages
}

// getDefaultOpenAIConfig returns default configuration values
func getDefaultOpenAIConfig() *OpenAIConfig {
	return &OpenAIConfig{
		BaseURL:    "https://api.openai.com/v1",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
		UserAgent:  "lacquer/1.0",
	}
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
