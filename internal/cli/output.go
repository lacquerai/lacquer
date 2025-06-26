package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss/v2"
	"gopkg.in/yaml.v3"
)

// printJSON outputs data as formatted JSON
func printJSON(data interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

// printYAML outputs data as YAML
func printYAML(data interface{}) {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	if err := encoder.Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding YAML: %v\n", err)
		os.Exit(1)
	}
	encoder.Close()
}

// printTable outputs data in a human-readable table format
func printTable(headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	for i, header := range headers {
		fmt.Printf("%-*s  ", widths[i], header)
	}
	fmt.Println()

	// Print separator
	for i := range headers {
		for j := 0; j < widths[i]; j++ {
			fmt.Print("-")
		}
		fmt.Print("  ")
	}
	fmt.Println()

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				fmt.Printf("%-*s  ", widths[i], cell)
			}
		}
		fmt.Println()
	}
}

// Success prints a success message with styling
func Success(message string) {
	icon := lipgloss.NewStyle().Foreground(lipgloss.Color("#66BB6A")).Bold(true).Render("✓")
	msg := lipgloss.NewStyle().Foreground(lipgloss.Color("#66BB6A")).Render(message)
	fmt.Printf("%s %s\n", icon, msg)
}

// Error prints an error message with styling
func Error(message string) {
	icon := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Bold(true).Render("✗")
	msg := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Render(message)
	fmt.Fprintf(os.Stderr, "%s %s\n", icon, msg)
}

// Warning prints a warning message with styling
func Warning(message string) {
	icon := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA726")).Bold(true).Render("⚠")
	msg := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA726")).Render(message)
	fmt.Printf("%s %s\n", icon, msg)
}

// Info prints an info message with styling
func Info(message string) {
	icon := lipgloss.NewStyle().Foreground(lipgloss.Color("#42A5F5")).Bold(true).Render("ℹ")
	msg := lipgloss.NewStyle().Foreground(lipgloss.Color("#42A5F5")).Render(message)
	fmt.Printf("%s %s\n", icon, msg)
}
