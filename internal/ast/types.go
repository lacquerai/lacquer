package ast

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lacquerai/lacquer/internal/schema"
	"gopkg.in/yaml.v3"
)

// Position represents a position in a source file
type Position struct {
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Offset int    `json:"offset"`
	File   string `json:"file,omitempty"`
}

// String returns a human-readable representation of the position
func (p Position) String() string {
	if p.File != "" {
		return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Column)
	}
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// ExtractPosition extracts position information from YAML parsing errors
func ExtractPosition(source []byte, offset int) Position {
	lines := strings.Split(string(source), "\n")

	currentOffset := 0
	for lineNum, line := range lines {
		lineLength := len(line) + 1 // +1 for newline character
		if currentOffset+lineLength > offset {
			column := offset - currentOffset + 1
			return Position{
				Line:   lineNum + 1, // 1-indexed
				Column: column,
				Offset: offset,
			}
		}
		currentOffset += lineLength
	}

	// Fallback if position is at end of file
	return Position{
		Line:   len(lines),
		Column: len(lines[len(lines)-1]) + 1,
		Offset: offset,
	}
}

// ExtractContext extracts contextual lines around a position for error reporting
func ExtractContext(source []byte, position Position, contextLines int) string {
	lines := strings.Split(string(source), "\n")

	if position.Line <= 0 || position.Line > len(lines) {
		return ""
	}

	start := max(0, position.Line-contextLines-1)
	end := min(len(lines), position.Line+contextLines)

	var context strings.Builder
	for i := start; i < end; i++ {
		lineNum := i + 1
		prefix := "   "
		if lineNum == position.Line {
			prefix = ">> "
		}

		context.WriteString(fmt.Sprintf("%s%4d | %s\n", prefix, lineNum, lines[i]))

		// Add a pointer to the specific column for the error line
		if lineNum == position.Line && position.Column > 0 {
			pointer := strings.Repeat(" ", 8+min(position.Column-1, len(lines[i]))) + "^"
			context.WriteString(pointer + "\n")
		}
	}

	return context.String()
}

// Helper functions for min/max
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Workflow represents the root of a Lacquer workflow
type Workflow struct {
	Version      string            `yaml:"version" json:"version" validate:"required,eq=1.0"`
	Metadata     *WorkflowMetadata `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Agents       map[string]*Agent `yaml:"agents,omitempty" json:"agents,omitempty"`
	Requirements *Requirements     `yaml:"requirements,omitempty" json:"requirements,omitempty"`
	Workflow     *WorkflowDef      `yaml:"workflow" json:"workflow" validate:"required"`

	// Internal fields for tracking
	SourceFile string   `yaml:"-" json:"-"`
	Position   Position `yaml:"-" json:"-"`
}

// Requirements represents the requirements for the workflow
type Requirements struct {
	Runtimes []Runtime `yaml:"runtimes" json:"runtimes" validate:"required"`

	Position Position `yaml:"-" json:"-"`
}

// RuntimeType represents a runtime type, e.g. "node" or "go"
type RuntimeType string

var (
	RuntimeTypeNode RuntimeType = "node"
	RuntimeTypeGo   RuntimeType = "go"
)

// Runtime represents a runtime requirement, e.g. "node" or "go"
type Runtime struct {
	Name    RuntimeType `yaml:"name" json:"name" validate:"required"`
	Version string      `yaml:"version,omitempty" json:"version,omitempty"`
}

// WorkflowMetadata contains descriptive information about the workflow
type WorkflowMetadata struct {
	Name        string   `yaml:"name" json:"name" validate:"required"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Author      string   `yaml:"author,omitempty" json:"author,omitempty"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Version     string   `yaml:"version,omitempty" json:"version,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// Agent represents an AI agent configuration
type Agent struct {
	Name         string                 `yaml:"-" json:"name,omitempty"`
	Provider     string                 `yaml:"provider,omitempty" json:"provider,omitempty" validate:"omitempty,oneof=anthropic openai local"`
	Model        string                 `yaml:"model,omitempty" json:"model,omitempty"`
	Temperature  *float64               `yaml:"temperature,omitempty" json:"temperature,omitempty" validate:"omitempty,min=0,max=2"`
	SystemPrompt string                 `yaml:"system_prompt,omitempty" json:"system_prompt,omitempty"`
	MaxTokens    *int                   `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty" validate:"omitempty,min=1"`
	TopP         *float64               `yaml:"top_p,omitempty" json:"top_p,omitempty" validate:"omitempty,min=0,max=1"`
	Tools        []*Tool                `yaml:"tools,omitempty" json:"tools,omitempty"`
	Uses         string                 `yaml:"uses,omitempty" json:"uses,omitempty"`
	With         map[string]interface{} `yaml:"with,omitempty" json:"with,omitempty"`
	Policies     *AgentPolicies         `yaml:"policies,omitempty" json:"policies,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// AgentPolicies defines policies for agent behavior
type AgentPolicies struct {
	MaxRetries           *int      `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	Timeout              *Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	RequireHumanApproval bool      `yaml:"require_human_approval,omitempty" json:"require_human_approval,omitempty"`
	CostLimit            string    `yaml:"cost_limit,omitempty" json:"cost_limit,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

type ToolType string

var (
	ToolTypeScript   ToolType = "script"
	ToolTypeWorkflow ToolType = "workflow"
	ToolTypeMCP      ToolType = "mcp"
	ToolTypeOfficial ToolType = "official"
)

// Tool represents a tool available to an agent
type Tool struct {
	Name        string `yaml:"name" json:"name" validate:"required"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	Uses       string                 `yaml:"uses,omitempty" json:"uses,omitempty"`
	Script     string                 `yaml:"script,omitempty" json:"script,omitempty"`
	Runtime    string                 `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Version    string                 `yaml:"version,omitempty" json:"version,omitempty"`
	Parameters schema.JSON            `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	MCPServer  *MCPServerConfig       `yaml:"mcp_server,omitempty" json:"mcp_server,omitempty"`
	Config     map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// MCPServerConfig represents the configuration for an MCP server
type MCPServerConfig struct {
	Type    string                 `yaml:"type,omitempty" json:"type,omitempty" validate:"omitempty,oneof=local remote"`
	URL     string                 `yaml:"url,omitempty" json:"url,omitempty"`         // For remote servers
	Command string                 `yaml:"command,omitempty" json:"command,omitempty"` // For local servers
	Args    []string               `yaml:"args,omitempty" json:"args,omitempty"`       // For local servers
	Env     map[string]string      `yaml:"env,omitempty" json:"env,omitempty"`         // Environment variables
	Auth    *MCPAuthConfig         `yaml:"auth,omitempty" json:"auth,omitempty"`       // Authentication config
	Timeout *Duration              `yaml:"timeout,omitempty" json:"timeout,omitempty"` // Connection timeout
	Options map[string]interface{} `yaml:"options,omitempty" json:"options,omitempty"` // Additional server-specific options

	Position Position `yaml:"-" json:"-"`
}

// MCPAuthConfig represents authentication configuration for MCP servers
type MCPAuthConfig struct {
	Type         string `yaml:"type" json:"type" validate:"required,oneof=oauth2 api_key basic none"`
	ClientID     string `yaml:"client_id,omitempty" json:"client_id,omitempty"`         // OAuth2
	ClientSecret string `yaml:"client_secret,omitempty" json:"client_secret,omitempty"` // OAuth2
	TokenURL     string `yaml:"token_url,omitempty" json:"token_url,omitempty"`         // OAuth2
	Scopes       string `yaml:"scopes,omitempty" json:"scopes,omitempty"`               // OAuth2
	APIKey       string `yaml:"api_key,omitempty" json:"api_key,omitempty"`             // API Key auth
	Username     string `yaml:"username,omitempty" json:"username,omitempty"`           // Basic auth
	Password     string `yaml:"password,omitempty" json:"password,omitempty"`           // Basic auth

	Position Position `yaml:"-" json:"-"`
}

func (t Tool) Type() ToolType {
	if t.IsScript() {
		return ToolTypeScript
	}

	if t.IsMCPTool() {
		return ToolTypeMCP
	}

	if t.IsOfficialTool() {
		return ToolTypeOfficial
	}

	return ToolTypeWorkflow
}

// WorkflowDef contains the main workflow definition
type WorkflowDef struct {
	Inputs  map[string]*InputParam `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	State   map[string]interface{} `yaml:"state,omitempty" json:"state,omitempty"`
	Steps   []*Step                `yaml:"steps" json:"steps" validate:"required,min=1"`
	Outputs map[string]interface{} `yaml:"outputs,omitempty" json:"outputs,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// OutputSchema defines the schema for a block output parameter
type OutputSchema struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description,omitempty"`
}

// InputParam defines an input parameter for the workflow
type InputParam struct {
	Type        string      `yaml:"type,omitempty" json:"type,omitempty"`
	Description string      `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool        `yaml:"required,omitempty" json:"required,omitempty"`
	Default     interface{} `yaml:"default,omitempty" json:"default,omitempty"`
	Pattern     string      `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	Minimum     *float64    `yaml:"minimum,omitempty" json:"minimum,omitempty"`
	Maximum     *float64    `yaml:"maximum,omitempty" json:"maximum,omitempty"`
	MinItems    *int        `yaml:"min_items,omitempty" json:"min_items,omitempty"`
	MaxItems    *int        `yaml:"max_items,omitempty" json:"max_items,omitempty"`
	Enum        []string    `yaml:"enum,omitempty" json:"enum,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// UnmarshalYAML implements custom unmarshaling for InputParam to handle shorthand syntax
func (ip *InputParam) UnmarshalYAML(value *yaml.Node) error {
	// Handle shorthand syntax like "topic: string"
	if value.Kind == yaml.ScalarNode {
		ip.Type = value.Value
		ip.Required = true
		return nil
	}

	// Handle full object syntax
	type inputParamAlias InputParam
	var temp inputParamAlias
	if err := value.Decode(&temp); err != nil {
		return err
	}

	*ip = InputParam(temp)
	return nil
}

// Step represents a workflow execution step
type Step struct {
	ID        string                 `yaml:"id" json:"id" validate:"required"`
	Agent     string                 `yaml:"agent,omitempty" json:"agent,omitempty"`
	Prompt    string                 `yaml:"prompt,omitempty" json:"prompt,omitempty"`
	Uses      string                 `yaml:"uses,omitempty" json:"uses,omitempty"`
	Run       string                 `yaml:"run,omitempty" json:"run,omitempty"`
	Container string                 `yaml:"container,omitempty" json:"container,omitempty"`
	With      map[string]interface{} `yaml:"with,omitempty" json:"with,omitempty"`
	Action    string                 `yaml:"action,omitempty" json:"action,omitempty"`
	Updates   map[string]interface{} `yaml:"updates,omitempty" json:"updates,omitempty"`
	Condition string                 `yaml:"condition,omitempty" json:"condition,omitempty"`
	SkipIf    string                 `yaml:"skip_if,omitempty" json:"skip_if,omitempty"`
	Outputs   map[string]schema.JSON `yaml:"outputs,omitempty" json:"outputs,omitempty"`
	Timeout   *Duration              `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Retry     *RetryConfig           `yaml:"retry,omitempty" json:"retry,omitempty"`
	OnError   []*ErrorHandler        `yaml:"on_error,omitempty" json:"on_error,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// RetryConfig defines retry behavior for steps
type RetryConfig struct {
	MaxAttempts  int       `yaml:"max_attempts" json:"max_attempts" validate:"min=1"`
	Backoff      string    `yaml:"backoff,omitempty" json:"backoff,omitempty" validate:"omitempty,oneof=linear exponential"`
	InitialDelay *Duration `yaml:"initial_delay,omitempty" json:"initial_delay,omitempty"`
	MaxDelay     *Duration `yaml:"max_delay,omitempty" json:"max_delay,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// ErrorHandler defines error handling strategies
type ErrorHandler struct {
	Log      string                 `yaml:"log,omitempty" json:"log,omitempty"`
	Fallback string                 `yaml:"fallback,omitempty" json:"fallback,omitempty"`
	Output   map[string]interface{} `yaml:"output,omitempty" json:"output,omitempty"`
	Return   map[string]interface{} `yaml:"return,omitempty" json:"return,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// Duration wraps time.Duration with custom YAML/JSON marshaling
type Duration struct {
	time.Duration
}

// UnmarshalYAML implements custom unmarshaling for Duration
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}

	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration format '%s': %w", s, err)
	}

	d.Duration = dur
	return nil
}

// MarshalYAML implements custom marshaling for Duration
func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// UnmarshalJSON implements custom unmarshaling for Duration from JSON
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration format '%s': %w", s, err)
	}

	d.Duration = dur
	return nil
}

// MarshalJSON implements custom marshaling for Duration to JSON
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

// String returns the string representation of the Duration
func (d Duration) String() string {
	return d.Duration.String()
}
