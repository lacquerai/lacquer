package runtime

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// TemplateEngine handles variable interpolation and template rendering
type TemplateEngine struct {
	// Variable pattern: {{ variable.path }}
	variablePattern *regexp.Regexp
}

// NewTemplateEngine creates a new template engine
func NewTemplateEngine() *TemplateEngine {
	// Pattern to match {{ variable.path }} with optional whitespace
	pattern := regexp.MustCompile(`\{\{\s*([^}]+)\s*\}\}`)

	return &TemplateEngine{
		variablePattern: pattern,
	}
}

// Render renders a template string with variables from the execution context
func (te *TemplateEngine) Render(template string, execCtx *ExecutionContext) (string, error) {
	if template == "" {
		return "", nil
	}

	// Find all variable references
	matches := te.variablePattern.FindAllStringSubmatch(template, -1)
	if len(matches) == 0 {
		return template, nil // No variables to substitute
	}

	result := template

	// Replace each variable reference
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		fullMatch := match[0]                  // Full match including {{ }}
		varPath := strings.TrimSpace(match[1]) // Variable path

		// Resolve the variable value
		value, err := te.resolveVariable(varPath, execCtx)
		if err != nil {
			return "", fmt.Errorf("failed to resolve variable %s: %w", varPath, err)
		}

		// Convert value to string
		strValue := te.valueToString(value)

		// Replace in result
		result = strings.ReplaceAll(result, fullMatch, strValue)
	}

	return result, nil
}

// resolveVariable resolves a variable path from the execution context
func (te *TemplateEngine) resolveVariable(varPath string, execCtx *ExecutionContext) (interface{}, error) {
	if varPath == "" {
		return "", nil
	}

	// Split the path into components
	parts := strings.Split(varPath, ".")

	// Handle different variable scopes
	switch parts[0] {
	case "inputs":
		if len(parts) < 2 {
			return nil, fmt.Errorf("inputs variable requires a parameter name")
		}
		value, exists := execCtx.GetInput(parts[1])
		if !exists {
			return nil, fmt.Errorf("input parameter %s not found", parts[1])
		}
		return te.resolveNestedPath(value, parts[2:])

	case "state":
		if len(parts) < 2 {
			return nil, fmt.Errorf("state variable requires a key name")
		}
		value, exists := execCtx.GetState(parts[1])
		if !exists {
			return nil, fmt.Errorf("state variable %s not found", parts[1])
		}
		return te.resolveNestedPath(value, parts[2:])

	case "steps":
		if len(parts) < 3 {
			return nil, fmt.Errorf("steps variable requires step_id and field name")
		}
		stepID := parts[1]
		field := parts[2]

		result, exists := execCtx.GetStepResult(stepID)
		if !exists {
			return nil, fmt.Errorf("step %s not found", stepID)
		}

		return te.resolveStepField(result, field, parts[3:])

	case "metadata":
		if len(parts) < 2 {
			return nil, fmt.Errorf("metadata variable requires a field name")
		}
		value, exists := execCtx.GetMetadata(parts[1])
		if !exists {
			return nil, fmt.Errorf("metadata field %s not found", parts[1])
		}
		return te.resolveNestedPath(value, parts[2:])

	case "env":
		if len(parts) < 2 {
			return nil, fmt.Errorf("env variable requires a variable name")
		}
		value, exists := execCtx.GetEnvironment(parts[1])
		if !exists {
			return "", nil // Environment variables default to empty string
		}
		return value, nil

	case "workflow":
		return te.resolveWorkflowVariable(parts[1:], execCtx)

	default:
		return nil, fmt.Errorf("unknown variable scope: %s", parts[0])
	}
}

// resolveStepField resolves a field from a step result
func (te *TemplateEngine) resolveStepField(result *StepResult, field string, remaining []string) (interface{}, error) {
	var value interface{}

	switch field {
	case "status":
		value = string(result.Status)
	case "duration":
		value = result.Duration.String()
	case "output":
		value = result.Response
	case "error":
		if result.Error != nil {
			value = result.Error.Error()
		} else {
			value = ""
		}
	case "success":
		value = result.Status == StepStatusCompleted
	case "failed":
		value = result.Status == StepStatusFailed
	default:
		// Try to find field in output
		if result.Output != nil {
			if outputValue, exists := result.Output[field]; exists {
				value = outputValue
			} else {
				return nil, fmt.Errorf("step field %s not found", field)
			}
		} else {
			return nil, fmt.Errorf("step field %s not found", field)
		}
	}

	return te.resolveNestedPath(value, remaining)
}

// resolveWorkflowVariable resolves workflow-level variables
func (te *TemplateEngine) resolveWorkflowVariable(parts []string, execCtx *ExecutionContext) (interface{}, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("workflow variable requires a field name")
	}

	switch parts[0] {
	case "run_id":
		return execCtx.RunID, nil
	case "start_time":
		return execCtx.StartTime.Format("2006-01-02T15:04:05Z07:00"), nil
	case "step_index":
		return execCtx.CurrentStepIndex + 1, nil // 1-based for templates
	case "total_steps":
		return execCtx.TotalSteps, nil
	case "completed_at":
		if execCtx.IsCompleted() {
			return execCtx.StartTime.Add(time.Since(execCtx.StartTime)).Format("2006-01-02T15:04:05Z07:00"), nil
		}
		return "", nil
	default:
		return nil, fmt.Errorf("unknown workflow variable: %s", parts[0])
	}
}

// resolveNestedPath resolves a nested path within a value
func (te *TemplateEngine) resolveNestedPath(value interface{}, path []string) (interface{}, error) {
	current := value

	for _, key := range path {
		switch val := current.(type) {
		case map[string]interface{}:
			if next, exists := val[key]; exists {
				current = next
			} else {
				return nil, fmt.Errorf("field %s not found", key)
			}
		case map[interface{}]interface{}:
			// Handle YAML-style maps
			if next, exists := val[key]; exists {
				current = next
			} else {
				return nil, fmt.Errorf("field %s not found", key)
			}
		default:
			return nil, fmt.Errorf("cannot access field %s on non-object value", key)
		}
	}

	return current, nil
}

// valueToString converts a value to its string representation
func (te *TemplateEngine) valueToString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []interface{}:
		// Convert arrays to comma-separated strings
		strs := make([]string, len(v))
		for i, item := range v {
			strs[i] = te.valueToString(item)
		}
		return strings.Join(strs, ", ")
	default:
		return fmt.Sprintf("%v", value)
	}
}

// ValidateTemplate validates that a template string has valid syntax
func (te *TemplateEngine) ValidateTemplate(template string) error {
	if template == "" {
		return nil
	}

	// Find all variable references
	matches := te.variablePattern.FindAllStringSubmatch(template, -1)

	// Check each variable reference for basic syntax
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		varPath := strings.TrimSpace(match[1])
		if varPath == "" {
			return fmt.Errorf("empty variable reference in template")
		}

		// Basic path validation
		parts := strings.Split(varPath, ".")
		for _, part := range parts {
			if part == "" {
				return fmt.Errorf("invalid variable path: %s", varPath)
			}
		}

		// Validate scope
		scope := parts[0]
		validScopes := []string{"inputs", "state", "steps", "metadata", "env", "workflow"}
		isValidScope := false
		for _, validScope := range validScopes {
			if scope == validScope {
				isValidScope = true
				break
			}
		}

		if !isValidScope {
			return fmt.Errorf("invalid variable scope: %s", scope)
		}
	}

	return nil
}
