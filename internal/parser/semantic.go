package parser

import (
	"fmt"
	"strings"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/expression"
)

// SemanticValidator provides comprehensive semantic validation for workflows
type SemanticValidator struct {
}

// NewSemanticValidator creates a new semantic validator
func NewSemanticValidator() *SemanticValidator {
	return &SemanticValidator{}
}

// ValidateWorkflow performs comprehensive semantic validation
func (sv *SemanticValidator) ValidateWorkflow(w *ast.Workflow) *ast.ValidationResult {
	result := &ast.ValidationResult{Valid: true}

	// First perform basic AST validation
	astValidator := ast.NewValidator(w)
	astResult := astValidator.ValidateWorkflow()
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

	sv.buildValidationContext(ctx)

	sv.validateStepDependencies(ctx, result)
	sv.validateControlFlow(ctx, result)
	sv.validateResourceUsage(ctx, result)

	return result
}

// validationContext holds all workflow elements for cross-validation
type validationContext struct {
	workflow  *ast.Workflow
	agents    map[string]*ast.Agent
	stepIDs   map[string]int
	inputs    map[string]*ast.InputParam
	outputs   map[string]interface{}
	variables map[string]bool
}

// buildValidationContext populates the validation context
func (sv *SemanticValidator) buildValidationContext(ctx *validationContext) {
	w := ctx.workflow

	if w.Agents != nil {
		for name, agent := range w.Agents {
			ctx.agents[name] = agent
		}
	}

	if w.Workflow != nil && w.Workflow.Steps != nil {
		for i, step := range w.Workflow.Steps {
			if step.ID != "" {
				ctx.stepIDs[step.ID] = i
			}
		}
	}

	if w.Inputs != nil {
		for name, param := range w.Inputs {
			ctx.inputs[name] = param
		}
	}

	if w.Workflow != nil && w.Workflow.State != nil {
		for name := range w.Workflow.State {
			ctx.variables[fmt.Sprintf("state.%s", name)] = true
		}
	}

	if w.Workflow != nil && w.Workflow.Steps != nil {
		for _, step := range w.Workflow.Steps {
			if step.Outputs != nil {
				for key := range step.Outputs {
					ctx.variables[fmt.Sprintf("steps.%s.%s", step.ID, key)] = true
				}
			}
			ctx.variables[fmt.Sprintf("steps.%s.output", step.ID)] = true

			commonOutputs := []string{"result", "data", "content", "findings", "summary", "analysis"}
			for _, output := range commonOutputs {
				ctx.variables[fmt.Sprintf("steps.%s.%s", step.ID, output)] = true
			}
		}
	}

	ctx.variables["metadata.name"] = true
	ctx.variables["metadata.version"] = true
	ctx.variables["metadata.description"] = true
	ctx.variables["env."] = true
}

// validateStepDependencies checks for circular dependencies and forward references
func (sv *SemanticValidator) validateStepDependencies(ctx *validationContext, result *ast.ValidationResult) {
	if ctx.workflow.Workflow == nil || ctx.workflow.Workflow.Steps == nil {
		return
	}

	steps := ctx.workflow.Workflow.Steps

	dependencies := make(map[string][]string)

	for _, step := range steps {
		deps := sv.extractStepDependencies(step)
		dependencies[step.ID] = deps
	}

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

	for i, step := range steps {
		deps := dependencies[step.ID]
		for _, depStepID := range deps {
			// Allow self-references - a step can reference itself
			if depStepID == step.ID {
				continue
			}
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

	if step.Prompt != "" {
		deps = append(deps, sv.extractVariableReferences(step.Prompt)...)
	}

	if step.Condition != "" {
		deps = append(deps, sv.extractVariableReferences(step.Condition)...)
	}

	if step.SkipIf != "" {
		deps = append(deps, sv.extractVariableReferences(step.SkipIf)...)
	}

	if step.With != nil {
		for _, value := range step.With {
			if str, ok := value.(string); ok {
				deps = append(deps, sv.extractVariableReferences(str)...)
			}
		}
	}

	if step.Updates != nil {
		for _, value := range step.Updates {
			if str, ok := value.(string); ok {
				deps = append(deps, sv.extractVariableReferences(str)...)
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
		if dep == stepID {
			continue
		}

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

// extractAllVariableReferences extracts all {{ variable }} references from text
func (sv *SemanticValidator) extractAllVariableReferences(text string) []string {
	matches := expression.VariablePattern.FindAllStringSubmatch(text, -1)

	var variables []string
	for _, match := range matches {
		if len(match) > 1 {
			variable := strings.TrimSpace(match[2])
			variables = append(variables, variable)
		}
	}

	return variables
}

// extractVariableReferences extracts step ID references from variable references
func (sv *SemanticValidator) extractVariableReferences(text string) []string {
	variables := sv.extractAllVariableReferences(text)
	var stepIDs []string

	for _, variable := range variables {
		if strings.HasPrefix(variable, "steps.") {
			parts := strings.Split(variable, ".")
			if len(parts) >= 2 {
				stepIDs = append(stepIDs, parts[1])
			}
		}
	}

	return stepIDs
}

// validateControlFlow validates control flow logic and conditions
func (sv *SemanticValidator) validateControlFlow(ctx *validationContext, result *ast.ValidationResult) {
	if ctx.workflow.Workflow == nil || ctx.workflow.Workflow.Steps == nil {
		return
	}

	for i, step := range ctx.workflow.Workflow.Steps {
		stepPath := fmt.Sprintf("workflow.steps[%d]", i)

		if step.Condition != "" {
			sv.validateConditionSyntax(step.Condition, stepPath+".condition", result)
		}

		if step.SkipIf != "" {
			sv.validateConditionSyntax(step.SkipIf, stepPath+".skip_if", result)
		}
	}
}

// validateConditionSyntax validates the syntax of condition expressions
func (sv *SemanticValidator) validateConditionSyntax(condition, path string, result *ast.ValidationResult) {
	condition = strings.TrimSpace(condition)

	if condition == "" {
		result.AddError(path, "condition cannot be empty")
		return
	}

	if !sv.hasBalancedParentheses(condition) {
		result.AddError(path, "unbalanced parentheses in condition")
	}
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

// validateResourceUsage validates resource usage patterns and limits
func (sv *SemanticValidator) validateResourceUsage(ctx *validationContext, result *ast.ValidationResult) {
	expensiveSteps := 0

	if ctx.workflow.Workflow != nil && ctx.workflow.Workflow.Steps != nil {
		for i, step := range ctx.workflow.Workflow.Steps {
			stepPath := fmt.Sprintf("workflow.steps[%d]", i)

			if step.Agent != "" && step.Prompt != "" {
				expensiveSteps++
			}

			if step.Agent != "" {
				if agent, exists := ctx.agents[step.Agent]; exists {
					if agent.MaxTokens != nil && *agent.MaxTokens > 4000 {
						result.AddWarning(stepPath+".agent", "high token limit detected - consider breaking into smaller steps")
					}
				}
			}
		}
	}
}

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
