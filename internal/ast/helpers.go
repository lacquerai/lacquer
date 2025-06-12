package ast

import (
	"fmt"
	"strings"
)

// Workflow helper methods

// GetAgent retrieves an agent by name
func (w *Workflow) GetAgent(name string) (*Agent, bool) {
	if w.Agents == nil {
		return nil, false
	}
	agent, exists := w.Agents[name]
	return agent, exists
}

// GetStep retrieves a step by ID
func (w *Workflow) GetStep(id string) (*Step, bool) {
	if w.Workflow == nil || w.Workflow.Steps == nil {
		return nil, false
	}
	
	for _, step := range w.Workflow.Steps {
		if step.ID == id {
			return step, true
		}
	}
	return nil, false
}

// GetSteps returns all workflow steps
func (w *Workflow) GetSteps() []*Step {
	if w.Workflow == nil {
		return nil
	}
	return w.Workflow.Steps
}

// GetInputParam retrieves an input parameter by name
func (w *Workflow) GetInputParam(name string) (*InputParam, bool) {
	if w.Workflow == nil || w.Workflow.Inputs == nil {
		return nil, false
	}
	param, exists := w.Workflow.Inputs[name]
	return param, exists
}

// ListAgents returns a list of all agent names
func (w *Workflow) ListAgents() []string {
	if w.Agents == nil {
		return nil
	}
	
	names := make([]string, 0, len(w.Agents))
	for name := range w.Agents {
		names = append(names, name)
	}
	return names
}

// ListStepIDs returns a list of all step IDs
func (w *Workflow) ListStepIDs() []string {
	steps := w.GetSteps()
	if steps == nil {
		return nil
	}
	
	ids := make([]string, len(steps))
	for i, step := range steps {
		ids[i] = step.ID
	}
	return ids
}

// Validate performs basic structural validation
func (w *Workflow) Validate() error {
	if w.Version != "1.0" {
		return fmt.Errorf("unsupported version: %s (expected 1.0)", w.Version)
	}
	
	if w.Workflow == nil {
		return fmt.Errorf("workflow definition is required")
	}
	
	if len(w.Workflow.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}
	
	// Check for duplicate step IDs
	stepIDs := make(map[string]bool)
	for _, step := range w.Workflow.Steps {
		if stepIDs[step.ID] {
			return fmt.Errorf("duplicate step ID: %s", step.ID)
		}
		stepIDs[step.ID] = true
	}
	
	// Validate each step
	for _, step := range w.Workflow.Steps {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("step %s: %w", step.ID, err)
		}
	}
	
	return nil
}

// Step helper methods

// IsAgentStep returns true if this is an agent execution step
func (s *Step) IsAgentStep() bool {
	return s.Agent != "" && s.Prompt != ""
}

// IsBlockStep returns true if this is a block usage step
func (s *Step) IsBlockStep() bool {
	return s.Uses != ""
}

// IsActionStep returns true if this is a system action step
func (s *Step) IsActionStep() bool {
	return s.Action != ""
}

// GetStepType returns the type of step as a string
func (s *Step) GetStepType() string {
	switch {
	case s.IsAgentStep():
		return "agent"
	case s.IsBlockStep():
		return "block"
	case s.IsActionStep():
		return "action"
	default:
		return "unknown"
	}
}

// Validate performs basic validation for the step
func (s *Step) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("step ID is required")
	}
	
	// Check that exactly one execution method is specified
	methods := 0
	if s.IsAgentStep() {
		methods++
	}
	if s.IsBlockStep() {
		methods++
	}
	if s.IsActionStep() {
		methods++
	}
	
	if methods == 0 {
		return fmt.Errorf("step must specify either agent+prompt, uses, or action")
	}
	if methods > 1 {
		return fmt.Errorf("step cannot specify multiple execution methods")
	}
	
	// Validate agent steps
	if s.IsAgentStep() {
		if s.Agent == "" {
			return fmt.Errorf("agent name is required for agent steps")
		}
		if s.Prompt == "" {
			return fmt.Errorf("prompt is required for agent steps")
		}
	}
	
	// Validate block steps
	if s.IsBlockStep() {
		if s.Uses == "" {
			return fmt.Errorf("uses is required for block steps")
		}
	}
	
	// Validate action steps
	if s.IsActionStep() {
		validActions := []string{"human_input", "update_state"}
		if !contains(validActions, s.Action) {
			return fmt.Errorf("invalid action: %s (valid actions: %s)", 
				s.Action, strings.Join(validActions, ", "))
		}
		
		if s.Action == "update_state" && len(s.Updates) == 0 {
			return fmt.Errorf("update_state action requires updates field")
		}
	}
	
	return nil
}

// HasOutput checks if the step produces a specific output
func (s *Step) HasOutput(name string) bool {
	if s.Outputs == nil {
		return false
	}
	_, exists := s.Outputs[name]
	return exists
}

// ListOutputs returns a list of all output names for this step
func (s *Step) ListOutputs() []string {
	if s.Outputs == nil {
		return nil
	}
	
	outputs := make([]string, 0, len(s.Outputs))
	for name := range s.Outputs {
		outputs = append(outputs, name)
	}
	return outputs
}

// Agent helper methods

// IsPreBuilt returns true if this agent uses a pre-built configuration
func (a *Agent) IsPreBuilt() bool {
	return a.Uses != ""
}

// IsCustom returns true if this agent has a custom configuration
func (a *Agent) IsCustom() bool {
	return a.Model != ""
}

// Validate performs basic validation for the agent
func (a *Agent) Validate() error {
	// Must specify either a model or uses clause
	if !a.IsCustom() && !a.IsPreBuilt() {
		return fmt.Errorf("agent must specify either model or uses")
	}
	
	if a.IsCustom() && a.IsPreBuilt() {
		return fmt.Errorf("agent cannot specify both model and uses")
	}
	
	// Validate model if specified
	if a.IsCustom() {
		validModels := []string{
			"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo",
			"claude-3-opus", "claude-3-sonnet", "claude-3-haiku",
			"gemini-pro", "gemini-pro-vision",
		}
		if !contains(validModels, a.Model) {
			return fmt.Errorf("unsupported model: %s", a.Model)
		}
	}
	
	// Validate temperature range
	if a.Temperature != nil && (*a.Temperature < 0 || *a.Temperature > 2) {
		return fmt.Errorf("temperature must be between 0 and 2, got %f", *a.Temperature)
	}
	
	// Validate top_p range
	if a.TopP != nil && (*a.TopP < 0 || *a.TopP > 1) {
		return fmt.Errorf("top_p must be between 0 and 1, got %f", *a.TopP)
	}
	
	// Validate max_tokens
	if a.MaxTokens != nil && *a.MaxTokens < 1 {
		return fmt.Errorf("max_tokens must be positive, got %d", *a.MaxTokens)
	}
	
	return nil
}

// HasTool checks if the agent has a specific tool
func (a *Agent) HasTool(name string) bool {
	for _, tool := range a.Tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

// GetTool retrieves a tool by name
func (a *Agent) GetTool(name string) (*Tool, bool) {
	for _, tool := range a.Tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return nil, false
}

// ListTools returns a list of all tool names
func (a *Agent) ListTools() []string {
	names := make([]string, len(a.Tools))
	for i, tool := range a.Tools {
		names[i] = tool.Name
	}
	return names
}

// InputParam helper methods

// IsRequired returns true if the input parameter is required
func (ip *InputParam) IsRequired() bool {
	return ip.Required
}

// HasDefault returns true if the input parameter has a default value
func (ip *InputParam) HasDefault() bool {
	return ip.Default != nil
}

// GetTypeString returns the type as a string, defaulting to "string" if not specified
func (ip *InputParam) GetTypeString() string {
	if ip.Type == "" {
		return "string"
	}
	return ip.Type
}

// Tool helper methods

// IsOfficialTool returns true if this is an official Lacquer tool
func (t *Tool) IsOfficialTool() bool {
	return strings.HasPrefix(t.Uses, "lacquer/")
}

// IsScript returns true if this tool is a script
func (t *Tool) IsScript() bool {
	return t.Script != ""
}

// IsMCPTool returns true if this tool uses MCP
func (t *Tool) IsMCPTool() bool {
	return t.MCPServer != ""
}

// GetToolType returns the type of tool as a string
func (t *Tool) GetToolType() string {
	switch {
	case t.IsOfficialTool():
		return "official"
	case t.IsScript():
		return "script"
	case t.IsMCPTool():
		return "mcp"
	default:
		return "unknown"
	}
}

// Utility functions

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}