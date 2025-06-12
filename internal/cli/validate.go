package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lacquer/lacquer/internal/parser"
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
	File     string        `json:"file" yaml:"file"`
	Valid    bool          `json:"valid" yaml:"valid"`
	Duration time.Duration `json:"duration_ms" yaml:"duration_ms"`
	Errors   []string      `json:"errors,omitempty" yaml:"errors,omitempty"`
	Warnings []string      `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

// ValidationSummary represents the summary of all validation results
type ValidationSummary struct {
	Total     int                `json:"total" yaml:"total"`
	Valid     int                `json:"valid" yaml:"valid"`
	Invalid   int                `json:"invalid" yaml:"invalid"`
	Duration  time.Duration      `json:"total_duration_ms" yaml:"total_duration_ms"`
	Results   []ValidationResult `json:"results" yaml:"results"`
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
			} else {
				Error(fmt.Sprintf("%s (%v)", file, result.Duration))
				for _, errMsg := range result.Errors {
					fmt.Printf("  %s\n", errMsg)
				}
			}
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
	}

	// Parse and validate the file
	workflow, err := p.ParseFile(filename)
	result.Duration = time.Since(start)

	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
		return result
	}

	// Additional semantic validation
	if err := workflow.Validate(); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
	}

	log.Debug().
		Str("file", filename).
		Bool("valid", result.Valid).
		Dur("duration", result.Duration).
		Msg("Validated workflow file")

	return result
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

		if viper.GetBool("verbose") {
			fmt.Printf("\nDetailed results:\n")
			headers := []string{"File", "Status", "Duration"}
			rows := make([][]string, len(summary.Results))
			
			for i, result := range summary.Results {
				status := "✅ Valid"
				if !result.Valid {
					status = "❌ Invalid"
				}
				rows[i] = []string{
					result.File,
					status,
					result.Duration.String(),
				}
			}
			
			printTable(headers, rows)
		}
	}
}