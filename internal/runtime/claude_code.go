package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ClaudeCodeProvider implements the ModelProvider interface using Claude Code CLI
type ClaudeCodeProvider struct {
	session                 *ClaudeCodeSession
	name                    string
	executablePath          string
	workingDir              string
	config                  *ClaudeCodeConfig
	defaultProgressCallback ProgressCallback
}

// ClaudeCodeConfig contains configuration for Claude Code provider
type ClaudeCodeConfig struct {
	ExecutablePath   string           `yaml:"executable_path"`
	WorkingDirectory string           `yaml:"working_directory"`
	SessionTimeout   time.Duration    `yaml:"session_timeout"`
	MaxSessions      int              `yaml:"max_sessions"`
	Model            string           `yaml:"model"`
	EnableTools      bool             `yaml:"enable_tools"`
	LogLevel         string           `yaml:"log_level"`
	EnableStreaming  bool             `yaml:"enable_streaming"`
	ShowProgress     bool             `yaml:"show_progress"`
	ProgressCallback ProgressCallback `yaml:"-"` // Custom progress callback, not serializable
}

// ClaudeCodeResponse represents a response from Claude Code
type ClaudeCodeResponse struct {
	Content   string                 `json:"content"`
	ToolUses  []ClaudeCodeToolUse    `json:"tool_uses,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Error     string                 `json:"error,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
}

// ClaudeCodeToolUse represents a tool usage in Claude Code
type ClaudeCodeToolUse struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
	Result     string                 `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// StreamMessage represents a message in the streaming JSON output
type StreamMessage struct {
	Type            string            `json:"type"`
	Subtype         string            `json:"subtype,omitempty"`
	Message         *AssistantMessage `json:"message,omitempty"`
	SessionID       string            `json:"session_id,omitempty"`
	ParentToolUseID string            `json:"parent_tool_use_id,omitempty"`
	IsError         bool              `json:"is_error,omitempty"`
	Result          string            `json:"result,omitempty"`
	DurationMS      int               `json:"duration_ms,omitempty"`
	DurationAPIMS   int               `json:"duration_api_ms,omitempty"`
	NumTurns        int               `json:"num_turns,omitempty"`
	TotalCostUSD    float64           `json:"total_cost_usd,omitempty"`
	Usage           *Usage            `json:"usage,omitempty"`
	CWD             string            `json:"cwd,omitempty"`
	Tools           []string          `json:"tools,omitempty"`
	Model           string            `json:"model,omitempty"`
	PermissionMode  string            `json:"permissionMode,omitempty"`
	APIKeySource    string            `json:"apiKeySource,omitempty"`
	MCPServers      []interface{}     `json:"mcp_servers,omitempty"`
}

// AssistantMessage represents the message content in assistant responses
type AssistantMessage struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Model        string         `json:"model"`
	Content      []ContentBlock `json:"content"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence"`
	Usage        *Usage         `json:"usage"`
}

// ContentBlock represents a content block in the message
type ContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Content   string                 `json:"content,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
}

// Usage represents token usage information
type Usage struct {
	InputTokens              int                    `json:"input_tokens"`
	CacheCreationInputTokens int                    `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int                    `json:"cache_read_input_tokens,omitempty"`
	OutputTokens             int                    `json:"output_tokens"`
	ServiceTier              string                 `json:"service_tier,omitempty"`
	ServerToolUse            map[string]interface{} `json:"server_tool_use,omitempty"`
}

// ProgressCallback is a function that receives progress updates during streaming
type ProgressCallback func(status string, details map[string]interface{})

// StreamingOptions contains options for streaming responses
type StreamingOptions struct {
	EnableStreaming  bool
	ProgressCallback ProgressCallback
	ShowToolUse      bool
	ShowThinking     bool
}

// Default configuration for Claude Code
func DefaultClaudeCodeConfig() *ClaudeCodeConfig {
	return &ClaudeCodeConfig{
		ExecutablePath:   "claude", // Assumes claude is in PATH
		WorkingDirectory: "",       // Use current directory
		SessionTimeout:   30 * time.Minute,
		MaxSessions:      5,
		Model:            "sonnet",
		EnableTools:      true,
		LogLevel:         "info",
		EnableStreaming:  true, // Enable streaming by default
		ShowProgress:     true, // Show progress by default
	}
}

// NewClaudeCodeProvider creates a new Claude Code model provider
func NewClaudeCodeProvider(config *ClaudeCodeConfig) (*ClaudeCodeProvider, error) {
	if config == nil {
		config = DefaultClaudeCodeConfig()
	}

	// Detect Claude Code executable
	execPath, err := detectClaudeCodeExecutable(config.ExecutablePath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect Claude Code executable: %w", err)
	}

	// Set working directory
	workingDir := config.WorkingDirectory
	if workingDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workingDir = wd
		} else {
			workingDir = "."
		}
	}

	// Set up default progress callback
	var defaultProgressCallback ProgressCallback
	if config.ProgressCallback != nil {
		// Use custom callback if provided
		defaultProgressCallback = config.ProgressCallback
	} else if config.ShowProgress {
		// Use default progress callback that logs to console
		defaultProgressCallback = func(status string, details map[string]interface{}) {
			log.Info().
				Str("status", status).
				Interface("details", details).
				Msg("Claude Code progress")
		}
	}
	// If ShowProgress is false and no custom callback, defaultProgressCallback will be nil

	provider := &ClaudeCodeProvider{
		name:                    "local",
		executablePath:          execPath,
		workingDir:              workingDir,
		config:                  config,
		defaultProgressCallback: defaultProgressCallback,
	}

	log.Info().
		Str("executable", execPath).
		Str("working_dir", workingDir).
		Str("model", config.Model).
		Msg("Claude Code provider initialized")

	return provider, nil
}

// NewClaudeCodeProviderWithCallback creates a new Claude Code provider with a custom progress callback
func NewClaudeCodeProviderWithCallback(config *ClaudeCodeConfig, progressCallback ProgressCallback) (*ClaudeCodeProvider, error) {
	if config == nil {
		config = DefaultClaudeCodeConfig()
	}

	// Set the custom progress callback
	config.ProgressCallback = progressCallback

	return NewClaudeCodeProvider(config)
}

// Generate generates a response using Claude Code with streaming enabled by default
func (p *ClaudeCodeProvider) Generate(ctx context.Context, request *ModelRequest) (string, *TokenUsage, error) {
	// Use streaming with default progress callback if enabled
	options := &StreamingOptions{
		EnableStreaming:  p.config.EnableStreaming,
		ProgressCallback: p.defaultProgressCallback,
		ShowToolUse:      true,
		ShowThinking:     true,
	}
	return p.GenerateWithOptions(ctx, request, options)
}

// GenerateWithOptions generates a response using Claude Code with streaming options
func (p *ClaudeCodeProvider) GenerateWithOptions(ctx context.Context, request *ModelRequest, options *StreamingOptions) (string, *TokenUsage, error) {
	// Send request to Claude Code with streaming support
	response, err := p.sendRequestWithOptions(ctx, request, options)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send request to Claude Code: %w", err)
	}

	// Parse response and extract content
	content := response.Content
	if response.Error != "" {
		return "", nil, fmt.Errorf("Claude Code error: %s", response.Error)
	}

	// Extract token usage from response metadata if available
	tokenUsage := &TokenUsage{
		PromptTokens:     estimateTokens(request.Prompt + request.SystemPrompt),
		CompletionTokens: estimateTokens(content),
		TotalTokens:      estimateTokens(request.Prompt + request.SystemPrompt + content),
		EstimatedCost:    0.0, // Claude Code is free
	}

	return content, tokenUsage, nil
}

func (p *ClaudeCodeProvider) Close() error {
	if p.session != nil {
		p.session.Stderr.Close()
		p.session.Stdout.Close()
		p.session.Stdin.Close()
		p.session.Process.Process.Kill()
		p.session = nil
	}

	return nil
}

// GenerateWithProgress generates a response using Claude Code with progress callbacks
func (p *ClaudeCodeProvider) GenerateWithProgress(ctx context.Context, request *ModelRequest, progressCallback ProgressCallback) (string, *TokenUsage, error) {
	options := &StreamingOptions{
		EnableStreaming:  true,
		ProgressCallback: progressCallback,
		ShowToolUse:      true,
		ShowThinking:     true,
	}
	return p.GenerateWithOptions(ctx, request, options)
}

// GetName returns the provider name
func (p *ClaudeCodeProvider) GetName() string {
	return p.name
}

// ListModels returns the Claude Code model (single model for local provider)
func (p *ClaudeCodeProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Claude Code provider only supports the "claude-code" model
	models := []ModelInfo{
		{
			ID:          "claude-code",
			Name:        "Claude Code",
			Provider:    p.name,
			CreatedAt:   "",
			Deprecated:  false,
			Description: "Claude Code CLI interface for local development",
			Features:    []string{"text-generation", "chat", "code-analysis", "tool-use"},
		},
	}

	log.Debug().
		Int("model_count", len(models)).
		Str("provider", p.name).
		Msg("Returning Claude Code model")

	return models, nil
}

type ClaudeCodeSession struct {
	Process      *exec.Cmd
	Stdin        io.WriteCloser
	Stdout       io.ReadCloser
	Stderr       io.ReadCloser
	Scanner      *bufio.Scanner
	ErrorScanner *bufio.Scanner
	WorkingDir   string
}

// createSessionUnsafe creates a new Claude Code session (caller must hold lock)
func (p *ClaudeCodeProvider) execute(ctx context.Context, request *ModelRequest) (*ClaudeCodeSession, error) {
	prompt := request.Prompt
	if request.SystemPrompt != "" {
		prompt = fmt.Sprintf("System: %s\n\nUser: %s", request.SystemPrompt, request.Prompt)
	}
	args := []string{
		"--p",
		"--verbose",
		"--output-format", "stream-json",
		"--model", p.config.Model,
		prompt,
	}

	// Create command
	cmd := exec.CommandContext(ctx, p.executablePath, args...)
	cmd.Dir = p.workingDir

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start Claude Code process: %w", err)
	}

	session := &ClaudeCodeSession{
		Process:      cmd,
		Stdin:        stdin,
		Stdout:       stdout,
		Stderr:       stderr,
		Scanner:      bufio.NewScanner(stdout),
		ErrorScanner: bufio.NewScanner(stderr),
		WorkingDir:   p.workingDir,
	}
	p.session = session
	return session, nil
}

// sendRequestWithOptions sends a request to Claude Code session with streaming options
func (p *ClaudeCodeProvider) sendRequestWithOptions(ctx context.Context, request *ModelRequest, options *StreamingOptions) (*ClaudeCodeResponse, error) {
	session, err := p.execute(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Claude Code: %w", err)
	}

	// Read response with timeout and optional progress callback
	responseChan := make(chan *ClaudeCodeResponse, 1)
	errorChan := make(chan error, 1)

	go func() {
		var progressCallback ProgressCallback
		if options != nil && options.EnableStreaming && options.ProgressCallback != nil {
			progressCallback = options.ProgressCallback
		}

		response, err := p.readStreamingResponse(session, progressCallback)
		if err != nil {
			errorChan <- err
		} else {
			responseChan <- response
		}
	}()

	// Wait for response or timeout
	select {
	case response := <-responseChan:
		return response, nil
	case err := <-errorChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("request timeout: %w", ctx.Err())
	}
}

// readStreamingResponse reads a streaming JSON response from Claude Code session
func (p *ClaudeCodeProvider) readStreamingResponse(session *ClaudeCodeSession, progressCallback ProgressCallback) (*ClaudeCodeResponse, error) {
	var finalResponse *ClaudeCodeResponse
	var responseContent strings.Builder
	var toolUses []ClaudeCodeToolUse

	// Read streaming JSON messages
	for session.Scanner.Scan() {
		line := strings.TrimSpace(session.Scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse JSON message
		var message StreamMessage
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			// If it's not JSON, it might be legacy output - handle gracefully
			log.Debug().Str("line", line).Msg("Non-JSON output received")
			continue
		}

		// Handle different message types
		switch message.Type {
		case "system":
			if message.Subtype == "init" && progressCallback != nil {
				progressCallback("Initializing Claude Code session", map[string]interface{}{
					"model": message.Model,
					"tools": message.Tools,
					"cwd":   message.CWD,
				})
			}

		case "assistant":
			if message.Message != nil {
				// Process assistant message content
				for _, content := range message.Message.Content {
					switch content.Type {
					case "text":
						responseContent.WriteString(content.Text)
						if progressCallback != nil {
							progressCallback("Generating response", map[string]interface{}{
								"content_preview": truncateString(content.Text, 100),
							})
						}

					case "tool_use":
						toolUse := ClaudeCodeToolUse{
							Name:       content.Name,
							Parameters: content.Input,
						}
						toolUses = append(toolUses, toolUse)

						if progressCallback != nil {
							progressCallback("Using tool", map[string]interface{}{
								"tool_name": content.Name,
								"tool_id":   content.ID,
							})
						}

					case "tool_result":
						// Update the corresponding tool use with result
						for i := range toolUses {
							if toolUses[i].Name != "" { // Find matching tool use
								toolUses[i].Result = content.Content
								break
							}
						}

						if progressCallback != nil {
							progressCallback("Tool completed", map[string]interface{}{
								"tool_result_preview": truncateString(content.Content, 100),
							})
						}
					}
				}

				// Usage information is handled in the final response metadata
			}

		case "result":
			// Final result message
			if progressCallback != nil {
				status := "Completed"
				if message.IsError {
					status = "Error"
				}
				progressCallback(status, map[string]interface{}{
					"duration_ms":     message.DurationMS,
					"duration_api_ms": message.DurationAPIMS,
					"num_turns":       message.NumTurns,
					"total_cost_usd":  message.TotalCostUSD,
				})
			}

			// Build final response
			finalResponse = &ClaudeCodeResponse{
				Content:   responseContent.String(),
				ToolUses:  toolUses,
				SessionID: message.SessionID,
				Metadata: map[string]interface{}{
					"working_dir":     session.WorkingDir,
					"session_id":      message.SessionID,
					"duration_ms":     message.DurationMS,
					"duration_api_ms": message.DurationAPIMS,
					"num_turns":       message.NumTurns,
					"total_cost_usd":  message.TotalCostUSD,
				},
			}

			if message.IsError {
				finalResponse.Error = message.Result
			}

			// We have the final result, break out of the loop
			break
		}
	}

	if err := session.Scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading from Claude Code stdout: %w", err)
	}

	if finalResponse == nil {
		return nil, fmt.Errorf("no final response received from Claude Code")
	}

	return finalResponse, nil
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Helper functions

// detectClaudeCodeExecutable detects the Claude Code executable path
func detectClaudeCodeExecutable(configPath string) (string, error) {
	// Try configured path first
	if configPath != "" {
		// If it's an absolute path, check if it exists
		if filepath.IsAbs(configPath) {
			if _, err := os.Stat(configPath); err == nil {
				return configPath, nil
			} else {
				// If user specified an absolute path that doesn't exist, fail immediately
				return "", fmt.Errorf("specified Claude Code executable not found: %s", configPath)
			}
		} else {
			// If it's a relative path or command name, look in PATH
			if path, err := exec.LookPath(configPath); err == nil {
				return path, nil
			} else {
				// If user specified a command that doesn't exist, fail immediately
				return "", fmt.Errorf("specified Claude Code executable not found in PATH: %s", configPath)
			}
		}
	}

	// Try common names
	candidates := []string{"claude", "claude-code", "claude_code"}

	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}

	// Try common installation paths
	commonPaths := []string{
		"/usr/local/bin/claude",
		"/opt/claude/bin/claude",
		filepath.Join(os.Getenv("HOME"), ".local/bin/claude"),
		filepath.Join(os.Getenv("HOME"), "bin/claude"),
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("Claude Code executable not found. Please install Claude Code CLI or set executable_path in configuration")
}

// estimateTokens provides a rough token count estimate
func estimateTokens(text string) int {
	// Rough estimation: ~4 characters per token for English text
	return len(text) / 4
}
