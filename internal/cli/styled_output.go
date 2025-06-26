package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
)

var (
	// Color palette
	errorColor   = lipgloss.Color("#FF6B6B")
	warningColor = lipgloss.Color("#FFA726")
	successColor = lipgloss.Color("#66BB6A")
	infoColor    = lipgloss.Color("#42A5F5")
	mutedColor   = lipgloss.Color("#6C757D")
	accentColor  = lipgloss.Color("#7C3AED")
	codeColor    = lipgloss.Color("#D4D4D4")

	// Base styles
	errorStyle   = lipgloss.NewStyle().Foreground(errorColor).Bold(true)
	warningStyle = lipgloss.NewStyle().Foreground(warningColor).Bold(true)
	successStyle = lipgloss.NewStyle().Foreground(successColor).Bold(true)
	infoStyle    = lipgloss.NewStyle().Foreground(infoColor).Bold(true)
	mutedStyle   = lipgloss.NewStyle().Foreground(mutedColor)
	accentStyle  = lipgloss.NewStyle().Foreground(accentColor)

	// Component styles
	fileStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true).
			Underline(true)

	positionStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	titleStyle = lipgloss.NewStyle().
			Bold(true)

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E4E4E7"))

	codeStyle = lipgloss.NewStyle().
			Foreground(codeColor).
			Background(lipgloss.Color("#1A1B26")).
			Padding(0, 1)

	lineNumberStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Width(5).
			Align(lipgloss.Right)

	errorLineStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3D2020"))

	highlightStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	suggestionTitleStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	suggestionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B8BCC2"))

	docsLinkStyle = lipgloss.NewStyle().
			Foreground(infoColor).
			Underline(true)

	// Box styles
	errorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(errorColor).
			Padding(1, 2).
			Margin(1, 0)

	warningBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(warningColor).
			Padding(1, 2).
			Margin(1, 0)

	infoBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(infoColor).
			Padding(1, 2).
			Margin(1, 0)

	// Section styles
	sectionStyle = lipgloss.NewStyle().
			Margin(0, 0, 1, 0)

	contextBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.Border{
			Top:         "─",
			Bottom:      "─",
			Left:        "│",
			Right:       "│",
			TopLeft:     "╭",
			TopRight:    "╮",
			BottomLeft:  "╰",
			BottomRight: "╯",
		}).
		BorderForeground(mutedColor).
		Padding(0, 1).
		Margin(0, 2)
)

// getSeverityIcon returns the appropriate icon for the severity level
func getSeverityIcon(severity string) string {
	switch severity {
	case "error":
		return errorStyle.Render("✗")
	case "warning":
		return warningStyle.Render("⚠")
	case "info":
		return infoStyle.Render("ℹ")
	default:
		return mutedStyle.Render("•")
	}
}

// getSeverityStyle returns the appropriate style for the severity level
func getSeverityStyle(severity string) lipgloss.Style {
	switch severity {
	case "error":
		return errorStyle
	case "warning":
		return warningStyle
	case "info":
		return infoStyle
	default:
		return mutedStyle
	}
}

// renderCodeLine renders a line of code with optional highlighting
func renderCodeLine(lineNum int, content string, isError bool, highlight *highlightInfo) string {
	lineNumStr := lineNumberStyle.Render(fmt.Sprintf("%d", lineNum))
	separator := mutedStyle.Render(" │ ")

	if isError {
		// Apply error background to the entire line
		contentWithBg := errorLineStyle.Render(content)
		return fmt.Sprintf("%s%s%s", lineNumStr, separator, contentWithBg)
	}

	return fmt.Sprintf("%s%s%s", lineNumStr, separator, content)
}

// highlightInfo contains information about what to highlight in a line
type highlightInfo struct {
	startCol int
	endCol   int
}

// renderHighlightIndicator renders the caret indicators below an error line
func renderHighlightIndicator(startCol, length int) string {
	if length <= 0 {
		return ""
	}

	// Create the spacing before the highlight
	spaces := strings.Repeat(" ", startCol-1)

	// Create the highlight indicators
	carets := strings.Repeat("^", length)

	// Style the carets
	highlightedCarets := highlightStyle.Render(carets)

	// Add line number width + separator
	padding := lineNumberStyle.Render("     ") + mutedStyle.Render(" │ ")

	return fmt.Sprintf("%s%s%s", padding, spaces, highlightedCarets)
}

// renderSuggestion renders a suggestion with proper styling
func renderSuggestion(title, description string, examples []string, docsURL string) string {
	var result strings.Builder

	// Title
	result.WriteString(suggestionTitleStyle.Render("💡 " + title))
	if description != "" {
		result.WriteString(suggestionStyle.Render(": " + description))
	}
	result.WriteString("\n")

	// Examples
	if len(examples) > 0 {
		result.WriteString("\n")
		result.WriteString(mutedStyle.Render("    Examples:") + "\n")
		for _, example := range examples {
			result.WriteString("      " + codeStyle.Render(example) + "\n")
		}
	}

	// Documentation link
	if docsURL != "" {
		result.WriteString("\n")
		result.WriteString("    📖 " + mutedStyle.Render("See: ") + docsLinkStyle.Render(docsURL) + "\n")
	}

	return result.String()
}

// formatFilePath formats a file path with proper styling
func formatFilePath(path string) string {
	return fileStyle.Render(path)
}

// formatPosition formats a position (line:column) with proper styling
func formatPosition(line, column int) string {
	return positionStyle.Render(fmt.Sprintf("%d:%d", line, column))
}
