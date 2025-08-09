package claudecode

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

	"github.com/lacquerai/lacquer/internal/events"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/provider"
	"github.com/lacquerai/lacquer/internal/style"
	pkgEvents "github.com/lacquerai/lacquer/pkg/events"
	"github.com/rs/zerolog/log"
)

// ClaudeCodeProvider implements the ModelProvider interface using Claude Code CLI
type ClaudeCodeProvider struct {
	session        *ClaudeCodeSession
	name           string
	executablePath string
	workingDir     string
	config         *ClaudeCodeConfig
	progressChan   chan<- pkgEvents.ExecutionEvent
}

// ClaudeCodeConfig contains configuration for Claude Code provider
type ClaudeCodeConfig struct {
	ExecutablePath             string        `yaml:"executable_path"`
	WorkingDirectory           string        `yaml:"working_directory"`
	SessionTimeout             time.Duration `yaml:"session_timeout"`
	MaxSessions                int           `yaml:"max_sessions"`
	Model                      string        `yaml:"model"`
	LogLevel                   string        `yaml:"log_level"`
	EnableStreaming            bool          `yaml:"enable_streaming"`
	DangerouslySkipPermissions bool          `yaml:"dangerously_skip_permissions"`
	WhitelistedTools           []string      `yaml:"whitelisted_tools"`
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

// StreamingOptions contains options for streaming responses
type StreamingOptions struct {
	ShowToolUse  bool
	ShowThinking bool
	ProgressChan chan<- pkgEvents.ExecutionEvent
}

// Default configuration for Claude Code
func DefaultClaudeCodeConfig() *ClaudeCodeConfig {
	return &ClaudeCodeConfig{
		ExecutablePath:             "claude", // Assumes claude is in PATH
		WorkingDirectory:           "",       // Use current directory
		SessionTimeout:             30 * time.Minute,
		MaxSessions:                5,
		Model:                      "sonnet",
		DangerouslySkipPermissions: false,
		WhitelistedTools: []string{
			"Read",
			"Write",
			"Edit",
			"Grep",
			"Bash",
		},
		LogLevel: "info",
	}
}

// NewProvider creates a new Claude Code model provider
func NewProvider(yamlConfig map[string]interface{}) (*ClaudeCodeProvider, error) {
	config := DefaultClaudeCodeConfig()
	provider.MergeConfig(config, yamlConfig)

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

	provider := &ClaudeCodeProvider{
		name:           "local",
		executablePath: execPath,
		workingDir:     workingDir,
		config:         config,
	}

	log.Info().
		Str("executable", execPath).
		Str("working_dir", workingDir).
		Str("model", config.Model).
		Msg("Claude Code provider initialized")

	return provider, nil
}

// Generate generates a response using Claude Code with streaming enabled by default
func (p *ClaudeCodeProvider) Generate(ctx provider.GenerateContext, request *provider.Request, progressChan chan<- pkgEvents.ExecutionEvent) ([]provider.Message, *execcontext.TokenUsage, error) {
	p.progressChan = progressChan

	response, err := p.sendRequestWithOptions(ctx, request)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send request to Claude Code: %w", err)
	}

	content := response.Content
	if response.Error != "" {
		return nil, nil, fmt.Errorf("claude Code error: %s", response.Error)
	}

	return []provider.Message{
		{
			Role:    "assistant",
			Content: []provider.ContentBlockParamUnion{provider.NewTextBlock(content)},
		},
	}, nil, nil
}

func (p *ClaudeCodeProvider) Close() error {
	if p.session != nil {
		_ = p.session.Stderr.Close()
		_ = p.session.Stdout.Close()
		_ = p.session.Process.Process.Kill()
		p.session = nil
	}

	return nil
}

// GetName returns the provider name
func (p *ClaudeCodeProvider) GetName() string {
	return p.name
}

// ListModels returns the Claude Code model (single model for local provider)
func (p *ClaudeCodeProvider) ListModels(ctx context.Context) ([]provider.Info, error) {
	// Claude Code provider only supports the "claude-code" model
	models := []provider.Info{
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
	Stdout       io.ReadCloser
	Stderr       io.ReadCloser
	Scanner      *bufio.Scanner
	ErrorScanner *bufio.Scanner
	WorkingDir   string
}

// createSessionUnsafe creates a new Claude Code session (caller must hold lock)
func (p *ClaudeCodeProvider) execute(ctx provider.GenerateContext, request *provider.Request) (*ClaudeCodeSession, error) {
	prompt := request.GetPrompt()

	if request.SystemPrompt != "" {
		prompt = fmt.Sprintf("System: %s\n\nUser: %s", request.SystemPrompt, prompt)
	}

	args := []string{
		"--print",
		"--verbose",
		"--output-format", "stream-json",
		"--model", p.config.Model,
		prompt,
	}

	if p.config.DangerouslySkipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}

	if len(p.config.WhitelistedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(p.config.WhitelistedTools, ","))
	}

	// Test if executable exists and is accessible
	execPath := p.executablePath
	if !filepath.IsAbs(execPath) {
		// If it's not an absolute path, try to find it in PATH
		if foundPath, err := exec.LookPath(execPath); err != nil {
			return nil, fmt.Errorf("claude Code executable '%s' not found in PATH: %w", execPath, err)
		} else {
			execPath = foundPath
			log.Debug().Str("found_path", foundPath).Msg("Located Claude Code executable")
		}
	}

	log.Debug().
		Str("executable", execPath).
		Str("working_dir", p.workingDir).
		Strs("args", args).
		Str("prompt_preview", truncateString(prompt, 100)).
		Msg("Executing Claude Code command")

	cmd := exec.CommandContext(ctx.Context, execPath, args...) // #nosec G204 - execPath is validated internally
	cmd.Dir = p.workingDir

	stdErrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	stdOutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start Claude Code process: %w", err)
	}

	session := &ClaudeCodeSession{
		Process:      cmd,
		Stdout:       stdOutPipe,
		Stderr:       stdErrPipe,
		Scanner:      bufio.NewScanner(stdOutPipe),
		ErrorScanner: bufio.NewScanner(stdErrPipe),
		WorkingDir:   p.workingDir,
	}
	p.session = session

	log.Debug().
		Str("working_dir", session.WorkingDir).
		Msg("Claude Code session created successfully")

	return session, nil
}

// sendRequestWithOptions sends a request to Claude Code session with streaming options
func (p *ClaudeCodeProvider) sendRequestWithOptions(ctx provider.GenerateContext, request *provider.Request) (*ClaudeCodeResponse, error) {
	session, err := p.execute(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Claude Code: %w", err)
	}

	response, err := p.readStreamingResponse(ctx, session)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// readStreamingResponse reads a streaming JSON response from Claude Code session
func (p *ClaudeCodeProvider) readStreamingResponse(ctx provider.GenerateContext, session *ClaudeCodeSession) (*ClaudeCodeResponse, error) {
	var finalResponse *ClaudeCodeResponse

	// Read stderr to capture any errors
	var stderrLines []string
	go func() {
		for session.ErrorScanner.Scan() {
			errLine := session.ErrorScanner.Text()
			stderrLines = append(stderrLines, errLine)
			log.Debug().Str("stderr", errLine).Msg("Claude Code stderr")
		}
	}()

	log.Debug().Msg("Starting to read streaming JSON messages")

	// Read streaming JSON messages
	lineCount := 0

	// Check if process has exited before we start reading
	select {
	case <-time.After(100 * time.Millisecond):
		// Process is still running, continue
	default:
		// Check if process has already exited
		if session.Process.ProcessState != nil && session.Process.ProcessState.Exited() {
			log.Debug().
				Int("exit_code", session.Process.ProcessState.ExitCode()).
				Msg("Claude Code process exited before reading output")
		}
	}

	log.Debug().Msg("Starting scanner loop")

	// Read all output from the process
	for session.Scanner.Scan() {
		line := strings.TrimSpace(session.Scanner.Text())
		lineCount++

		log.Debug().
			Int("line_count", lineCount).
			Str("line", line).
			Msg("Received line from Claude Code")

		if line != "" {
			if err := p.processLine(ctx, line, &finalResponse); err != nil {
				log.Debug().Err(err).Msg("Error processing line")
			}

			// If we got a result message, we can break out
			if finalResponse != nil {
				break
			}
		}
	}

	log.Debug().
		Int("total_lines", lineCount).
		Bool("has_final_response", finalResponse != nil).
		Msg("Finished reading from Claude Code")

	if err := session.Scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading from Claude Code stdout: %w", err)
	}

	// Process has already completed when using cmd.Output(), so just log the state
	if session.Process != nil && session.Process.ProcessState != nil {
		log.Debug().
			Int("exit_code", session.Process.ProcessState.ExitCode()).
			Msg("Claude Code process completed")
	}

	if finalResponse == nil {
		// Try to provide more helpful error information
		var stderrText string
		if len(stderrLines) > 0 {
			stderrText = strings.Join(stderrLines, "\n")
		}

		if stderrText != "" {
			return nil, fmt.Errorf("no final response received from Claude Code after reading %d lines - stderr: %s", lineCount, stderrText)
		} else {
			return nil, fmt.Errorf("no final response received from Claude Code after reading %d lines - process may have failed or produced no output", lineCount)
		}
	}

	log.Debug().
		Str("content_preview", truncateString(finalResponse.Content, 100)).
		Msg("Successfully parsed Claude Code response")

	return finalResponse, nil
}

// processLine processes a single line of output from Claude Code
func (p *ClaudeCodeProvider) processLine(ctx provider.GenerateContext, line string, finalResponse **ClaudeCodeResponse) error { //nolint:unparam // error is intentionally always nil
	// Parse JSON message
	var message StreamMessage
	if err := json.Unmarshal([]byte(line), &message); err != nil {
		// If it's not JSON, it might be legacy output - handle gracefully
		log.Debug().
			Str("line", line).
			Err(err).
			Msg("Non-JSON output received - could be plain text response")

		// If we can't parse as JSON, treat as plain text response
		if *finalResponse == nil {
			*finalResponse = &ClaudeCodeResponse{
				Content: line,
				Metadata: map[string]interface{}{
					"response_type": "plain_text",
				},
			}
		}
		return nil
	}

	// Handle different message types
	switch message.Type {
	case "system":
		if message.Subtype == "init" {
			p.progressChan <- events.NewGenericActionEvent(ctx.StepID, "system", ctx.RunID, "Booting up...")
		}
	case "assistant":
		if message.Message != nil {
			// Process assistant message content
			for _, content := range message.Message.Content {
				switch content.Type {
				case "tool_use":
					var sb strings.Builder

					switch content.Name {
					case "TodoWrite":
						sb.WriteString("Updating todo list...")
					default:
						sb.WriteString(fmt.Sprintf("Using tool %s ", style.InfoStyle.Render(content.Name)))
						if len(content.Input) > 0 {
							sb.WriteString("(")
							var i int
							for key, value := range content.Input {
								sb.WriteString(fmt.Sprintf("%s: %v", style.MutedStyle.Render(key), style.MutedStyle.Render(fmt.Sprintf("%v", value))))
								if i != len(content.Input)-1 {
									sb.WriteString("; ")
								}
								i++
							}

							sb.WriteString(")")
						}
					}

					p.progressChan <- events.NewToolUseEvent(ctx.StepID, content.ID, content.Name, ctx.RunID, sb.String())
				case "tool_result":
					p.progressChan <- events.NewToolUseCompletedEvent(ctx.StepID, content.ID, content.Name, ctx.RunID)
				default:
					p.progressChan <- events.NewGenericActionEvent(ctx.StepID, content.ID, ctx.RunID, content.Text)
				}
			}
		}

	case "result":
		// Build final response
		*finalResponse = &ClaudeCodeResponse{
			Content:   message.Result,
			SessionID: message.SessionID,
			Metadata: map[string]interface{}{
				"session_id":      message.SessionID,
				"duration_ms":     message.DurationMS,
				"duration_api_ms": message.DurationAPIMS,
				"num_turns":       message.NumTurns,
				"total_cost_usd":  message.TotalCostUSD,
			},
		}

		if message.IsError {
			(*finalResponse).Error = message.Result
		}
	}

	return nil
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

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

	return "", fmt.Errorf("claude Code executable not found. Please install Claude Code CLI or set executable_path in configuration")
}
