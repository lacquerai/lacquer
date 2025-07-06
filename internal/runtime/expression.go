package runtime

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ExpressionEvaluator handles GitHub Actions-style expression evaluation
type ExpressionEvaluator struct {
	variableResolver *VariableResolver
	functions        *FunctionRegistry
}

// NewExpressionEvaluator creates a new expression evaluator
func NewExpressionEvaluator() *ExpressionEvaluator {
	return &ExpressionEvaluator{
		variableResolver: NewVariableResolver(),
		functions:        NewFunctionRegistry(),
	}
}

// Token represents a lexical token in an expression
type Token struct {
	Type  TokenType
	Value string
}

// TokenType represents the type of a token
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenIdentifier
	TokenNumber
	TokenString
	TokenBoolean
	TokenNull

	// Operators
	TokenEQ    // ==
	TokenNE    // !=
	TokenLT    // <
	TokenGT    // >
	TokenLE    // <=
	TokenGE    // >=
	TokenAND   // &&
	TokenOR    // ||
	TokenNOT   // !
	TokenPLUS  // +
	TokenMINUS // -
	TokenMUL   // *
	TokenDIV   // /
	TokenMOD   // %
	TokenQUEST // ?
	TokenCOLON // :

	// Delimiters
	TokenLPAREN   // (
	TokenRPAREN   // )
	TokenLBRACKET // [
	TokenRBRACKET // ]
	TokenDOT      // .
	TokenCOMMA    // ,
)

// Evaluate evaluates a GitHub Actions-style expression
func (ee *ExpressionEvaluator) Evaluate(expression string, execCtx *ExecutionContext) (interface{}, error) {
	// First, resolve any variable references in the expression
	resolvedExpr, err := ee.resolveVariables(expression, execCtx)
	if err != nil {
		return nil, err
	}

	// Parse and evaluate the expression
	lexer := newLexer(resolvedExpr)
	parser := newParser(lexer, ee.functions, execCtx)
	return parser.parse()
}

// resolveVariables resolves variable references in an expression
func (ee *ExpressionEvaluator) resolveVariables(expression string, execCtx *ExecutionContext) (string, error) {
	// Pattern to match variable references like inputs.name, steps.step1.output
	// But avoid matching those inside string literals
	varPattern := regexp.MustCompile(`\b(inputs|state|steps|metadata|env|workflow)\.[\w.]+\b`)

	result := expression
	matches := varPattern.FindAllString(expression, -1)

	// Track which parts of the string are inside string literals
	inString := false
	var stringChar rune
	stringRanges := [][]int{}

	runes := []rune(expression)
	start := -1
	for i, r := range runes {
		if !inString && (r == '\'' || r == '"') {
			inString = true
			stringChar = r
			start = i
		} else if inString && r == stringChar {
			inString = false
			stringRanges = append(stringRanges, []int{start, i})
		}
	}

	for _, match := range matches {
		// Check if this match is inside a string literal
		matchPos := strings.Index(result, match)
		if matchPos == -1 {
			continue
		}

		insideString := false
		for _, rang := range stringRanges {
			if matchPos >= rang[0] && matchPos <= rang[1] {
				insideString = true
				break
			}
		}

		if insideString {
			continue // Skip variables inside string literals
		}

		value, err := ee.variableResolver.ResolveVariable(match, execCtx)
		if err != nil {
			return "", err
		}
		// Convert value to expression literal
		literal := ee.valueToLiteral(value)
		result = strings.ReplaceAll(result, match, literal)
	}

	return result, nil
}

// valueToLiteral converts a value to an expression literal
func (ee *ExpressionEvaluator) valueToLiteral(value interface{}) string {
	switch v := value.(type) {
	case string:
		// Escape quotes in string
		escaped := strings.ReplaceAll(v, "'", "\\'")
		return "'" + escaped + "'"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", value)
	}
}

// Lexer tokenizes expressions
type Lexer struct {
	input    string
	position int
	current  rune
}

func newLexer(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.position >= len(l.input) {
		l.current = 0
	} else {
		l.current = rune(l.input[l.position])
	}
	l.position++
}

func (l *Lexer) peekChar() rune {
	if l.position >= len(l.input) {
		return 0
	}
	return rune(l.input[l.position])
}

func (l *Lexer) nextToken() Token {
	l.skipWhitespace()

	if l.current == 0 {
		return Token{Type: TokenEOF}
	}

	// Handle two-character operators
	if l.current == '=' && l.peekChar() == '=' {
		l.readChar()
		l.readChar()
		return Token{Type: TokenEQ, Value: "=="}
	}
	if l.current == '!' && l.peekChar() == '=' {
		l.readChar()
		l.readChar()
		return Token{Type: TokenNE, Value: "!="}
	}
	if l.current == '<' && l.peekChar() == '=' {
		l.readChar()
		l.readChar()
		return Token{Type: TokenLE, Value: "<="}
	}
	if l.current == '>' && l.peekChar() == '=' {
		l.readChar()
		l.readChar()
		return Token{Type: TokenGE, Value: ">="}
	}
	if l.current == '&' && l.peekChar() == '&' {
		l.readChar()
		l.readChar()
		return Token{Type: TokenAND, Value: "&&"}
	}
	if l.current == '|' && l.peekChar() == '|' {
		l.readChar()
		l.readChar()
		return Token{Type: TokenOR, Value: "||"}
	}

	// Handle single-character operators and delimiters
	switch l.current {
	case '<':
		l.readChar()
		return Token{Type: TokenLT, Value: "<"}
	case '>':
		l.readChar()
		return Token{Type: TokenGT, Value: ">"}
	case '!':
		l.readChar()
		return Token{Type: TokenNOT, Value: "!"}
	case '+':
		l.readChar()
		return Token{Type: TokenPLUS, Value: "+"}
	case '-':
		l.readChar()
		return Token{Type: TokenMINUS, Value: "-"}
	case '*':
		l.readChar()
		return Token{Type: TokenMUL, Value: "*"}
	case '/':
		l.readChar()
		return Token{Type: TokenDIV, Value: "/"}
	case '%':
		l.readChar()
		return Token{Type: TokenMOD, Value: "%"}
	case '?':
		l.readChar()
		return Token{Type: TokenQUEST, Value: "?"}
	case ':':
		l.readChar()
		return Token{Type: TokenCOLON, Value: ":"}
	case '(':
		l.readChar()
		return Token{Type: TokenLPAREN, Value: "("}
	case ')':
		l.readChar()
		return Token{Type: TokenRPAREN, Value: ")"}
	case '[':
		l.readChar()
		return Token{Type: TokenLBRACKET, Value: "["}
	case ']':
		l.readChar()
		return Token{Type: TokenRBRACKET, Value: "]"}
	case '.':
		l.readChar()
		return Token{Type: TokenDOT, Value: "."}
	case ',':
		l.readChar()
		return Token{Type: TokenCOMMA, Value: ","}
	}

	// Handle strings
	if l.current == '\'' || l.current == '"' {
		return l.readString()
	}

	// Handle numbers
	if isDigit(l.current) {
		return l.readNumber()
	}

	// Handle identifiers and keywords
	if isLetter(l.current) {
		return l.readIdentifier()
	}

	// Unknown character
	ch := l.current
	l.readChar()
	return Token{Type: TokenEOF, Value: string(ch)}
}

func (l *Lexer) skipWhitespace() {
	for l.current == ' ' || l.current == '\t' || l.current == '\n' || l.current == '\r' {
		l.readChar()
	}
}

func (l *Lexer) readString() Token {
	quote := l.current
	l.readChar()

	start := l.position - 1
	for l.current != quote && l.current != 0 {
		if l.current == '\\' {
			l.readChar() // Skip escaped character
		}
		l.readChar()
	}

	value := l.input[start : l.position-1]
	l.readChar() // Skip closing quote

	// Unescape the string
	value = strings.ReplaceAll(value, "\\'", "'")
	value = strings.ReplaceAll(value, "\\\"", "\"")
	value = strings.ReplaceAll(value, "\\\\", "\\")

	return Token{Type: TokenString, Value: value}
}

func (l *Lexer) readNumber() Token {
	start := l.position - 1
	hasDecimal := false

	for isDigit(l.current) || (l.current == '.' && !hasDecimal) {
		if l.current == '.' {
			hasDecimal = true
		}
		l.readChar()
	}

	return Token{Type: TokenNumber, Value: l.input[start : l.position-1]}
}

func (l *Lexer) readIdentifier() Token {
	start := l.position - 1

	for isLetter(l.current) || isDigit(l.current) || l.current == '_' {
		l.readChar()
	}

	value := l.input[start : l.position-1]

	// Check for boolean and null keywords
	switch value {
	case "true", "false":
		return Token{Type: TokenBoolean, Value: value}
	case "null":
		return Token{Type: TokenNull, Value: value}
	}

	return Token{Type: TokenIdentifier, Value: value}
}

func isLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z'
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

// Parser parses and evaluates expressions
type Parser struct {
	lexer     *Lexer
	current   Token
	peek      Token
	functions *FunctionRegistry
	execCtx   *ExecutionContext
}

func newParser(lexer *Lexer, functions *FunctionRegistry, execCtx *ExecutionContext) *Parser {
	p := &Parser{
		lexer:     lexer,
		functions: functions,
		execCtx:   execCtx,
	}
	// Read two tokens to initialize current and peek
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) nextToken() {
	p.current = p.peek
	p.peek = p.lexer.nextToken()
}

func (p *Parser) parse() (interface{}, error) {
	expr, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	if p.current.Type != TokenEOF {
		return nil, fmt.Errorf("unexpected token: %s", p.current.Value)
	}

	return expr, nil
}

// parseExpression parses an expression with operator precedence
func (p *Parser) parseExpression(precedence int) (interface{}, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for precedence < p.getPrecedence(p.current.Type) {
		if p.current.Type == TokenQUEST {
			// Handle ternary operator with special case
			return p.parseTernary(left)
		}

		op := p.current
		p.nextToken()

		right, err := p.parseExpression(p.getPrecedence(op.Type) + 1)
		if err != nil {
			return nil, err
		}

		left, err = p.applyOperator(op, left, right)
		if err != nil {
			return nil, err
		}
	}

	return left, nil
}

// parsePrimary parses primary expressions
func (p *Parser) parsePrimary() (interface{}, error) {
	switch p.current.Type {
	case TokenNumber:
		val := p.current.Value
		p.nextToken()
		if strings.Contains(val, ".") {
			return strconv.ParseFloat(val, 64)
		}
		return strconv.ParseInt(val, 10, 64)

	case TokenString:
		val := p.current.Value
		p.nextToken()
		return val, nil

	case TokenBoolean:
		val := p.current.Value == "true"
		p.nextToken()
		return val, nil

	case TokenNull:
		p.nextToken()
		return nil, nil

	case TokenIdentifier:
		return p.parseIdentifier()

	case TokenNOT:
		p.nextToken()
		expr, err := p.parseExpression(p.getPrecedence(TokenNOT))
		if err != nil {
			return nil, err
		}
		return !toBool(expr), nil

	case TokenMINUS:
		p.nextToken()
		expr, err := p.parseExpression(p.getPrecedence(TokenMINUS))
		if err != nil {
			return nil, err
		}
		return -toNumber(expr), nil

	case TokenLPAREN:
		p.nextToken()
		expr, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		if p.current.Type != TokenRPAREN {
			return nil, fmt.Errorf("expected ')', got %s", p.current.Value)
		}
		p.nextToken()
		return expr, nil

	default:
		return nil, fmt.Errorf("unexpected token: %s", p.current.Value)
	}
}

// parseIdentifier parses identifiers and function calls
func (p *Parser) parseIdentifier() (interface{}, error) {
	name := p.current.Value
	p.nextToken()

	// Check if it's a function call
	if p.current.Type == TokenLPAREN {
		return p.parseFunctionCall(name)
	}

	// Check if it's an array/object access
	if p.current.Type == TokenLBRACKET {
		return p.parseArrayAccess(name)
	}

	// Otherwise, it's a variable reference that wasn't resolved
	return nil, fmt.Errorf("undefined variable: %s", name)
}

// parseFunctionCall parses function calls
func (p *Parser) parseFunctionCall(name string) (interface{}, error) {
	p.nextToken() // skip '('

	var args []interface{}
	for p.current.Type != TokenRPAREN {
		arg, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)

		if p.current.Type == TokenCOMMA {
			p.nextToken()
		} else if p.current.Type != TokenRPAREN {
			return nil, fmt.Errorf("expected ',' or ')', got %s", p.current.Value)
		}
	}

	p.nextToken() // skip ')'

	// Call the function
	return p.functions.Call(name, args, p.execCtx)
}

// parseArrayAccess parses array/object access with brackets
func (p *Parser) parseArrayAccess(name string) (interface{}, error) {
	// Parse the opening bracket
	if p.current.Type != TokenLBRACKET {
		return nil, fmt.Errorf("expected '[', got %s", p.current.Value)
	}
	p.nextToken() // consume '['

	// Parse the index/key expression
	index, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	// Parse the closing bracket
	if p.current.Type != TokenRBRACKET {
		return nil, fmt.Errorf("expected ']', got %s", p.current.Value)
	}
	p.nextToken() // consume ']'

	// Resolve the base variable
	resolver := NewVariableResolver()
	baseValue, err := resolver.ResolveVariable(name, p.execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve variable %s: %v", name, err)
	}

	// Access the index/key from the base value
	return accessValue(baseValue, index)
}

// accessValue accesses a value from a map or array using the given index/key
func accessValue(baseValue interface{}, index interface{}) (interface{}, error) {
	switch base := baseValue.(type) {
	case map[string]interface{}:
		key := toString(index)
		if value, exists := base[key]; exists {
			return value, nil
		}
		return nil, fmt.Errorf("key '%s' not found", key)

	case []interface{}:
		idx := int(toNumber(index))
		if idx < 0 || idx >= len(base) {
			return nil, fmt.Errorf("index %d out of bounds", idx)
		}
		return base[idx], nil

	case map[interface{}]interface{}:
		if value, exists := base[index]; exists {
			return value, nil
		}
		return nil, fmt.Errorf("key '%v' not found", index)

	// Handle slices of specific types
	case []string:
		idx := int(toNumber(index))
		if idx < 0 || idx >= len(base) {
			return nil, fmt.Errorf("index %d out of bounds", idx)
		}
		return base[idx], nil

	case []int:
		idx := int(toNumber(index))
		if idx < 0 || idx >= len(base) {
			return nil, fmt.Errorf("index %d out of bounds", idx)
		}
		return base[idx], nil

	case []float64:
		idx := int(toNumber(index))
		if idx < 0 || idx >= len(base) {
			return nil, fmt.Errorf("index %d out of bounds", idx)
		}
		return base[idx], nil

	default:
		return nil, fmt.Errorf("cannot access index on type %T", baseValue)
	}
}

// parseTernary parses ternary expressions (condition ? true : false)
func (p *Parser) parseTernary(condition interface{}) (interface{}, error) {
	p.nextToken() // consume '?'

	trueExpr, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	if p.current.Type != TokenCOLON {
		return nil, fmt.Errorf("expected ':', got %s", p.current.Value)
	}
	p.nextToken() // consume ':'

	falseExpr, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	if toBool(condition) {
		return trueExpr, nil
	}
	return falseExpr, nil
}

// getPrecedence returns the precedence of an operator
func (p *Parser) getPrecedence(tokenType TokenType) int {
	switch tokenType {
	case TokenQUEST:
		return 1 // Ternary has low precedence but not lowest
	case TokenOR:
		return 2
	case TokenAND:
		return 3
	case TokenEQ, TokenNE:
		return 4
	case TokenLT, TokenGT, TokenLE, TokenGE:
		return 5
	case TokenPLUS, TokenMINUS:
		return 6
	case TokenMUL, TokenDIV, TokenMOD:
		return 7
	default:
		return -1
	}
}

// applyOperator applies a binary operator
func (p *Parser) applyOperator(op Token, left, right interface{}) (interface{}, error) {
	switch op.Type {
	case TokenEQ:
		return isEqual(left, right), nil
	case TokenNE:
		return !isEqual(left, right), nil
	case TokenLT:
		return toNumber(left) < toNumber(right), nil
	case TokenGT:
		return toNumber(left) > toNumber(right), nil
	case TokenLE:
		return toNumber(left) <= toNumber(right), nil
	case TokenGE:
		return toNumber(left) >= toNumber(right), nil
	case TokenAND:
		return toBool(left) && toBool(right), nil
	case TokenOR:
		return toBool(left) || toBool(right), nil
	case TokenPLUS:
		// Handle string concatenation or numeric addition
		if isString(left) || isString(right) {
			return toString(left) + toString(right), nil
		}
		return toNumber(left) + toNumber(right), nil
	case TokenMINUS:
		return toNumber(left) - toNumber(right), nil
	case TokenMUL:
		return toNumber(left) * toNumber(right), nil
	case TokenDIV:
		r := toNumber(right)
		if r == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return toNumber(left) / r, nil
	case TokenMOD:
		r := toNumber(right)
		if r == 0 {
			return nil, fmt.Errorf("modulo by zero")
		}
		return int64(toNumber(left)) % int64(r), nil
	default:
		return nil, fmt.Errorf("unknown operator: %s", op.Value)
	}
}

// Type conversion helpers

func toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != ""
	case int64:
		return val != 0
	case float64:
		return val != 0
	case nil:
		return false
	default:
		return true
	}
}

func toNumber(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case bool:
		if val {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func isString(v interface{}) bool {
	_, ok := v.(string)
	return ok
}

func isEqual(a, b interface{}) bool {
	// Handle nil cases
	if a == nil || b == nil {
		return a == b
	}

	// Try to compare as numbers if both can be converted
	if isNumeric(a) && isNumeric(b) {
		return toNumber(a) == toNumber(b)
	}

	// Otherwise, compare as strings
	return toString(a) == toString(b)
}

func isNumeric(v interface{}) bool {
	switch v.(type) {
	case float64, int64, int:
		return true
	case string:
		_, err := strconv.ParseFloat(v.(string), 64)
		return err == nil
	default:
		return false
	}
}
