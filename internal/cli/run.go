package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lacquerai/lacquer/internal/engine"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/parser"
	"github.com/lacquerai/lacquer/internal/style"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [workflow.laq.yaml]",
	Short: "Execute a Lacquer workflow",
	Long: `Execute a Lacquer workflow locally with real-time progress reporting.

This command:
- Parses and validates the workflow syntax
- Initializes the runtime engine with configured providers
- Executes workflow steps sequentially with proper error handling
- Provides real-time progress updates and logging
- Supports graceful shutdown on interruption signals

Examples:
  laq run workflow.laq.yaml                    # Run workflow with default settings
  laq run workflow.laq.yaml --input key=value # Provide input parameters
  laq run workflow.laq.yaml --input-json '{"key": "value"}' # Provide input parameters as JSON
  laq run workflow.laq.yaml --output json     # JSON output for automation
  laq run workflow.laq.yaml --save-state      # Persist state for debugging`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Setup signal handling for graceful shutdown
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigChan
			log.Info().Msg("Received interrupt signal, shutting down gracefully...")
			cancel()
		}()

		// Apply timeout if specified
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		runCtx := execcontext.RunContext{
			Context: ctx,
			StdOut:  cmd.OutOrStdout(),
			StdErr:  cmd.OutOrStderr(),
		}

		inputsMap := make(map[string]interface{})
		if inputJSONRaw != "" {
			json.Unmarshal([]byte(inputJSONRaw), &inputsMap)
		}

		for k, v := range inputs {
			inputsMap[k] = v
		}

		err := runWorkflow(runCtx, args[0], inputsMap)
		if err != nil {
			os.Exit(1)
		}
	},
}

var (
	// Input parameters
	inputs       map[string]string
	inputJSONRaw string
	maxRetries   int
	timeout      time.Duration
)

func init() {
	rootCmd.AddCommand(runCmd)

	// Input flags
	runCmd.Flags().StringToStringVarP(&inputs, "input", "i", map[string]string{}, "input parameters (key=value)")
	runCmd.Flags().StringVarP(&inputJSONRaw, "input-json", "j", "", "input parameters as JSON")

	runCmd.Flags().IntVar(&maxRetries, "max-retries", 3, "maximum number of retries for failed steps")
	runCmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "overall execution timeout")
}

func runWorkflow(ctx execcontext.RunContext, workflowFile string, inputs map[string]interface{}) error {
	runner := engine.NewRunner(engine.NewProgressTracker(ctx, ""))
	result, err := runner.RunWorkflow(ctx, workflowFile, inputs)
	runner.Close()
	if err != nil {
		switch err.(type) {
		case *engine.InputValidationResult:
			printValidationErrors(ctx, err.(*engine.InputValidationResult))
		case *parser.MultiErrorEnhanced:
			result := NewValidationResult(workflowFile)
			result.CollectError(err)
			summary := ValidationSummary{
				Total:   1,
				Results: []ValidationResult{*result},
				Invalid: 1,
			}

			printValidationSummary(ctx, summary)
		}

		return err
	}

	outputResults(ctx, result)
	return nil
}

func outputResults(w io.Writer, result *engine.ExecutionResult) {
	outputFormat := viper.GetString("output")

	switch outputFormat {
	case "json":
		style.PrintJSON(w, result.Outputs)
	case "yaml":
		style.PrintYAML(w, result.Outputs)
	default:
		printExecutionSummary(w, result)
	}
}

func printExecutionSummary(w io.Writer, result *engine.ExecutionResult) {
	if viper.GetBool("quiet") {
		return
	}

	fmt.Fprintf(w, "\n")

	// Show success or failure with duration
	if result.Status == "completed" {
		fmt.Fprintf(w, "%s Workflow completed %s (%s)\n", style.SuccessIcon(), style.SuccessStyle.Render("successfully"), formatDuration(result.Duration))

	} else {
		fmt.Fprintf(w, "%s Workflow failed\n\n", style.ErrorIcon())
		// Show error details for failures
		if result.Error != "" {
			fmt.Fprintf(w, "%s\n", style.ErrorStyle.Render(result.Error))
		}
	}

	if len(result.Outputs) > 0 {
		var outputContent strings.Builder
		outputContent.WriteString("\n")
		outputContent.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render("Outputs"))
		outputContent.WriteString("\n\n")
		var i int
		keys := make([]string, 0, len(result.Outputs))
		for k := range result.Outputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := result.Outputs[k]
			outputContent.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render(k))
			outputContent.WriteString(fmt.Sprintf(": %v", v))
			if i < len(result.Outputs)-1 {
				outputContent.WriteString("\n")
			}
			i++
		}
		fmt.Fprintf(w, "%s\n", outputContent.String())
	}

}

func formatDuration(duration time.Duration) string {
	return fmt.Sprintf("%.2fs", duration.Seconds())
}

func printValidationErrors(w io.Writer, validationResult *engine.InputValidationResult) {
	fmt.Fprintf(w, "\n❌ Input validation failed:\n\n")

	for i, err := range validationResult.Errors {
		// Add spacing between errors for better readability
		if i > 0 {
			fmt.Fprintln(w)
		}

		// Format field name with color/emphasis
		fmt.Fprintf(w, "   Field: %s\n", err.Field)
		fmt.Fprintf(w, "   Error: %s\n", err.Message)

		// Show the actual value if provided
		if err.Value != nil {
			fmt.Fprintf(w, "   Value: %v\n", err.Value)
		}
	}

	fmt.Fprintf(w, "\n💡 Please check your input parameters and try again.\n")
}
