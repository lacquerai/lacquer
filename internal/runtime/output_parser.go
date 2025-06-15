package runtime

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lacquer/lacquer/internal/ast"
)

// OutputParser handles parsing and extraction of structured outputs from agent responses
type OutputParser struct {
	// Patterns for extracting common output formats
	jsonPattern      *regexp.Regexp
	codeBlockPattern *regexp.Regexp
	listPattern      *regexp.Regexp
	keyValuePattern  *regexp.Regexp
	schemaGuided     bool // Whether to prioritize JSON parsing for schema-guided responses
}

// NewOutputParser creates a new output parser instance
func NewOutputParser() *OutputParser {
	return &OutputParser{
		jsonPattern:      regexp.MustCompile(`(?s)\{.*\}|\[.*\]`),
		codeBlockPattern: regexp.MustCompile("(?s)```(?:json)?\\s*\\n([\\s\\S]*?)\\n```"),
		listPattern:      regexp.MustCompile(`(?m)^[\s-\*]+(.+)$`),
		keyValuePattern:  regexp.MustCompile(`(?m)^([a-zA-Z_]\w*):\s*(.+)$`),
	}
}

// ParseStepOutput parses the agent response according to the step's output definitions
func (p *OutputParser) ParseStepOutput(step *ast.Step, response string) (map[string]interface{}, error) {
	// If no outputs are defined, return the raw response as default output
	if step.Outputs == nil || len(step.Outputs) == 0 {
		return map[string]interface{}{
			"output": response,
		}, nil
	}

	// Try to parse the response based on output definitions
	parsedOutputs := make(map[string]interface{})

	// For schema-guided responses, prioritize JSON parsing with better error handling
	if p.isSchemaGuidedResponse(response) {
		if jsonData := p.extractJSON(response); jsonData != nil {
			if mapped := p.mapJSONToOutputs(step.Outputs, jsonData); mapped != nil {
				parsedOutputs = mapped
			}
		}

		// If JSON parsing failed but response looks like it should be JSON, try to fix common issues
		if len(parsedOutputs) == 0 && p.looksLikeJSON(response) {
			if fixedJSON := p.attemptJSONFix(response); fixedJSON != nil {
				if mapped := p.mapJSONToOutputs(step.Outputs, fixedJSON); mapped != nil {
					parsedOutputs = mapped
				}
			}
		}
	} else {
		// First attempt: Try to parse as JSON if the response looks like JSON
		if jsonData := p.extractJSON(response); jsonData != nil {
			// If we got JSON, try to map it to the expected outputs
			if mapped := p.mapJSONToOutputs(step.Outputs, jsonData); mapped != nil {
				parsedOutputs = mapped
			}
		}
	}

	// Second attempt: Try to extract structured data from natural language
	if len(parsedOutputs) == 0 {
		extracted := p.extractStructuredData(step.Outputs, response)
		if len(extracted) > 0 {
			parsedOutputs = extracted
		}
	}

	// Always include the raw response for fallback access
	parsedOutputs["response"] = response

	// If we have a default output field defined, ensure it's populated
	if _, hasOutput := step.Outputs["output"]; hasOutput && parsedOutputs["output"] == nil {
		parsedOutputs["output"] = response
	}

	return parsedOutputs, nil
}

// extractJSON attempts to extract JSON data from the response
func (p *OutputParser) extractJSON(response string) interface{} {
	// Try to find JSON in code blocks first
	matches := p.codeBlockPattern.FindStringSubmatch(response)
	if len(matches) > 1 {
		response = matches[1]
	}

	// Clean up the response
	response = strings.TrimSpace(response)

	// Try to parse as JSON
	var result interface{}
	if err := json.Unmarshal([]byte(response), &result); err == nil {
		return result
	}

	// Try to find JSON-like structures in the text
	jsonMatches := p.jsonPattern.FindAllString(response, -1)
	for _, match := range jsonMatches {
		if err := json.Unmarshal([]byte(match), &result); err == nil {
			return result
		}
	}

	return nil
}

// mapJSONToOutputs maps parsed JSON to the expected output structure
func (p *OutputParser) mapJSONToOutputs(outputs map[string]interface{}, jsonData interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// If jsonData is already a map, try direct mapping first
	if dataMap, ok := jsonData.(map[string]interface{}); ok {
		hasDirectMapping := false
		for key, outputDef := range outputs {
			if value, exists := dataMap[key]; exists {
				result[key] = p.coerceType(value, outputDef)
				hasDirectMapping = true
			}
		}
		if hasDirectMapping {
			return result
		}

		// If there's only one output field and no direct mapping, assign the whole object
		if len(outputs) == 1 {
			for key, outputDef := range outputs {
				result[key] = p.coerceType(jsonData, outputDef)
				return result
			}
		}
	} else {
		// For non-object JSON (arrays, primitives), if there's only one output field, assign it
		if len(outputs) == 1 {
			for key, outputDef := range outputs {
				result[key] = p.coerceType(jsonData, outputDef)
				return result
			}
		}
	}

	return nil
}

// extractStructuredData attempts to extract structured data from natural language
func (p *OutputParser) extractStructuredData(outputs map[string]interface{}, response string) map[string]interface{} {
	result := make(map[string]interface{})
	hasAnyMatches := false

	for key, outputDef := range outputs {
		value := p.extractValueForKey(key, outputDef, response)
		if value != nil {
			result[key] = value
			hasAnyMatches = true
		}
	}

	// If no structured data was extracted, return empty result to trigger fallback
	if !hasAnyMatches {
		return make(map[string]interface{})
	}

	return result
}

// extractValueForKey extracts a value for a specific output key from the response
func (p *OutputParser) extractValueForKey(key string, outputDef interface{}, response string) interface{} {
	// Get the expected type
	expectedType := p.getExpectedType(outputDef)

	// For arrays, look for lists after the key first (before general pattern)
	if expectedType == "array" {
		// Look for "key:" followed by list items
		listPattern := regexp.MustCompile(fmt.Sprintf(`(?si)%s[:\s]*\n((?:[\s-\*•]+.+\n?)+)`, regexp.QuoteMeta(key)))
		matches := listPattern.FindStringSubmatch(response)
		if len(matches) > 1 {
			return p.parseList(matches[1])
		}

		// Alternative: look for "key findings:" or similar followed by list
		altPattern := regexp.MustCompile(fmt.Sprintf(`(?si)%s[:\s]*\n((?:[\s-\*•]+.+(?:\n|$))+)`, regexp.QuoteMeta(key)))
		matches = altPattern.FindStringSubmatch(response)
		if len(matches) > 1 {
			return p.parseList(matches[1])
		}
	}

	// For booleans, look for yes/no, true/false patterns
	if expectedType == "boolean" {
		boolPattern := regexp.MustCompile(fmt.Sprintf(`(?i)\b%s\b[:\s]*(yes|no|true|false)\b`, regexp.QuoteMeta(key)))
		matches := boolPattern.FindStringSubmatch(response)
		if len(matches) > 1 {
			return p.parseBoolean(matches[1])
		}
	}

	// Look for key-value patterns (general case) - key should be at start of line or after whitespace, followed by colon
	pattern := regexp.MustCompile(fmt.Sprintf(`(?im)(?:^|\s)%s:\s*(.+?)(?:\n|$)`, regexp.QuoteMeta(key)))
	matches := pattern.FindStringSubmatch(response)
	if len(matches) > 1 {
		return p.parseValue(strings.TrimSpace(matches[1]), expectedType)
	}

	return nil
}

// getExpectedType determines the expected type from output definition
func (p *OutputParser) getExpectedType(outputDef interface{}) string {
	switch v := outputDef.(type) {
	case string:
		return v
	case map[string]interface{}:
		if typeStr, ok := v["type"].(string); ok {
			return typeStr
		}
	}
	return "string"
}

// parseValue parses a string value according to the expected type
func (p *OutputParser) parseValue(value string, expectedType string) interface{} {
	switch expectedType {
	case "integer":
		if i, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return i
		}
	case "float", "number":
		if f, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return f
		}
	case "boolean":
		return p.parseBoolean(value)
	case "array":
		return p.parseList(value)
	case "object":
		// Try to parse as JSON
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(value), &obj); err == nil {
			return obj
		}
	}
	return value
}

// parseBoolean parses various boolean representations
func (p *OutputParser) parseBoolean(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "yes" || value == "1" || value == "on"
}

// parseList parses a list from text
func (p *OutputParser) parseList(text string) []interface{} {
	items := make([]interface{}, 0) // Initialize empty slice instead of nil
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Remove list markers
		if strings.HasPrefix(line, "-") {
			line = strings.TrimSpace(line[1:])
		} else if strings.HasPrefix(line, "*") {
			line = strings.TrimSpace(line[1:])
		} else if strings.HasPrefix(line, "•") {
			line = strings.TrimSpace(line[1:])
		}

		if line != "" {
			items = append(items, line)
		}
	}

	return items
}

// coerceType attempts to coerce a value to match the expected output type
func (p *OutputParser) coerceType(value interface{}, outputDef interface{}) interface{} {
	expectedType := p.getExpectedType(outputDef)

	switch expectedType {
	case "string":
		return fmt.Sprintf("%v", value)
	case "integer":
		switch v := value.(type) {
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	case "float", "number":
		switch v := value.(type) {
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	case "boolean":
		switch v := value.(type) {
		case bool:
			return v
		case string:
			return p.parseBoolean(v)
		}
	case "array":
		switch v := value.(type) {
		case []interface{}:
			return v
		case string:
			// Try to parse as JSON array
			var arr []interface{}
			if err := json.Unmarshal([]byte(v), &arr); err == nil {
				return arr
			}
		}
	}

	return value
}

// isSchemaGuidedResponse determines if the response appears to be from a schema-guided prompt
func (p *OutputParser) isSchemaGuidedResponse(response string) bool {
	// Look for indicators that this was a schema-guided response
	indicators := []string{
		"```json",
		"\"type\":",
		"\"properties\":",
		"schema",
		"JSON object",
		"valid JSON",
	}

	lowerResponse := strings.ToLower(response)
	for _, indicator := range indicators {
		if strings.Contains(lowerResponse, strings.ToLower(indicator)) {
			return true
		}
	}

	// Also check if response starts/ends with JSON-like structure
	trimmed := strings.TrimSpace(response)
	return (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))
}

// looksLikeJSON determines if the response appears to be intended as JSON
func (p *OutputParser) looksLikeJSON(response string) bool {
	trimmed := strings.TrimSpace(response)

	// Check for JSON-like structure
	hasJSONStructure := (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))

	// Check for JSON-like content patterns (including single quotes)
	hasJSONContent := strings.Contains(response, "\":") ||
		strings.Contains(response, "\": ") ||
		strings.Contains(response, "':") ||
		strings.Contains(response, "': ") ||
		(strings.Contains(response, "\"") && strings.Contains(response, ":")) ||
		(strings.Contains(response, "'") && strings.Contains(response, ":"))

	return hasJSONStructure && hasJSONContent
}

// attemptJSONFix attempts to fix common JSON formatting issues
func (p *OutputParser) attemptJSONFix(response string) interface{} {
	// Extract content from code blocks first
	codeBlockMatches := p.codeBlockPattern.FindStringSubmatch(response)
	if len(codeBlockMatches) > 1 {
		response = codeBlockMatches[1]
	}

	response = strings.TrimSpace(response)

	// Try multiple fixing strategies in sequence
	fixes := []func(string) string{
		p.fixSingleQuotes, // Do this first before other quote-related fixes
		p.fixTrailingCommas,
		p.fixUnquotedKeys,
		p.fixNewlinesInStrings,
	}

	current := response
	for _, fix := range fixes {
		current = fix(current)
		// Try parsing after each fix
		var result interface{}
		if err := json.Unmarshal([]byte(current), &result); err == nil {
			return result
		}
	}

	return nil
}

// fixTrailingCommas removes trailing commas from JSON
func (p *OutputParser) fixTrailingCommas(jsonStr string) string {
	// Remove trailing commas before closing braces/brackets
	re := regexp.MustCompile(`,(\s*[}\]])`)
	return re.ReplaceAllString(jsonStr, "$1")
}

// fixUnquotedKeys adds quotes around unquoted object keys
func (p *OutputParser) fixUnquotedKeys(jsonStr string) string {
	// Add quotes around unquoted keys (simple heuristic)
	re := regexp.MustCompile(`(\n\s*)([a-zA-Z_][a-zA-Z0-9_]*)\s*:`)
	return re.ReplaceAllString(jsonStr, `$1"$2":`)
}

// fixSingleQuotes converts single quotes to double quotes
func (p *OutputParser) fixSingleQuotes(jsonStr string) string {
	// Simple replacement (doesn't handle escaped quotes)
	return strings.ReplaceAll(jsonStr, "'", "\"")
}

// fixNewlinesInStrings attempts to fix newlines within string values
func (p *OutputParser) fixNewlinesInStrings(jsonStr string) string {
	// This is a basic implementation - a full solution would need proper parsing
	lines := strings.Split(jsonStr, "\n")
	var result []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}

	return strings.Join(result, " ")
}
