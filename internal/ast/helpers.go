package ast

import (
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

// IsScriptStep returns true if this is a script execution step
func (s *Step) IsScriptStep() bool {
	return s.Run != ""
}

// IsContainerStep returns true if this is a container execution step
func (s *Step) IsContainerStep() bool {
	return s.Container != ""
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
	case s.IsScriptStep():
		return "script"
	case s.IsContainerStep():
		return "container"
	default:
		return "unknown"
	}
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
	return t.MCPServer != nil
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
