package style

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
	"gopkg.in/yaml.v3"
)

var (
	// Color palette
	ErrorColor   = lipgloss.Color("#FF6B6B")
	WarningColor = lipgloss.Color("#FFA726")
	SuccessColor = lipgloss.Color("#66BB6A")
	InfoColor    = lipgloss.Color("#42A5F5")
	MutedColor   = lipgloss.Color("#6C757D")
	AccentColor  = lipgloss.Color("#7C3AED")
	CodeColor    = lipgloss.Color("#D4D4D4")

	// Base styles
	ErrorStyle   = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true)
	WarningStyle = lipgloss.NewStyle().Foreground(WarningColor).Bold(true)
	SuccessStyle = lipgloss.NewStyle().Foreground(SuccessColor).Bold(true)
	InfoStyle    = lipgloss.NewStyle().Foreground(InfoColor).Bold(true)
	MutedStyle   = lipgloss.NewStyle().Foreground(MutedColor)
	AccentStyle  = lipgloss.NewStyle().Foreground(AccentColor)

	// Component styles
	FileStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true).
			Underline(true)

	PositionStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)

	TitleStyle = lipgloss.NewStyle().
			Bold(true)

	MessageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E4E4E7"))

	CodeStyle = lipgloss.NewStyle().
			Foreground(CodeColor).
			Background(lipgloss.Color("#1A1B26")).
			Padding(0, 1)

	LineNumberStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Width(5).
			Align(lipgloss.Right)

	ErrorLineStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3D2020"))

	HighlightStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	SuggestionTitleStyle = lipgloss.NewStyle().
				Foreground(SuccessColor).
				Bold(true)

	SuggestionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B8BCC2"))

	DocsLinkStyle = lipgloss.NewStyle().
			Foreground(InfoColor).
			Underline(true)

	// Box styles
	ErrorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ErrorColor).
			Padding(1, 2).
			Margin(1, 0)

	WarningBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(WarningColor).
			Padding(1, 2).
			Margin(1, 0)

	InfoBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(InfoColor).
			Padding(1, 2).
			Margin(1, 0)

	ContextBoxStyle = lipgloss.NewStyle().
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
		BorderForeground(MutedColor).
		Padding(0, 1).
		Margin(0, 2)
)

// getSeverityIcon returns the appropriate icon for the severity level
func GetSeverityIcon(severity string) string {
	switch severity {
	case "error":
		return ErrorStyle.Render("✗")
	case "warning":
		return WarningStyle.Render("⚠")
	case "info":
		return InfoStyle.Render("ℹ")
	default:
		return MutedStyle.Render("•")
	}
}

// getSeverityStyle returns the appropriate style for the severity level
func GetSeverityStyle(severity string) lipgloss.Style {
	switch severity {
	case "error":
		return ErrorStyle
	case "warning":
		return WarningStyle
	case "info":
		return InfoStyle
	default:
		return MutedStyle
	}
}

// renderCodeLine renders a line of code with optional highlighting
func RenderCodeLine(lineNum int, content string, isError bool) string {
	lineNumStr := LineNumberStyle.Render(fmt.Sprintf("%d", lineNum))
	separator := MutedStyle.Render(" │ ")

	if isError {
		// Apply error background to the entire line
		contentWithBg := ErrorLineStyle.Render(content)
		return fmt.Sprintf("%s%s%s", lineNumStr, separator, contentWithBg)
	}

	return fmt.Sprintf("%s%s%s", lineNumStr, separator, content)
}

// renderHighlightIndicator renders the caret indicators below an error line
func RenderHighlightIndicator(startCol, length int) string {
	if length <= 0 {
		return ""
	}

	// Create the spacing before the highlight
	spaces := strings.Repeat(" ", startCol-1)

	// Create the highlight indicators
	carets := strings.Repeat("^", length)

	// Style the carets
	highlightedCarets := HighlightStyle.Render(carets)

	// Add line number width + separator
	padding := LineNumberStyle.Render("     ") + MutedStyle.Render(" │ ")

	return fmt.Sprintf("%s%s%s", padding, spaces, highlightedCarets)
}

// renderSuggestion renders a suggestion with proper styling
func RenderSuggestion(title, description string, examples []string, docsURL string) string {
	var result strings.Builder

	// Title
	result.WriteString(SuggestionTitleStyle.Render("💡 " + title))
	if description != "" {
		result.WriteString(SuggestionStyle.Render(": " + description))
	}
	result.WriteString("\n")

	// Examples
	if len(examples) > 0 {
		result.WriteString("\n")
		result.WriteString(MutedStyle.Render("    Examples:") + "\n")
		for _, example := range examples {
			result.WriteString("      " + CodeStyle.Render(example) + "\n")
		}
	}

	// Documentation link
	if docsURL != "" {
		result.WriteString("\n")
		result.WriteString("    📖 " + MutedStyle.Render("See: ") + DocsLinkStyle.Render(docsURL) + "\n")
	}

	return result.String()
}

// Progress styles for run command
var (
	StepRunningStyle = lipgloss.NewStyle().
				Foreground(InfoColor)

	StepCompletedStyle = lipgloss.NewStyle().
				Foreground(SuccessColor).
				Bold(true)

	StepFailedStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	StepNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E4E4E7"))

	DurationStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)
)

// formatFilePath formats a file path with proper styling
func FormatFilePath(path string) string {
	return FileStyle.Render(path)
}

// formatPosition formats a position (line:column) with proper styling
func FormatPosition(line int) string {
	return PositionStyle.Render(fmt.Sprintf("%d", line))
}

// printJSON outputs data as formatted JSON
func PrintJSON(w io.Writer, data interface{}) {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		fmt.Fprintf(w, "Error encoding JSON: %v\n", err)
	}
}

// printYAML outputs data as YAML
func PrintYAML(w io.Writer, data interface{}) {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	if err := encoder.Encode(data); err != nil {
		fmt.Fprintf(w, "Error encoding YAML: %v\n", err)
	}
	encoder.Close()
}

// Success prints a success message with styling
func Success(w io.Writer, message string) {
	icon := lipgloss.NewStyle().Foreground(SuccessColor).Bold(true).Render("✓")
	msg := lipgloss.NewStyle().Foreground(SuccessColor).Render(message)
	fmt.Fprintf(w, "%s %s\n", icon, msg)
}

func SuccessIcon() string {
	return lipgloss.NewStyle().Foreground(SuccessColor).Bold(true).Render("✓")
}

// Success prints a success message with styling
func SuccessString(message string) string {
	icon := lipgloss.NewStyle().Foreground(SuccessColor).Bold(true).Render("✓")
	return fmt.Sprintf("%s %s", icon, message)
}

func ErrorIcon() string {
	return lipgloss.NewStyle().Foreground(ErrorColor).Bold(true).Render("✗")
}

// Error prints an error message with styling
func Error(w io.Writer, message string) {
	icon := lipgloss.NewStyle().Foreground(ErrorColor).Bold(true).Render("✗")
	msg := lipgloss.NewStyle().Foreground(ErrorColor).Render(message)
	fmt.Fprintf(w, "%s %s\n", icon, msg)
}

func WarningIcon() string {
	return lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render("⚠")
}

// Warning prints a warning message with styling
func Warning(w io.Writer, message string) {
	icon := lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render("⚠")
	msg := lipgloss.NewStyle().Foreground(WarningColor).Render(message)
	fmt.Fprintf(w, "%s %s\n", icon, msg)
}

// Info prints an info message with styling
func Info(w io.Writer, message string) {
	icon := lipgloss.NewStyle().Foreground(InfoColor).Bold(true).Render("ℹ")
	msg := lipgloss.NewStyle().Foreground(InfoColor).Render(message)
	fmt.Fprintf(w, "%s %s\n", icon, msg)
}
