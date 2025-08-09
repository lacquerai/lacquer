package engine

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/lacquerai/lacquer/internal/ast"
)

// OutputParser handles parsing and extraction of structured outputs from agent responses
type OutputParser struct {
	jsonPattern      *regexp.Regexp
	codeBlockPattern *regexp.Regexp
}

// NewOutputParser creates a new output parser instance
func NewOutputParser() *OutputParser {
	return &OutputParser{
		jsonPattern:      regexp.MustCompile(`(?s)\{.*\}|\[.*\]`),
		codeBlockPattern: regexp.MustCompile("(?s)```(?:json)?\\s*\\n([\\s\\S]*?)\\n```"),
	}
}

// ParseStepOutput parses the agent response according to the step's output definitions
func (p *OutputParser) ParseStepOutput(step *ast.Step, response string) interface{} {
	return p.extractJSON(response)
}

// extractJSON attempts to extract JSON data from the response
// @TODO: in future we might want to use the output schema to
// create a more robust type that conforms to the user specified schema
// but for now this is fine.
func (p *OutputParser) extractJSON(response string) map[string]interface{} {
	matches := p.codeBlockPattern.FindStringSubmatch(response)
	if len(matches) > 1 {
		response = matches[1]
	}

	response = strings.TrimSpace(response)

	var result map[string]interface{}
	err := json.Unmarshal([]byte(response), &result)
	if err == nil {
		return result
	}

	jsonMatches := p.jsonPattern.FindAllString(response, -1)
	for _, match := range jsonMatches {
		err := json.Unmarshal([]byte(match), &result)
		if err == nil {
			return result
		}
	}

	return nil
}
