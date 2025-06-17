package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lacquerai/lacquer/internal/parser"
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
		validateWorkflows(args)
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

func validateWorkflows(args []string) {
	start := time.Now()

	// Collect files to validate
	files, err := collectFiles(args, recursive)
	if err != nil {
		Error(fmt.Sprintf("Failed to collect files: %v", err))
		os.Exit(1)
	}

	if len(files) == 0 {
		Warning("No workflow files found to validate")
		return
	}

	// Create parser
	yamlParser, err := parser.NewYAMLParser()
	if err != nil {
		Error(fmt.Sprintf("Failed to create parser: %v", err))
		os.Exit(1)
	}

	// Validate each file
	results := make([]ValidationResult, 0, len(files))

	for _, file := range files {
		result := validateSingleFile(yamlParser, file)
		results = append(results, result)

		// Show progress if not quiet and not JSON/YAML output
		if !viper.GetBool("quiet") && viper.GetString("output") == "text" {
			if result.Valid {
				if showAll {
					Success(fmt.Sprintf("%s (%v)", file, result.Duration))
				}
			}
			// Invalid results will be shown in the detailed summary section
		}
	}

	// Create summary
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
		printJSON(summary)
	case "yaml":
		printYAML(summary)
	default:
		printValidationSummary(summary)
	}

	// Exit with error code if any validations failed
	if summary.Invalid > 0 {
		os.Exit(1)
	}
}

func validateSingleFile(p parser.Parser, filename string) ValidationResult {
	start := time.Now()
	result := ValidationResult{
		File:     filename,
		Valid:    true,
		Duration: 0,
		Errors:   []string{},
		Warnings: []string{},
		Issues:   []*ValidationIssue{},
	}

	// Parse and validate the file
	workflow, err := p.ParseFile(filename)
	result.Duration = time.Since(start)

	if err != nil {
		result.Valid = false

		// Check if it's an enhanced error (might be wrapped)
		var enhancedErr *parser.MultiErrorEnhanced
		if errors.As(err, &enhancedErr) {
			// Store the enhanced error directly for full context
			result.EnhancedError = enhancedErr

			// Also process individual issues for compatibility
			for _, issue := range enhancedErr.GetAllIssues() {
				validationIssue := convertEnhancedErrorToIssue(issue)
				result.Issues = append(result.Issues, validationIssue)

				// Add simple error messages for backward compatibility
				if issue.Severity == parser.SeverityError {
					result.Errors = append(result.Errors, issue.Title)
				} else if issue.Severity == parser.SeverityWarning {
					result.Warnings = append(result.Warnings, issue.Title)
				}
			}
		} else {
			// Fallback to simple error handling
			result.Errors = append(result.Errors, err.Error())
		}

		return result
	}

	// Additional semantic validation (if not already done by parser)
	if err := workflow.Validate(); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
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

func printValidationSummary(summary ValidationSummary) {
	if !viper.GetBool("quiet") {
		fmt.Printf("\n")
		if summary.Invalid == 0 {
			Success(fmt.Sprintf("All %d workflow(s) are valid (%v)", summary.Total, summary.Duration))
		} else {
			Error(fmt.Sprintf("%d of %d workflow(s) failed validation (%v)", summary.Invalid, summary.Total, summary.Duration))
		}

		// Show detailed error information for failed files
		for _, result := range summary.Results {
			if !result.Valid {
				printValidationResult(result)
			}
		}

		if viper.GetBool("verbose") {
			fmt.Printf("\nDetailed results:\n")
			headers := []string{"File", "Status", "Duration", "Issues"}
			rows := make([][]string, len(summary.Results))

			for i, result := range summary.Results {
				status := "✅ Valid"
				issues := "0"
				if !result.Valid {
					status = "❌ Invalid"
					issues = fmt.Sprintf("%d", len(result.Issues))
				}
				rows[i] = []string{
					result.File,
					status,
					result.Duration.String(),
					issues,
				}
			}

			printTable(headers, rows)
		}
	}
}

// printValidationResult prints detailed information about a validation result
func printValidationResult(result ValidationResult) {
	if result.Valid {
		return // Don't print valid results in summary
	}

	fmt.Printf("\n❌ %s (%v)\n", result.File, result.Duration)

	// Print enhanced error details if available
	if result.EnhancedError != nil {
		for _, issue := range result.EnhancedError.GetAllIssues() {
			printEnhancedIssue(issue)
		}
	} else if len(result.Issues) > 0 {
		for _, issue := range result.Issues {
			printValidationIssue(issue)
		}
	} else {
		// Fallback to simple error messages
		for _, errMsg := range result.Errors {
			fmt.Printf("  %s\n", errMsg)
		}
	}
}

// printValidationIssue prints a detailed validation issue
func printValidationIssue(issue *ValidationIssue) {
	// Error header with position
	severityIcon := "❌"
	if issue.Severity == "warning" {
		severityIcon = "⚠️"
	} else if issue.Severity == "info" {
		severityIcon = "ℹ️"
	}

	// Add a separator line before each error (except the first one)
	fmt.Printf("  ┌─ %s %s at %d:%d: %s\n", severityIcon, issue.Severity, issue.Line, issue.Column, issue.Title)

	if issue.Message != "" && issue.Message != issue.Title {
		fmt.Printf("  │  %s\n", issue.Message)
	}

	if issue.Suggestion != nil {
		fmt.Printf("  │\n") // Add spacing before suggestions
		fmt.Printf("  │  💡 %s", issue.Suggestion.Title)
		if issue.Suggestion.Description != "" {
			fmt.Printf(": %s", issue.Suggestion.Description)
		}
		fmt.Printf("\n")

		// Show examples if available
		if len(issue.Suggestion.Examples) > 0 {
			fmt.Printf("  │\n") // Add spacing before examples
			fmt.Printf("  │  Example:\n")
			for _, example := range issue.Suggestion.Examples {
				fmt.Printf("  │    %s\n", example)
			}
		}

		// Show documentation link
		if issue.Suggestion.DocsURL != "" {
			fmt.Printf("  │\n") // Add spacing before docs link
			fmt.Printf("  │  📖 See: %s\n", issue.Suggestion.DocsURL)
		}
	}

	fmt.Printf("  └─\n\n") // Clear end separator with extra spacing
}

// printEnhancedIssue prints a detailed enhanced error with full context
func printEnhancedIssue(issue *parser.EnhancedError) {
	// Error header with position
	severityIcon := "❌"
	if issue.Severity == parser.SeverityWarning {
		severityIcon = "⚠️"
	} else if issue.Severity == parser.SeverityInfo {
		severityIcon = "ℹ️"
	}

	fmt.Printf("  ┌─ %s %s at %d:%d: %s\n", severityIcon, issue.Severity, issue.Position.Line, issue.Position.Column, issue.Title)

	if issue.Message != "" && issue.Message != issue.Title {
		fmt.Printf("  │  %s\n", issue.Message)
	}

	// Print source context if available
	if issue.Context != nil && len(issue.Context.Lines) > 0 {
		fmt.Printf("  │\n") // Add spacing before context
		for _, line := range issue.Context.Lines {
			if line.IsError {
				// Highlight the error line
				fmt.Printf("  │→ %4d | %s\n", line.Number, line.Content)
				// Add highlighting indicator if available
				if issue.Context.Highlight.Length > 0 {
					spaces := fmt.Sprintf("%*s", issue.Context.Highlight.StartColumn-1, "")
					highlight := fmt.Sprintf("%*s", issue.Context.Highlight.Length, "")
					for i := range highlight {
						highlight = highlight[:i] + "^" + highlight[i+1:]
					}
					fmt.Printf("  │  %4s   %s%s\n", "", spaces, highlight)
				}
			} else {
				// Regular context line
				fmt.Printf("  │  %4d | %s\n", line.Number, line.Content)
			}
		}
	}

	if issue.Suggestion != nil {
		fmt.Printf("  │\n") // Add spacing before suggestions
		fmt.Printf("  │  💡 %s", issue.Suggestion.Title)
		if issue.Suggestion.Description != "" {
			fmt.Printf(": %s", issue.Suggestion.Description)
		}
		fmt.Printf("\n")

		// Show examples if available
		if len(issue.Suggestion.Examples) > 0 {
			fmt.Printf("  │\n") // Add spacing before examples
			fmt.Printf("  │  Example:\n")
			for _, example := range issue.Suggestion.Examples {
				fmt.Printf("  │    %s\n", example)
			}
		}

		// Show documentation link
		if issue.Suggestion.DocsURL != "" {
			fmt.Printf("  │\n") // Add spacing before docs link
			fmt.Printf("  │  📖 See: %s\n", issue.Suggestion.DocsURL)
		}
	}

	fmt.Printf("  └─\n\n") // Clear end separator with extra spacing
}
