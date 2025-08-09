package cli

import (
	"context"
	"fmt"
	"image/color"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lacquerai/lacquer/internal/style"
)

var (
	// Global flags
	cfgFile      string
	logLevel     string
	outputFormat string
	quiet        bool
	verbose      bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "laq",
	Short: "Lacquer - Where AI workflows get their shine",
	Long: `Lacquer is a domain-specific language (DSL) and runtime for orchestrating AI agent workflows.

The laq CLI tool enables developers to create, validate, and run AI workflows using a declarative YAML-based syntax.

Visit https://lacquer.ai/docs for documentation and examples.`,
	Version: getVersion(),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initLogging()
		go triggerBackgroundUpdateCheck()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		showUpdateNotificationIfAvailable()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return fang.Execute(context.Background(), rootCmd, fang.WithColorSchemeFunc(func(lightDark lipgloss.LightDarkFunc) fang.ColorScheme {
		return fang.ColorScheme{
			Base:           style.PrimaryTextColor,
			Title:          style.AccentColor,
			Description:    style.PrimaryTextColor,
			Codeblock:      style.CodeColor,
			Program:        style.AccentColor,
			DimmedArgument: style.MutedColor,
			Comment:        style.MutedColor,
			Flag:           style.InfoColor,
			FlagDefault:    style.MutedColor,
			Command:        style.SuccessColor,
			QuotedString:   style.WarningColor,
			Argument:       style.PrimaryTextColor,
			Help:           style.InfoColor,
			Dash:           style.MutedColor,
			ErrorHeader:    [2]color.Color{style.ErrorColor, style.ErrorBgColor},
			ErrorDetails:   style.ErrorColor,
		}
	}))
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.lacquer/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "disabled", "log level (debug, info, warn, error) (default: disabled)")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output", "text", "output format (text, json, yaml)")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Bind flags to viper
	_ = viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	_ = godotenv.Load()

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".lacquer" (without extension).
		viper.AddConfigPath(home + "/.lacquer")
		viper.AddConfigPath(".")
		viper.AddConfigPath(".lacquer")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// Environment variables
	viper.SetEnvPrefix("LACQUER")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
		}
	}
}

// initLogging configures the global logger
func initLogging() {
	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Set log level
	level := viper.GetString("log-level")
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.Disabled)
	}

	// Configure console output for better readability
	if !viper.GetBool("quiet") && outputFormat == "text" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
}

// getVersion returns the version information
func getVersion() string {
	// This will be populated by build-time variables
	var (
		version   = "dev"
		commit    = "unknown"
		date      = "unknown"
		goVersion = "unknown"
	)

	return fmt.Sprintf("%s (commit: %s, built: %s, go: %s)", version, commit, date, goVersion)
}

// triggerBackgroundUpdateCheck performs a background check for updates if the cache is expired
func triggerBackgroundUpdateCheck() {
	// Create a dummy command for the update check (won't be used for output)
	dummyCmd := &cobra.Command{}

	// This will check for updates and cache the result for future use
	// It runs silently in the background and doesn't print anything
	checkForUpdate(dummyCmd, false)
}

// showUpdateNotificationIfAvailable checks for available updates and shows a notification
func showUpdateNotificationIfAvailable() {
	// Skip notification if quiet mode is enabled
	if viper.GetBool("quiet") {
		return
	}

	// Check if an update is available (from cache only, no network calls)
	updateInfo := ShouldShowUpdateNotification()
	if updateInfo != nil {
		// Print the update notification on the last line
		fmt.Fprintf(os.Stderr, "\n%s A newer version (%s) is available! Run 'laq update' to upgrade.\n",
			style.InfoIcon(), updateInfo.LatestVersion)
	}
}
