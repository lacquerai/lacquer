package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute(t *testing.T) {
	// Create a minimal command for testing
	originalRootCmd := rootCmd

	// Create test command
	testCmd := &cobra.Command{
		Use:   "laq",
		Short: "Test command",
		Run: func(cmd *cobra.Command, args []string) {
			// Do nothing
		},
	}

	rootCmd = testCmd
	defer func() { rootCmd = originalRootCmd }()

	// Test execution
	err := Execute()
	assert.NoError(t, err)
}

func TestGetVersion(t *testing.T) {
	version := getVersion()
	assert.Contains(t, version, "dev")
	assert.Contains(t, version, "unknown")
}

func TestInitLogging(t *testing.T) {
	// Test that initLogging doesn't panic
	require.NotPanics(t, func() {
		initLogging()
	})
}

func TestInitConfig(t *testing.T) {
	// Test that initConfig doesn't panic
	require.NotPanics(t, func() {
		initConfig()
	})
}

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	// Create a copy of the root command to avoid modifying the global one
	cmd := &cobra.Command{
		Use:   root.Use,
		Short: root.Short,
		Long:  root.Long,
		Run:   root.Run,
	}

	// Copy all subcommands
	for _, subCmd := range root.Commands() {
		cmd.AddCommand(subCmd)
	}

	// Copy flags
	cmd.Flags().AddFlagSet(root.Flags())
	cmd.PersistentFlags().AddFlagSet(root.PersistentFlags())

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err = cmd.Execute()
	return buf.String(), err
}

func TestRootCommand(t *testing.T) {
	output, err := executeCommand(rootCmd, "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Lacquer is a domain-specific language")
	assert.Contains(t, output, "Available Commands:")
}

func TestGlobalFlags(t *testing.T) {
	// Test that global flags are properly defined
	flag := rootCmd.PersistentFlags().Lookup("config")
	assert.NotNil(t, flag)
	assert.Equal(t, "string", flag.Value.Type())

	flag = rootCmd.PersistentFlags().Lookup("log-level")
	assert.NotNil(t, flag)
	assert.Equal(t, "info", flag.DefValue)

	flag = rootCmd.PersistentFlags().Lookup("output")
	assert.NotNil(t, flag)
	assert.Equal(t, "text", flag.DefValue)

	flag = rootCmd.PersistentFlags().Lookup("quiet")
	assert.NotNil(t, flag)
	assert.Equal(t, "bool", flag.Value.Type())

	flag = rootCmd.PersistentFlags().Lookup("verbose")
	assert.NotNil(t, flag)
	assert.Equal(t, "bool", flag.Value.Type())
}

func TestCommandAvailability(t *testing.T) {
	commands := []string{"init", "validate", "version"}

	for _, cmdName := range commands {
		cmd, _, err := rootCmd.Find([]string{cmdName})
		assert.NoError(t, err, "Command %s should be available", cmdName)
		assert.Equal(t, cmdName, cmd.Name(), "Command name should match")
	}
}

// Test helper to set environment variables
func setEnv(t *testing.T, key, value string) {
	originalValue := os.Getenv(key)
	err := os.Setenv(key, value)
	require.NoError(t, err)

	t.Cleanup(func() {
		if originalValue == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, originalValue)
		}
	})
}
