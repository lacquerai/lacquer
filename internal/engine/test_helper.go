package engine

import (
	"os"
	"testing"

	// Import shared test helper for logging configuration
	_ "github.com/lacquerai/lacquer/internal/testhelper"
)

// TestMain runs before all tests in this package
func TestMain(m *testing.M) {
	// Run tests - logging setup is handled by testhelper package
	code := m.Run()

	// Exit with the same code as the tests
	os.Exit(code)
}
