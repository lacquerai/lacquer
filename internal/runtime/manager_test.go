package runtime_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	rt "github.com/lacquerai/lacquer/internal/runtime"
	"github.com/lacquerai/lacquer/internal/runtime/cache"
	"github.com/lacquerai/lacquer/internal/runtime/golang"
	"github.com/lacquerai/lacquer/internal/runtime/node"
	"github.com/lacquerai/lacquer/internal/runtime/types"
	"github.com/lacquerai/lacquer/internal/runtime/utils"
)

// TestMain sets up and tears down test environment
func TestMain(m *testing.M) {
	// Create temp directory for test cache
	tempDir, err := os.MkdirTemp("", "runtime-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	// Set test cache directory
	_ = os.Setenv("TEST_CACHE_DIR", tempDir)

	// Run tests
	code := m.Run()

	// Cleanup
	_ = os.RemoveAll(tempDir) // Cleanup; ignore error
	os.Exit(code)
}

// getTestCacheDir returns the test cache directory
func getTestCacheDir(t *testing.T) string {
	dir := os.Getenv("TEST_CACHE_DIR")
	if dir == "" {
		t.Fatal("TEST_CACHE_DIR not set")
	}
	return dir
}

// TestGoRuntimeDownload tests downloading Go runtime
func TestGoRuntimeDownload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping download test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cache, err := cache.NewFileCache(filepath.Join(getTestCacheDir(t), "go-test"))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	downloader := utils.NewDefaultDownloader()
	goRuntime := golang.New(cache, downloader)

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{
			name:    "Download specific version",
			version: "1.21.5",
			wantErr: false,
		},
		{
			name:    "Download with go prefix",
			version: "go1.21.5",
			wantErr: false,
		},
		{
			name:    "Invalid version",
			version: "1.0.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := goRuntime.Get(ctx, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify installation
				goBin := filepath.Join(path, "bin", "go")
				if runtime.GOOS == "windows" {
					goBin += ".exe"
				}

				if _, err := os.Stat(goBin); err != nil {
					t.Errorf("Go binary not found at %s: %v", goBin, err)
				}

				// Test execution
				cmd := exec.Command("./go", "version")
				cmd.Dir = filepath.Dir(goBin)
				output, err := cmd.Output()
				if err != nil {
					t.Errorf("Failed to run go version: %v", err)
				}

				if !strings.Contains(string(output), "go1.21.5") {
					t.Errorf("Unexpected version output: %s", output)
				}
			}
		})
	}
}

// TestGoRuntimeCache tests caching behavior
func TestGoRuntimeCache(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping download test in short mode")
	}

	ctx := context.Background()
	cache, err := cache.NewFileCache(filepath.Join(getTestCacheDir(t), "cache-test"))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	downloader := utils.NewDefaultDownloader()
	goRuntime := golang.New(cache, downloader)

	version := "1.21.0"

	// First download
	start := time.Now()
	path1, err := goRuntime.Get(ctx, version)
	if err != nil {
		t.Fatalf("First download failed: %v", err)
	}
	firstDuration := time.Since(start)

	// Second download (should use cache)
	start = time.Now()
	path2, err := goRuntime.Get(ctx, version)
	if err != nil {
		t.Fatalf("Second download failed: %v", err)
	}
	secondDuration := time.Since(start)

	// Paths should be the same
	if path1 != path2 {
		t.Errorf("Cache returned different paths: %s vs %s", path1, path2)
	}

	// Second "download" should be much faster (from cache)
	if secondDuration > firstDuration/10 {
		t.Errorf("Cache doesn't seem to be working: first=%v, second=%v",
			firstDuration, secondDuration)
	}
}

// TestNodeRuntimeDownload tests downloading Node.js runtime
func TestNodeRuntimeDownload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping download test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cache, err := cache.NewFileCache(filepath.Join(getTestCacheDir(t), "node-test"))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	downloader := utils.NewDefaultDownloader()
	nodeRuntime := node.New(cache, downloader)

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{
			name:    "Download specific version",
			version: "18.19.0",
			wantErr: false,
		},
		{
			name:    "Download with v prefix",
			version: "v18.19.0",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := nodeRuntime.Get(ctx, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify installation
				nodeBin := filepath.Join(path, "bin", "node")
				if runtime.GOOS == "windows" {
					nodeBin = filepath.Join(path, "node.exe")
				}

				if _, err := os.Stat(nodeBin); err != nil {
					t.Errorf("Node binary not found at %s: %v", nodeBin, err)
				}

				// Test execution
				cmd := exec.Command(nodeBin, "--version") // #nosec G204 - test code with controlled binary
				output, err := cmd.Output()
				if err != nil {
					t.Errorf("Failed to run node --version: %v", err)
				}

				if !strings.Contains(string(output), "v18.19.0") {
					t.Errorf("Unexpected version output: %s", output)
				}
			}
		})
	}
}

// TestListVersions tests listing available versions
func TestListVersions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping download test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cache, err := cache.NewFileCache(filepath.Join(getTestCacheDir(t), "list-test"))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	downloader := utils.NewDefaultDownloader()

	t.Run("Go versions", func(t *testing.T) {
		goRuntime := golang.New(cache, downloader)
		versions, err := goRuntime.List(ctx)
		if err != nil {
			t.Fatalf("Failed to list Go versions: %v", err)
		}

		if len(versions) < 10 {
			t.Errorf("Expected at least 10 Go versions, got %d", len(versions))
		}

		// Check for known versions
		found := false
		for _, v := range versions {
			if v.Version == "go1.21.5" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find go1.21.5 in version list")
		}
	})

	t.Run("Node versions", func(t *testing.T) {
		nodeRuntime := node.New(cache, downloader)
		versions, err := nodeRuntime.List(ctx)
		if err != nil {
			t.Fatalf("Failed to list Node versions: %v", err)
		}

		if len(versions) < 10 {
			t.Errorf("Expected at least 10 Node versions, got %d", len(versions))
		}

		// Check for LTS versions
		ltsCount := 0
		for _, v := range versions {
			if v.Stable {
				ltsCount++
			}
		}
		if ltsCount < 3 {
			t.Errorf("Expected at least 3 LTS versions, got %d", ltsCount)
		}
	})
}

// TestManager tests the runtime manager
func TestManager(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping download test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	manager, err := rt.NewManager(filepath.Join(getTestCacheDir(t), "manager-test"))
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	t.Run("List runtimes", func(t *testing.T) {
		runtimes := manager.ListRuntimes()
		expected := []string{"go", "node"}

		if len(runtimes) != len(expected) {
			t.Errorf("Expected %d runtimes, got %d", len(expected), len(runtimes))
		}

		for _, exp := range expected {
			found := false
			for _, r := range runtimes {
				if r == exp {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected runtime %s not found", exp)
			}
		}
	})

	t.Run("Download through manager", func(t *testing.T) {
		// Download a small/old version to speed up test
		path, err := manager.Get(ctx, "go", "1.19.0")
		if err != nil {
			t.Fatalf("Failed to download through manager: %v", err)
		}

		// Verify path exists
		if _, err := os.Stat(path); err != nil {
			t.Errorf("Downloaded path doesn't exist: %v", err)
		}
	})

	t.Run("Get latest", func(t *testing.T) {
		// This might take a while, so only test listing
		versions, err := manager.List(ctx, "node")
		if err != nil {
			t.Fatalf("Failed to list versions: %v", err)
		}

		if len(versions) == 0 {
			t.Error("No versions returned")
		}

		// First version should be stable/LTS
		if !versions[0].Stable {
			t.Log("Warning: First version is not marked as stable")
		}
	})

	t.Run("Get installed", func(t *testing.T) {
		// First ensure we have something installed
		_, err := manager.Get(ctx, "go", "1.21.0")
		if err != nil {
			t.Fatalf("Failed to download: %v", err)
		}

		installed, err := manager.GetInstalled()
		if err != nil {
			t.Fatalf("Failed to get installed: %v", err)
		}

		if len(installed) == 0 {
			t.Error("Expected at least one installed runtime")
		}

		// Check the installed runtime
		found := false
		for _, info := range installed {
			if info.Name == "go" && strings.Contains(info.Version, "1.21.0") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find go1.19.0 in installed list")
		}
	})
}

// TestNetworkFailure tests behavior with network issues
func TestNetworkFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cache, err := cache.NewFileCache(filepath.Join(getTestCacheDir(t), "network-test"))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create a failing downloader
	failingDownloader := &failingDownloader{}
	goRuntime := golang.New(cache, failingDownloader)

	_, err = goRuntime.Get(ctx, "1.21.5")
	if err == nil {
		t.Error("Expected error with failing downloader")
	}

	if !strings.Contains(err.Error(), "network failure") {
		t.Errorf("Expected network failure error, got: %v", err)
	}
}

// failingDownloader always fails
type failingDownloader struct{}

func (d *failingDownloader) Download(ctx context.Context, url string, writer io.Writer) error {
	return fmt.Errorf("network failure simulation")
}

func TestConcurrentDownloads(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping download test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	manager, err := rt.NewManager(filepath.Join(getTestCacheDir(t), "concurrent-test"))
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Define downloads
	downloads := []struct {
		runtime string
		version string
	}{
		{"go", "1.20.0"},
		{"go", "1.19.0"},
		{"node", "18.0.0"},
		{"node", "16.20.0"},
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(downloads))

	for _, dl := range downloads {
		wg.Add(1)
		go func(runtime, version string) {
			defer wg.Done()

			start := time.Now()
			path, err := manager.Get(ctx, runtime, version)
			duration := time.Since(start)

			if err != nil {
				errors <- fmt.Errorf("%s %s: %v", runtime, version, err)
				return
			}

			t.Logf("Downloaded %s %s to %s in %v", runtime, version, path, duration)

			// Verify installation
			if _, err := os.Stat(path); err != nil {
				errors <- fmt.Errorf("%s %s: path not found: %v", runtime, version, err)
			}
		}(dl.runtime, dl.version)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}
}

// TestCachePersistence tests that cache persists across manager instances
func TestCachePersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping download test in short mode")
	}

	ctx := context.Background()
	cacheDir := filepath.Join(getTestCacheDir(t), "persistence-test")

	// First manager downloads
	manager1, err := rt.NewManager(cacheDir)
	if err != nil {
		t.Fatalf("Failed to create first manager: %v", err)
	}

	path1, err := manager1.Get(ctx, "go", "1.19.5")
	if err != nil {
		t.Fatalf("Failed to download with first manager: %v", err)
	}

	// Create new manager with same cache directory
	manager2, err := rt.NewManager(cacheDir)
	if err != nil {
		t.Fatalf("Failed to create second manager: %v", err)
	}

	// Should get from cache instantly
	start := time.Now()
	path2, err := manager2.Get(ctx, "go", "1.19.5")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to get from cache with second manager: %v", err)
	}

	if path1 != path2 {
		t.Errorf("Different paths returned: %s vs %s", path1, path2)
	}

	if duration > 1*time.Second {
		t.Errorf("Cache lookup took too long: %v", duration)
	}

	// Check installed runtimes
	installed, err := manager2.GetInstalled()
	if err != nil {
		t.Fatalf("Failed to get installed: %v", err)
	}

	found := false
	for _, info := range installed {
		if info.Name == "go" && info.Version == "go1.19.5" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find go1.19.5 in installed list")
	}
}

// TestWorkflowIntegration tests using runtimes in a workflow scenario
func TestWorkflowIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping download test in short mode")
	}

	ctx := context.Background()
	manager, err := rt.NewManager(filepath.Join(getTestCacheDir(t), "workflow-test"))
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Download runtimes for workflow
	goPath, err := manager.Get(ctx, "go", "1.20.0")
	if err != nil {
		t.Fatalf("Failed to download Go: %v", err)
	}

	nodePath, err := manager.Get(ctx, "node", "18.0.0")
	if err != nil {
		t.Fatalf("Failed to download Node: %v", err)
	}

	// Create a test project
	projectDir := filepath.Join(getTestCacheDir(t), "test-project")
	if err := os.MkdirAll(projectDir, 0750); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create a simple Go file
	goFile := filepath.Join(projectDir, "main.go")
	goContent := `package main

import "fmt"

func main() {
    fmt.Println("Hello from Go!")
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0600); err != nil {
		t.Fatalf("Failed to write Go file: %v", err)
	}

	// Create a simple Node.js file
	jsFile := filepath.Join(projectDir, "app.js")
	jsContent := `console.log("Hello from Node.js!");`
	if err := os.WriteFile(jsFile, []byte(jsContent), 0600); err != nil {
		t.Fatalf("Failed to write JS file: %v", err)
	}

	// Set up environment
	env := os.Environ()

	// Add Go to environment
	goBin := filepath.Join(goPath, "bin")
	env = append(env, fmt.Sprintf("GOROOT=%s", goPath))
	env = updatePath(env, goBin)

	// Add Node to environment
	nodeBin := filepath.Join(nodePath, "bin")
	if runtime.GOOS == "windows" {
		nodeBin = nodePath
	}
	env = updatePath(env, nodeBin)

	// Run Go program
	t.Run("Run Go program", func(t *testing.T) {
		goExe := "go"
		if runtime.GOOS == "windows" {
			goExe = "go.exe"
		}

		cmd := exec.Command(filepath.Join(goBin, goExe), "run", "main.go") // #nosec G204 - test code
		cmd.Dir = projectDir
		cmd.Env = env

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run Go program: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(string(output), "Hello from Go!") {
			t.Errorf("Unexpected Go output: %s", output)
		}
	})

	// Run Node program
	t.Run("Run Node program", func(t *testing.T) {
		nodeExe := "node"
		if runtime.GOOS == "windows" {
			nodeExe = "node.exe"
		}

		cmd := exec.Command(filepath.Join(nodeBin, nodeExe), "app.js") // #nosec G204 - test code
		cmd.Dir = projectDir
		cmd.Env = env

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run Node program: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(string(output), "Hello from Node.js!") {
			t.Errorf("Unexpected Node output: %s", output)
		}
	})
}

// TestCleanup tests the cleanup functionality
func TestCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping download test in short mode")
	}

	ctx := context.Background()
	cacheDir := filepath.Join(getTestCacheDir(t), "cleanup-test")

	manager, err := rt.NewManager(cacheDir)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Download some runtimes
	_, err = manager.Get(ctx, "go", "1.19.0")
	if err != nil {
		t.Fatalf("Failed to download Go: %v", err)
	}

	_, err = manager.Get(ctx, "go", "1.20.0")
	if err != nil {
		t.Fatalf("Failed to download Go: %v", err)
	}

	// Verify they're installed
	installed, err := manager.GetInstalled()
	if err != nil {
		t.Fatalf("Failed to get installed: %v", err)
	}

	goCount := 0
	for _, info := range installed {
		if info.Name == "go" {
			goCount++
		}
	}

	if goCount != 2 {
		t.Errorf("Expected 2 Go installations, got %d", goCount)
	}

	// Clean Go cache
	if err := manager.Clean("go"); err != nil {
		t.Fatalf("Failed to clean Go cache: %v", err)
	}

	// Verify cleanup
	installed, err = manager.GetInstalled()
	if err != nil {
		t.Fatalf("Failed to get installed after cleanup: %v", err)
	}

	for _, info := range installed {
		if info.Name == "go" {
			t.Errorf("Found Go installation after cleanup: %+v", info)
		}
	}

	// Verify cache directory is gone
	goCacheDir := filepath.Join(cacheDir, "go")
	if _, err := os.Stat(goCacheDir); !os.IsNotExist(err) {
		t.Error("Go cache directory still exists after cleanup")
	}
}

// TestExtractors tests different archive formats
func TestExtractors(t *testing.T) {
	tests := []struct {
		filename string
		wantType string
		wantErr  bool
	}{
		{"file.tar.gz", "*utils.TarGzExtractor", false},
		{"file.tgz", "*utils.TarGzExtractor", false},
		{"file.zip", "*utils.ZipExtractor", false},
		{"file.txt", "", true},
		{"file.tar", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			extractor, err := utils.GetExtractor(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetExtractor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				gotType := fmt.Sprintf("%T", extractor)
				if gotType != tt.wantType {
					t.Errorf("GetExtractor() = %v, want %v", gotType, tt.wantType)
				}
			}
		})
	}
}

// TestPlatformDetection tests platform detection
func TestPlatformDetection(t *testing.T) {
	platform := types.GetPlatform()

	if platform.OS == "" {
		t.Error("Platform OS is empty")
	}

	if platform.Arch == "" {
		t.Error("Platform Arch is empty")
	}

	// Verify against runtime package
	if platform.OS != runtime.GOOS {
		t.Errorf("Platform OS = %s, want %s", platform.OS, runtime.GOOS)
	}

	if platform.Arch != runtime.GOARCH {
		t.Errorf("Platform Arch = %s, want %s", platform.Arch, runtime.GOARCH)
	}
}

// updatePath is a helper to update PATH in environment
func updatePath(env []string, dir string) []string {
	pathSep := ":"
	if runtime.GOOS == "windows" {
		pathSep = ";"
	}

	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = e + pathSep + dir
			return env
		}
	}

	return append(env, "PATH="+dir)
}

// BenchmarkCacheLookup benchmarks cache lookup performance
func BenchmarkCacheLookup(b *testing.B) {
	manager, err := rt.NewManager(filepath.Join(getTestCacheDir(&testing.T{}), "bench-test"))
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}

	// Ensure something is cached
	ctx := context.Background()
	_, err = manager.Get(ctx, "go", "1.19.0")
	if err != nil {
		b.Fatalf("Failed to download: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := manager.Get(ctx, "go", "1.19.0")
		if err != nil {
			b.Fatalf("Cache lookup failed: %v", err)
		}
	}
}
