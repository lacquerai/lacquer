package expression

import (
	"context"
	"io"
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpressionEvaluator_BasicOperators(t *testing.T) {
	evaluator := NewExpressionEvaluator()
	execCtx := createTestExecutionContext()

	testCases := []struct {
		name       string
		expression string
		expected   interface{}
	}{
		// Comparison operators
		{
			name:       "Equal true",
			expression: "5 == 5",
			expected:   true,
		},
		{
			name:       "Equal false",
			expression: "5 == 3",
			expected:   false,
		},
		{
			name:       "Not equal true",
			expression: "5 != 3",
			expected:   true,
		},
		{
			name:       "Not equal false",
			expression: "5 != 5",
			expected:   false,
		},
		{
			name:       "Greater than true",
			expression: "5 > 3",
			expected:   true,
		},
		{
			name:       "Greater than false",
			expression: "3 > 5",
			expected:   false,
		},
		{
			name:       "Less than true",
			expression: "3 < 5",
			expected:   true,
		},
		{
			name:       "Less than false",
			expression: "5 < 3",
			expected:   false,
		},
		{
			name:       "Greater than or equal true",
			expression: "5 >= 5",
			expected:   true,
		},
		{
			name:       "Less than or equal true",
			expression: "3 <= 5",
			expected:   true,
		},

		// Logical operators
		{
			name:       "Logical AND true",
			expression: "true && true",
			expected:   true,
		},
		{
			name:       "Logical AND false",
			expression: "true && false",
			expected:   false,
		},
		{
			name:       "Logical OR true",
			expression: "true || false",
			expected:   true,
		},
		{
			name:       "Logical OR false",
			expression: "false || false",
			expected:   false,
		},
		{
			name:       "Logical NOT true",
			expression: "!false",
			expected:   true,
		},
		{
			name:       "Logical NOT false",
			expression: "!true",
			expected:   false,
		},

		// Arithmetic operators
		{
			name:       "Addition",
			expression: "3 + 2",
			expected:   float64(5),
		},
		{
			name:       "Subtraction",
			expression: "5 - 2",
			expected:   float64(3),
		},
		{
			name:       "Multiplication",
			expression: "3 * 4",
			expected:   float64(12),
		},
		{
			name:       "Division",
			expression: "10 / 2",
			expected:   float64(5),
		},
		{
			name:       "Modulo",
			expression: "10 % 3",
			expected:   float64(1),
		},

		// String operations
		{
			name:       "String equality",
			expression: "'hello' == 'hello'",
			expected:   true,
		},
		{
			name:       "String concatenation",
			expression: "'hello' + ' world'",
			expected:   "hello world",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tc.expression, execCtx)
			require.NoError(t, err, "Expression evaluation failed: %s", tc.expression)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExpressionEvaluator_TernaryOperator(t *testing.T) {
	evaluator := NewExpressionEvaluator()
	execCtx := createTestExecutionContext()

	testCases := []struct {
		name       string
		expression string
		expected   interface{}
	}{
		{
			name:       "Ternary true",
			expression: "true ? 'yes' : 'no'",
			expected:   "yes",
		},
		{
			name:       "Ternary false",
			expression: "false ? 'yes' : 'no'",
			expected:   "no",
		},
		{
			name:       "Ternary with comparison",
			expression: "5 > 3 ? 'bigger' : 'smaller'",
			expected:   "bigger",
		},
		{
			name:       "Nested ternary",
			expression: "true ? (false ? 'a' : 'b') : 'c'",
			expected:   "b",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tc.expression, execCtx)
			require.NoError(t, err, "Expression evaluation failed: %s", tc.expression)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExpressionEvaluator_FunctionCalls(t *testing.T) {
	evaluator := NewExpressionEvaluator()
	execCtx := createTestExecutionContext()

	testCases := []struct {
		name       string
		expression string
		expected   interface{}
	}{
		{
			name:       "contains function true",
			expression: "contains('hello world', 'world')",
			expected:   true,
		},
		{
			name:       "contains function false",
			expression: "contains('hello world', 'xyz')",
			expected:   false,
		},
		{
			name:       "startsWith function true",
			expression: "startsWith('hello world', 'hello')",
			expected:   true,
		},
		{
			name:       "startsWith function false",
			expression: "startsWith('hello world', 'world')",
			expected:   false,
		},
		{
			name:       "endsWith function true",
			expression: "endsWith('hello world', 'world')",
			expected:   true,
		},
		{
			name:       "endsWith function false",
			expression: "endsWith('hello world', 'hello')",
			expected:   false,
		},
		{
			name:       "format function",
			expression: "format('Hello {0}!', 'world')",
			expected:   "Hello world!",
		},
		{
			name:       "join function",
			expression: "join('a,b,c', '|')",
			expected:   "a|b|c",
		},
		{
			name:       "success function",
			expression: "success()",
			expected:   true,
		},
		{
			name:       "always function",
			expression: "always()",
			expected:   true,
		},
		{
			name:       "cancelled function",
			expression: "cancelled()",
			expected:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tc.expression, execCtx)
			require.NoError(t, err, "Expression evaluation failed: %s", tc.expression)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExpressionEvaluator_VariableResolution(t *testing.T) {
	evaluator := NewExpressionEvaluator()
	execCtx := createTestExecutionContextWithData()

	testCases := []struct {
		name       string
		expression string
		expected   interface{}
	}{
		{
			name:       "Input variable in expression",
			expression: "inputs.count > 5",
			expected:   true,
		},
		{
			name:       "State variable in expression",
			expression: "state.enabled == true",
			expected:   true,
		},
		{
			name:       "String input variable",
			expression: "inputs.name == 'test'",
			expected:   true,
		},
		{
			name:       "Combined variables",
			expression: "inputs.count + state.counter",
			expected:   float64(15),
		},
		{
			name:       "Variable with function",
			expression: "contains(inputs.name, 'est')",
			expected:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tc.expression, execCtx)
			require.NoError(t, err, "Expression evaluation failed: %s", tc.expression)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExpressionEvaluator_ComplexExpressions(t *testing.T) {
	evaluator := NewExpressionEvaluator()
	execCtx := createTestExecutionContextWithData()

	testCases := []struct {
		name       string
		expression string
		expected   interface{}
	}{
		{
			name:       "Complex logical expression",
			expression: "inputs.count > 5 && state.enabled == true",
			expected:   true,
		},
		{
			name:       "Complex expression with parentheses",
			expression: "(inputs.count + state.counter) > 10",
			expected:   true,
		},
		{
			name:       "Complex ternary with functions",
			expression: "contains(inputs.name, 'test') ? 'found' : 'not found'",
			expected:   "found",
		},
		{
			name:       "Nested function calls",
			expression: "startsWith(format('Hello {0}', inputs.name), 'Hello')",
			expected:   true,
		},
		{
			name:       "Multiple operators",
			expression: "inputs.count * 2 + state.counter - 5",
			expected:   float64(20),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tc.expression, execCtx)
			require.NoError(t, err, "Expression evaluation failed: %s", tc.expression)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExpressionEvaluator_ErrorCases(t *testing.T) {
	evaluator := NewExpressionEvaluator()
	execCtx := createTestExecutionContext()

	testCases := []struct {
		name       string
		expression string
		expectErr  bool
	}{
		{
			name:       "Division by zero",
			expression: "10 / 0",
			expectErr:  true,
		},
		{
			name:       "Modulo by zero",
			expression: "10 % 0",
			expectErr:  true,
		},
		{
			name:       "Unknown function",
			expression: "unknownFunction()",
			expectErr:  true,
		},
		{
			name:       "Invalid syntax",
			expression: "5 +",
			expectErr:  true,
		},
		{
			name:       "Undefined variable",
			expression: "undefined.variable > 5",
			expectErr:  false,
		},
		{
			name:       "Mismatched parentheses",
			expression: "(5 + 3",
			expectErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := evaluator.Evaluate(tc.expression, execCtx)
			if tc.expectErr {
				assert.Error(t, err, "Expected error for expression: %s", tc.expression)
			} else {
				assert.NoError(t, err, "Unexpected error for expression: %s", tc.expression)
			}
		})
	}
}

func TestExpressionEvaluator_TypeConversions(t *testing.T) {
	evaluator := NewExpressionEvaluator()
	execCtx := createTestExecutionContext()

	testCases := []struct {
		name       string
		expression string
		expected   interface{}
	}{
		{
			name:       "String to number comparison",
			expression: "'5' == 5",
			expected:   true,
		},
		{
			name:       "Boolean to number",
			expression: "true + 1",
			expected:   float64(2),
		},
		{
			name:       "Number to boolean",
			expression: "!0",
			expected:   true,
		},
		{
			name:       "String concatenation with number",
			expression: "'Count: ' + 5",
			expected:   "Count: 5",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tc.expression, execCtx)
			require.NoError(t, err, "Expression evaluation failed: %s", tc.expression)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func createTestExecutionContext() *execcontext.ExecutionContext {
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	return execcontext.NewExecutionContext(execcontext.RunContext{
		Context: context.Background(),
		StdOut:  io.Discard,
		StdErr:  io.Discard,
	}, workflow, nil, "")
}

func createTestExecutionContextWithData() *execcontext.ExecutionContext {
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			State: map[string]interface{}{
				"counter": 5,
				"enabled": true,
			},
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	inputs := map[string]interface{}{
		"count": 10,
		"name":  "test",
	}

	return execcontext.NewExecutionContext(execcontext.RunContext{
		Context: context.Background(),
		StdOut:  io.Discard,
		StdErr:  io.Discard,
	}, workflow, inputs, "")
}
