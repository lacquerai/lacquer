package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/parser"
	"github.com/lacquerai/lacquer/internal/style"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate [files...]",
	Short: "Validate workflow syntax and semantics",
	Long: `Validate Lacquer workflow files for syntax errors, schema compliance, and semantic correctness.

This command checks:
- YAML syntax validity
- JSON schema compliance
- Agent reference validation
- Step dependency analysis
- Variable interpolation syntax

Examples:
  laq validate workflow.laq.yaml           # Validate single file
  laq validate *.laq.yaml                  # Validate multiple files
  laq validate --recursive ./workflows    # Validate directory recursively
  laq validate --output json workflow.laq.yaml  # JSON output for CI/CD`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runCtx := execcontext.RunContext{
			Context: context.Background(),
			StdOut:  cmd.OutOrStdout(),
			StdErr:  cmd.OutOrStderr(),
		}
		err := validateWorkflows(runCtx, args)
		if err != nil {
			os.Exit(1)
		}
	},
}

var (
	recursive bool
	showAll   bool
)

func init() {
	rootCmd.AddCommand(validateCmd)

	validateCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "recursively validate files in directories")
	validateCmd.Flags().BoolVar(&showAll, "show-all", false, "show all validation results, including successful ones")
}

// ValidationResult represents the result of validating a workflow
type ValidationResult struct {
	File          string                     `json:"file" yaml:"file"`
	Valid         bool                       `json:"valid" yaml:"valid"`
	Duration      time.Duration              `json:"duration_ms" yaml:"duration_ms"`
	Errors        []string                   `json:"errors,omitempty" yaml:"errors,omitempty"`
	Warnings      []string                   `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Issues        []*ValidationIssue         `json:"issues,omitempty" yaml:"issues,omitempty"`
	EnhancedError *parser.MultiErrorEnhanced `json:"-" yaml:"-"` // For internal use only
}

func NewValidationResult(file string) *ValidationResult {
	return &ValidationResult{
		File:  file,
		Valid: true,
	}
}

func (v *ValidationResult) CollectError(err error) {
	if err == nil {
		return
	}

	v.Valid = false

	var enhancedErr *parser.MultiErrorEnhanced
	if errors.As(err, &enhancedErr) {
		// Store the enhanced error directly for full context
		v.EnhancedError = enhancedErr

		// Also process individual issues for compatibility
		for _, issue := range enhancedErr.GetAllIssues() {
			validationIssue := convertEnhancedErrorToIssue(issue)
			v.Issues = append(v.Issues, validationIssue)

			// Add simple error messages for backward compatibility
			if issue.Severity == parser.SeverityError {
				v.Errors = append(v.Errors, issue.Title)
			} else if issue.Severity == parser.SeverityWarning {
				v.Warnings = append(v.Warnings, issue.Title)
			}
		}
	} else {
		// Fallback to simple error handling
		v.Errors = append(v.Errors, err.Error())
	}
}

// ValidationIssue represents a detailed validation issue
type ValidationIssue struct {
	ID         string           `json:"id" yaml:"id"`
	Severity   string           `json:"severity" yaml:"severity"`
	Title      string           `json:"title" yaml:"title"`
	Message    string           `json:"message" yaml:"message"`
	Line       int              `json:"line" yaml:"line"`
	Column     int              `json:"column" yaml:"column"`
	Category   string           `json:"category" yaml:"category"`
	Suggestion *IssueSuggestion `json:"suggestion,omitempty" yaml:"suggestion,omitempty"`
}

// IssueSuggestion provides actionable advice
type IssueSuggestion struct {
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	Examples    []string `json:"examples,omitempty" yaml:"examples,omitempty"`
	DocsURL     string   `json:"docs_url,omitempty" yaml:"docs_url,omitempty"`
}

// ValidationSummary represents the summary of all validation results
type ValidationSummary struct {
	Total    int                `json:"total" yaml:"total"`
	Valid    int                `json:"valid" yaml:"valid"`
	Invalid  int                `json:"invalid" yaml:"invalid"`
	Duration time.Duration      `json:"total_duration_ms" yaml:"total_duration_ms"`
	Results  []ValidationResult `json:"results" yaml:"results"`
}

func validateWorkflows(runCtx execcontext.RunContext, args []string) error {
	start := time.Now()

	files, err := collectFiles(args, recursive)
	if err != nil {
		style.Error(runCtx, fmt.Sprintf("Failed to collect files: %v", err))
		return err
	}

	if len(files) == 0 {
		style.Warning(runCtx, "No workflow files found to validate")
		return nil
	}

	yamlParser, err := parser.NewYAMLParser()
	if err != nil {
		style.Error(runCtx, fmt.Sprintf("Failed to create parser: %v", err))
		return err
	}

	results := make([]ValidationResult, 0, len(files))

	for _, file := range files {
		result := validateSingleFile(yamlParser, file)
		results = append(results, *result)

		if !viper.GetBool("quiet") && viper.GetString("output") == "text" {
			if result.Valid {
				if showAll {
					style.Success(runCtx, fmt.Sprintf("%s (%v)", file, result.Duration))
				}
			}
		}
	}

	summary := ValidationSummary{
		Total:    len(results),
		Duration: time.Since(start),
		Results:  results,
	}

	for _, result := range results {
		if result.Valid {
			summary.Valid++
		} else {
			summary.Invalid++
		}
	}

	// Output results
	outputFormat := viper.GetString("output")
	switch outputFormat {
	case "json":
		style.PrintJSON(runCtx, summary)
	case "yaml":
		style.PrintYAML(runCtx, summary)
	default:
		printValidationSummary(runCtx, summary)
	}

	if summary.Invalid > 0 {
		return fmt.Errorf("validation failed")
	}

	return nil
}

func validateSingleFile(p parser.Parser, filename string) *ValidationResult {
	start := time.Now()
	result := NewValidationResult(filename)

	// Parse and validate the file
	_, err := p.ParseFile(filename)
	result.Duration = time.Since(start)
	if err != nil {
		result.CollectError(err)
		return result
	}

	log.Debug().
		Str("file", filename).
		Bool("valid", result.Valid).
		Dur("duration", result.Duration).
		Int("issues", len(result.Issues)).
		Msg("Validated workflow file")

	return result
}

// convertEnhancedErrorToIssue converts an enhanced error to a validation issue
func convertEnhancedErrorToIssue(err *parser.EnhancedError) *ValidationIssue {
	issue := &ValidationIssue{
		ID:       err.ID,
		Severity: string(err.Severity),
		Title:    err.Title,
		Message:  err.Message,
		Line:     err.Position.Line,
		Column:   err.Position.Column,
		Category: err.Category,
	}

	if err.Suggestion != nil {
		issue.Suggestion = &IssueSuggestion{
			Title:       err.Suggestion.Title,
			Description: err.Suggestion.Description,
			Examples:    err.Suggestion.Examples,
			DocsURL:     err.Suggestion.DocsURL,
		}
	}

	return issue
}

func collectFiles(args []string, recursive bool) ([]string, error) {
	var files []string

	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, fmt.Errorf("cannot access %s: %w", arg, err)
		}

		if info.IsDir() {
			if recursive {
				err := filepath.Walk(arg, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if isLacquerFile(path) {
						files = append(files, path)
					}
					return nil
				})
				if err != nil {
					return nil, fmt.Errorf("error walking directory %s: %w", arg, err)
				}
			} else {
				return nil, fmt.Errorf("%s is a directory, use --recursive to validate directories", arg)
			}
		} else if isLacquerFile(arg) {
			files = append(files, arg)
		} else {
			return nil, fmt.Errorf("%s is not a Lacquer workflow file (.laq.yaml or .laq.yml)", arg)
		}
	}

	return files, nil
}

func isLacquerFile(filename string) bool {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filepath.Base(filename), ext)

	return (ext == ".yaml" || ext == ".yml") && strings.HasSuffix(base, ".laq")
}

func printValidationSummary(w io.Writer, summary ValidationSummary) {
	if !viper.GetBool("quiet") {
		fmt.Fprintf(w, "\n")
		if summary.Invalid == 0 {
			style.Success(w, fmt.Sprintf("All %d workflow(s) are valid", summary.Total))
		} else {
			style.Error(w, fmt.Sprintf("%d of %d workflow(s) failed validation", summary.Invalid, summary.Total))
		}

		// Show detailed error information for failed files
		for _, result := range summary.Results {
			if !result.Valid {
				printValidationResultStyled(w, result)
			}
		}
	}
}

// printValidationResultStyled prints detailed information about a validation result with styling
func printValidationResultStyled(w io.Writer, result ValidationResult) {
	if result.Valid {
		return // Don't print valid results in summary
	}

	// Print enhanced error details if available
	if result.EnhancedError != nil {
		for _, issue := range result.EnhancedError.GetAllIssues() {
			printEnhancedIssueStyled(w, result, issue)
		}
	} else if len(result.Issues) > 0 {
		for _, issue := range result.Issues {
			printValidationIssueStyled(w, result, issue)
		}
	} else {
		// Fallback to simple error messages
		for _, errMsg := range result.Errors {
			fmt.Fprintf(w, "  %s\n", style.ErrorStyle.Render(errMsg))
		}
	}
}

// printValidationIssueStyled prints a detailed validation issue with styling
func printValidationIssueStyled(w io.Writer, result ValidationResult, issue *ValidationIssue) {
	var output strings.Builder

	// Build the header
	severityIcon := style.GetSeverityIcon(issue.Severity)
	severityStyled := style.GetSeverityStyle(issue.Severity)

	// Create the issue box
	header := fmt.Sprintf("%s %s at %s:%d", severityIcon, severityStyled.Render(issue.Severity), style.FileStyle.Render(result.File), issue.Line)
	output.WriteString(header + "\n")

	if issue.Message != "" && issue.Message != issue.Title {
		output.WriteString("\n" + style.MessageStyle.Render(issue.Message) + "\n")
	}

	if issue.Suggestion != nil {
		suggestionText := style.RenderSuggestion(
			issue.Suggestion.Title,
			issue.Suggestion.Description,
			issue.Suggestion.Examples,
			issue.Suggestion.DocsURL,
		)
		output.WriteString("\n" + suggestionText)
	}

	// Apply box styling based on severity
	var boxStyle lipgloss.Style
	switch issue.Severity {
	case "error":
		boxStyle = style.ErrorBoxStyle
	case "warning":
		boxStyle = style.WarningBoxStyle
	default:
		boxStyle = style.InfoBoxStyle
	}

	fmt.Fprint(w, boxStyle.Render(output.String()))
}

// printEnhancedIssueStyled prints a detailed enhanced error with full context and styling
func printEnhancedIssueStyled(w io.Writer, result ValidationResult, issue *parser.EnhancedError) {
	var output strings.Builder

	// Build the header
	severityIcon := style.GetSeverityIcon(string(issue.Severity))
	severityStyled := style.GetSeverityStyle(string(issue.Severity))
	position := style.FormatPosition(issue.Position.Line)

	// Create the issue header
	header := fmt.Sprintf("%s %s at %s:%s", severityIcon, severityStyled.Render(string(issue.Severity)), style.FileStyle.Render(result.File), position)
	output.WriteString(header + "\n")

	if issue.Message != "" && issue.Message != issue.Title {
		output.WriteString("\n" + style.MessageStyle.Render(issue.Message) + "\n")
	}

	// Print source context if available
	if issue.Context != nil && len(issue.Context.Lines) > 0 {
		var contextLines strings.Builder
		for _, line := range issue.Context.Lines {
			renderedLine := style.RenderCodeLine(line.Number, line.Content, line.IsError)
			contextLines.WriteString(renderedLine + "\n")

			// Add highlighting indicator if this is the error line
			if line.IsError && issue.Context.Highlight.Length > 0 {
				indicator := style.RenderHighlightIndicator(issue.Context.Highlight.StartColumn, issue.Context.Highlight.Length)
				contextLines.WriteString(indicator + "\n")
			}
		}
		output.WriteString("\n")
		output.WriteString(style.ContextBoxStyle.Render(strings.TrimSuffix(contextLines.String(), "\n")))
		output.WriteString("\n")
	}

	if issue.Suggestion != nil {
		suggestionText := style.RenderSuggestion(
			issue.Suggestion.Title,
			issue.Suggestion.Description,
			issue.Suggestion.Examples,
			issue.Suggestion.DocsURL,
		)
		output.WriteString("\n" + suggestionText)
	}

	// Apply box styling based on severity
	var boxStyle lipgloss.Style
	switch issue.Severity {
	case parser.SeverityError:
		boxStyle = style.ErrorBoxStyle
	case parser.SeverityWarning:
		boxStyle = style.WarningBoxStyle
	default:
		boxStyle = style.InfoBoxStyle
	}

	fmt.Fprint(w, boxStyle.Render(output.String()))
}
