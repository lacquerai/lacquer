package testhelper

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
)

// init disables logging for tests unless explicitly enabled
func init() {
	// Check if we're running tests (testing.Testing() or common test environment variables)
	if isTesting() {
		// Disable logging for all tests unless LACQUER_TEST_LOG is set
		if os.Getenv("LACQUER_TEST_LOG") == "" {
			zerolog.SetGlobalLevel(zerolog.Disabled)
		}
	}
}

// isTesting returns true if we're currently running tests
func isTesting() bool {
	// Check for common test indicators
	return testing.Testing() ||
		os.Getenv("GO_TEST") != "" ||
		os.Args[0] == "go test" ||
		(len(os.Args) > 0 && os.Args[0] == "test") ||
		(len(os.Args) > 1 && os.Args[1] == "test")
}

// TestMain runs before all tests in this package
func TestMain(m *testing.M) {
	// Ensure logging is disabled for tests (redundant safety check)
	if os.Getenv("LACQUER_TEST_LOG") == "" {
		zerolog.SetGlobalLevel(zerolog.Disabled)
	}

	// Run tests
	code := m.Run()

	// Exit with the same code as the tests
	os.Exit(code)
}
