package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// OpenAIProvider implements the ModelProvider interface using OpenAI's API
type OpenAIProvider struct {
	name       string
	apiKey     string
	baseURL    string
	models     []string
	httpClient *http.Client
	config     *OpenAIConfig
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

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: config.Timeout,
	}

	provider := &OpenAIProvider{
		name:       "openai",
		apiKey:     config.APIKey,
		baseURL:    config.BaseURL,
		models:     getSupportedOpenAIModels(),
		httpClient: httpClient,
		config:     config,
	}

	log.Info().
		Str("base_url", config.BaseURL).
		Int("supported_models", len(provider.models)).
		Msg("OpenAI provider initialized")

	return provider, nil
}

// Generate generates a response using the OpenAI API
func (p *OpenAIProvider) Generate(ctx context.Context, request *ModelRequest) (string, *TokenUsage, error) {
	// Build the OpenAI request
	openaiReq, err := p.buildOpenAIRequest(request)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build OpenAI request: %w", err)
	}

	// Make the API call with retries
	response, err := p.makeAPICall(ctx, openaiReq)
	if err != nil {
		return "", nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	// Extract the response content
	content := p.extractResponseContent(response)

	// Calculate token usage and cost
	tokenUsage := &TokenUsage{
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
		EstimatedCost:    p.calculateCost(request.Model, response.Usage),
	}

	log.Debug().
		Str("model", request.Model).
		Int("prompt_tokens", tokenUsage.PromptTokens).
		Int("completion_tokens", tokenUsage.CompletionTokens).
		Float64("estimated_cost", tokenUsage.EstimatedCost).
		Msg("OpenAI API call completed")

	return content, tokenUsage, nil
}

// GetName returns the provider name
func (p *OpenAIProvider) GetName() string {
	return p.name
}

// SupportedModels returns the list of supported models
func (p *OpenAIProvider) SupportedModels() []string {
	return p.models
}

// IsModelSupported checks if a model is supported
func (p *OpenAIProvider) IsModelSupported(model string) bool {
	for _, supported := range p.models {
		if supported == model {
			return true
		}
	}
	return false
}

// Close cleans up resources
func (p *OpenAIProvider) Close() error {
	// No persistent connections to close for HTTP client
	return nil
}

// buildOpenAIRequest converts a ModelRequest to an OpenAI API request
func (p *OpenAIProvider) buildOpenAIRequest(request *ModelRequest) (*OpenAIRequest, error) {
	// Build messages array
	messages := []OpenAIMessage{}

	// Add system message if provided
	if request.SystemPrompt != "" {
		messages = append(messages, OpenAIMessage{
			Role:    "system",
			Content: request.SystemPrompt,
		})
	}

	// Add user message
	messages = append(messages, OpenAIMessage{
		Role:    "user",
		Content: request.Prompt,
	})

	openaiReq := &OpenAIRequest{
		Model:       request.Model,
		Messages:    messages,
		Temperature: request.Temperature,
		MaxTokens:   request.MaxTokens,
		TopP:        request.TopP,
		N:           1,
		Stream:      false,
		Stop:        request.Stop,
	}

	// Set user ID for tracking
	if request.RequestID != "" {
		openaiReq.User = request.RequestID
	}

	return openaiReq, nil
}

// makeAPICall makes the actual HTTP request to OpenAI with retries
func (p *OpenAIProvider) makeAPICall(ctx context.Context, request *OpenAIRequest) (*OpenAIResponse, error) {
	// Serialize request
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(p.config.RetryDelay * time.Duration(attempt)):
			}

			log.Warn().
				Int("attempt", attempt).
				Err(lastErr).
				Msg("Retrying OpenAI API call")
		}

		// Create HTTP request
		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(requestBody))
		if err != nil {
			lastErr = fmt.Errorf("failed to create HTTP request: %w", err)
			continue
		}

		// Set headers
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
		httpReq.Header.Set("User-Agent", p.config.UserAgent)

		if p.config.OrgID != "" {
			httpReq.Header.Set("OpenAI-Organization", p.config.OrgID)
		}

		// Make the request
		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			continue
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		// Check for HTTP errors
		if resp.StatusCode != http.StatusOK {
			var openaiErr OpenAIError
			if json.Unmarshal(body, &openaiErr) == nil {
				lastErr = &openaiErr
			} else {
				lastErr = fmt.Errorf("OpenAI API error: %s (status: %d)", string(body), resp.StatusCode)
			}

			// Don't retry for certain error types
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				break
			}
			continue
		}

		// Parse successful response
		var openaiResponse OpenAIResponse
		if err := json.Unmarshal(body, &openaiResponse); err != nil {
			lastErr = fmt.Errorf("failed to parse response: %w", err)
			continue
		}

		return &openaiResponse, nil
	}

	return nil, fmt.Errorf("OpenAI API call failed after %d retries: %w", p.config.MaxRetries, lastErr)
}

// extractResponseContent extracts the text content from the API response
func (p *OpenAIProvider) extractResponseContent(response *OpenAIResponse) string {
	if len(response.Choices) == 0 {
		return ""
	}

	return strings.TrimSpace(response.Choices[0].Message.Content)
}

// calculateCost estimates the cost based on token usage and model
func (p *OpenAIProvider) calculateCost(model string, usage OpenAIUsage) float64 {
	// Pricing per 1K tokens (as of 2024)
	costs := map[string]struct {
		prompt     float64
		completion float64
	}{
		"gpt-4":                  {0.03, 0.06},
		"gpt-4-0314":             {0.03, 0.06},
		"gpt-4-0613":             {0.03, 0.06},
		"gpt-4-32k":              {0.06, 0.12},
		"gpt-4-32k-0314":         {0.06, 0.12},
		"gpt-4-32k-0613":         {0.06, 0.12},
		"gpt-4-turbo":            {0.01, 0.03},
		"gpt-4-turbo-2024-04-09": {0.01, 0.03},
		"gpt-4-turbo-preview":    {0.01, 0.03},
		"gpt-4-0125-preview":     {0.01, 0.03},
		"gpt-4-1106-preview":     {0.01, 0.03},
		"gpt-4o":                 {0.005, 0.015},
		"gpt-4o-2024-05-13":      {0.005, 0.015},
		"gpt-4o-mini":            {0.00015, 0.0006},
		"gpt-4o-mini-2024-07-18": {0.00015, 0.0006},
		"gpt-3.5-turbo":          {0.0015, 0.002},
		"gpt-3.5-turbo-0125":     {0.0005, 0.0015},
		"gpt-3.5-turbo-1106":     {0.001, 0.002},
		"gpt-3.5-turbo-0613":     {0.0015, 0.002},
		"gpt-3.5-turbo-16k":      {0.003, 0.004},
		"gpt-3.5-turbo-16k-0613": {0.003, 0.004},
		"gpt-3.5-turbo-instruct": {0.0015, 0.002},
	}

	cost, exists := costs[model]
	if !exists {
		// Default to GPT-4 pricing for unknown models
		cost = costs["gpt-4"]
	}

	promptCost := float64(usage.PromptTokens) / 1000.0 * cost.prompt
	completionCost := float64(usage.CompletionTokens) / 1000.0 * cost.completion

	return promptCost + completionCost
}

// getSupportedOpenAIModels returns the list of supported OpenAI models
func getSupportedOpenAIModels() []string {
	return []string{
		"gpt-4o",
		"gpt-4o-2024-05-13",
		"gpt-4o-mini",
		"gpt-4o-mini-2024-07-18",
		"gpt-4-turbo",
		"gpt-4-turbo-2024-04-09",
		"gpt-4-turbo-preview",
		"gpt-4-0125-preview",
		"gpt-4-1106-preview",
		"gpt-4",
		"gpt-4-0314",
		"gpt-4-0613",
		"gpt-4-32k",
		"gpt-4-32k-0314",
		"gpt-4-32k-0613",
		"gpt-3.5-turbo",
		"gpt-3.5-turbo-0125",
		"gpt-3.5-turbo-1106",
		"gpt-3.5-turbo-0613",
		"gpt-3.5-turbo-16k",
		"gpt-3.5-turbo-16k-0613",
		"gpt-3.5-turbo-instruct",
	}
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
