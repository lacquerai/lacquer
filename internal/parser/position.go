package parser

import (
	"fmt"
	"strings"
)

// Position represents a position in a source file
type Position struct {
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Offset int    `json:"offset"`
	File   string `json:"file,omitempty"`
}

// String returns a human-readable representation of the position
func (p Position) String() string {
	if p.File != "" {
		return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Column)
	}
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// ExtractPosition extracts position information from YAML parsing errors
func ExtractPosition(source []byte, offset int) Position {
	lines := strings.Split(string(source), "\n")
	
	currentOffset := 0
	for lineNum, line := range lines {
		lineLength := len(line) + 1 // +1 for newline character
		if currentOffset+lineLength > offset {
			column := offset - currentOffset + 1
			return Position{
				Line:   lineNum + 1, // 1-indexed
				Column: column,
				Offset: offset,
			}
		}
		currentOffset += lineLength
	}
	
	// Fallback if position is at end of file
	return Position{
		Line:   len(lines),
		Column: len(lines[len(lines)-1]) + 1,
		Offset: offset,
	}
}

// ExtractContext extracts contextual lines around a position for error reporting
func ExtractContext(source []byte, position Position, contextLines int) string {
	lines := strings.Split(string(source), "\n")
	
	if position.Line <= 0 || position.Line > len(lines) {
		return ""
	}
	
	start := max(0, position.Line-contextLines-1)
	end := min(len(lines), position.Line+contextLines)
	
	var context strings.Builder
	for i := start; i < end; i++ {
		lineNum := i + 1
		prefix := "   "
		if lineNum == position.Line {
			prefix = ">> "
		}
		
		context.WriteString(fmt.Sprintf("%s%4d | %s\n", prefix, lineNum, lines[i]))
		
		// Add a pointer to the specific column for the error line
		if lineNum == position.Line && position.Column > 0 {
			pointer := strings.Repeat(" ", 8+min(position.Column-1, len(lines[i]))) + "^"
			context.WriteString(pointer + "\n")
		}
	}
	
	return context.String()
}

// Helper functions for min/max
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}