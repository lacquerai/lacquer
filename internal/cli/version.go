package cli

import (
	"fmt"
	"io"
	"runtime"

	"github.com/lacquerai/lacquer/internal/style"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Build-time variables (set by goreleaser or build scripts)
var (
	Version   = "dev"
	Commit    = "unknown"
	Date      = "unknown"
	BuiltBy   = "unknown"
	GoVersion = runtime.Version()
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version information for laq, including build details and component versions.`,
	Example: `
  laq version              # Show basic version info
  laq version --output json # Show version info as JSON`,
	Run: func(cmd *cobra.Command, args []string) {
		showVersion(cmd)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// VersionInfo represents version information
type VersionInfo struct {
	Version   string `json:"version" yaml:"version"`
	Commit    string `json:"commit" yaml:"commit"`
	Date      string `json:"date" yaml:"date"`
	BuiltBy   string `json:"built_by" yaml:"built_by"`
	GoVersion string `json:"go_version" yaml:"go_version"`
	Platform  string `json:"platform" yaml:"platform"`
}

func showVersion(cmd *cobra.Command) {
	versionInfo := VersionInfo{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		BuiltBy:   BuiltBy,
		GoVersion: GoVersion,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	outputFormat := viper.GetString("output")

	switch outputFormat {
	case "json":
		style.PrintJSON(cmd.OutOrStdout(), versionInfo)
	case "yaml":
		style.PrintYAML(cmd.OutOrStdout(), versionInfo)
	default:
		printText(cmd.OutOrStdout(), versionInfo)
	}
}

func printText(w io.Writer, info VersionInfo) {
	fmt.Fprintf(w, "%s", info.Version)
}
