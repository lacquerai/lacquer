package ast

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ValidProviders = []string{"anthropic", "openai", "local"}
	ValidRuntimes  = []string{"go", "node"}
	ValidStepTypes = []string{"agent", "uses", "run", "container", "action"}
	ValidToolTypes = []string{"uses", "script", "mcp"}
)

func ListToReadable(list []string) string {
	if len(list) == 0 {
		return ""
	}

	if len(list) == 1 {
		return list[0]
	}

	builder := strings.Builder{}
	for i, v := range list {
		builder.WriteString(v)
		if len(list)-2 == i {
			builder.WriteString(" or ")
		} else {
			builder.WriteString(", ")
		}
	}

	return builder.String()
}

// ValidationError represents a validation error
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// Error implements the error interface
func (ve *ValidationError) Error() string {
	if ve.Path != "" {
		return fmt.Sprintf("%s: %s", ve.Path, ve.Message)
	}
	return ve.Message
}

// ValidationResult contains the results of AST validation
type ValidationResult struct {
	Valid  bool               `json:"valid"`
	Errors []*ValidationError `json:"errors,omitempty"`
	// TODO: add warning output to the CLI, currently we only collect these
	// as potential improvements
	Warnings []*ValidationError `json:"warnings,omitempty"`
}

// AddError adds a validation error
func (vr *ValidationResult) AddError(path, message string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, &ValidationError{
		Path:    path,
		Message: message,
	})
}

// AddWarning adds a validation warning
func (vr *ValidationResult) AddWarning(path, message string) {
	vr.Warnings = append(vr.Warnings, &ValidationError{
		Path:    path,
		Message: message,
	})
}

// AddFieldError adds a validation error for a specific field
func (vr *ValidationResult) AddFieldError(path, field, message string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, &ValidationError{
		Path:    path,
		Field:   field,
		Message: message,
	})
}

// HasErrors returns true if there are validation errors
func (vr *ValidationResult) HasErrors() bool {
	return len(vr.Errors) > 0
}

// ToError returns a combined error if there are validation errors
func (vr *ValidationResult) ToError() error {
	if !vr.HasErrors() {
		return nil
	}

	var messages []string
	for _, err := range vr.Errors {
		messages = append(messages, err.Error())
	}

	return fmt.Errorf("validation failed: %s", strings.Join(messages, "; "))
}

// Validator provides comprehensive validation for AST structures
type Validator struct {
	wd       string
	workflow *Workflow
	result   *ValidationResult
}

// NewValidator creates a new AST validator
func NewValidator(w *Workflow) *Validator {
	wd := filepath.Dir(w.SourceFile)
	return &Validator{
		wd:       wd,
		workflow: w,
		result:   &ValidationResult{Valid: true},
	}
}

// ValidateWorkflow performs comprehensive validation of a workflow
func (v *Validator) ValidateWorkflow() *ValidationResult {
	w := v.workflow

	// Basic structure validation
	if w.Version != "1.0" {
		v.result.AddFieldError("", "version", fmt.Sprintf("unsupported version: %s", w.Version))
	}

	if w.Workflow == nil {
		v.result.AddError("", "workflow section is required")
		return v.result
	}

	// Validate agents
	if w.Agents != nil {
		v.validateAgents()
	}

	if w.Requirements != nil {
		v.validateRequirements()
	}

	v.validateWorkflowDef()

	return v.result
}

func (v *Validator) validateRequirements() {
	for _, rr := range v.workflow.Requirements.Runtimes {
		isValidRuntime := false
		for _, v := range ValidRuntimes {
			if string(rr.Name) == v {
				isValidRuntime = true
				break
			}
		}

		if !isValidRuntime {
			v.result.AddFieldError("requirements", "runtimes", fmt.Sprintf("runtimes must be one of: %s", ListToReadable(ValidRuntimes)))
		}
	}
}

// validateAgents validates all agent definitions
func (v *Validator) validateAgents() {
	path := "agents"

	for name, agent := range v.workflow.Agents {
		agentPath := fmt.Sprintf("%s.%s", path, name)

		if !isValidIdentifier(name) {
			v.result.AddError(agentPath, "agent name must be a valid identifier")
		}

		v.validateAgent(agent, agentPath)
	}
}

// validateAgent validates a single agent
func (v *Validator) validateAgent(agent *Agent, path string) {
	if agent.Model == "" && agent.Uses == "" {
		v.result.AddError(path, "agent must specify either 'model' or 'uses'")
		return
	}

	if agent.Model != "" && agent.Uses != "" {
		v.result.AddFieldError(path, "model", "agent cannot specify both 'model' and 'uses'")
		return
	}

	if agent.Model != "" {
		if agent.Provider == "" {
			v.result.AddFieldError(path, "provider", "provider is required when using a model")
		} else {
			isValidProvider := false
			for _, provider := range ValidProviders {
				if agent.Provider == provider {
					isValidProvider = true
					break
				}
			}

			if !isValidProvider {
				v.result.AddFieldError(path, "provider", fmt.Sprintf("provider must be one of: %v", ValidProviders))
			}
		}
	}

	if agent.Uses != "" {
		if err := isValidBlockReference(v.wd, agent.Uses); err != nil {
			v.result.AddFieldError(path, "uses", err.Error())
		}
	}

	if agent.Temperature != nil && (*agent.Temperature < 0 || *agent.Temperature > 2) {
		v.result.AddFieldError(path, "temperature", "temperature must be between 0 and 2")
	}

	if agent.TopP != nil && (*agent.TopP < 0 || *agent.TopP > 1) {
		v.result.AddFieldError(path, "top_p", "top_p must be between 0 and 1")
	}

	if agent.MaxTokens != nil && *agent.MaxTokens < 1 {
		v.result.AddFieldError(path, "max_tokens", "max_tokens must be positive")
	}

	v.validateTools(agent.Tools, fmt.Sprintf("%s.tools", path))
}

// validateTools validates agent tools
func (v *Validator) validateTools(tools []*Tool, path string) {
	toolNames := make(map[string]bool)

	for i, tool := range tools {
		toolPath := fmt.Sprintf("%s[%d]", path, i)

		if toolNames[tool.Name] {
			v.result.AddError(toolPath, fmt.Sprintf("duplicate tool name: %s", tool.Name))
		}
		toolNames[tool.Name] = true

		v.validateTool(tool, toolPath)
	}
}

// validateTool validates a single tool
func (v *Validator) validateTool(tool *Tool, path string) {
	if tool.Name == "" {
		v.result.AddFieldError(path, "name", "tool name is required")
		return
	}

	if !isValidIdentifier(tool.Name) {
		v.result.AddFieldError(path, "name", "tool name must be a valid identifier")
	}

	toolTypes := make(map[string]bool)
	if tool.Uses != "" {
		toolTypes["uses"] = true
	}
	if tool.Script != "" {
		toolTypes["script"] = true
	}
	if tool.MCPServer != nil {
		toolTypes["mcp"] = true
	}

	if len(toolTypes) == 0 {
		v.result.AddError(path, fmt.Sprintf("tool must specify one of: %s", ListToReadable(ValidToolTypes)))
	} else if len(toolTypes) > 1 {
		names := make([]string, 0, len(toolTypes))
		for v := range toolTypes {
			names = append(names, v)
		}
		sort.Strings(names)

		v.result.AddError(path, fmt.Sprintf("tool cannot specify multiple tool types, please choose one of %s", ListToReadable(names)))
	}

	if tool.Uses != "" {
		if err := isValidBlockReference(v.wd, tool.Uses); err != nil {
			v.result.AddFieldError(path, "uses", err.Error())
		}
	}

	if tool.Script != "" {
		v.validateScriptTool(tool, path)
	}

	// Validate MCP server tools
	if tool.MCPServer != nil {
		v.validateMCPTool(tool, path)
	}

	// Validate tool configuration
	v.validateToolConfig(tool, path)
}

// validateScriptTool validates script-specific configuration
func (v *Validator) validateScriptTool(tool *Tool, path string) {
	if strings.HasPrefix(tool.Script, "./") || strings.HasPrefix(tool.Script, "/") {
		if !isValidFilePath(tool.Script) {
			v.result.AddFieldError(path, "script", "invalid script file path")
		}
	} else {
		if strings.TrimSpace(tool.Script) == "" {
			v.result.AddFieldError(path, "script", "script content cannot be empty")
		}
	}
}

// validateMCPTool validates MCP server-specific configuration
func (v *Validator) validateMCPTool(tool *Tool, path string) {
	if tool.MCPServer == nil {
		return
	}

	// Validate based on server type
	switch tool.MCPServer.Type {
	case "local", "":
		if tool.MCPServer.Command == "" {
			v.result.AddFieldError(path, "mcp_server.command", "command is required for local MCP servers")
		}
	case "remote":
		if tool.MCPServer.URL == "" {
			v.result.AddFieldError(path, "mcp_server.url", "URL is required for remote MCP servers")
		} else if !isValidURL(tool.MCPServer.URL) {
			v.result.AddFieldError(path, "mcp_server.url", "invalid MCP server URL format")
		}
	default:
		v.result.AddFieldError(path, "mcp_server.type", "invalid MCP server type: must be 'local' or 'remote'")
	}

	if tool.MCPServer.Auth != nil {
		v.validateMCPAuth(tool.MCPServer.Auth, path+".auth")
	}
}

// validateMCPAuth validates MCP authentication configuration
func (v *Validator) validateMCPAuth(auth *MCPAuthConfig, path string) {
	switch auth.Type {
	case "oauth2":
		if auth.ClientID == "" {
			v.result.AddFieldError(path, "client_id", "client_id is required for OAuth2 authentication")
		}
		if auth.ClientSecret == "" {
			v.result.AddFieldError(path, "client_secret", "client_secret is required for OAuth2 authentication")
		}
		if auth.TokenURL == "" {
			v.result.AddFieldError(path, "token_url", "token_url is required for OAuth2 authentication")
		} else if !isValidURL(auth.TokenURL) {
			v.result.AddFieldError(path, "token_url", "invalid token URL format")
		}
	case "api_key":
		if auth.APIKey == "" {
			v.result.AddFieldError(path, "api_key", "api_key is required for API key authentication")
		}
	case "basic":
		if auth.Username == "" {
			v.result.AddFieldError(path, "username", "username is required for basic authentication")
		}
		if auth.Password == "" {
			v.result.AddFieldError(path, "password", "password is required for basic authentication")
		}
	case "none":
		// No validation needed
	default:
		v.result.AddFieldError(path, "type", "invalid authentication type: must be 'oauth2', 'api_key', 'basic', or 'none'")
	}
}

// validateToolConfig validates tool configuration parameters
func (v *Validator) validateToolConfig(tool *Tool, path string) {
	if tool.Config == nil {
		return
	}

	for key, value := range tool.Config {
		switch key {
		case "timeout":
			if timeoutStr, ok := value.(string); ok {
				if !isValidDuration(timeoutStr) {
					v.result.AddFieldError(path, "config.timeout", "invalid timeout duration format")
				}
			}
		case "retries":
			if retries, ok := value.(int); ok {
				if retries < 0 {
					v.result.AddFieldError(path, "config.retries", "retries must be non-negative")
				}
			}
		case "max_retries":
			if maxRetries, ok := value.(int); ok {
				if maxRetries < 0 {
					v.result.AddFieldError(path, "config.max_retries", "max_retries must be non-negative")
				}
			}
		}
	}
}

// validateWorkflowDef validates the workflow definition
func (v *Validator) validateWorkflowDef() {
	path := "workflow"
	workflow := v.workflow.Workflow
	if len(workflow.Steps) == 0 {
		v.result.AddFieldError(path, "steps", "workflow must have at least one step")
		return
	}

	if workflow.Inputs != nil {
		v.validateInputs(workflow.Inputs, fmt.Sprintf("%s.inputs", path))
	}

	v.validateSteps()
}

// validateInputs validates workflow input parameters
func (v *Validator) validateInputs(inputs map[string]*InputParam, path string) {
	for name, param := range inputs {
		paramPath := fmt.Sprintf("%s.%s", path, name)

		if !isValidIdentifier(name) {
			v.result.AddError(paramPath, "input parameter name must be a valid identifier")
		}

		v.validateInputParam(param, paramPath)
	}
}

// validateInputParam validates a single input parameter
func (v *Validator) validateInputParam(param *InputParam, path string) {
	// Validate type
	if param.Type != "" {
		validTypes := []string{"string", "integer", "boolean", "array", "object"}
		if !contains(validTypes, param.Type) {
			v.result.AddFieldError(path, "type", fmt.Sprintf("invalid type: %s", param.Type))
		}
	}

	// Validate numeric constraints
	if param.Minimum != nil && param.Maximum != nil && *param.Minimum > *param.Maximum {
		v.result.AddFieldError(path, "minimum", "minimum cannot be greater than maximum")
	}

	if param.MinItems != nil && *param.MinItems < 0 {
		v.result.AddFieldError(path, "min_items", "min_items cannot be negative")
	}

	if param.MaxItems != nil && *param.MaxItems < 0 {
		v.result.AddFieldError(path, "max_items", "max_items cannot be negative")
	}

	if param.MinItems != nil && param.MaxItems != nil && *param.MinItems > *param.MaxItems {
		v.result.AddFieldError(path, "min_items", "min_items cannot be greater than max_items")
	}
}

// validateSteps validates all workflow steps
func (v *Validator) validateSteps() {
	path := "workflow.steps"
	stepIDs := make(map[string]bool)

	for i, step := range v.workflow.Workflow.Steps {
		stepPath := fmt.Sprintf("%s[%d]", path, i)

		v.validateStep(step, stepPath)

		if stepIDs[step.ID] {
			v.result.AddError(stepPath, fmt.Sprintf("duplicate step ID: %s", step.ID))
		}

		stepIDs[step.ID] = true
	}
}

// validateStep validates a single step
func (v *Validator) validateStep(step *Step, path string) {
	if step.ID == "" {
		v.result.AddFieldError(path, "id", "step ID is required")
		return
	}

	if !isValidIdentifier(step.ID) {
		v.result.AddFieldError(path, "id", "step ID must be a valid identifier")
	}

	stepTypes := make(map[string]bool)
	if step.Agent != "" {
		stepTypes["agent"] = true
	}

	if step.Uses != "" {
		stepTypes["uses"] = true
	}

	if step.Action != "" {
		stepTypes["action"] = true
	}

	if step.Run != "" {
		stepTypes["run"] = true
	}

	if step.Container != "" {
		stepTypes["containter"] = true
	}

	if len(stepTypes) == 0 {
		v.result.AddError(path, fmt.Sprintf("step must specify either %s", ListToReadable(ValidStepTypes)))
	} else if len(stepTypes) > 1 {
		types := make([]string, 0, len(stepTypes))
		for v := range stepTypes {
			types = append(types, v)
		}
		sort.Strings(types)

		v.result.AddError(path, fmt.Sprintf("step cannot specify multiple execution methods, please choose one of %s", ListToReadable(types)))
	}

	if step.Agent != "" || step.Prompt != "" {
		v.validateAgentStep(path, step)
	}

	if step.Action != "" {
		validActions := []string{"human_input", "update_state"}
		if !contains(validActions, step.Action) {
			v.result.AddFieldError(path, "action", fmt.Sprintf("invalid action: %s", step.Action))
		}

		if step.Action == "update_state" && len(step.Updates) == 0 {
			v.result.AddFieldError(path, "updates", "update_state action requires updates field")
		}
	}

	if step.Uses != "" {
		if err := isValidBlockReference(v.wd, step.Uses); err != nil {
			v.result.AddFieldError(path, "uses", err.Error())
		}
	}

	if step.Run != "" {
		if strings.HasPrefix(step.Run, "./") {
			if err := isValidLocalPath(v.wd, step.Run); err != nil {
				v.result.AddFieldError(path, "run", err.Error())
			}
		}
	}

	if step.Container != "" {
		if strings.HasPrefix(step.Run, "./") {
			if err := isValidLocalPath(v.wd, step.Run); err != nil {
				v.result.AddFieldError(path, "container", err.Error())
			}
		}
	}
}

func (v *Validator) validateAgentStep(path string, step *Step) {
	valid := true

	if step.Agent == "" {
		v.result.AddFieldError(path, "agent", "agent is required when prompt is specified")
		valid = false
	}

	if step.Prompt == "" {
		v.result.AddFieldError(path, "prompt", "prompt is required when agent is specified")
		valid = false
	}

	if !valid {
		return
	}

	if v.workflow.Agents == nil {
		v.result.AddFieldError(path, "agent", fmt.Sprintf("please define an agents section with valid configuration for agent %s", step.Agent))
	}

	if _, ok := v.workflow.Agents[step.Agent]; !ok {
		v.result.AddFieldError(path, "agent", fmt.Sprintf("agent %q must exist in the agents section", step.Agent))
	}
}

// isValidIdentifier checks if a string is a valid identifier
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	// Must start with letter or underscore, followed by letters, digits, or underscores
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, s)
	return matched
}

// isValidBlockReference checks if a block reference is valid
func isValidBlockReference(wd, ref string) error {
	if ref == "" {
		return fmt.Errorf("reference cannot be empty")
	}

	if strings.HasPrefix(ref, "lacquer/") {
		matched, _ := regexp.MatchString(`^lacquer/[a-z0-9-]+(@v[0-9]+(\.[0-9]+)*)?$`, ref)
		if matched {
			return nil
		}

		return fmt.Errorf("lacquer ref must be in the format lacquer/package@v1 where v1 is a valid version number")
	}

	if strings.HasPrefix(ref, "github.com/") {
		matched, _ := regexp.MatchString(`^github\.com/[^/]+/[^/]+(@[^/]+)?$`, ref)
		if matched {
			return nil
		}

		return fmt.Errorf("github ref must be in the format github.com/user/repo@v1 where v1 is a valid version number")
	}

	if strings.HasPrefix(ref, "./") {
		return isValidLocalPath(wd, ref)
	}

	return fmt.Errorf("ref %s is not a valid ref, must be either a valid lacquer ref, github ref or local path", ref)
}

func isValidLocalPath(wd, ref string) error {
	rel, err := filepath.Rel(wd, ref)
	if err != nil {
		return fmt.Errorf("ref %s cannot be made relative to the current working directory %s", ref, wd)
	}

	_, err = os.Stat(rel)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("ref %s does not exist, please ensure that this is a valid path", ref)
	}

	return nil
}

// isValidFilePath checks if a file path is valid
func isValidFilePath(path string) bool {
	if path == "" {
		return false
	}

	// Basic path validation
	if strings.Contains(path, "..") {
		return false // Prevent directory traversal
	}

	// Must have a file extension for scripts
	if !strings.HasPrefix(path, "./") && !strings.HasPrefix(path, "/") {
		return false
	}

	return true
}

// isValidURL checks if a URL is valid
func isValidURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	// Basic URL validation
	matched, _ := regexp.MatchString(`^https?://[^\s/$.?#].[^\s]*$`, urlStr)
	return matched
}

// isValidDuration checks if a duration string is valid
func isValidDuration(duration string) bool {
	if duration == "" {
		return false
	}

	// Use Go's time.ParseDuration for validation
	_, err := parseTemplateDuration(duration)
	return err == nil
}

// parseTemplateDuration parses duration strings similar to time.ParseDuration
func parseTemplateDuration(s string) (time.Duration, error) {
	// Simple regex for duration validation
	matched, _ := regexp.MatchString(`^[0-9]+[a-zA-Z]+$`, s)
	if !matched {
		return 0, fmt.Errorf("invalid duration format")
	}

	// For validation purposes, just check the format
	return time.Second, nil
}
