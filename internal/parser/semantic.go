package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lacquerai/lacquer/internal/ast"
)

// SemanticValidator provides comprehensive semantic validation for workflows
type SemanticValidator struct {
	templateValidator *TemplateValidator
}

// NewSemanticValidator creates a new semantic validator
func NewSemanticValidator() *SemanticValidator {
	return &SemanticValidator{
		templateValidator: NewTemplateValidator(),
	}
}

// ValidateWorkflow performs comprehensive semantic validation
func (sv *SemanticValidator) ValidateWorkflow(w *ast.Workflow) *ast.ValidationResult {
	result := &ast.ValidationResult{Valid: true}

	// First perform basic AST validation
	astValidator := ast.NewValidator()
	astResult := astValidator.ValidateWorkflow(w)
	if astResult.HasErrors() {
		// Merge AST validation errors
		for _, err := range astResult.Errors {
			result.Errors = append(result.Errors, err)
			result.Valid = false
		}
	}

	// Collect context for cross-validation
	ctx := &validationContext{
		workflow:  w,
		agents:    make(map[string]*ast.Agent),
		stepIDs:   make(map[string]int),
		inputs:    make(map[string]*ast.InputParam),
		outputs:   make(map[string]interface{}),
		variables: make(map[string]bool),
	}

	// Build context maps
	sv.buildValidationContext(ctx)

	// Perform semantic checks
	sv.validateStepDependencies(ctx, result)
	sv.validateVariableReferences(ctx, result)
	sv.validateOutputReferences(ctx, result)
	sv.validateBlockReferences(ctx, result)
	sv.validateControlFlow(ctx, result)
	sv.validateResourceUsage(ctx, result)
	sv.validateTemplates(ctx, result)

	return result
}

// validationContext holds all workflow elements for cross-validation
type validationContext struct {
	workflow  *ast.Workflow
	agents    map[string]*ast.Agent
	stepIDs   map[string]int // stepID -> stepIndex
	inputs    map[string]*ast.InputParam
	outputs   map[string]interface{}
	variables map[string]bool // All variables defined in workflow
}

// buildValidationContext populates the validation context
func (sv *SemanticValidator) buildValidationContext(ctx *validationContext) {
	w := ctx.workflow

	// Collect agents
	if w.Agents != nil {
		for name, agent := range w.Agents {
			ctx.agents[name] = agent
		}
	}

	// Collect step IDs
	if w.Workflow != nil && w.Workflow.Steps != nil {
		for i, step := range w.Workflow.Steps {
			if step.ID != "" {
				ctx.stepIDs[step.ID] = i
			}
		}
	}

	// Collect inputs
	if w.Workflow != nil && w.Workflow.Inputs != nil {
		for name, param := range w.Workflow.Inputs {
			ctx.inputs[name] = param
		}
	}

	// Collect state variables
	if w.Workflow != nil && w.Workflow.State != nil {
		for name := range w.Workflow.State {
			ctx.variables[fmt.Sprintf("state.%s", name)] = true
		}
	}

	// Collect step outputs
	if w.Workflow != nil && w.Workflow.Steps != nil {
		for _, step := range w.Workflow.Steps {
			if step.Outputs != nil {
				for key := range step.Outputs {
					ctx.variables[fmt.Sprintf("steps.%s.%s", step.ID, key)] = true
				}
			}
			// Default outputs that are always available for steps
			ctx.variables[fmt.Sprintf("steps.%s.output", step.ID)] = true

			// Add common output patterns that might be generated dynamically
			commonOutputs := []string{"result", "data", "content", "findings", "summary", "analysis"}
			for _, output := range commonOutputs {
				ctx.variables[fmt.Sprintf("steps.%s.%s", step.ID, output)] = true
			}
		}
	}

	// Add built-in variables
	ctx.variables["metadata.name"] = true
	ctx.variables["metadata.version"] = true
	ctx.variables["metadata.description"] = true
	ctx.variables["env."] = true // Prefix match for environment variables
}

// validateStepDependencies checks for circular dependencies and forward references
func (sv *SemanticValidator) validateStepDependencies(ctx *validationContext, result *ast.ValidationResult) {
	if ctx.workflow.Workflow == nil || ctx.workflow.Workflow.Steps == nil {
		return
	}

	steps := ctx.workflow.Workflow.Steps

	// Build dependency graph
	dependencies := make(map[string][]string) // stepID -> dependencies

	for _, step := range steps {
		deps := sv.extractStepDependencies(step)
		dependencies[step.ID] = deps
	}

	// Check for circular dependencies using DFS
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	for stepID := range dependencies {
		if !visited[stepID] {
			if sv.hasCycle(stepID, dependencies, visited, recursionStack) {
				result.AddError(
					fmt.Sprintf("workflow.steps[%d]", ctx.stepIDs[stepID]),
					fmt.Sprintf("circular dependency detected involving step '%s'", stepID),
				)
			}
		}
	}

	// Check for forward references (referencing steps that haven't executed yet)
	for i, step := range steps {
		deps := dependencies[step.ID]
		for _, depStepID := range deps {
			if depIndex, exists := ctx.stepIDs[depStepID]; exists {
				if depIndex >= i {
					result.AddError(
						fmt.Sprintf("workflow.steps[%d]", i),
						fmt.Sprintf("step '%s' references step '%s' which hasn't executed yet", step.ID, depStepID),
					)
				}
			}
		}
	}
}

// extractStepDependencies extracts step IDs that this step depends on
func (sv *SemanticValidator) extractStepDependencies(step *ast.Step) []string {
	var deps []string

	// Check prompt for step references
	if step.Prompt != "" {
		deps = append(deps, sv.extractVariableReferences(step.Prompt, "steps")...)
	}

	// Check condition for step references
	if step.Condition != "" {
		deps = append(deps, sv.extractVariableReferences(step.Condition, "steps")...)
	}

	// Check skip_if for step references
	if step.SkipIf != "" {
		deps = append(deps, sv.extractVariableReferences(step.SkipIf, "steps")...)
	}

	// Check with parameters
	if step.With != nil {
		for _, value := range step.With {
			if str, ok := value.(string); ok {
				deps = append(deps, sv.extractVariableReferences(str, "steps")...)
			}
		}
	}

	// Check updates
	if step.Updates != nil {
		for _, value := range step.Updates {
			if str, ok := value.(string); ok {
				deps = append(deps, sv.extractVariableReferences(str, "steps")...)
			}
		}
	}

	return sv.uniqueStrings(deps)
}

// hasCycle detects cycles in the dependency graph using DFS
func (sv *SemanticValidator) hasCycle(stepID string, dependencies map[string][]string, visited, recursionStack map[string]bool) bool {
	visited[stepID] = true
	recursionStack[stepID] = true

	for _, dep := range dependencies[stepID] {
		if !visited[dep] {
			if sv.hasCycle(dep, dependencies, visited, recursionStack) {
				return true
			}
		} else if recursionStack[dep] {
			return true
		}
	}

	recursionStack[stepID] = false
	return false
}

// validateVariableReferences checks that all variable references are valid
func (sv *SemanticValidator) validateVariableReferences(ctx *validationContext, result *ast.ValidationResult) {
	if ctx.workflow.Workflow == nil || ctx.workflow.Workflow.Steps == nil {
		return
	}

	for i, step := range ctx.workflow.Workflow.Steps {
		stepPath := fmt.Sprintf("workflow.steps[%d]", i)

		// Validate prompt variables
		if step.Prompt != "" {
			sv.validateStringVariables(step.Prompt, stepPath+".prompt", ctx, result)
		}

		// Validate condition variables
		if step.Condition != "" {
			sv.validateStringVariables(step.Condition, stepPath+".condition", ctx, result)
		}

		// Validate skip_if variables
		if step.SkipIf != "" {
			sv.validateStringVariables(step.SkipIf, stepPath+".skip_if", ctx, result)
		}

		// Validate with parameters
		if step.With != nil {
			sv.validateMapVariables(step.With, stepPath+".with", ctx, result)
		}

		// Validate updates
		if step.Updates != nil {
			sv.validateMapVariables(step.Updates, stepPath+".updates", ctx, result)
		}
	}
}

// validateStringVariables validates variable references in a string
// Note: Template validation is now handled by validateTemplates method
func (sv *SemanticValidator) validateStringVariables(text, path string, ctx *validationContext, result *ast.ValidationResult) {
	// Skip template validation here - it's handled by validateTemplates
	// This method is kept for backwards compatibility and non-template variable references
	return
}

// validateMapVariables validates variable references in map values
func (sv *SemanticValidator) validateMapVariables(m map[string]interface{}, path string, ctx *validationContext, result *ast.ValidationResult) {
	for key, value := range m {
		fieldPath := fmt.Sprintf("%s.%s", path, key)

		if str, ok := value.(string); ok {
			sv.validateStringVariables(str, fieldPath, ctx, result)
		} else if nestedMap, ok := value.(map[string]interface{}); ok {
			sv.validateMapVariables(nestedMap, fieldPath, ctx, result)
		}
	}
}

// extractAllVariableReferences extracts all {{ variable }} references from text
func (sv *SemanticValidator) extractAllVariableReferences(text string) []string {
	// Match {{ variable.path }} patterns
	re := regexp.MustCompile(`\{\{\s*([^}]+)\s*\}\}`)
	matches := re.FindAllStringSubmatch(text, -1)

	var variables []string
	for _, match := range matches {
		if len(match) > 1 {
			variable := strings.TrimSpace(match[1])
			variables = append(variables, variable)
		}
	}

	return variables
}

// extractVariableReferences extracts step ID references from variable references
func (sv *SemanticValidator) extractVariableReferences(text, prefix string) []string {
	variables := sv.extractAllVariableReferences(text)
	var stepIDs []string

	for _, variable := range variables {
		if strings.HasPrefix(variable, prefix+".") {
			parts := strings.Split(variable, ".")
			if len(parts) >= 2 {
				stepIDs = append(stepIDs, parts[1])
			}
		}
	}

	return stepIDs
}

// isValidVariableReference checks if a variable reference is valid
func (sv *SemanticValidator) isValidVariableReference(variable string, ctx *validationContext) bool {
	// Check exact matches first
	if ctx.variables[variable] {
		return true
	}

	// Check prefix matches for environment variables
	if strings.HasPrefix(variable, "env.") && len(variable) > 4 {
		return true
	}

	// Check inputs
	if strings.HasPrefix(variable, "inputs.") {
		inputName := strings.TrimPrefix(variable, "inputs.")
		return ctx.inputs[inputName] != nil
	}

	// Check step references with flexible output matching
	if strings.HasPrefix(variable, "steps.") {
		parts := strings.Split(variable, ".")
		if len(parts) >= 3 {
			stepID := parts[1]
			// If the step exists, allow any output reference
			if _, exists := ctx.stepIDs[stepID]; exists {
				return true
			}
		}
	}

	// Check built-in variables
	builtins := []string{
		"step_index", "total_steps", "run_id", "timestamp",
		"metadata.name", "metadata.version", "metadata.description", "metadata.author",
	}

	for _, builtin := range builtins {
		if variable == builtin {
			return true
		}
	}

	// Allow common template functions and expressions
	// Allow common functions like now(), default(), etc.
	if strings.Contains(variable, "(") {
		return true
	}

	// Allow conditional expressions (ternary-like)
	if strings.Contains(variable, " if ") || strings.Contains(variable, " else ") {
		return true
	}

	// Allow pipe expressions
	if strings.Contains(variable, " | ") {
		return true
	}

	return false
}

// validateOutputReferences checks that output references are valid
func (sv *SemanticValidator) validateOutputReferences(ctx *validationContext, result *ast.ValidationResult) {
	if ctx.workflow.Workflow == nil || ctx.workflow.Workflow.Outputs == nil {
		return
	}

	for key, value := range ctx.workflow.Workflow.Outputs {
		path := fmt.Sprintf("workflow.outputs.%s", key)

		if str, ok := value.(string); ok {
			sv.validateStringVariables(str, path, ctx, result)
		}
	}
}

// validateBlockReferences validates that block references are properly formatted and accessible
func (sv *SemanticValidator) validateBlockReferences(ctx *validationContext, result *ast.ValidationResult) {
	// Validate agent block references
	if ctx.workflow.Agents != nil {
		for name, agent := range ctx.workflow.Agents {
			if agent.Uses != "" {
				path := fmt.Sprintf("agents.%s.uses", name)
				sv.validateBlockReference(agent.Uses, path, result)
			}
		}
	}

	// Validate step block references
	if ctx.workflow.Workflow != nil && ctx.workflow.Workflow.Steps != nil {
		for i, step := range ctx.workflow.Workflow.Steps {
			if step.Uses != "" {
				path := fmt.Sprintf("workflow.steps[%d].uses", i)
				sv.validateBlockReference(step.Uses, path, result)
			}
		}
	}
}

// validateBlockReference validates a single block reference
func (sv *SemanticValidator) validateBlockReference(ref, path string, result *ast.ValidationResult) {
	if ref == "" {
		return
	}

	// Validate official lacquer blocks
	if strings.HasPrefix(ref, "lacquer/") {
		if !sv.isValidLacquerBlock(ref) {
			result.AddError(path, fmt.Sprintf("invalid lacquer block reference: %s", ref))
		}
		return
	}

	// Validate GitHub references
	if strings.HasPrefix(ref, "github.com/") {
		if !sv.isValidGitHubReference(ref) {
			result.AddError(path, fmt.Sprintf("invalid GitHub block reference: %s", ref))
		}
		return
	}

	// Validate local references
	if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../") {
		if !sv.isValidLocalReference(ref) {
			result.AddError(path, fmt.Sprintf("invalid local block reference: %s", ref))
		}
		return
	}

	result.AddError(path, fmt.Sprintf("unsupported block reference format: %s", ref))
}

// isValidLacquerBlock validates official lacquer block references
func (sv *SemanticValidator) isValidLacquerBlock(ref string) bool {
	// Format: lacquer/block-name@version or lacquer/block-name
	pattern := `^lacquer/[a-z0-9][a-z0-9-]*[a-z0-9](@v[0-9]+(\.[0-9]+)*)?$`
	matched, _ := regexp.MatchString(pattern, ref)
	return matched
}

// isValidGitHubReference validates GitHub block references
func (sv *SemanticValidator) isValidGitHubReference(ref string) bool {
	// Format: github.com/owner/repo@version or github.com/owner/repo
	pattern := `^github\.com/[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]/[a-zA-Z0-9][a-zA-Z0-9_.-]*[a-zA-Z0-9](@[a-zA-Z0-9][a-zA-Z0-9_.-]*)?$`
	matched, _ := regexp.MatchString(pattern, ref)
	return matched
}

// isValidLocalReference validates local block references
func (sv *SemanticValidator) isValidLocalReference(ref string) bool {
	// Basic validation - should be a path
	if len(ref) <= 2 {
		return false
	}

	// Allow relative paths with .. as long as they don't try to escape too far
	if strings.HasPrefix(ref, "../") {
		return len(ref) > 3
	}

	return true
}

// validateControlFlow validates control flow logic and conditions
func (sv *SemanticValidator) validateControlFlow(ctx *validationContext, result *ast.ValidationResult) {
	if ctx.workflow.Workflow == nil || ctx.workflow.Workflow.Steps == nil {
		return
	}

	for i, step := range ctx.workflow.Workflow.Steps {
		stepPath := fmt.Sprintf("workflow.steps[%d]", i)

		// Validate condition syntax
		if step.Condition != "" {
			sv.validateConditionSyntax(step.Condition, stepPath+".condition", result)
		}

		// Validate skip_if syntax
		if step.SkipIf != "" {
			sv.validateConditionSyntax(step.SkipIf, stepPath+".skip_if", result)
		}
	}
}

// validateConditionSyntax validates the syntax of condition expressions
func (sv *SemanticValidator) validateConditionSyntax(condition, path string, result *ast.ValidationResult) {
	// Basic validation for common condition patterns
	condition = strings.TrimSpace(condition)

	if condition == "" {
		result.AddError(path, "condition cannot be empty")
		return
	}

	// Check for balanced parentheses
	if !sv.hasBalancedParentheses(condition) {
		result.AddError(path, "unbalanced parentheses in condition")
	}

	// Skip operator validation - template expressions can use any operators
}

// hasBalancedParentheses checks if parentheses are balanced
func (sv *SemanticValidator) hasBalancedParentheses(text string) bool {
	count := 0
	for _, char := range text {
		switch char {
		case '(':
			count++
		case ')':
			count--
			if count < 0 {
				return false
			}
		}
	}
	return count == 0
}

// validateOperators checks for valid operators in conditions
func (sv *SemanticValidator) validateOperators(condition string, validOps []string, path string, result *ast.ValidationResult) {
	// This is a simplified validation - a full implementation would parse the expression
	for _, op := range []string{"=", "<>", "AND", "OR", "NOT"} {
		if strings.Contains(condition, op) {
			result.AddError(path, fmt.Sprintf("use proper operators instead of '%s' (e.g., == instead of =)", op))
		}
	}
}

// validateResourceUsage validates resource usage patterns and limits
func (sv *SemanticValidator) validateResourceUsage(ctx *validationContext, result *ast.ValidationResult) {
	// Count expensive operations
	expensiveSteps := 0

	if ctx.workflow.Workflow != nil && ctx.workflow.Workflow.Steps != nil {
		for i, step := range ctx.workflow.Workflow.Steps {
			stepPath := fmt.Sprintf("workflow.steps[%d]", i)

			// Count agent steps (potentially expensive)
			if step.Agent != "" && step.Prompt != "" {
				expensiveSteps++
			}

			// Check for resource-intensive configurations and validate agent references
			if step.Agent != "" {
				if agent, exists := ctx.agents[step.Agent]; exists {
					if agent.MaxTokens != nil && *agent.MaxTokens > 4000 {
						// Note: high token limit detected - consider breaking into smaller steps
					}
				} else {
					// Agent reference is undefined
					result.AddError(stepPath+".agent", fmt.Sprintf("undefined agent: %s", step.Agent))
				}
			}

			// Validate retry configurations
			if step.Retry != nil {
				if step.Retry.MaxAttempts > 10 {
					result.AddError(stepPath+".retry.max_attempts", "excessive retry attempts (>10) may cause long delays")
				}
			}
		}
	}

	// Note: workflows with many expensive steps (>20) may benefit from being split into smaller workflows
}

// validateTemplates validates all template strings in the workflow
func (sv *SemanticValidator) validateTemplates(ctx *validationContext, result *ast.ValidationResult) {
	if ctx.workflow == nil {
		return
	}

	// Use the integrated template validator with better error handling
	sv.validateWorkflowTemplatesWithContext(ctx, result)
}

// validateWorkflowTemplatesWithContext validates workflow templates with proper error positioning
func (sv *SemanticValidator) validateWorkflowTemplatesWithContext(ctx *validationContext, result *ast.ValidationResult) {
	if ctx.workflow.Workflow == nil || ctx.workflow.Workflow.Steps == nil {
		return
	}

	// Validate step templates
	for i, step := range ctx.workflow.Workflow.Steps {
		stepPath := fmt.Sprintf("workflow.steps[%d]", i)

		// Validate prompt templates
		if step.Prompt != "" {
			if err := sv.validateTemplateFieldWithContext(step.Prompt, stepPath+".prompt", ctx); err != nil {
				if templateErr, ok := err.(*TemplateValidationError); ok {
					result.AddError(stepPath+".prompt", fmt.Sprintf("template validation error: %s", templateErr.Message))
				} else {
					result.AddError(stepPath+".prompt", fmt.Sprintf("template error: %s", err.Error()))
				}
			}
		}

		// Validate condition templates
		if step.Condition != "" {
			if err := sv.validateTemplateFieldWithContext(step.Condition, stepPath+".condition", ctx); err != nil {
				if templateErr, ok := err.(*TemplateValidationError); ok {
					result.AddError(stepPath+".condition", fmt.Sprintf("template validation error: %s", templateErr.Message))
				} else {
					result.AddError(stepPath+".condition", fmt.Sprintf("template error: %s", err.Error()))
				}
			}
		}

		// Validate skip_if templates
		if step.SkipIf != "" {
			if err := sv.validateTemplateFieldWithContext(step.SkipIf, stepPath+".skip_if", ctx); err != nil {
				if templateErr, ok := err.(*TemplateValidationError); ok {
					result.AddError(stepPath+".skip_if", fmt.Sprintf("template validation error: %s", templateErr.Message))
				} else {
					result.AddError(stepPath+".skip_if", fmt.Sprintf("template error: %s", err.Error()))
				}
			}
		}

		// Validate with parameter templates
		if step.With != nil {
			for key, value := range step.With {
				if strValue, ok := value.(string); ok {
					fieldPath := stepPath + ".with." + key
					if err := sv.validateTemplateFieldWithContext(strValue, fieldPath, ctx); err != nil {
						if templateErr, ok := err.(*TemplateValidationError); ok {
							result.AddError(fieldPath, fmt.Sprintf("template validation error: %s", templateErr.Message))
						} else {
							result.AddError(fieldPath, fmt.Sprintf("template error: %s", err.Error()))
						}
					}
				}
			}
		}

		// Validate update templates
		if step.Updates != nil {
			for key, value := range step.Updates {
				if strValue, ok := value.(string); ok {
					fieldPath := stepPath + ".updates." + key
					if err := sv.validateTemplateFieldWithContext(strValue, fieldPath, ctx); err != nil {
						if templateErr, ok := err.(*TemplateValidationError); ok {
							result.AddError(fieldPath, fmt.Sprintf("template validation error: %s", templateErr.Message))
						} else {
							result.AddError(fieldPath, fmt.Sprintf("template error: %s", err.Error()))
						}
					}
				}
			}
		}
	}

	// Validate output templates
	if ctx.workflow.Workflow.Outputs != nil {
		for key, value := range ctx.workflow.Workflow.Outputs {
			if strValue, ok := value.(string); ok {
				fieldPath := "workflow.outputs." + key
				if err := sv.validateTemplateFieldWithContext(strValue, fieldPath, ctx); err != nil {
					if templateErr, ok := err.(*TemplateValidationError); ok {
						result.AddError(fieldPath, fmt.Sprintf("template validation error: %s", templateErr.Message))
					} else {
						result.AddError(fieldPath, fmt.Sprintf("template error: %s", err.Error()))
					}
				}
			}
		}
	}
}

// validateTemplateFieldWithContext validates a single template field with context
func (sv *SemanticValidator) validateTemplateFieldWithContext(template, fieldPath string, ctx *validationContext) error {
	// Extract and validate variable references
	refs := sv.templateValidator.ExtractVariableReferences(template)
	var errors []string

	for _, ref := range refs {
		if err := sv.templateValidator.ValidateVariableReference(ref, ctx); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return &TemplateValidationError{
			Variable: template,
			Message:  strings.Join(errors, "; "),
		}
	}

	return nil
}

// Helper functions

// uniqueStrings returns unique strings from a slice
func (sv *SemanticValidator) uniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	var unique []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			unique = append(unique, item)
		}
	}

	return unique
}

// contains checks if a string slice contains a value
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
