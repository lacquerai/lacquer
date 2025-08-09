package block

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lacquerai/lacquer/internal/execcontext"
)

// DockerExecutor executes Docker blocks
type DockerExecutor struct {
	pullTimeout  time.Duration
	buildTimeout time.Duration
}

// NewDockerExecutor creates a new Docker block executor
func NewDockerExecutor() *DockerExecutor {
	return &DockerExecutor{
		pullTimeout:  5 * time.Minute,
		buildTimeout: 10 * time.Minute,
	}
}

// Validate checks if the executor can handle the given block
func (e *DockerExecutor) Validate(block *Block) error {
	if block.Runtime != RuntimeDocker {
		return fmt.Errorf("invalid runtime for docker executor: %s", block.Runtime)
	}
	if block.Image == "" {
		return fmt.Errorf("docker block missing image")
	}

	if err := e.checkDockerAvailable(); err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}

	if e.isLocalPath(block.Image) {
		if err := e.validateLocalPath(block.Image); err != nil {
			return fmt.Errorf("invalid local path: %w", err)
		}
	}

	return nil
}

// Execute runs a Docker block
func (e *DockerExecutor) Execute(execCtx *execcontext.ExecutionContext, block *Block, inputs map[string]interface{}) (interface{}, error) {
	var imageName string
	var err error

	if e.isLocalPath(block.Image) {
		imageName, err = e.buildImageFromLocal(execCtx, block.Image, execCtx.Cwd)
		if err != nil {
			return nil, fmt.Errorf("failed to build image from local path: %w", err)
		}
	} else {
		imageName = block.Image
		if err := e.pullImageIfNeeded(execCtx.Context.Context, imageName); err != nil {
			return nil, fmt.Errorf("failed to pull image: %w", err)
		}
	}

	execInput := ExecutionInput{
		Inputs: inputs,
		Env:    make(map[string]string),
	}

	for key, value := range block.Env {
		execInput.Env[key] = value
	}

	inputJSON, err := json.Marshal(execInput)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	args := []string{"run", "--rm"}
	args = append(args, "-e", fmt.Sprintf("LACQUER_INPUTS=%s", string(inputJSON)))
	for key, value := range execInput.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	args = append(args, imageName)

	if len(block.Command) > 0 {
		args = append(args, block.Command...)
	}

	cmd := exec.CommandContext(execCtx.Context.Context, "docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		// Check for error in stderr
		if stderr.Len() > 0 {
			var execErr ExecutionError
			if jsonErr := json.Unmarshal(stderr.Bytes(), &execErr); jsonErr == nil {
				return nil, fmt.Errorf("container execution failed: %s", execErr.Message)
			}
			return nil, fmt.Errorf("container execution failed: %s", stderr.String())
		}
		return nil, fmt.Errorf("container execution failed: %w", err)
	}

	// Parse output
	var output map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return stdout.String(), nil //nolint:nilerr // Intentional: fallback to raw string output
	}

	return output, nil
}

// isLocalPath determines if the image reference is a local path
func (e *DockerExecutor) isLocalPath(image string) bool {
	// Check for common patterns that indicate local paths
	return strings.HasPrefix(image, "./") ||
		strings.HasPrefix(image, "../") ||
		strings.Contains(image, "/Dockerfile") ||
		image == "Dockerfile" ||
		(strings.Contains(image, "/") && !strings.Contains(image, ":") && !strings.Contains(image, "@"))
}

// validateLocalPath validates that a local path exists and is accessible
func (e *DockerExecutor) validateLocalPath(path string) error {
	resolvedPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	if info.IsDir() {
		dockerfilePath := filepath.Join(resolvedPath, "Dockerfile")
		if _, err := os.Stat(dockerfilePath); err != nil {
			return fmt.Errorf("directory does not contain Dockerfile: %w", err)
		}
	} else if filepath.Base(resolvedPath) != "Dockerfile" {
		return fmt.Errorf("file must be named 'Dockerfile'")
	}

	return nil
}

// buildImageFromLocal builds a Docker image from a local path
func (e *DockerExecutor) buildImageFromLocal(execCtx *execcontext.ExecutionContext, imagePath string, workspaceDir string) (string, error) {
	var buildContext, dockerfilePath string

	if filepath.IsAbs(imagePath) {
		buildContext = filepath.Dir(imagePath)
		dockerfilePath = imagePath
	} else {
		if workspaceDir != "" {
			buildContext = filepath.Join(workspaceDir, filepath.Dir(imagePath))
			dockerfilePath = filepath.Join(workspaceDir, imagePath)
		} else {
			buildContext = filepath.Dir(imagePath)
			dockerfilePath = imagePath
		}
	}

	buildContext, err := filepath.Abs(buildContext)
	if err != nil {
		return "", fmt.Errorf("failed to resolve build context: %w", err)
	}

	dockerfilePath, err = filepath.Abs(dockerfilePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve dockerfile path: %w", err)
	}

	if info, err := os.Stat(dockerfilePath); err == nil && info.IsDir() {
		buildContext = dockerfilePath
		dockerfilePath = filepath.Join(dockerfilePath, "Dockerfile")
	}

	imageName, err := e.generateImageName(dockerfilePath)
	if err != nil {
		return "", fmt.Errorf("failed to generate image name: %w", err)
	}

	if e.imageExists(execCtx.Context.Context, imageName) {
		return imageName, nil
	}

	buildCtx, cancel := context.WithTimeout(execCtx.Context.Context, e.buildTimeout)
	defer cancel()
	relDockerfilePath, err := filepath.Rel(buildContext, dockerfilePath)
	if err != nil {
		relDockerfilePath = "Dockerfile"
	}

	args := []string{"build", "-t", imageName, "-f", relDockerfilePath, buildContext}
	cmd := exec.CommandContext(buildCtx, "docker", args...) // #nosec G204 - args are controlled and validated
	cmd.Dir = buildContext

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to build image %s: %s", imageName, stderr.String())
	}

	return imageName, nil
}

// generateImageName generates a unique image name based on dockerfile path and content
func (e *DockerExecutor) generateImageName(dockerfilePath string) (string, error) {
	content, err := os.ReadFile(dockerfilePath) // #nosec G304 - dockerfilePath is validated
	if err != nil {
		return "", fmt.Errorf("failed to read dockerfile: %w", err)
	}

	hasher := sha256.New()
	hasher.Write([]byte(dockerfilePath))
	hasher.Write(content)
	hash := fmt.Sprintf("%x", hasher.Sum(nil))[:12] // Use first 12 chars

	imageName := fmt.Sprintf("lacquer-local:%s", hash)
	return imageName, nil
}

// imageExists checks if a Docker image exists locally
func (e *DockerExecutor) imageExists(ctx context.Context, imageName string) bool {
	cmd := exec.CommandContext(ctx, "docker", "image", "inspect", imageName)
	return cmd.Run() == nil
}

func (e *DockerExecutor) checkDockerAvailable() error {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon not available or not running")
	}
	return nil
}

func (e *DockerExecutor) pullImageIfNeeded(ctx context.Context, image string) error {
	cmd := exec.CommandContext(ctx, "docker", "image", "inspect", image)
	if err := cmd.Run(); err == nil {
		return nil
	}

	pullCtx, cancel := context.WithTimeout(ctx, e.pullTimeout)
	defer cancel()

	cmd = exec.CommandContext(pullCtx, "docker", "pull", image)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull image %s: %s", image, stderr.String())
	}

	return nil
}
