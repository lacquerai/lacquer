package expression

import (
	"context"
	"testing"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Type System Tests ---

func TestTypeSystem(t *testing.T) {
	t.Run("Primitive Types", func(t *testing.T) {
		assert.Equal(t, "string", String.String())
		assert.Equal(t, "number", Number.String())
		assert.Equal(t, "bool", Bool.String())
		assert.Equal(t, "null", Null.String())

		assert.True(t, String.IsPrimitive())
		assert.True(t, String.IsNullable())
		assert.True(t, String.Equals(String))
		assert.False(t, String.Equals(Number))
	})

	t.Run("Collection Types", func(t *testing.T) {
		listType := List(String)
		assert.Equal(t, "list(string)", listType.String())
		assert.False(t, listType.IsPrimitive())
		assert.True(t, listType.IsNullable())

		mapType := Map(Number)
		assert.Equal(t, "map(number)", mapType.String())

		assert.True(t, listType.Equals(List(String)))
		assert.False(t, listType.Equals(List(Number)))
	})

	t.Run("Type Compatibility", func(t *testing.T) {
		assert.True(t, IsTypeCompatible(String, String))
		assert.True(t, IsTypeCompatible(Null, String))
		assert.True(t, IsTypeCompatible(Dynamic, String))
		assert.True(t, IsTypeCompatible(String, Dynamic))
		assert.False(t, IsTypeCompatible(String, Number))

		assert.True(t, IsTypeCompatible(List(String), List(String)))
		assert.False(t, IsTypeCompatible(List(String), List(Number)))
	})
}

// --- Value Tests ---

func TestValues(t *testing.T) {
	t.Run("String Values", func(t *testing.T) {
		v := StringVal("hello")
		assert.Equal(t, String, v.Type())
		assert.False(t, v.IsNull())
		assert.Equal(t, "hello", v.GoValue())
		assert.Equal(t, `"hello"`, v.String())

		str, err := v.(*stringValue).AsString()
		assert.NoError(t, err)
		assert.Equal(t, "hello", str)

		nullStr := NullStringVal()
		assert.True(t, nullStr.IsNull())
		assert.Nil(t, nullStr.GoValue())
	})

	t.Run("Number Values", func(t *testing.T) {
		v := NumberVal(42.5)
		assert.Equal(t, Number, v.Type())
		assert.False(t, v.IsNull())
		assert.Equal(t, 42.5, v.GoValue())
		assert.Equal(t, "42.5", v.String())

		v2 := IntVal(42)
		assert.Equal(t, "42", v2.String())
	})

	t.Run("Boolean Values", func(t *testing.T) {
		v := BoolVal(true)
		assert.Equal(t, Bool, v.Type())
		assert.False(t, v.IsNull())
		assert.Equal(t, true, v.GoValue())
		assert.Equal(t, "true", v.String())

		v2 := BoolVal(false)
		assert.Equal(t, "false", v2.String())
	})

	t.Run("List Values", func(t *testing.T) {
		elements := []Value{
			StringVal("a"),
			StringVal("b"),
			StringVal("c"),
		}
		v := ListVal(elements)

		assert.Equal(t, "list(string)", v.Type().String())
		assert.False(t, v.IsNull())
		assert.Equal(t, 3, v.(*listValue).Length())
		assert.Equal(t, `["a", "b", "c"]`, v.String())

		elem, err := v.(*listValue).Index(1)
		assert.NoError(t, err)
		assert.Equal(t, StringVal("b"), elem)

		_, err = v.(*listValue).Index(10)
		assert.Error(t, err)
	})

	t.Run("Map Values", func(t *testing.T) {
		attrs := map[string]Value{
			"name": StringVal("John"),
			"age":  NumberVal(30),
		}
		v := MapVal(attrs)

		assert.Equal(t, "map(dynamic)", v.Type().String())
		assert.False(t, v.IsNull())

		val, err := v.(*mapValue).GetAttr("name")
		assert.NoError(t, err)
		assert.Equal(t, StringVal("John"), val)

		assert.True(t, v.(*mapValue).HasAttr("age"))
		assert.False(t, v.(*mapValue).HasAttr("missing"))

		names := v.(*mapValue).AttrNames()
		assert.ElementsMatch(t, []string{"name", "age"}, names)
	})

	t.Run("Value Conversion", func(t *testing.T) {
		// GoToValue conversion
		assert.Equal(t, StringVal("test"), GoToValue("test"))
		assert.Equal(t, NumberVal(42), GoToValue(42))
		assert.Equal(t, BoolVal(true), GoToValue(true))
		assert.True(t, GoToValue(nil).IsNull())

		// Complex conversions
		list := GoToValue([]interface{}{"a", "b", "c"})
		assert.Equal(t, 3, list.(*listValue).Length())

		mapVal := GoToValue(map[string]interface{}{
			"key": "value",
		})
		val, _ := mapVal.(*mapValue).GetAttr("key")
		assert.Equal(t, StringVal("value"), val)
	})
}

// --- Tokenizer Tests ---

func TestTokenizer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []TokenType
	}{
		{
			name:  "Simple expression",
			input: "a + b",
			expected: []TokenType{
				TokenIdent, TokenPlus, TokenIdent, TokenEOF,
			},
		},
		{
			name:  "Comparison",
			input: "x >= 10",
			expected: []TokenType{
				TokenIdent, TokenGe, TokenNumber, TokenEOF,
			},
		},
		{
			name:  "String literal",
			input: `"hello world"`,
			expected: []TokenType{
				TokenString, TokenEOF,
			},
		},
		{
			name:  "Function call",
			input: "contains(text, 'test')",
			expected: []TokenType{
				TokenIdent, TokenLParen, TokenIdent, TokenComma, TokenString, TokenRParen, TokenEOF,
			},
		},
		{
			name:  "Ternary",
			input: "a ? b : c",
			expected: []TokenType{
				TokenIdent, TokenQuestion, TokenIdent, TokenColon, TokenIdent, TokenEOF,
			},
		},
		{
			name:  "List literal",
			input: "[1, 2, 3]",
			expected: []TokenType{
				TokenLBracket, TokenNumber, TokenComma, TokenNumber, TokenComma, TokenNumber, TokenRBracket, TokenEOF,
			},
		},
		{
			name:  "Map literal",
			input: `{"key": "value"}`,
			expected: []TokenType{
				TokenLBrace, TokenString, TokenColon, TokenString, TokenRBrace, TokenEOF,
			},
		},
		{
			name:  "Keywords",
			input: "true false null for in if",
			expected: []TokenType{
				TokenTrue, TokenFalse, TokenNull, TokenFor, TokenIn, TokenIf, TokenEOF,
			},
		},
		{
			name:  "Comments",
			input: "a + // comment\nb",
			expected: []TokenType{
				TokenIdent, TokenPlus, TokenIdent, TokenEOF,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewTokenizer(tt.input)
			var types []TokenType

			for {
				tok := tokenizer.Next()
				types = append(types, tok.Type)
				if tok.Type == TokenEOF || tok.Type == TokenError {
					break
				}
			}

			assert.Equal(t, tt.expected, types)
		})
	}
}

// --- Parser Tests ---

func TestParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		// Literals
		{
			name:     "Number literal",
			input:    "42",
			expected: "42",
		},
		{
			name:     "String literal",
			input:    `"hello"`,
			expected: `"hello"`,
		},
		{
			name:     "Boolean true",
			input:    "true",
			expected: "true",
		},
		{
			name:     "Boolean false",
			input:    "false",
			expected: "false",
		},
		{
			name:     "Null",
			input:    "null",
			expected: "null",
		},

		// Binary expressions
		{
			name:     "Addition",
			input:    "1 + 2",
			expected: "(1 + 2)",
		},
		{
			name:     "Comparison",
			input:    "a > b",
			expected: "(a > b)",
		},
		{
			name:     "Logical AND",
			input:    "a && b",
			expected: "(a && b)",
		},

		// Unary expressions
		{
			name:     "Negation",
			input:    "-42",
			expected: "(-42)",
		},
		{
			name:     "Not",
			input:    "!true",
			expected: "(!true)",
		},

		// Complex expressions
		{
			name:     "Ternary",
			input:    "a ? b : c",
			expected: "(a ? b : c)",
		},
		{
			name:     "Attribute access",
			input:    "obj.attr",
			expected: "obj.attr",
		},
		{
			name:     "Index access",
			input:    "arr[0]",
			expected: "arr[0]",
		},
		{
			name:     "Function call",
			input:    "func(a, b)",
			expected: "func(a, b)",
		},
		{
			name:     "List literal",
			input:    "[1, 2, 3]",
			expected: "[1, 2, 3]",
		},
		{
			name:     "Map literal",
			input:    `{"a": 1, "b": 2}`,
			expected: `{"a": 1, "b": 2}`,
		},
		{
			name:     "For expression",
			input:    "[for x in list : x * 2]",
			expected: "[for x in list : (x * 2)]",
		},
		{
			name:     "Complex nested",
			input:    "obj.list[0].value",
			expected: "obj.list[0].value",
		},

		// Error cases
		{
			name:    "Invalid syntax",
			input:   "a +",
			wantErr: true,
		},
		{
			name:    "Mismatched parens",
			input:   "(a + b",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := ParseExpression(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, expr.String())
		})
	}
}

// --- Evaluator Tests ---

func TestEvaluator(t *testing.T) {
	// Create test context
	ctx := NewEvalContext()

	// Add variables
	ctx.SetVariable("a", NumberVal(10))
	ctx.SetVariable("b", NumberVal(5))
	ctx.SetVariable("str", StringVal("hello"))
	ctx.SetVariable("list", ListVal([]Value{
		NumberVal(1),
		NumberVal(2),
		NumberVal(3),
	}))
	ctx.SetVariable("obj", MapVal(map[string]Value{
		"name":  StringVal("test"),
		"value": NumberVal(42),
		"items": ListVal([]Value{
			MapVal(map[string]Value{
				"id": NumberVal(1),
			}),
		}),
	}))

	// Add functions
	RegisterBuiltinFunctions(ctx)

	tests := []struct {
		name     string
		expr     string
		expected interface{}
		wantErr  bool
	}{
		// Basic arithmetic
		{
			name:     "Addition",
			expr:     "a + b",
			expected: 15.0,
		},
		{
			name:     "Subtraction",
			expr:     "a - b",
			expected: 5.0,
		},
		{
			name:     "Multiplication",
			expr:     "a * 2",
			expected: 20.0,
		},
		{
			name:     "Division",
			expr:     "a / 2",
			expected: 5.0,
		},

		// Comparisons
		{
			name:     "Greater than",
			expr:     "a > b",
			expected: true,
		},
		{
			name:     "Less than",
			expr:     "a < b",
			expected: false,
		},
		{
			name:     "Equality",
			expr:     "a == 10",
			expected: true,
		},

		// Logical
		{
			name:     "AND true",
			expr:     "true && true",
			expected: true,
		},
		{
			name:     "AND false",
			expr:     "true && false",
			expected: false,
		},
		{
			name:     "OR",
			expr:     "false || true",
			expected: true,
		},
		{
			name:     "NOT",
			expr:     "!false",
			expected: true,
		},

		// Ternary
		{
			name:     "Ternary true",
			expr:     "a > b ? 'yes' : 'no'",
			expected: "yes",
		},
		{
			name:     "Ternary false",
			expr:     "a < b ? 'yes' : 'no'",
			expected: "no",
		},

		// String operations
		{
			name:     "String concatenation",
			expr:     "str + ' world'",
			expected: "hello world",
		},

		// Object access
		{
			name:     "Object attribute",
			expr:     "obj.name",
			expected: "test",
		},
		{
			name:     "Nested access",
			expr:     "obj.items[0].id",
			expected: 1.0,
		},

		// List operations
		{
			name:     "List index",
			expr:     "list[1]",
			expected: 2.0,
		},
		{
			name:     "List length",
			expr:     "length(list)",
			expected: 3.0,
		},

		// Function calls
		{
			name:     "Contains true",
			expr:     "contains(str, 'ello')",
			expected: true,
		},
		{
			name:     "Contains false",
			expr:     "contains(str, 'xyz')",
			expected: false,
		},
		{
			name:     "Upper",
			expr:     "upper(str)",
			expected: "HELLO",
		},
		{
			name:     "Format",
			expr:     "format('Hello {0}!', 'World')",
			expected: "Hello World!",
		},

		// List comprehension
		{
			name:     "Simple for expression",
			expr:     "[for x in list : x * 2]",
			expected: []interface{}{2.0, 4.0, 6.0},
		},

		// Null coalescing
		{
			name:     "Null coalescing with value",
			expr:     "a ?? 0",
			expected: 10.0,
		},
		{
			name:     "Null coalescing with null",
			expr:     "null ?? 'default'",
			expected: "default",
		},

		// Error cases
		{
			name:    "Undefined variable",
			expr:    "undefined",
			wantErr: true,
		},
		{
			name:    "Division by zero",
			expr:    "1 / 0",
			wantErr: true,
		},
		{
			name:    "Invalid function",
			expr:    "invalid()",
			wantErr: true,
		},
	}

	evaluator := NewEvaluator(ctx)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := ParseExpression(tt.expr)
			require.NoError(t, err, "Failed to parse expression")

			val, err := evaluator.Eval(expr)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, val.GoValue())
		})
	}
}

// --- Integration Tests ---

func TestExpressionEvaluatorV2(t *testing.T) {
	// Create workflow context
	workflow := &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name:        "test-workflow",
			Description: "Test workflow",
			Author:      "Test",
		},
		Workflow: &ast.WorkflowDef{
			State: map[string]interface{}{
				"counter": 5,
				"enabled": true,
			},
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
				{ID: "step2", Agent: "agent2", Prompt: "test"},
			},
		},
	}

	inputs := map[string]interface{}{
		"count": 10,
		"name":  "test",
		"data": map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"id": 1, "value": "a"},
				map[string]interface{}{"id": 2, "value": "b"},
			},
		},
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, inputs)

	// Add a step result
	stepResult := &StepResult{
		StepID:   "step1",
		Status:   StepStatusCompleted,
		Response: "Step completed",
		Output: map[string]interface{}{
			"result": "success",
			"score":  85,
		},
		Duration: 5 * time.Second,
	}
	execCtx.SetStepResult("step1", stepResult)

	// Test with new evaluator
	evaluator := NewEvaluator(ctx)

	tests := []struct {
		name     string
		expr     string
		expected interface{}
		wantErr  bool
	}{
		// Variable access
		{
			name:     "Input variable",
			expr:     "inputs.count",
			expected: 10.0,
		},
		{
			name:     "State variable",
			expr:     "state.counter",
			expected: 5.0,
		},
		{
			name:     "Step result",
			expr:     "steps.step1.result",
			expected: "success",
		},
		{
			name:     "Nested data",
			expr:     "inputs.data.items[0].value",
			expected: "a",
		},

		// Expressions
		{
			name:     "Comparison",
			expr:     "inputs.count > 5",
			expected: true,
		},
		{
			name:     "Complex condition",
			expr:     "inputs.count > 5 && state.enabled",
			expected: true,
		},
		{
			name:     "Ternary",
			expr:     "steps.step1.success ? 'passed' : 'failed'",
			expected: "passed",
		},

		// Functions
		{
			name:     "Contains function",
			expr:     "contains(inputs.name, 'test')",
			expected: true,
		},
		{
			name:     "Success function",
			expr:     "success()",
			expected: true,
		},
		{
			name:     "Format function",
			expr:     "format('Step {0} completed', 'step1')",
			expected: "Step step1 completed",
		},

		// Workflow variables
		{
			name:     "Workflow step index",
			expr:     "workflow.step_index",
			expected: 1.0,
		},
		{
			name:     "Workflow total steps",
			expr:     "workflow.total_steps",
			expected: 2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expr, execCtx)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Template Engine V2 Tests ---

func TestTemplateEngineV2(t *testing.T) {
	// Create workflow context
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	inputs := map[string]interface{}{
		"name":    "Alice",
		"count":   10,
		"enabled": true,
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, inputs)

	// Add step result
	stepResult := &StepResult{
		StepID:   "step1",
		Status:   StepStatusCompleted,
		Response: "Hello from step 1",
		Output: map[string]interface{}{
			"message": "Success",
		},
	}
	execCtx.SetStepResult("step1", stepResult)

	engine := NewTemplateEngineV2(true)

	tests := []struct {
		name     string
		template string
		expected string
		wantErr  bool
	}{
		{
			name:     "Simple variable",
			template: "Hello, {{ inputs.name }}!",
			expected: "Hello, Alice!",
		},
		{
			name:     "Multiple variables",
			template: "Name: {{ inputs.name }}, Count: {{ inputs.count }}",
			expected: "Name: Alice, Count: 10",
		},
		{
			name:     "Expression",
			template: "Status: {{ inputs.enabled ? 'active' : 'inactive' }}",
			expected: "Status: active",
		},
		{
			name:     "Function call",
			template: "Message: {{ upper(inputs.name) }}",
			expected: "Message: ALICE",
		},
		{
			name:     "Step output",
			template: "Step said: {{ steps.step1.message }}",
			expected: "Step said: Success",
		},
		{
			name:     "Complex expression",
			template: "Result: {{ inputs.count > 5 && steps.step1.success ? 'Good' : 'Bad' }}",
			expected: "Result: Good",
		},
		{
			name:     "No expressions",
			template: "Just plain text",
			expected: "Just plain text",
		},
		{
			name:     "Empty template",
			template: "",
			expected: "",
		},
		{
			name:     "Invalid expression",
			template: "Error: {{ invalid expression }}",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Render(tt.template, execCtx)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Performance Comparison Test ---

func BenchmarkExpressionEvaluation(b *testing.B) {
	// Setup context
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			Steps: []*ast.Step{
				{ID: "step1", Agent: "agent1", Prompt: "test"},
			},
		},
	}

	inputs := map[string]interface{}{
		"data": map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"value": 42},
			},
		},
	}

	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, inputs)

	expression := "inputs.data.items[0].value > 40"

	b.Run("OldSystem", func(b *testing.B) {
		evaluator := NewExpressionEvaluator()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := evaluator.Evaluate(expression, execCtx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("NewSystem", func(b *testing.B) {
		evaluator := NewExpressionEvaluatorV2(true)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := evaluator.Evaluate(expression, execCtx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
