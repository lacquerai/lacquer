package engine

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/lacquerai/lacquer/internal/ast"
)

// InputValidationError represents a validation error for a specific input field
type InputValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

// Error implements the error interface
func (e *InputValidationError) Error() string {
	return fmt.Sprintf("field '%s': %s", e.Field, e.Message)
}

// InputValidationResult holds the results of input validation
type InputValidationResult struct {
	Valid           bool                    `json:"valid"`
	Errors          []*InputValidationError `json:"errors,omitempty"`
	ProcessedInputs map[string]any          `json:"processed_inputs,omitempty"`
}

func (r *InputValidationResult) Error() string {
	return fmt.Sprintf("Valid: %t\nErrors: %v\nProcessedInputs: %v", r.Valid, r.Errors, r.ProcessedInputs)
}

// AddError adds a validation error
func (r *InputValidationResult) AddError(field, message string, value any) {
	r.Valid = false
	r.Errors = append(r.Errors, &InputValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// ValidateWorkflowInputs validates provided inputs against workflow input definitions
func ValidateWorkflowInputs(workflow *ast.Workflow, providedInputs map[string]any) *InputValidationResult {
	result := &InputValidationResult{
		Valid:           true,
		ProcessedInputs: make(map[string]any),
	}

	// If workflow has no input definitions, accept any inputs
	if workflow.Workflow.Inputs == nil {
		result.ProcessedInputs = providedInputs
		return result
	}

	// Process each defined input parameter
	for paramName, paramDef := range workflow.Workflow.Inputs {
		providedValue, hasValue := providedInputs[paramName]

		// Check if required parameter is missing
		if !hasValue {
			if paramDef.Required {
				result.AddError(paramName, "required field is missing", nil)
				continue
			}
			// Apply default value if available
			if paramDef.Default != nil {
				result.ProcessedInputs[paramName] = paramDef.Default
				continue
			}
			// Optional field without default - skip
			continue
		}

		// Validate and process the provided value
		processedValue, err := validateInputValue(paramName, providedValue, paramDef)
		if err != nil {
			result.AddError(paramName, err.Error(), providedValue)
		} else {
			result.ProcessedInputs[paramName] = processedValue
		}
	}

	// Check for unexpected input fields
	for inputName := range providedInputs {
		if _, defined := workflow.Workflow.Inputs[inputName]; !defined {
			result.AddError(inputName, "unexpected input field", providedInputs[inputName])
		}
	}

	return result
}

// validateInputValue validates a single input value against its parameter definition
func validateInputValue(fieldName string, value any, param *ast.InputParam) (any, error) {
	// If no type is specified, accept any value
	if param.Type == "" {
		return value, nil
	}

	// Validate and convert type
	convertedValue, err := convertAndValidateType(value, param.Type)
	if err != nil {
		return nil, fmt.Errorf("invalid type: expected %s, got %T", param.Type, value)
	}

	// Apply additional constraints based on type
	switch param.Type {
	case "string":
		return validateStringConstraints(convertedValue.(string), param)
	case "integer":
		return validateNumericConstraints(convertedValue, param)
	case "array":
		return validateArrayConstraints(convertedValue, param)
	case "boolean":
		return convertedValue, nil
	case "object":
		return convertedValue, nil
	default:
		return convertedValue, nil
	}
}

// convertAndValidateType converts and validates the basic type
func convertAndValidateType(value any, expectedType string) (any, error) {
	switch expectedType {
	case "string":
		switch v := value.(type) {
		case string:
			return v, nil
		case int, int32, int64, float32, float64:
			return fmt.Sprintf("%v", v), nil
		case bool:
			return strconv.FormatBool(v), nil
		default:
			return nil, fmt.Errorf("cannot convert %T to string", value)
		}

	case "integer":
		switch v := value.(type) {
		case int:
			return v, nil
		case int32:
			return int(v), nil
		case int64:
			return int(v), nil
		case float32:
			if v == float32(int(v)) {
				return int(v), nil
			}
			return nil, fmt.Errorf("float value %v cannot be converted to integer", v)
		case float64:
			if v == float64(int(v)) {
				return int(v), nil
			}
			return nil, fmt.Errorf("float value %v cannot be converted to integer", v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i, nil
			}
			return nil, fmt.Errorf("string value %q cannot be converted to integer", v)
		default:
			return nil, fmt.Errorf("cannot convert %T to integer", value)
		}

	case "boolean":
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			if b, err := strconv.ParseBool(v); err == nil {
				return b, nil
			}
			return nil, fmt.Errorf("string value %q cannot be converted to boolean", v)
		default:
			return nil, fmt.Errorf("cannot convert %T to boolean", value)
		}

	case "array":
		if reflect.TypeOf(value).Kind() == reflect.Slice {
			return value, nil
		}
		return nil, fmt.Errorf("expected array, got %T", value)

	case "object":
		if reflect.TypeOf(value).Kind() == reflect.Map {
			return value, nil
		}
		return nil, fmt.Errorf("expected object, got %T", value)

	default:
		return value, nil
	}
}

// validateStringConstraints validates string-specific constraints
func validateStringConstraints(value string, param *ast.InputParam) (string, error) {
	// Validate pattern
	if param.Pattern != "" {
		matched, err := regexp.MatchString(param.Pattern, value)
		if err != nil {
			return "", fmt.Errorf("invalid pattern regex: %v", err)
		}
		if !matched {
			return "", fmt.Errorf("value does not match required pattern: %s", param.Pattern)
		}
	}

	// Validate enum
	if len(param.Enum) > 0 {
		found := false
		for _, enumValue := range param.Enum {
			if enumValue == value {
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("value must be one of: %s", strings.Join(param.Enum, ", "))
		}
	}

	return value, nil
}

// validateNumericConstraints validates numeric constraints
func validateNumericConstraints(value any, param *ast.InputParam) (any, error) {
	var numValue float64

	switch v := value.(type) {
	case int:
		numValue = float64(v)
	case int32:
		numValue = float64(v)
	case int64:
		numValue = float64(v)
	case float32:
		numValue = float64(v)
	case float64:
		numValue = v
	default:
		return value, nil // Not a numeric type, skip numeric validation
	}

	// Validate minimum
	if param.Minimum != nil && numValue < *param.Minimum {
		return nil, fmt.Errorf("value %v is less than minimum %v", numValue, *param.Minimum)
	}

	// Validate maximum
	if param.Maximum != nil && numValue > *param.Maximum {
		return nil, fmt.Errorf("value %v is greater than maximum %v", numValue, *param.Maximum)
	}

	return value, nil
}

// validateArrayConstraints validates array-specific constraints
func validateArrayConstraints(value any, param *ast.InputParam) (any, error) {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("expected array, got %T", value)
	}

	length := rv.Len()

	// Validate minimum items
	if param.MinItems != nil && length < *param.MinItems {
		return nil, fmt.Errorf("array has %d items, minimum required is %d", length, *param.MinItems)
	}

	// Validate maximum items
	if param.MaxItems != nil && length > *param.MaxItems {
		return nil, fmt.Errorf("array has %d items, maximum allowed is %d", length, *param.MaxItems)
	}

	return value, nil
}
