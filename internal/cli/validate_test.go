package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Version(t *testing.T) {
	newSingleDirectoryValidateTest(t)
}

func Test_Valid(t *testing.T) {
	newSingleDirectoryValidateTest(t)
}

func newSingleDirectoryValidateTest(t *testing.T) {
	t.Helper()

	// get the function name from the caller (i.e. the function that called this function)
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	if idx := strings.LastIndex(funcName, "."); idx != -1 {
		funcName = funcName[idx+1:]
	}

	funcName = strings.TrimPrefix(funcName, "Test_")

	funcName = camelToSnake(funcName)
	directory := "testdata/validate/" + funcName

	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			t.Fatalf("panic in run execution: %s\n%s", r, stack)
		}
	}()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	runCtx := execcontext.RunContext{
		Context: context.Background(),
		StdOut:  stdout,
		StdErr:  stderr,
	}

	_ = validateWorkflows(runCtx, []string{filepath.Join(directory, "workflow.laq.yml")})
	assertGoldenFile(t, directory, stdout, stderr)
}

func assertGoldenFile(t *testing.T, directory string, stdout *bytes.Buffer, stderr *bytes.Buffer) {
	t.Helper()

	goldenFile := filepath.Join(directory, "golden.txt")
	golden, err := os.ReadFile(goldenFile)

	// Remove ANSI codes and normalize time strings
	stdout_clean := re.ReplaceAllString(stdout.String(), "")
	stderr_clean := re.ReplaceAllString(stderr.String(), "")
	stdout_normalized := timeRe.ReplaceAllString(stdout_clean, "(TIME)")
	stderr_normalized := timeRe.ReplaceAllString(stderr_clean, "(TIME)")
	actual := stdout_normalized + "\nSTDERR:\n" + stderr_normalized

	if os.IsNotExist(err) {
		golden = []byte(actual)
		err = os.WriteFile(goldenFile, golden, 0644)
		require.NoError(t, err)
	} else {
		require.NoError(t, err)
	}

	if !assert.Equal(t, string(golden), actual) {
		os.WriteFile(filepath.Join(directory, "actual.txt"), []byte(actual), 0644)
	}
}
