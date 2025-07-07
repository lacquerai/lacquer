package runtime

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Value represents any value in the expression system
type Value interface {
	Type() ValueType
	GoValue() interface{}
	String() string
	Equals(Value) bool
}

// ValueType represents the type of a value
type ValueType string

const (
	TypeNil     ValueType = "nil"
	TypeBool    ValueType = "bool"
	TypeNumber  ValueType = "number"
	TypeString  ValueType = "string"
	TypeList    ValueType = "list"
	TypeMap     ValueType = "map"
	TypeUnknown ValueType = "unknown"
)

// Basic value types

type NilValue struct{}

func (v NilValue) Type() ValueType         { return TypeNil }
func (v NilValue) GoValue() interface{}    { return nil }
func (v NilValue) String() string          { return "null" }
func (v NilValue) Equals(other Value) bool { return other.Type() == TypeNil }

type BoolValue struct {
	Val bool
}

func (v BoolValue) Type() ValueType      { return TypeBool }
func (v BoolValue) GoValue() interface{} { return v.Val }
func (v BoolValue) String() string {
	if v.Val {
		return "true"
	}
	return "false"
}
func (v BoolValue) Equals(other Value) bool {
	if other.Type() != TypeBool {
		return false
	}
	return v.Val == other.(BoolValue).Val
}

type NumberValue struct {
	Val float64
}

func (v NumberValue) Type() ValueType      { return TypeNumber }
func (v NumberValue) GoValue() interface{} { return v.Val }
func (v NumberValue) String() string       { return fmt.Sprintf("%g", v.Val) }
func (v NumberValue) Equals(other Value) bool {
	switch other.Type() {
	case TypeNumber:
		return v.Val == other.(NumberValue).Val
	case TypeString:
		// Attempt to convert string to number for comparison
		if f, err := strconv.ParseFloat(other.(StringValue).Val, 64); err == nil {
			return v.Val == f
		}
		return false
	default:
		return false
	}
}

type StringValue struct {
	Val string
}

func (v StringValue) Type() ValueType      { return TypeString }
func (v StringValue) GoValue() interface{} { return v.Val }
func (v StringValue) String() string       { return v.Val }
func (v StringValue) Equals(other Value) bool {
	switch other.Type() {
	case TypeString:
		return v.Val == other.(StringValue).Val
	case TypeNumber:
		// Attempt to convert string to number for comparison
		if f, err := strconv.ParseFloat(v.Val, 64); err == nil {
			return f == other.(NumberValue).Val
		}
		return false
	default:
		return false
	}
}

type ListValue struct {
	Vals []Value
}

func (v ListValue) Type() ValueType { return TypeList }
func (v ListValue) GoValue() interface{} {
	result := make([]interface{}, len(v.Vals))
	for i, val := range v.Vals {
		result[i] = val.GoValue()
	}
	return result
}
func (v ListValue) String() string {
	parts := make([]string, len(v.Vals))
	for i, val := range v.Vals {
		parts[i] = val.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
func (v ListValue) Equals(other Value) bool {
	if other.Type() != TypeList {
		return false
	}
	otherList := other.(ListValue)
	if len(v.Vals) != len(otherList.Vals) {
		return false
	}
	for i, val := range v.Vals {
		if !val.Equals(otherList.Vals[i]) {
			return false
		}
	}
	return true
}

type MapValue struct {
	Vals map[string]Value
}

func (v MapValue) Type() ValueType { return TypeMap }
func (v MapValue) GoValue() interface{} {
	result := make(map[string]interface{})
	for k, val := range v.Vals {
		result[k] = val.GoValue()
	}
	return result
}
func (v MapValue) String() string {
	parts := make([]string, 0, len(v.Vals))
	for k, val := range v.Vals {
		parts = append(parts, fmt.Sprintf("%s: %s", k, val.String()))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
func (v MapValue) Equals(other Value) bool {
	if other.Type() != TypeMap {
		return false
	}
	otherMap := other.(MapValue)
	if len(v.Vals) != len(otherMap.Vals) {
		return false
	}
	for k, val := range v.Vals {
		otherVal, ok := otherMap.Vals[k]
		if !ok || !val.Equals(otherVal) {
			return false
		}
	}
	return true
}

// ExpressionEvaluator handles expression evaluation
type ExpressionEvaluator struct {
	functions *FunctionRegistry
}

// NewExpressionEvaluator creates a new expression evaluator
func NewExpressionEvaluator() *ExpressionEvaluator {
	return &ExpressionEvaluator{
		functions: NewFunctionRegistry(),
	}
}

// Evaluate evaluates an expression
func (ee *ExpressionEvaluator) Evaluate(expression string, execCtx *ExecutionContext) (interface{}, error) {
	// Parse the expression
	expr, err := Parse(expression)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Create evaluation context
	evalCtx := &EvalContext{
		Variables: NewVariableScope(execCtx),
		Functions: ee.functions,
		ExecCtx:   execCtx,
	}

	// Evaluate the expression
	val, err := expr.Eval(evalCtx)
	if err != nil {
		return nil, fmt.Errorf("evaluation error: %w", err)
	}

	return val.GoValue(), nil
}

// EvalContext contains the context for expression evaluation
type EvalContext struct {
	Variables *VariableScope
	Functions *FunctionRegistry
	ExecCtx   *ExecutionContext
}

// VariableScope manages variable resolution
type VariableScope struct {
	execCtx *ExecutionContext
}

// NewVariableScope creates a new variable scope
func NewVariableScope(execCtx *ExecutionContext) *VariableScope {
	return &VariableScope{execCtx: execCtx}
}

// Get retrieves a variable value
func (vs *VariableScope) Get(name string) (Value, error) {
	// Handle special built-in variables
	switch name {
	case "true":
		return BoolValue{Val: true}, nil
	case "false":
		return BoolValue{Val: false}, nil
	case "null":
		return NilValue{}, nil
	}

	// Try to resolve as a context variable
	parts := strings.Split(name, ".")
	if len(parts) > 0 {
		switch parts[0] {
		case "inputs", "state", "steps", "metadata", "env", "workflow":
			resolver := &VariableResolver{}
			val, err := resolver.ResolveVariable(name, vs.execCtx)
			if err != nil {
				return nil, err
			}
			return GoToValue(val), nil
		}
	}

	return nil, fmt.Errorf("undefined variable: %s", name)
}

// GoToValue converts a Go value to an expression Value
func GoToValue(v interface{}) Value {
	if v == nil {
		return NilValue{}
	}

	switch val := v.(type) {
	case bool:
		return BoolValue{Val: val}
	case int:
		return NumberValue{Val: float64(val)}
	case int32:
		return NumberValue{Val: float64(val)}
	case int64:
		return NumberValue{Val: float64(val)}
	case float32:
		return NumberValue{Val: float64(val)}
	case float64:
		return NumberValue{Val: val}
	case string:
		return StringValue{Val: val}
	case []interface{}:
		result := make([]Value, len(val))
		for i, item := range val {
			result[i] = GoToValue(item)
		}
		return ListValue{Vals: result}
	case []string:
		result := make([]Value, len(val))
		for i, item := range val {
			result[i] = StringValue{Val: item}
		}
		return ListValue{Vals: result}
	case map[string]interface{}:
		result := make(map[string]Value)
		for k, v := range val {
			result[k] = GoToValue(v)
		}
		return MapValue{Vals: result}
	case map[interface{}]interface{}:
		result := make(map[string]Value)
		for k, v := range val {
			result[fmt.Sprintf("%v", k)] = GoToValue(v)
		}
		return MapValue{Vals: result}
	default:
		// For unknown types, convert to string
		return StringValue{Val: fmt.Sprintf("%v", v)}
	}
}

// Expression types

type Expression interface {
	Eval(*EvalContext) (Value, error)
}

// LiteralExpr represents a literal value
type LiteralExpr struct {
	Value Value
}

func (e *LiteralExpr) Eval(ctx *EvalContext) (Value, error) {
	return e.Value, nil
}

// VariableExpr represents a variable reference
type VariableExpr struct {
	Name string
}

func (e *VariableExpr) Eval(ctx *EvalContext) (Value, error) {
	return ctx.Variables.Get(e.Name)
}

// BinaryOpExpr represents a binary operation
type BinaryOpExpr struct {
	Left  Expression
	Op    string
	Right Expression
}

func (e *BinaryOpExpr) Eval(ctx *EvalContext) (Value, error) {
	left, err := e.Left.Eval(ctx)
	if err != nil {
		return nil, err
	}

	// Short-circuit evaluation for logical operators
	if e.Op == "&&" {
		if !ToBool(left) {
			return BoolValue{Val: false}, nil
		}
	} else if e.Op == "||" {
		if ToBool(left) {
			return BoolValue{Val: true}, nil
		}
	}

	right, err := e.Right.Eval(ctx)
	if err != nil {
		return nil, err
	}

	switch e.Op {
	case "==":
		return BoolValue{Val: left.Equals(right)}, nil
	case "!=":
		return BoolValue{Val: !left.Equals(right)}, nil
	case "<":
		return BoolValue{Val: ToNumber(left) < ToNumber(right)}, nil
	case ">":
		return BoolValue{Val: ToNumber(left) > ToNumber(right)}, nil
	case "<=":
		return BoolValue{Val: ToNumber(left) <= ToNumber(right)}, nil
	case ">=":
		return BoolValue{Val: ToNumber(left) >= ToNumber(right)}, nil
	case "&&":
		return BoolValue{Val: ToBool(left) && ToBool(right)}, nil
	case "||":
		return BoolValue{Val: ToBool(left) || ToBool(right)}, nil
	case "+":
		// Handle string concatenation or numeric addition
		if left.Type() == TypeString || right.Type() == TypeString {
			return StringValue{Val: ToString(left) + ToString(right)}, nil
		}
		return NumberValue{Val: ToNumber(left) + ToNumber(right)}, nil
	case "-":
		return NumberValue{Val: ToNumber(left) - ToNumber(right)}, nil
	case "*":
		return NumberValue{Val: ToNumber(left) * ToNumber(right)}, nil
	case "/":
		r := ToNumber(right)
		if r == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return NumberValue{Val: ToNumber(left) / r}, nil
	case "%":
		r := ToNumber(right)
		if r == 0 {
			return nil, fmt.Errorf("modulo by zero")
		}
		return NumberValue{Val: float64(int64(ToNumber(left)) % int64(r))}, nil
	default:
		return nil, fmt.Errorf("unknown operator: %s", e.Op)
	}
}

// UnaryOpExpr represents a unary operation
type UnaryOpExpr struct {
	Op   string
	Expr Expression
}

func (e *UnaryOpExpr) Eval(ctx *EvalContext) (Value, error) {
	val, err := e.Expr.Eval(ctx)
	if err != nil {
		return nil, err
	}

	switch e.Op {
	case "!":
		return BoolValue{Val: !ToBool(val)}, nil
	case "-":
		return NumberValue{Val: -ToNumber(val)}, nil
	default:
		return nil, fmt.Errorf("unknown unary operator: %s", e.Op)
	}
}

// ConditionalExpr represents a ternary conditional expression
type ConditionalExpr struct {
	Condition Expression
	TrueExpr  Expression
	FalseExpr Expression
}

func (e *ConditionalExpr) Eval(ctx *EvalContext) (Value, error) {
	cond, err := e.Condition.Eval(ctx)
	if err != nil {
		return nil, err
	}

	if ToBool(cond) {
		return e.TrueExpr.Eval(ctx)
	}
	return e.FalseExpr.Eval(ctx)
}

// CallExpr represents a function call
type CallExpr struct {
	Name string
	Args []Expression
}

func (e *CallExpr) Eval(ctx *EvalContext) (Value, error) {
	// Evaluate arguments
	args := make([]interface{}, len(e.Args))
	for i, arg := range e.Args {
		val, err := arg.Eval(ctx)
		if err != nil {
			return nil, err
		}
		args[i] = val.GoValue()
	}

	// Call the function
	result, err := ctx.Functions.Call(e.Name, args, ctx.ExecCtx)
	if err != nil {
		return nil, err
	}

	return GoToValue(result), nil
}

// IndexExpr represents array/map indexing
type IndexExpr struct {
	Object Expression
	Index  Expression
}

func (e *IndexExpr) Eval(ctx *EvalContext) (Value, error) {
	obj, err := e.Object.Eval(ctx)
	if err != nil {
		return nil, err
	}

	idx, err := e.Index.Eval(ctx)
	if err != nil {
		return nil, err
	}

	switch o := obj.(type) {
	case ListValue:
		i := int(ToNumber(idx))
		if i < 0 || i >= len(o.Vals) {
			return nil, fmt.Errorf("index %d out of bounds", i)
		}
		return o.Vals[i], nil
	case MapValue:
		key := ToString(idx)
		val, ok := o.Vals[key]
		if !ok {
			return NilValue{}, nil
		}
		return val, nil
	default:
		return nil, fmt.Errorf("cannot index %s", obj.Type())
	}
}

// DotExpr represents object property access
type DotExpr struct {
	Object Expression
	Field  string
}

func (e *DotExpr) Eval(ctx *EvalContext) (Value, error) {
	// Special handling for root-level accesses like inputs.name
	if varExpr, ok := e.Object.(*VariableExpr); ok {
		fullPath := varExpr.Name + "." + e.Field
		if val, err := ctx.Variables.Get(fullPath); err == nil {
			return val, nil
		}
	}

	obj, err := e.Object.Eval(ctx)
	if err != nil {
		return nil, err
	}

	switch o := obj.(type) {
	case MapValue:
		val, ok := o.Vals[e.Field]
		if !ok {
			return NilValue{}, nil
		}
		return val, nil
	default:
		return nil, fmt.Errorf("cannot access field %s on %s", e.Field, obj.Type())
	}
}

// Type conversion functions

func ToBool(v Value) bool {
	switch val := v.(type) {
	case BoolValue:
		return val.Val
	case NumberValue:
		return val.Val != 0
	case StringValue:
		return val.Val != ""
	case NilValue:
		return false
	case ListValue:
		return len(val.Vals) > 0
	case MapValue:
		return len(val.Vals) > 0
	default:
		return true
	}
}

func ToNumber(v Value) float64 {
	switch val := v.(type) {
	case NumberValue:
		return val.Val
	case BoolValue:
		if val.Val {
			return 1
		}
		return 0
	case StringValue:
		f, _ := strconv.ParseFloat(val.Val, 64)
		return f
	default:
		return 0
	}
}

func ToString(v Value) string {
	return v.String()
}

// Parser

type Parser struct {
	tokens []Token
	pos    int
}

func Parse(input string) (Expression, error) {
	tokens, err := Tokenize(input)
	if err != nil {
		return nil, err
	}

	p := &Parser{tokens: tokens}
	return p.parseExpression()
}

func (p *Parser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() {
	p.pos++
}

func (p *Parser) parseExpression() (Expression, error) {
	return p.parseTernary()
}

func (p *Parser) parseTernary() (Expression, error) {
	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}

	if p.current().Type == TokenQuestion {
		p.advance() // consume ?
		trueExpr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		if p.current().Type != TokenColon {
			return nil, fmt.Errorf("expected : in ternary expression")
		}
		p.advance() // consume :

		falseExpr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		return &ConditionalExpr{
			Condition: expr,
			TrueExpr:  trueExpr,
			FalseExpr: falseExpr,
		}, nil
	}

	return expr, nil
}

func (p *Parser) parseOr() (Expression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenOr {
		op := p.current().Value
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryOpExpr{Left: left, Op: op, Right: right}
	}

	return left, nil
}

func (p *Parser) parseAnd() (Expression, error) {
	left, err := p.parseEquality()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenAnd {
		op := p.current().Value
		p.advance()
		right, err := p.parseEquality()
		if err != nil {
			return nil, err
		}
		left = &BinaryOpExpr{Left: left, Op: op, Right: right}
	}

	return left, nil
}

func (p *Parser) parseEquality() (Expression, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenEq || p.current().Type == TokenNe {
		op := p.current().Value
		p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = &BinaryOpExpr{Left: left, Op: op, Right: right}
	}

	return left, nil
}

func (p *Parser) parseComparison() (Expression, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}

	for {
		switch p.current().Type {
		case TokenLt, TokenGt, TokenLe, TokenGe:
			op := p.current().Value
			p.advance()
			right, err := p.parseAdditive()
			if err != nil {
				return nil, err
			}
			left = &BinaryOpExpr{Left: left, Op: op, Right: right}
		default:
			return left, nil
		}
	}
}

func (p *Parser) parseAdditive() (Expression, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenPlus || p.current().Type == TokenMinus {
		op := p.current().Value
		p.advance()
		right, err := p.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		left = &BinaryOpExpr{Left: left, Op: op, Right: right}
	}

	return left, nil
}

func (p *Parser) parseMultiplicative() (Expression, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		switch p.current().Type {
		case TokenMul, TokenDiv, TokenMod:
			op := p.current().Value
			p.advance()
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = &BinaryOpExpr{Left: left, Op: op, Right: right}
		default:
			return left, nil
		}
	}
}

func (p *Parser) parseUnary() (Expression, error) {
	switch p.current().Type {
	case TokenNot:
		op := p.current().Value
		p.advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryOpExpr{Op: op, Expr: expr}, nil
	case TokenMinus:
		op := p.current().Value
		p.advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryOpExpr{Op: op, Expr: expr}, nil
	default:
		return p.parsePostfix()
	}
}

func (p *Parser) parsePostfix() (Expression, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		switch p.current().Type {
		case TokenDot:
			p.advance()
			if p.current().Type != TokenIdent {
				return nil, fmt.Errorf("expected identifier after .")
			}
			field := p.current().Value
			p.advance()
			expr = &DotExpr{Object: expr, Field: field}
		case TokenLBracket:
			p.advance()
			index, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if p.current().Type != TokenRBracket {
				return nil, fmt.Errorf("expected ]")
			}
			p.advance()
			expr = &IndexExpr{Object: expr, Index: index}
		case TokenLParen:
			// Function call
			if varExpr, ok := expr.(*VariableExpr); ok {
				p.advance() // consume (
				args := []Expression{}
				for p.current().Type != TokenRParen {
					arg, err := p.parseExpression()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.current().Type == TokenComma {
						p.advance()
					} else if p.current().Type != TokenRParen {
						return nil, fmt.Errorf("expected , or )")
					}
				}
				p.advance() // consume )
				expr = &CallExpr{Name: varExpr.Name, Args: args}
			} else {
				return nil, fmt.Errorf("invalid function call")
			}
		default:
			return expr, nil
		}
	}
}

func (p *Parser) parsePrimary() (Expression, error) {
	switch p.current().Type {
	case TokenNumber:
		val, err := strconv.ParseFloat(p.current().Value, 64)
		if err != nil {
			return nil, err
		}
		p.advance()
		return &LiteralExpr{Value: NumberValue{Val: val}}, nil
	case TokenString:
		val := p.current().Value
		p.advance()
		return &LiteralExpr{Value: StringValue{Val: val}}, nil
	case TokenIdent:
		name := p.current().Value
		p.advance()

		// Check for boolean literals
		switch name {
		case "true":
			return &LiteralExpr{Value: BoolValue{Val: true}}, nil
		case "false":
			return &LiteralExpr{Value: BoolValue{Val: false}}, nil
		case "null":
			return &LiteralExpr{Value: NilValue{}}, nil
		}

		return &VariableExpr{Name: name}, nil
	case TokenLParen:
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.current().Type != TokenRParen {
			return nil, fmt.Errorf("expected )")
		}
		p.advance()
		return expr, nil
	default:
		return nil, fmt.Errorf("unexpected token: %s", p.current().Value)
	}
}

// Tokenizer

type TokenType int

const (
	TokenEOF TokenType = iota
	TokenIdent
	TokenNumber
	TokenString

	// Operators
	TokenEq       // ==
	TokenNe       // !=
	TokenLt       // <
	TokenGt       // >
	TokenLe       // <=
	TokenGe       // >=
	TokenAnd      // &&
	TokenOr       // ||
	TokenNot      // !
	TokenPlus     // +
	TokenMinus    // -
	TokenMul      // *
	TokenDiv      // /
	TokenMod      // %
	TokenQuestion // ?
	TokenColon    // :

	// Delimiters
	TokenLParen   // (
	TokenRParen   // )
	TokenLBracket // [
	TokenRBracket // ]
	TokenDot      // .
	TokenComma    // ,
)

type Token struct {
	Type  TokenType
	Value string
}

func Tokenize(input string) ([]Token, error) {
	var tokens []Token
	i := 0

	for i < len(input) {
		// Skip whitespace
		for i < len(input) && unicode.IsSpace(rune(input[i])) {
			i++
		}

		if i >= len(input) {
			break
		}

		// Multi-character operators
		if i+1 < len(input) {
			two := input[i : i+2]
			switch two {
			case "==":
				tokens = append(tokens, Token{Type: TokenEq, Value: two})
				i += 2
				continue
			case "!=":
				tokens = append(tokens, Token{Type: TokenNe, Value: two})
				i += 2
				continue
			case "<=":
				tokens = append(tokens, Token{Type: TokenLe, Value: two})
				i += 2
				continue
			case ">=":
				tokens = append(tokens, Token{Type: TokenGe, Value: two})
				i += 2
				continue
			case "&&":
				tokens = append(tokens, Token{Type: TokenAnd, Value: two})
				i += 2
				continue
			case "||":
				tokens = append(tokens, Token{Type: TokenOr, Value: two})
				i += 2
				continue
			}
		}

		// Single character tokens
		switch input[i] {
		case '<':
			tokens = append(tokens, Token{Type: TokenLt, Value: "<"})
			i++
		case '>':
			tokens = append(tokens, Token{Type: TokenGt, Value: ">"})
			i++
		case '!':
			tokens = append(tokens, Token{Type: TokenNot, Value: "!"})
			i++
		case '+':
			tokens = append(tokens, Token{Type: TokenPlus, Value: "+"})
			i++
		case '-':
			tokens = append(tokens, Token{Type: TokenMinus, Value: "-"})
			i++
		case '*':
			tokens = append(tokens, Token{Type: TokenMul, Value: "*"})
			i++
		case '/':
			tokens = append(tokens, Token{Type: TokenDiv, Value: "/"})
			i++
		case '%':
			tokens = append(tokens, Token{Type: TokenMod, Value: "%"})
			i++
		case '?':
			tokens = append(tokens, Token{Type: TokenQuestion, Value: "?"})
			i++
		case ':':
			tokens = append(tokens, Token{Type: TokenColon, Value: ":"})
			i++
		case '(':
			tokens = append(tokens, Token{Type: TokenLParen, Value: "("})
			i++
		case ')':
			tokens = append(tokens, Token{Type: TokenRParen, Value: ")"})
			i++
		case '[':
			tokens = append(tokens, Token{Type: TokenLBracket, Value: "["})
			i++
		case ']':
			tokens = append(tokens, Token{Type: TokenRBracket, Value: "]"})
			i++
		case '.':
			tokens = append(tokens, Token{Type: TokenDot, Value: "."})
			i++
		case ',':
			tokens = append(tokens, Token{Type: TokenComma, Value: ","})
			i++
		case '\'', '"':
			// String
			quote := input[i]
			i++
			start := i
			for i < len(input) && input[i] != quote {
				if input[i] == '\\' && i+1 < len(input) {
					i += 2
				} else {
					i++
				}
			}
			if i >= len(input) {
				return nil, fmt.Errorf("unterminated string")
			}
			val := input[start:i]
			// Unescape
			val = strings.ReplaceAll(val, "\\\"", "\"")
			val = strings.ReplaceAll(val, "\\'", "'")
			val = strings.ReplaceAll(val, "\\\\", "\\")
			tokens = append(tokens, Token{Type: TokenString, Value: val})
			i++
		default:
			// Number
			if unicode.IsDigit(rune(input[i])) {
				start := i
				for i < len(input) && (unicode.IsDigit(rune(input[i])) || input[i] == '.') {
					i++
				}
				tokens = append(tokens, Token{Type: TokenNumber, Value: input[start:i]})
			} else if unicode.IsLetter(rune(input[i])) {
				// Identifier
				start := i
				for i < len(input) && (unicode.IsLetter(rune(input[i])) || unicode.IsDigit(rune(input[i])) || input[i] == '_') {
					i++
				}
				tokens = append(tokens, Token{Type: TokenIdent, Value: input[start:i]})
			} else {
				return nil, fmt.Errorf("unexpected character: %c", input[i])
			}
		}
	}

	tokens = append(tokens, Token{Type: TokenEOF})
	return tokens, nil
}
