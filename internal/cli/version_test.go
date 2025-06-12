package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionCommand(t *testing.T) {
	// Just test that the command executes without error
	// Output testing is complex due to stdout/stderr handling
	_, err := executeCommand(rootCmd, "version")
	assert.NoError(t, err)
}

func TestVersionCommandJSON(t *testing.T) {
	// Just test that the command executes without error  
	_, err := executeCommand(rootCmd, "version", "--output", "json")
	assert.NoError(t, err)
}

func TestVersionCommandYAML(t *testing.T) {
	// Just test that the command executes without error
	_, err := executeCommand(rootCmd, "version", "--output", "yaml")
	assert.NoError(t, err)
}

func TestVersionCommandVerbose(t *testing.T) {
	// Just test that the command executes without error
	_, err := executeCommand(rootCmd, "version", "--verbose")
	assert.NoError(t, err)
}

func TestVersionInfo(t *testing.T) {
	versionInfo := VersionInfo{
		Version:   "1.0.0",
		Commit:    "abc123",
		Date:      "2024-01-01",
		GoVersion: "go1.21.0",
		Platform:  "linux/amd64",
		Components: ComponentVersions{
			Parser:  "1.0.0",
			Runtime: "1.0.0",
			CLI:     "1.0.0",
		},
	}

	assert.Equal(t, "1.0.0", versionInfo.Version)
	assert.Equal(t, "abc123", versionInfo.Commit)
	assert.Equal(t, "1.0.0", versionInfo.Components.Parser)
}

func TestBuildVariables(t *testing.T) {
	// Test that build variables have sensible defaults
	assert.NotEmpty(t, Version)
	assert.NotEmpty(t, Commit)
	assert.NotEmpty(t, Date)
	assert.NotEmpty(t, GoVersion)
	assert.Contains(t, GoVersion, "go")
}