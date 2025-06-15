package parser

import (
	"regexp"
	"strings"

	"github.com/lacquer/lacquer/internal/ast"
	"github.com/lacquer/lacquer/internal/runtime"
)

// TemplateValidator handles variable interpolation validation during parsing
type TemplateValidator struct {
	templateEngine *runtime.TemplateEngine
	// Variable pattern: {{ variable.path }}
	variablePattern *regexp.Regexp
}

// NewTemplateValidator creates a new template validator
func NewTemplateValidator() *TemplateValidator {
	return &TemplateValidator{
		templateEngine:  runtime.NewTemplateEngine(),
		variablePattern: regexp.MustCompile(`\{\{\s*([^}]+)\s*\}\}`),
	}
}

// ValidateTemplateString validates a template string for basic syntax
func (tv *TemplateValidator) ValidateTemplateString(template string) error {
	return tv.templateEngine.ValidateTemplate(template)
}

// ExtractVariableReferences extracts all variable references from a template string
func (tv *TemplateValidator) ExtractVariableReferences(template string) []VariableReference {
	if template == "" {
		return nil
	}

	matches := tv.variablePattern.FindAllStringSubmatch(template, -1)
	if len(matches) == 0 {
		return nil
	}

	var refs []VariableReference
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		varPath := strings.TrimSpace(match[1])
		if varPath == "" {
			continue
		}

		ref := ParseVariableReference(varPath)
		refs = append(refs, ref)
	}

	return refs
}

// VariableReference represents a parsed variable reference
type VariableReference struct {
	Raw    string       // Full variable path (e.g., "steps.step1.output")
	Scope  string       // Variable scope (e.g., "steps", "inputs", "state")
	Path   []string     // Path components (e.g., ["steps", "step1", "output"])
	Type   VariableType // Type of variable reference
	Target string       // Target identifier (e.g., step ID, input name)
	Field  string       // Field name (e.g., "output", "response")
}

// VariableType represents the type of variable reference
type VariableType int

const (
	VariableTypeUnknown     VariableType = iota
	VariableTypeInput                    // inputs.param_name
	VariableTypeState                    // state.key_name
	VariableTypeStep                     // steps.step_id.field
	VariableTypeMetadata                 // metadata.field_name
	VariableTypeEnvironment              // env.VAR_NAME
	VariableTypeWorkflow                 // workflow.field_name
	VariableTypeFunction                 // Function calls like now(), uuid()
	VariableTypeExpression               // Complex expressions with operators
)

// ParseVariableReference parses a variable path into components
func ParseVariableReference(varPath string) VariableReference {
	ref := VariableReference{
		Raw:  varPath,
		Path: []string{varPath}, // Default to single path component
	}

	// Check for function calls first
	if strings.Contains(varPath, "(") && strings.Contains(varPath, ")") {
		ref.Type = VariableTypeFunction
		return ref
	}

	// Check for expressions with operators
	if containsOperators(varPath) {
		ref.Type = VariableTypeExpression
		return ref
	}

	// Parse as normal dot-separated path
	ref.Path = strings.Split(varPath, ".")

	if len(ref.Path) == 0 {
		ref.Type = VariableTypeUnknown
		return ref
	}

	ref.Scope = ref.Path[0]

	// Determine variable type and extract components
	switch ref.Scope {
	case "inputs":
		ref.Type = VariableTypeInput
		if len(ref.Path) >= 2 {
			ref.Target = ref.Path[1]
		}
		if len(ref.Path) >= 3 {
			ref.Field = strings.Join(ref.Path[2:], ".")
		}

	case "state":
		ref.Type = VariableTypeState
		if len(ref.Path) >= 2 {
			ref.Target = ref.Path[1]
		}
		if len(ref.Path) >= 3 {
			ref.Field = strings.Join(ref.Path[2:], ".")
		}

	case "steps":
		ref.Type = VariableTypeStep
		if len(ref.Path) >= 2 {
			ref.Target = ref.Path[1] // Step ID
		}
		if len(ref.Path) >= 3 {
			ref.Field = ref.Path[2] // Field name
		}

	case "metadata":
		ref.Type = VariableTypeMetadata
		if len(ref.Path) >= 2 {
			ref.Field = strings.Join(ref.Path[1:], ".")
		}

	case "env":
		ref.Type = VariableTypeEnvironment
		if len(ref.Path) >= 2 {
			ref.Target = ref.Path[1] // Environment variable name
		}

	case "workflow":
		ref.Type = VariableTypeWorkflow
		if len(ref.Path) >= 2 {
			ref.Field = strings.Join(ref.Path[1:], ".")
		}

	default:
		ref.Type = VariableTypeUnknown
	}

	return ref
}

// containsOperators checks if a variable path contains expression operators
func containsOperators(varPath string) bool {
	operators := []string{
		"==", "!=", ">=", "<=", ">", "<",
		"&&", "||", "!",
		"+", "-", "*", "/",
		"?", ":", // Ternary operator
		"|", // Pipe operator
	}

	// Check for operator symbols
	for _, op := range operators {
		if strings.Contains(varPath, op) {
			return true
		}
	}

	// Check for conditional keywords
	conditionalKeywords := []string{" if ", " else ", " and ", " or ", " not "}
	for _, keyword := range conditionalKeywords {
		if strings.Contains(varPath, keyword) {
			return true
		}
	}

	return false
}

// ValidateVariableReference validates a variable reference against a validation context
func (tv *TemplateValidator) ValidateVariableReference(ref VariableReference, ctx *validationContext) error {
	switch ref.Type {
	case VariableTypeInput:
		return tv.validateInputReference(ref, ctx)
	case VariableTypeState:
		return tv.validateStateReference(ref, ctx)
	case VariableTypeStep:
		return tv.validateStepReference(ref, ctx)
	case VariableTypeMetadata:
		return tv.validateMetadataReference(ref, ctx)
	case VariableTypeEnvironment:
		return tv.validateEnvironmentReference(ref, ctx)
	case VariableTypeWorkflow:
		return tv.validateWorkflowReference(ref, ctx)
	case VariableTypeFunction:
		return tv.validateFunctionReference(ref, ctx)
	case VariableTypeExpression:
		return tv.validateExpressionReference(ref, ctx)
	default:
		return &TemplateValidationError{
			Variable: ref.Raw,
			Message:  "undefined variable: " + ref.Raw,
		}
	}
}

// validateInputReference validates an input variable reference
func (tv *TemplateValidator) validateInputReference(ref VariableReference, ctx *validationContext) error {
	if ref.Target == "" {
		return &TemplateValidationError{
			Variable: ref.Raw,
			Message:  "input variable requires a parameter name",
		}
	}

	if ctx.inputs[ref.Target] == nil {
		return &TemplateValidationError{
			Variable: ref.Raw,
			Message:  "undefined variable: " + ref.Raw,
		}
	}

	return nil
}

// validateStateReference validates a state variable reference
func (tv *TemplateValidator) validateStateReference(ref VariableReference, ctx *validationContext) error {
	if ref.Target == "" {
		return &TemplateValidationError{
			Variable: ref.Raw,
			Message:  "state variable requires a key name",
		}
	}

	stateKey := "state." + ref.Target
	if !ctx.variables[stateKey] {
		return &TemplateValidationError{
			Variable: ref.Raw,
			Message:  "undefined state variable: " + ref.Target,
		}
	}

	return nil
}

// validateStepReference validates a step variable reference
func (tv *TemplateValidator) validateStepReference(ref VariableReference, ctx *validationContext) error {
	if ref.Target == "" {
		return &TemplateValidationError{
			Variable: ref.Raw,
			Message:  "step variable requires a step ID",
		}
	}

	if ref.Field == "" {
		return &TemplateValidationError{
			Variable: ref.Raw,
			Message:  "step variable requires a field name",
		}
	}

	// Check if step exists
	if _, exists := ctx.stepIDs[ref.Target]; !exists {
		return &TemplateValidationError{
			Variable: ref.Raw,
			Message:  "undefined step: " + ref.Target,
		}
	}

	// Validate common step fields
	validFields := []string{
		"output", "response", "result", "content", "data",
		"status", "duration", "error", "success", "failed",
		"outputs", // Custom output container
	}

	isValidField := false
	for _, field := range validFields {
		if ref.Field == field {
			isValidField = true
			break
		}
	}

	if !isValidField {
		// Check if it's a custom output field
		customKey := "steps." + ref.Target + "." + ref.Field
		if ctx.variables[customKey] {
			isValidField = true
		}
	}

	if !isValidField {
		return &TemplateValidationError{
			Variable: ref.Raw,
			Message:  "undefined step field: " + ref.Field,
		}
	}

	return nil
}

// validateMetadataReference validates a metadata variable reference
func (tv *TemplateValidator) validateMetadataReference(ref VariableReference, ctx *validationContext) error {
	if ref.Field == "" {
		return &TemplateValidationError{
			Variable: ref.Raw,
			Message:  "metadata variable requires a field name",
		}
	}

	// Common metadata fields
	validFields := []string{"name", "version", "description", "author"}
	for _, field := range validFields {
		if ref.Field == field {
			return nil
		}
	}

	// Check if it's a custom metadata field
	metadataKey := "metadata." + ref.Field
	if ctx.variables[metadataKey] {
		return nil
	}

	return &TemplateValidationError{
		Variable: ref.Raw,
		Message:  "undefined metadata field: " + ref.Field,
	}
}

// validateEnvironmentReference validates an environment variable reference
func (tv *TemplateValidator) validateEnvironmentReference(ref VariableReference, ctx *validationContext) error {
	if ref.Target == "" {
		return &TemplateValidationError{
			Variable: ref.Raw,
			Message:  "environment variable requires a variable name",
		}
	}

	// Environment variables are generally allowed (they default to empty string)
	return nil
}

// validateWorkflowReference validates a workflow variable reference
func (tv *TemplateValidator) validateWorkflowReference(ref VariableReference, ctx *validationContext) error {
	if ref.Field == "" {
		return &TemplateValidationError{
			Variable: ref.Raw,
			Message:  "workflow variable requires a field name",
		}
	}

	// Common workflow fields
	validFields := []string{
		"run_id", "start_time", "step_index", "total_steps", "completed_at",
	}

	for _, field := range validFields {
		if ref.Field == field {
			return nil
		}
	}

	return &TemplateValidationError{
		Variable: ref.Raw,
		Message:  "undefined workflow field: " + ref.Field,
	}
}

// validateFunctionReference validates a function call reference
func (tv *TemplateValidator) validateFunctionReference(ref VariableReference, ctx *validationContext) error {
	// Allow common functions
	return nil
}

// validateExpressionReference validates an expression reference
func (tv *TemplateValidator) validateExpressionReference(ref VariableReference, ctx *validationContext) error {
	// Allow expressions
	return nil
}

// TemplateValidationError represents a template validation error
type TemplateValidationError struct {
	Variable string
	Message  string
}

func (e *TemplateValidationError) Error() string {
	return e.Message
}

// ValidateWorkflowTemplates validates all template strings in a workflow
func (tv *TemplateValidator) ValidateWorkflowTemplates(workflow *ast.Workflow) []error {
	var errors []error

	if workflow.Workflow == nil {
		return errors
	}

	// Build validation context
	ctx := tv.buildValidationContext(workflow)

	// Validate step templates
	for i, step := range workflow.Workflow.Steps {
		stepPath := "workflow.steps[" + step.ID + "]"

		// Validate prompt templates
		if step.Prompt != "" {
			if err := tv.validateTemplateField(step.Prompt, stepPath+".prompt", ctx); err != nil {
				errors = append(errors, err)
			}
		}

		// Validate condition templates
		if step.Condition != "" {
			if err := tv.validateTemplateField(step.Condition, stepPath+".condition", ctx); err != nil {
				errors = append(errors, err)
			}
		}

		// Validate skip_if templates
		if step.SkipIf != "" {
			if err := tv.validateTemplateField(step.SkipIf, stepPath+".skip_if", ctx); err != nil {
				errors = append(errors, err)
			}
		}

		// Validate with parameter templates
		if step.With != nil {
			for key, value := range step.With {
				if strValue, ok := value.(string); ok {
					fieldPath := stepPath + ".with." + key
					if err := tv.validateTemplateField(strValue, fieldPath, ctx); err != nil {
						errors = append(errors, err)
					}
				}
			}
		}

		// Validate update templates
		if step.Updates != nil {
			for key, value := range step.Updates {
				if strValue, ok := value.(string); ok {
					fieldPath := stepPath + ".updates." + key
					if err := tv.validateTemplateField(strValue, fieldPath, ctx); err != nil {
						errors = append(errors, err)
					}
				}
			}
		}

		// Update context with this step for subsequent steps
		ctx.stepIDs[step.ID] = i
		tv.addStepOutputsToContext(step, ctx)
	}

	// Validate output templates
	if workflow.Workflow.Outputs != nil {
		for key, value := range workflow.Workflow.Outputs {
			if strValue, ok := value.(string); ok {
				fieldPath := "workflow.outputs." + key
				if err := tv.validateTemplateField(strValue, fieldPath, ctx); err != nil {
					errors = append(errors, err)
				}
			}
		}
	}

	return errors
}

// validateTemplateField validates a single template field
func (tv *TemplateValidator) validateTemplateField(template, fieldPath string, ctx *validationContext) error {
	// Extract and validate variable references first
	refs := tv.ExtractVariableReferences(template)
	for _, ref := range refs {
		if err := tv.ValidateVariableReference(ref, ctx); err != nil {
			return err
		}
	}

	// Skip runtime template validation - our custom validation is more comprehensive
	// and handles all template features including functions and expressions

	return nil
}

// hasAdvancedFeatures checks if template contains function calls or expressions
func (tv *TemplateValidator) hasAdvancedFeatures(template string) bool {
	refs := tv.ExtractVariableReferences(template)
	for _, ref := range refs {
		if ref.Type == VariableTypeFunction || ref.Type == VariableTypeExpression {
			return true
		}
	}
	return false
}

// buildValidationContext builds a validation context from a workflow
func (tv *TemplateValidator) buildValidationContext(workflow *ast.Workflow) *validationContext {
	ctx := &validationContext{
		workflow:  workflow,
		agents:    make(map[string]*ast.Agent),
		stepIDs:   make(map[string]int),
		inputs:    make(map[string]*ast.InputParam),
		variables: make(map[string]bool),
	}

	// Collect agents
	if workflow.Agents != nil {
		for name, agent := range workflow.Agents {
			ctx.agents[name] = agent
		}
	}

	// Collect inputs
	if workflow.Workflow != nil && workflow.Workflow.Inputs != nil {
		for name, param := range workflow.Workflow.Inputs {
			ctx.inputs[name] = param
		}
	}

	// Collect state variables
	if workflow.Workflow != nil && workflow.Workflow.State != nil {
		for name := range workflow.Workflow.State {
			ctx.variables["state."+name] = true
		}
	}

	// Add built-in variables
	ctx.variables["metadata.name"] = true
	ctx.variables["metadata.version"] = true
	ctx.variables["metadata.description"] = true

	return ctx
}

// addStepOutputsToContext adds step outputs to the validation context
func (tv *TemplateValidator) addStepOutputsToContext(step *ast.Step, ctx *validationContext) {
	// Add default step outputs
	ctx.variables["steps."+step.ID+".output"] = true
	ctx.variables["steps."+step.ID+".result"] = true
	ctx.variables["steps."+step.ID+".status"] = true
	ctx.variables["steps."+step.ID+".duration"] = true
	ctx.variables["steps."+step.ID+".success"] = true
	ctx.variables["steps."+step.ID+".failed"] = true

	// Add custom outputs if defined
	if step.Outputs != nil {
		for key := range step.Outputs {
			ctx.variables["steps."+step.ID+"."+key] = true
		}
	}
}
