package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/server"
	"github.com/lacquerai/lacquer/internal/style"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Serve command flags
	servePort        int
	serveHost        string
	serveConcurrency int
	serveTimeout     time.Duration
	serveWorkflows   []string
	serveWorkflowDir string
	serveMetrics     bool
	serveCORS        bool
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve [workflow files...]",
	Short: "Start HTTP server for workflow execution",
	Long: `Start an HTTP server that can execute workflows via REST API.

The server provides:
- REST API for triggering workflow executions
- WebSocket streaming for real-time progress updates
- Prometheus metrics endpoint
- Concurrent execution of multiple workflows

Examples:
  laq serve workflow.laq.yaml                    # Serve single workflow
  laq serve workflow1.laq.yaml workflow2.laq.yaml # Serve multiple workflows  
  laq serve --workflow-dir ./workflows          # Serve all workflows in directory
  laq serve --port 8080 --host 0.0.0.0         # Custom host and port
  laq serve --concurrency 10 workflow.laq.yaml # Allow 10 concurrent executions`,
	Run: func(cmd *cobra.Command, args []string) {
		runCtx := execcontext.RunContext{
			Context: cmd.Context(),
			StdOut:  cmd.OutOrStdout(),
			StdErr:  cmd.OutOrStderr(),
		}

		// Collect workflow files from args and directory
		workflowFiles := args
		if serveWorkflowDir != "" {
			dirFiles, err := findWorkflowFiles(serveWorkflowDir)
			if err != nil {
				style.Error(runCtx, fmt.Sprintf("Failed to scan workflow directory: %v", err))
				os.Exit(1)
			}
			workflowFiles = append(workflowFiles, dirFiles...)
		}

		if len(workflowFiles) == 0 && len(serveWorkflows) == 0 {
			style.Error(runCtx, "No workflow files specified. Use arguments or --workflow-dir")
			os.Exit(1)
		}

		workflowFiles = append(workflowFiles, serveWorkflows...)
		startServer(runCtx, workflowFiles)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Server configuration
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "server port")
	serveCmd.Flags().StringVar(&serveHost, "host", "localhost", "server host")
	serveCmd.Flags().IntVar(&serveConcurrency, "concurrency", 5, "maximum concurrent executions")
	serveCmd.Flags().DurationVar(&serveTimeout, "timeout", 30*time.Minute, "default execution timeout")

	// Workflow specification
	serveCmd.Flags().StringSliceVarP(&serveWorkflows, "workflow", "w", []string{}, "workflow files to serve")
	serveCmd.Flags().StringVar(&serveWorkflowDir, "workflow-dir", "", "directory containing workflow files")

	// Features
	serveCmd.Flags().BoolVar(&serveMetrics, "metrics", true, "enable Prometheus metrics endpoint")
	serveCmd.Flags().BoolVar(&serveCORS, "cors", true, "enable CORS headers")
}

func startServer(runCtx execcontext.RunContext, workflowFiles []string) {
	// Create server configuration
	config := &server.Config{
		Host:          serveHost,
		Port:          servePort,
		Concurrency:   serveConcurrency,
		Timeout:       serveTimeout,
		EnableMetrics: serveMetrics,
		EnableCORS:    serveCORS,
		WorkflowFiles: workflowFiles,
		WorkflowDir:   serveWorkflowDir,
	}

	// Create server
	srv, err := server.New(config)
	if err != nil {
		style.Error(runCtx, fmt.Sprintf("Failed to create server: %v", err))
		os.Exit(1)
	}

	// Load workflows
	if err := srv.LoadWorkflows(); err != nil {
		style.Error(runCtx, fmt.Sprintf("Failed to load workflows: %v", err))
		os.Exit(1)
	}

	// Display startup info
	if !viper.GetBool("quiet") {
		style.Success(runCtx, fmt.Sprintf("Lacquer server starting at http://%s", srv.GetAddr()))
		fmt.Fprintf(runCtx, "📋 Loaded workflows: %d\n", srv.GetWorkflowCount())
		fmt.Fprintf(runCtx, "🚀 API: http://%s/api/v1/workflows\n", srv.GetAddr())
		if serveMetrics {
			fmt.Fprintf(runCtx, "📊 Metrics: http://%s/metrics\n", srv.GetAddr())
		}
	}

	// Start server with graceful shutdown
	if err := srv.StartWithGracefulShutdown(); err != nil {
		style.Error(runCtx, fmt.Sprintf("Server error: %v", err))
		os.Exit(1)
	}
}

// findWorkflowFiles finds workflow files in a directory
func findWorkflowFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".laq.yaml") || strings.HasSuffix(path, ".laq.yml")) {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}
