package ast

import (
	"fmt"
	"regexp"
	"strings"
)

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
}

// AddError adds a validation error
func (vr *ValidationResult) AddError(path, message string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, &ValidationError{
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
}

// NewValidator creates a new AST validator
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateWorkflow performs comprehensive validation of a workflow
func (v *Validator) ValidateWorkflow(w *Workflow) *ValidationResult {
	result := &ValidationResult{Valid: true}
	
	// Basic structure validation
	if w.Version != "1.0" {
		result.AddFieldError("", "version", fmt.Sprintf("unsupported version: %s", w.Version))
	}
	
	if w.Workflow == nil {
		result.AddError("", "workflow section is required")
		return result
	}
	
	// Validate metadata
	if w.Metadata != nil {
		v.validateMetadata(w.Metadata, "metadata", result)
	}
	
	// Validate agents
	if w.Agents != nil {
		v.validateAgents(w.Agents, "agents", result)
	}
	
	// Validate workflow definition
	v.validateWorkflowDef(w.Workflow, "workflow", result)
	
	// Cross-reference validation
	v.validateCrossReferences(w, result)
	
	return result
}

// validateMetadata validates workflow metadata
func (v *Validator) validateMetadata(metadata *WorkflowMetadata, path string, result *ValidationResult) {
	if metadata.Name == "" {
		result.AddFieldError(path, "name", "name is required")
		return
	}
	
	// Validate name format (kebab-case)
	if !isValidKebabCase(metadata.Name) {
		result.AddFieldError(path, "name", "name must be in kebab-case format")
	}
	
	// Validate version format if specified
	if metadata.Version != "" && !isValidSemVer(metadata.Version) {
		result.AddFieldError(path, "version", "version must follow semantic versioning")
	}
}

// validateAgents validates all agent definitions
func (v *Validator) validateAgents(agents map[string]*Agent, path string, result *ValidationResult) {
	for name, agent := range agents {
		agentPath := fmt.Sprintf("%s.%s", path, name)
		
		// Validate agent name
		if !isValidIdentifier(name) {
			result.AddError(agentPath, "agent name must be a valid identifier")
		}
		
		// Validate agent configuration
		v.validateAgent(agent, agentPath, result)
	}
}

// validateAgent validates a single agent
func (v *Validator) validateAgent(agent *Agent, path string, result *ValidationResult) {
	// Must specify either model or uses
	if agent.Model == "" && agent.Uses == "" {
		result.AddError(path, "agent must specify either 'model' or 'uses'")
		return
	}
	
	if agent.Model != "" && agent.Uses != "" {
		result.AddError(path, "agent cannot specify both 'model' and 'uses'")
		return
	}
	
	// Validate model if specified
	if agent.Model != "" {
		if !isValidModel(agent.Model) {
			result.AddFieldError(path, "model", fmt.Sprintf("unsupported model: %s", agent.Model))
		}
	}
	
	// Validate uses reference if specified
	if agent.Uses != "" {
		if !isValidBlockReference(agent.Uses) {
			result.AddFieldError(path, "uses", "invalid block reference format")
		}
	}
	
	// Validate numeric parameters
	if agent.Temperature != nil && (*agent.Temperature < 0 || *agent.Temperature > 2) {
		result.AddFieldError(path, "temperature", "temperature must be between 0 and 2")
	}
	
	if agent.TopP != nil && (*agent.TopP < 0 || *agent.TopP > 1) {
		result.AddFieldError(path, "top_p", "top_p must be between 0 and 1")
	}
	
	if agent.MaxTokens != nil && *agent.MaxTokens < 1 {
		result.AddFieldError(path, "max_tokens", "max_tokens must be positive")
	}
	
	// Validate tools
	v.validateTools(agent.Tools, fmt.Sprintf("%s.tools", path), result)
}

// validateTools validates agent tools
func (v *Validator) validateTools(tools []*Tool, path string, result *ValidationResult) {
	toolNames := make(map[string]bool)
	
	for i, tool := range tools {
		toolPath := fmt.Sprintf("%s[%d]", path, i)
		
		// Check for duplicate tool names
		if toolNames[tool.Name] {
			result.AddError(toolPath, fmt.Sprintf("duplicate tool name: %s", tool.Name))
		}
		toolNames[tool.Name] = true
		
		v.validateTool(tool, toolPath, result)
	}
}

// validateTool validates a single tool
func (v *Validator) validateTool(tool *Tool, path string, result *ValidationResult) {
	if tool.Name == "" {
		result.AddFieldError(path, "name", "tool name is required")
		return
	}
	
	if !isValidIdentifier(tool.Name) {
		result.AddFieldError(path, "name", "tool name must be a valid identifier")
	}
	
	// Must specify exactly one tool type
	toolTypes := 0
	if tool.Uses != "" {
		toolTypes++
	}
	if tool.Script != "" {
		toolTypes++
	}
	if tool.MCPServer != "" {
		toolTypes++
	}
	
	if toolTypes == 0 {
		result.AddError(path, "tool must specify one of: uses, script, or mcp_server")
	} else if toolTypes > 1 {
		result.AddError(path, "tool cannot specify multiple tool types")
	}
	
	// Validate tool references
	if tool.Uses != "" && !isValidBlockReference(tool.Uses) {
		result.AddFieldError(path, "uses", "invalid tool reference format")
	}
}

// validateWorkflowDef validates the workflow definition
func (v *Validator) validateWorkflowDef(workflow *WorkflowDef, path string, result *ValidationResult) {
	if len(workflow.Steps) == 0 {
		result.AddFieldError(path, "steps", "workflow must have at least one step")
		return
	}
	
	// Validate inputs
	if workflow.Inputs != nil {
		v.validateInputs(workflow.Inputs, fmt.Sprintf("%s.inputs", path), result)
	}
	
	// Validate steps
	v.validateSteps(workflow.Steps, fmt.Sprintf("%s.steps", path), result)
}

// validateInputs validates workflow input parameters
func (v *Validator) validateInputs(inputs map[string]*InputParam, path string, result *ValidationResult) {
	for name, param := range inputs {
		paramPath := fmt.Sprintf("%s.%s", path, name)
		
		if !isValidIdentifier(name) {
			result.AddError(paramPath, "input parameter name must be a valid identifier")
		}
		
		v.validateInputParam(param, paramPath, result)
	}
}

// validateInputParam validates a single input parameter
func (v *Validator) validateInputParam(param *InputParam, path string, result *ValidationResult) {
	// Validate type
	if param.Type != "" {
		validTypes := []string{"string", "integer", "boolean", "array", "object"}
		if !contains(validTypes, param.Type) {
			result.AddFieldError(path, "type", fmt.Sprintf("invalid type: %s", param.Type))
		}
	}
	
	// Validate numeric constraints
	if param.Minimum != nil && param.Maximum != nil && *param.Minimum > *param.Maximum {
		result.AddError(path, "minimum cannot be greater than maximum")
	}
	
	if param.MinItems != nil && *param.MinItems < 0 {
		result.AddFieldError(path, "min_items", "min_items cannot be negative")
	}
	
	if param.MaxItems != nil && *param.MaxItems < 0 {
		result.AddFieldError(path, "max_items", "max_items cannot be negative")
	}
	
	if param.MinItems != nil && param.MaxItems != nil && *param.MinItems > *param.MaxItems {
		result.AddError(path, "min_items cannot be greater than max_items")
	}
}

// validateSteps validates all workflow steps
func (v *Validator) validateSteps(steps []*Step, path string, result *ValidationResult) {
	stepIDs := make(map[string]bool)
	
	for i, step := range steps {
		stepPath := fmt.Sprintf("%s[%d]", path, i)
		
		// Check for duplicate step IDs
		if stepIDs[step.ID] {
			result.AddError(stepPath, fmt.Sprintf("duplicate step ID: %s", step.ID))
		}
		stepIDs[step.ID] = true
		
		v.validateStep(step, stepPath, result)
	}
}

// validateStep validates a single step
func (v *Validator) validateStep(step *Step, path string, result *ValidationResult) {
	if step.ID == "" {
		result.AddFieldError(path, "id", "step ID is required")
		return
	}
	
	if !isValidIdentifier(step.ID) {
		result.AddFieldError(path, "id", "step ID must be a valid identifier")
	}
	
	// Validate step type
	stepTypes := 0
	if step.Agent != "" && step.Prompt != "" {
		stepTypes++
	}
	if step.Uses != "" {
		stepTypes++
	}
	if step.Action != "" {
		stepTypes++
	}
	
	if stepTypes == 0 {
		result.AddError(path, "step must specify either agent+prompt, uses, or action")
	} else if stepTypes > 1 {
		result.AddError(path, "step cannot specify multiple execution methods")
	}
	
	// Validate agent steps
	if step.Agent != "" || step.Prompt != "" {
		if step.Agent == "" {
			result.AddFieldError(path, "agent", "agent is required when prompt is specified")
		}
		if step.Prompt == "" {
			result.AddFieldError(path, "prompt", "prompt is required when agent is specified")
		}
	}
	
	// Validate action steps
	if step.Action != "" {
		validActions := []string{"human_input", "update_state"}
		if !contains(validActions, step.Action) {
			result.AddFieldError(path, "action", fmt.Sprintf("invalid action: %s", step.Action))
		}
		
		if step.Action == "update_state" && len(step.Updates) == 0 {
			result.AddFieldError(path, "updates", "update_state action requires updates field")
		}
	}
	
	// Validate block references
	if step.Uses != "" && !isValidBlockReference(step.Uses) {
		result.AddFieldError(path, "uses", "invalid block reference format")
	}
}

// validateCrossReferences validates references between workflow elements
func (v *Validator) validateCrossReferences(w *Workflow, result *ValidationResult) {
	// Collect available agents
	agentNames := make(map[string]bool)
	if w.Agents != nil {
		for name := range w.Agents {
			agentNames[name] = true
		}
	}
	
	// Agent references are validated by the semantic validator with better positioning
}

// Validation helper functions

// isValidIdentifier checks if a string is a valid identifier
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	// Must start with letter or underscore, followed by letters, digits, or underscores
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, s)
	return matched
}

// isValidKebabCase checks if a string is valid kebab-case
func isValidKebabCase(s string) bool {
	if s == "" {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-z0-9]+(-[a-z0-9]+)*$`, s)
	return matched
}

// isValidSemVer checks if a string is a valid semantic version
func isValidSemVer(s string) bool {
	// Simple semantic version regex (basic validation)
	matched, _ := regexp.MatchString(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[a-zA-Z0-9-]+)?(\+[a-zA-Z0-9-]+)?$`, s)
	return matched
}

// isValidModel checks if a model name is supported
func isValidModel(model string) bool {
	validModels := []string{
		"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo",
		"claude-3-opus", "claude-3-sonnet", "claude-3-haiku",
		"gemini-pro", "gemini-pro-vision",
	}
	return contains(validModels, model)
}

// isValidBlockReference checks if a block reference is valid
func isValidBlockReference(ref string) bool {
	if ref == "" {
		return false
	}
	
	// Check for official lacquer blocks
	if strings.HasPrefix(ref, "lacquer/") {
		matched, _ := regexp.MatchString(`^lacquer/[a-z0-9-]+(@v[0-9]+(\.[0-9]+)*)?$`, ref)
		return matched
	}
	
	// Check for GitHub references
	if strings.HasPrefix(ref, "github.com/") {
		matched, _ := regexp.MatchString(`^github\.com/[^/]+/[^/]+(@[^/]+)?$`, ref)
		return matched
	}
	
	// Check for local references
	if strings.HasPrefix(ref, "./") {
		return len(ref) > 2
	}
	
	return false
}