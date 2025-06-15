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

// AnthropicProvider implements the ModelProvider interface using Anthropic's API
type AnthropicProvider struct {
	name       string
	apiKey     string
	baseURL    string
	models     []string
	httpClient *http.Client
	config     *AnthropicConfig
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

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: config.Timeout,
	}

	provider := &AnthropicProvider{
		name:       "anthropic",
		apiKey:     config.APIKey,
		baseURL:    config.BaseURL,
		models:     getSupportedAnthropicModels(),
		httpClient: httpClient,
		config:     config,
	}

	log.Info().
		Str("base_url", config.BaseURL).
		Str("anthropic_version", config.AnthropicVersion).
		Int("supported_models", len(provider.models)).
		Msg("Anthropic provider initialized")

	return provider, nil
}

// Generate generates a response using the Anthropic API
func (p *AnthropicProvider) Generate(ctx context.Context, request *ModelRequest) (string, *TokenUsage, error) {
	// Build the Anthropic request
	anthropicReq, err := p.buildAnthropicRequest(request)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build Anthropic request: %w", err)
	}

	// Make the API call with retries
	response, err := p.makeAPICall(ctx, anthropicReq)
	if err != nil {
		return "", nil, fmt.Errorf("Anthropic API call failed: %w", err)
	}

	// Extract the response content
	content := p.extractResponseContent(response)
	if content == "" {
		return "", nil, fmt.Errorf("no content in Anthropic response")
	}

	// Convert usage information
	tokenUsage := &TokenUsage{
		PromptTokens:     response.Usage.InputTokens,
		CompletionTokens: response.Usage.OutputTokens,
		TotalTokens:      response.Usage.InputTokens + response.Usage.OutputTokens,
		EstimatedCost:    p.calculateCost(request.Model, response.Usage),
	}

	return content, tokenUsage, nil
}

// GetName returns the provider name
func (p *AnthropicProvider) GetName() string {
	return p.name
}

// SupportedModels returns the list of supported models
func (p *AnthropicProvider) SupportedModels() []string {
	return p.models
}

// ListModels dynamically fetches available models from the Anthropic API
func (p *AnthropicProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := fmt.Sprintf("%s/v1/models", p.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", p.config.AnthropicVersion)
	req.Header.Set("Content-Type", "application/json")
	if p.config.UserAgent != "" {
		req.Header.Set("User-Agent", p.config.UserAgent)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp AnthropicErrorResponse
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil {
			return nil, fmt.Errorf("Anthropic API error (%d): %s", resp.StatusCode, errorResp.Error.Message)
		}
		return nil, fmt.Errorf("Anthropic API error (%d): %s", resp.StatusCode, string(body))
	}

	var modelsResp AnthropicModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	// Convert Anthropic model info to our standard format
	models := make([]ModelInfo, len(modelsResp.Data))
	for i, model := range modelsResp.Data {
		models[i] = ModelInfo{
			ID:          model.ID,
			Name:        model.DisplayName,
			Provider:    p.name,
			CreatedAt:   model.CreatedAt,
			Deprecated:  false, // Anthropic doesn't provide this field directly
			Description: "",    // Not available in basic response
			Features:    []string{"text-generation", "chat"},
		}
	}

	log.Debug().
		Int("model_count", len(models)).
		Str("provider", p.name).
		Msg("Successfully fetched models from Anthropic API")

	return models, nil
}

// IsModelSupported checks if a model is supported
func (p *AnthropicProvider) IsModelSupported(model string) bool {
	for _, supported := range p.models {
		if supported == model {
			return true
		}
	}
	return false
}

// Close cleans up resources
func (p *AnthropicProvider) Close() error {
	// Close HTTP client connections
	if transport, ok := p.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}

	log.Info().Msg("Anthropic provider closed")
	return nil
}

// buildAnthropicRequest converts a ModelRequest to an AnthropicRequest
func (p *AnthropicProvider) buildAnthropicRequest(request *ModelRequest) (*AnthropicRequest, error) {
	// Build messages array
	messages := []AnthropicMessage{
		{
			Role: "user",
			Content: []AnthropicContent{
				{
					Type: "text",
					Text: request.Prompt,
				},
			},
		},
	}

	// Set default max tokens if not specified
	maxTokens := 4096
	if request.MaxTokens != nil {
		maxTokens = *request.MaxTokens
	}

	anthropicReq := &AnthropicRequest{
		Model:     request.Model,
		MaxTokens: maxTokens,
		Messages:  messages,
		System:    request.SystemPrompt,
	}

	// Set optional parameters
	if request.Temperature != nil {
		anthropicReq.Temperature = request.Temperature
	}

	if request.TopP != nil {
		anthropicReq.TopP = request.TopP
	}

	if len(request.Stop) > 0 {
		anthropicReq.StopSequences = request.Stop
	}

	// Add metadata if request ID is provided
	if request.RequestID != "" {
		anthropicReq.Metadata = &AnthropicMetadata{
			UserID: request.RequestID,
		}
	}

	return anthropicReq, nil
}

// makeAPICall makes the actual HTTP request to the Anthropic API
func (p *AnthropicProvider) makeAPICall(ctx context.Context, request *AnthropicRequest) (*AnthropicResponse, error) {
	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Prepare the HTTP request
	url := fmt.Sprintf("%s/v1/messages", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", p.config.AnthropicVersion)
	httpReq.Header.Set("User-Agent", p.config.UserAgent)

	// Log request for debugging
	log.Debug().
		Str("url", url).
		Str("model", request.Model).
		Int("max_tokens", request.MaxTokens).
		Msg("Making Anthropic API request")

	// Make the request with retries
	var response *AnthropicResponse
	var lastErr error

	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retrying
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(p.config.RetryDelay * time.Duration(attempt)):
			}

			log.Debug().
				Int("attempt", attempt).
				Dur("delay", p.config.RetryDelay*time.Duration(attempt)).
				Msg("Retrying Anthropic API request")
		}

		response, lastErr = p.doHTTPRequest(httpReq)
		if lastErr == nil {
			return response, nil
		}

		// Don't retry on certain types of errors
		if !p.shouldRetry(lastErr) {
			break
		}

		log.Warn().
			Err(lastErr).
			Int("attempt", attempt+1).
			Int("max_retries", p.config.MaxRetries).
			Msg("Anthropic API request failed, retrying")
	}

	return nil, fmt.Errorf("all retry attempts failed, last error: %w", lastErr)
}

// doHTTPRequest performs the actual HTTP request
func (p *AnthropicProvider) doHTTPRequest(req *http.Request) (*AnthropicResponse, error) {
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		var errorResp AnthropicErrorResponse
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil {
			return nil, fmt.Errorf("Anthropic API error (%d): %s", resp.StatusCode, errorResp.Error.Message)
		}
		return nil, fmt.Errorf("Anthropic API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Debug().
		Str("response_id", anthropicResp.ID).
		Str("model", anthropicResp.Model).
		Str("stop_reason", anthropicResp.StopReason).
		Int("input_tokens", anthropicResp.Usage.InputTokens).
		Int("output_tokens", anthropicResp.Usage.OutputTokens).
		Msg("Received Anthropic API response")

	return &anthropicResp, nil
}

// extractResponseContent extracts the text content from an Anthropic response
func (p *AnthropicProvider) extractResponseContent(response *AnthropicResponse) string {
	var textParts []string

	for _, content := range response.Content {
		if content.Type == "text" && content.Text != "" {
			textParts = append(textParts, content.Text)
		}
	}

	return strings.Join(textParts, "\n")
}

// shouldRetry determines if an error should trigger a retry
func (p *AnthropicProvider) shouldRetry(err error) bool {
	errStr := err.Error()

	// Retry on network errors
	if strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "temporary") {
		return true
	}

	// Retry on rate limit errors (HTTP 429)
	if strings.Contains(errStr, "429") {
		return true
	}

	// Retry on server errors (HTTP 5xx)
	if strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") {
		return true
	}

	// Don't retry on client errors (HTTP 4xx except 429)
	return false
}

// calculateCost estimates the cost of a request based on model and usage
func (p *AnthropicProvider) calculateCost(model string, usage AnthropicUsage) float64 {
	// Pricing per 1K tokens (as of 2024)
	var inputCostPer1K, outputCostPer1K float64

	switch model {
	case "claude-3-5-sonnet-20241022", "claude-3-5-sonnet-20240620":
		inputCostPer1K = 0.003  // $3 per MTok
		outputCostPer1K = 0.015 // $15 per MTok
	case "claude-3-opus-20240229":
		inputCostPer1K = 0.015  // $15 per MTok
		outputCostPer1K = 0.075 // $75 per MTok
	case "claude-3-sonnet-20240229":
		inputCostPer1K = 0.003  // $3 per MTok
		outputCostPer1K = 0.015 // $15 per MTok
	case "claude-3-haiku-20240307":
		inputCostPer1K = 0.00025  // $0.25 per MTok
		outputCostPer1K = 0.00125 // $1.25 per MTok
	default:
		// Default to Claude-3.5 Sonnet pricing
		inputCostPer1K = 0.003
		outputCostPer1K = 0.015
	}

	inputCost := float64(usage.InputTokens) * inputCostPer1K / 1000
	outputCost := float64(usage.OutputTokens) * outputCostPer1K / 1000

	return inputCost + outputCost
}

// getSupportedAnthropicModels returns the list of supported Anthropic models
func getSupportedAnthropicModels() []string {
	return []string{
		"claude-3-5-sonnet-20241022",
		"claude-3-5-sonnet-20240620",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}
}

// GetAnthropicAPIKeyFromEnv gets the API key from environment variables
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
