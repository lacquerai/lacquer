package cli

import (
	"fmt"
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
	Version    string            `json:"version" yaml:"version"`
	Commit     string            `json:"commit" yaml:"commit"`
	Date       string            `json:"date" yaml:"date"`
	GoVersion  string            `json:"go_version" yaml:"go_version"`
	Platform   string            `json:"platform" yaml:"platform"`
	Components ComponentVersions `json:"components" yaml:"components"`
}

// ComponentVersions represents versions of different components
type ComponentVersions struct {
	Parser  string `json:"parser" yaml:"parser"`
	Runtime string `json:"runtime" yaml:"runtime"`
	CLI     string `json:"cli" yaml:"cli"`
}

func showVersion(cmd *cobra.Command) {
	versionInfo := VersionInfo{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: GoVersion,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		Components: ComponentVersions{
			Parser:  "1.0.0", // Will be from parser package
			Runtime: "1.0.0", // Will be from runtime package
			CLI:     Version,
		},
	}

	outputFormat := viper.GetString("output")

	switch outputFormat {
	case "json":
		style.PrintJSON(cmd.OutOrStdout(), versionInfo)
	case "yaml":
		style.PrintYAML(cmd.OutOrStdout(), versionInfo)
	default:
		printText(versionInfo)
	}
}

func printText(info VersionInfo) {
	fmt.Printf("laq version %s\n", info.Version)
	fmt.Printf("Commit: %s\n", info.Commit)
	fmt.Printf("Built: %s\n", info.Date)
	fmt.Printf("Go version: %s\n", info.GoVersion)
	fmt.Printf("Platform: %s\n", info.Platform)

	if viper.GetBool("verbose") {
		fmt.Printf("\nComponent versions:\n")
		fmt.Printf("  Parser: %s\n", info.Components.Parser)
		fmt.Printf("  Runtime: %s\n", info.Components.Runtime)
		fmt.Printf("  CLI: %s\n", info.Components.CLI)
	}
}
