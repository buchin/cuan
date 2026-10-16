package deployer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// StorageEngine manages isolated filesystem storage for deployments.
// By default, files are sequestered in data/deployments/<id>/app/.
type StorageEngine struct {
	baseDir string
}

// NewStorageEngine creates a StorageEngine initialized with the specified base directory.
func NewStorageEngine(baseDir string) (*StorageEngine, error) {
	if baseDir == "" {
		baseDir = "data/deployments"
	}
	cleanBase := filepath.Clean(baseDir)
	if err := os.MkdirAll(cleanBase, 0755); err != nil {
		return nil, fmt.Errorf("initialize storage base directory %s: %w", cleanBase, err)
	}

	return &StorageEngine{baseDir: cleanBase}, nil
}

// BaseDir returns the root storage directory.
func (s *StorageEngine) BaseDir() string {
	return s.baseDir
}

// GetDeploymentDir returns the base path for a given deployment ID: data/deployments/<id>.
func (s *StorageEngine) GetDeploymentDir(id string) string {
	return filepath.Join(s.baseDir, id)
}

// GetAppDir returns the application source directory: data/deployments/<id>/app.
func (s *StorageEngine) GetAppDir(id string) string {
	return filepath.Join(s.baseDir, id, "app")
}

// PrepareDeploymentDir ensures that data/deployments/<id>/app exists and is clean.
func (s *StorageEngine) PrepareDeploymentDir(id string) (string, error) {
	appDir := s.GetAppDir(id)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", fmt.Errorf("prepare app directory for %s: %w", id, err)
	}
	return appDir, nil
}

// ExtractArchive extracts an uploaded ZIP archive directly into data/deployments/<id>/app
// and runs runtime stack auto-detection.
func (s *StorageEngine) ExtractArchive(id string, zipReader io.ReaderAt, size int64) (string, string, error) {
	appDir, err := s.PrepareDeploymentDir(id)
	if err != nil {
		return "", "", err
	}

	if err := ExtractZip(zipReader, size, appDir); err != nil {
		return "", "", fmt.Errorf("extract zip for deployment %s: %w", id, err)
	}

	detectedRuntime, err := DetectRuntime(appDir)
	if err != nil {
		return RuntimeCustom, appDir, nil
	}

	return detectedRuntime, appDir, nil
}

// DeleteDeploymentDir removes all stored data associated with a deployment.
func (s *StorageEngine) DeleteDeploymentDir(id string) error {
	dir := s.GetDeploymentDir(id)
	return os.RemoveAll(dir)
}
