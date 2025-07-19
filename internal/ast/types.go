package ast

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/lacquerai/lacquer/internal/schema"
	"gopkg.in/yaml.v3"
)

// Position represents a position in a source file for error reporting and debugging
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

// Workflow represents the root structure of a Lacquer workflow file.
// This is the top-level configuration that defines how AI agents, scripts,
// and other components work together to accomplish tasks.
type Workflow struct {
	// Version specifies the schema version of the workflow file. Currently must be "1.0".
	Version string `yaml:"version" json:"version" jsonschema:"required"`
	// Inputs defines the dynamic inputs that can be used within the workflow.
	// These inputs built before anything else and can be used anywhere across the workflow
	// file, including in prompts, conditions, and outputs, agents, steps, e.t.c.
	Inputs map[string]*InputParam `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	// Metadata contains descriptive information about the workflow such as name, description, and author.
	Metadata *WorkflowMetadata `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	// Agents defines AI agents that can be referenced in workflow steps.
	// Each agent has a unique name and configuration.
	Agents map[string]*Agent `yaml:"agents,omitempty" json:"agents,omitempty""`
	// Requirements specifies the runtime programs needed to execute this workflow.
	// These will requirements will be installed on the machine running the workflow.
	Requirements *Requirements `yaml:"requirements,omitempty" json:"requirements,omitempty"`
	// Workflow contains the main workflow definition including inputs, steps, and outputs.
	Workflow *WorkflowDef `yaml:"workflow" json:"workflow" validate:"required"`

	// Internal fields for tracking
	SourceFile string   `yaml:"-" json:"-"`
	Position   Position `yaml:"-" json:"-"`
}

// Requirements specifies the runtime environments and dependencies needed to execute the workflow
type Requirements struct {
	// Runtimes lists the required runtime environments (e.g., node, python, go)
	// with optional version constraints
	Runtimes []Runtime `yaml:"runtimes" json:"runtimes" jsonschema:"required"`

	Position Position `yaml:"-" json:"-"`
}

// RuntimeType represents supported runtime environments for executing scripts and tools
type RuntimeType string

var (
	// RuntimeTypeNode represents Node.js runtime for JavaScript execution
	RuntimeTypeNode RuntimeType = "node"
	// RuntimeTypeGo represents Go runtime for Go script execution
	RuntimeTypeGo RuntimeType = "go"
	// RuntimeTypePython represents Python runtime for Python script execution
	RuntimeTypePython RuntimeType = "python"
)

// Runtime specifies a required runtime environment with an optional version constraint
type Runtime struct {
	// Name specifies the runtime.
	Name RuntimeType `yaml:"name" json:"name" jsonschema:"enum=node,enum=go,enum=python,default=go"`
	// Version optionally specifies a version constraint for the runtime (e.g., ">=18.0.0", "3.9")
	// Leave empty to install the latest version.
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// WorkflowMetadata contains descriptive information about the workflow for documentation and discovery
type WorkflowMetadata struct {
	// Name is a human-readable name for the workflow
	Name string `yaml:"name" json:"name" validate:"required"`
	// Description provides a detailed explanation of what the workflow does and its purpose
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Author identifies who created or maintains this workflow
	Author string `yaml:"author,omitempty" json:"author,omitempty"`
	// Tags are keywords that help categorize and search for workflows
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	// Version tracks the version of this specific workflow
	Version string `yaml:"version,omitempty" json:"version,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// Agent represents an AI agent configuration that can be used in workflow steps to perform tasks requiring intelligence
type Agent struct {
	// Name is the identifier for this agent (used internally, not in schema)
	Name string `yaml:"-" json:"name,omitempty" jsonschema:"-"`
	// Provider specifies the AI service provider
	Provider string `yaml:"provider,omitempty" json:"provider,omitempty" jsonschema:"enum=anthropic,enum=openai,enum=local"`
	// Model specifies the specific AI model to use.
	Model string `yaml:"model,omitempty" json:"model,omitempty"`
	// Temperature controls randomness in AI responses (0.0 = deterministic, 1.0 = very creative)
	Temperature *float64 `yaml:"temperature,omitempty" json:"temperature,omitempty" validate:"omitempty,min=0,max=2"`
	// SystemPrompt provides instructions that define the agent's role and behavior
	SystemPrompt string `yaml:"system_prompt,omitempty" json:"system_prompt,omitempty"`
	// MaxTokens limits the maximum number of tokens the agent can generate in a single response
	MaxTokens *int `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty" validate:"omitempty,min=1"`
	// TopP controls nucleus sampling for response generation (0.0 to 1.0)
	TopP *float64 `yaml:"top_p,omitempty" json:"top_p,omitempty" validate:"omitempty,min=0,max=1"`
	// Tools defines the tools and capabilities available to this agent
	Tools []*Tool `yaml:"tools,omitempty" json:"tools,omitempty"`
	// Uses references a predefined agent configuration or template
	Uses string `yaml:"uses,omitempty" json:"uses,omitempty"`
	// With provides additional configuration parameters for the referenced agent
	With map[string]interface{} `yaml:"with,omitempty" json:"with,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// ToolType represents the different categories of tools available to agents
type ToolType string

var (
	// ToolTypeScript represents custom script tools that execute code
	ToolTypeScript ToolType = "script"
	// ToolTypeWorkflow represents tools that invoke other workflows
	ToolTypeWorkflow ToolType = "workflow"
	// ToolTypeMCP represents tools provided by Model Context Protocol servers
	ToolTypeMCP ToolType = "mcp"
	// ToolTypeOfficial represents pre-built tools provided by the platform
	ToolTypeOfficial ToolType = "official"
)

// Tool represents a capability or function that an agent can use to perform specific tasks
type Tool struct {
	// Name is the unique identifier for this tool within the agent
	Name string `yaml:"name" json:"name" jsonschema:"required"`
	// Description explains to the agent what this tool does and when to use it.
	// This is used to help the agent understand the tool and its capabilities.
	// Be as specific as possible when describing the tool.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Uses references a predefined tool or workflow by name or path.
	Uses string `yaml:"uses,omitempty" json:"uses,omitempty"`
	// Script contains the bash script to execute when this tool is called.
	// If you need a access to a specific runtime language (e.g. node, python, go), define it in the requirements section.
	// Break complex logic into external script calls to keep complex bash to a minimum.
	// e.g. "go run ./script.go" or "node ./script.js"
	Script string `yaml:"script,omitempty" json:"script,omitempty"`
	// Parameters defines the input schema for this tool using JSON Schema format
	Parameters schema.JSON `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	// MCPServer configures connection to a Model Context Protocol server
	MCPServer *MCPServerConfig `yaml:"mcp_server,omitempty" json:"mcp_server,omitempty"`
	// Config provides additional tool-specific configuration options
	Config map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// MCPServerConfig represents the configuration for connecting to a Model Context Protocol server
type MCPServerConfig struct {
	// Type specifies whether this is a local process or remote server
	Type string `yaml:"type,omitempty" json:"type,omitempty" jsonschema:"enum=local,enum=remote,default=local"`
	// URL is the endpoint for remote MCP servers (e.g., "wss://example.com/mcp")
	URL string `yaml:"url,omitempty" json:"url,omitempty" jsonschema:"oneof_required=url"`
	// Command is the executable path for local MCP servers
	Command string `yaml:"command,omitempty" json:"command,omitempty" jsonschema:"oneof_required=command"`
	// Args are command-line arguments to pass to local MCP servers
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`
	// Env defines environment variables to set for local MCP servers
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	// Auth configures authentication for connecting to the MCP server
	Auth *MCPAuthConfig `yaml:"auth,omitempty" json:"auth,omitempty"`
	// Timeout specifies the maximum time to wait for server connections
	Timeout *Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	// Options provides additional server-specific configuration parameters
	Options map[string]interface{} `yaml:"options,omitempty" json:"options,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// MCPAuthConfig represents authentication configuration for connecting to MCP servers
type MCPAuthConfig struct {
	// Type specifies the authentication method (oauth2, api_key, basic, or none)
	Type string `yaml:"type" json:"type" validate:"required,oneof=oauth2 api_key basic none"`
	// ClientID is the OAuth2 client identifier for oauth2 authentication
	ClientID string `yaml:"client_id,omitempty" json:"client_id,omitempty"`
	// ClientSecret is the OAuth2 client secret for oauth2 authentication
	ClientSecret string `yaml:"client_secret,omitempty" json:"client_secret,omitempty"`
	// TokenURL is the OAuth2 token endpoint for oauth2 authentication
	TokenURL string `yaml:"token_url,omitempty" json:"token_url,omitempty"`
	// Scopes specifies the requested OAuth2 permissions for oauth2 authentication
	Scopes string `yaml:"scopes,omitempty" json:"scopes,omitempty"`
	// APIKey is the authentication key for api_key authentication
	APIKey string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	// Username is the user identifier for basic authentication
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	// Password is the user credential for basic authentication
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

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

// WorkflowDef contains the main workflow definition including inputs, state, execution steps, and outputs
type WorkflowDef struct {
	// State defines variables that persist throughout the workflow execution and can be modified by steps
	State map[string]interface{} `yaml:"state,omitempty" json:"state,omitempty"`
	// Steps defines the sequence of actions to execute, including AI agent interactions,
	// scripts, and integrations.
	Steps []*Step `yaml:"steps" json:"steps" jsonschema:"required,minLength=1"`
	// Outputs defines the values that will be returned when the workflow completes
	Outputs map[string]interface{} `yaml:"outputs,omitempty" json:"outputs,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

// OutputSchema defines the expected structure and type for a workflow or block output parameter
type OutputSchema struct {
	// Type specifies the data type of the output (string, number, boolean, object, array)
	Type string `yaml:"type"`
	// Description explains what this output represents and when it's available
	Description string `yaml:"description,omitempty"`
}

// InputParam defines an input parameter that can be passed to a workflow with validation and constraints
type InputParam struct {
	// Type specifies the expected data type (string, integer, boolean, object, array)
	Type string `yaml:"type,omitempty" json:"type,omitempty" jsonschema:"required,enum=string,enum=integer,enum=boolean,enum=object,enum=array"`
	// Description explains what this input is used for and provides usage guidance
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Required indicates whether this input must be provided when the workflow starts.
	// If not provided, the workflow will fail.
	Required bool `yaml:"required,omitempty" json:"required,omitempty" jsonschema:"default=false"`
	// Default provides a fallback value when this input is not specified
	Default interface{} `yaml:"default,omitempty" json:"default,omitempty"`
	// Pattern defines a regular expression that string inputs must match
	Pattern string `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	// Minimum sets the lower bound for numeric inputs
	Minimum *float64 `yaml:"minimum,omitempty" json:"minimum,omitempty"`
	// Maximum sets the upper bound for numeric inputs
	Maximum *float64 `yaml:"maximum,omitempty" json:"maximum,omitempty"`
	// MinItems sets the minimum number of elements for array inputs
	MinItems *int `yaml:"min_items,omitempty" json:"min_items,omitempty"`
	// MaxItems sets the maximum number of elements for array inputs
	MaxItems *int `yaml:"max_items,omitempty" json:"max_items,omitempty"`
	// Enum restricts string inputs to a specific set of allowed values
	Enum []string `yaml:"enum,omitempty" json:"enum,omitempty"`

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

// Step represents a single execution unit in a workflow that can perform various actions like running scripts, calling AI agents, or updating state
type Step struct {
	// ID is a unique identifier for this step within the workflow, used for referencing
	// in conditions and dependencies
	ID string `yaml:"id" json:"id" jsonschema:"required"`
	// While specifies a condition that must be true for the step to execute. The expression
	// must evaluate to a boolean.
	While string `yaml:"while,omitempty" json:"while,omitempty" jsonschema:"oneof_required=while"`
	// Steps defines a list of sub steps to execute in sequence. In general it is discouraged to use
	// sub steps unless you are using a while loop or some other control flow mechanism.
	Steps []*Step `yaml:"steps,omitempty" json:"steps,omitempty"`
	// Agent specifies which AI agent to use for this step (references an agent defined in the agents section)
	Agent string `yaml:"agent,omitempty" json:"agent,omitempty" jsonschema:"oneof_required=agent"`
	// Prompt provides instructions or questions for the AI agent to process
	Prompt string `yaml:"prompt,omitempty" json:"prompt,omitempty"`
	// Uses references a predefined block, workflow, or action to execute
	Uses string `yaml:"uses,omitempty" json:"uses,omitempty" jsonschema:"oneof_required=uses"`
	// Run contains a bash script to execute directly in this step, this can call out to other
	// scripts in the runtime environment. If you need a access to a specific runtime language
	// (e.g. node, python, go), define it in the requirements section. Break complex logic into
	// external script calls to keep complex bash to a minimum. e.g. "go run ./script.go" or "node ./script.js"
	Run string `yaml:"run,omitempty" json:"run,omitempty" jsonschema:"oneof_required=run"`
	// Container specifies a Docker container image to run for this step
	Container string `yaml:"container,omitempty" json:"container,omitempty" jsonschema:"oneof_required=container"`
	// Command defines the command and arguments to execute in a container
	Command []string `yaml:"command,omitempty" json:"command,omitempty"`
	// With provides input parameters for the referenced script, workflow or block
	With map[string]interface{} `yaml:"with,omitempty" json:"with,omitempty"`
	// Action specifies a built-in action to perform (e.g., "update_state", "set_output")
	Action string `yaml:"action,omitempty" json:"action,omitempty" jsonschema:"enum=update_state,enum=set_output"`
	// Updates defines changes to make to the workflow state when this step completes
	Updates map[string]interface{} `yaml:"updates,omitempty" json:"updates,omitempty"`
	// Condition determines whether this step should execute based on workflow state or previous step results.
	// If the condition is not met, the step will be skipped.
	Condition string `yaml:"condition,omitempty" json:"condition,omitempty"`
	// SkipIf provides an alternative condition syntax that skips the step when the condition is true
	SkipIf string `yaml:"skip_if,omitempty" json:"skip_if,omitempty"`
	// Outputs defines values that this step makes available to subsequent steps and the final workflow output
	Outputs map[string]schema.JSON `yaml:"outputs,omitempty" json:"outputs,omitempty"`

	Position Position `yaml:"-" json:"-"`
}

func (s Step) JSONSchemaExtend(schema *jsonschema.Schema) {
	schema.DependentRequired = map[string][]string{
		"agent": []string{
			"prompt",
		},
	}
}

// Duration wraps time.Duration with custom YAML/JSON marshaling for human-readable duration strings
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
