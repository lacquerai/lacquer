package types

import (
	"context"
	"errors"
	"io"
	"runtime"
)

// GetPlatform returns the current platform
func GetPlatform() Platform {
	return Platform{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
}

// Runtime represents a downloadable runtime environment
type Runtime interface {
	// Get downloads and installs the specified version of the runtime
	// Returns the installation path
	Get(ctx context.Context, version string) (string, error)

	// GetLatest downloads and installs the latest stable version
	GetLatest(ctx context.Context) (string, error)

	// List returns available versions for the runtime
	List(ctx context.Context) ([]Version, error)

	// Name returns the name of the runtime (e.g., "go", "python", "node")
	Name() string
}

// Version represents a runtime version
type Version struct {
	Version      string
	Stable       bool
	ReleaseDate  string
	DownloadURLs map[string]string // key: platform-arch
}

// Platform represents the target platform
type Platform struct {
	OS   string
	Arch string
}

// Downloader handles downloading files
type Downloader interface {
	Download(ctx context.Context, url string, writer io.Writer) error
}

// Extractor handles archive extraction
type Extractor interface {
	Extract(src, dest string) error
}

// Cache manages downloaded runtimes
type Cache interface {
	Get(runtime, version string) (string, bool)
	Set(runtime, version, path string) error
	Path(runtime, version string) string
	SetManifest(runtime string, manifest []Version) error
	GetManifest(runtime string) ([]Version, error)
}

var (
	ErrManifestExpired = errors.New("manifest expired")
)
