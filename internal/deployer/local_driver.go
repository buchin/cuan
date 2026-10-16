package deployer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

// LocalDriver implements DeployDriver for local/co-located process execution (Sprint 3).
type LocalDriver struct {
	storage *StorageEngine
	store   Store
}

// NewLocalDriver instantiates a LocalDriver with storage and persistent store.
func NewLocalDriver(storage *StorageEngine, store Store) *LocalDriver {
	return &LocalDriver{
		storage: storage,
		store:   store,
	}
}

// Storage returns the underlying StorageEngine.
func (d *LocalDriver) Storage() *StorageEngine {
	return d.storage
}

// Store returns the underlying Store.
func (d *LocalDriver) Store() Store {
	return d.store
}

// Deploy extracts/stores project files, runs auto-detection, and persists deployment metadata.
func (d *LocalDriver) Deploy(ctx context.Context, req DeploymentRequest) (*DeploymentResult, error) {
	if req.ProjectName == "" {
		return nil, fmt.Errorf("project_name is required")
	}

	if req.ID == "" {
		req.ID = generateDeploymentID()
	}

	if req.Slug == "" {
		req.Slug = slugify(req.ProjectName)
	}

	if req.SourceType == "" {
		req.SourceType = SourceTypeZip
	}

	appDir := d.storage.GetAppDir(req.ID)
	detectedRuntime := req.Runtime

	// Process ZIP archive if supplied
	if req.ZipReader != nil {
		var readerAt io.ReaderAt
		var size int64

		if ra, ok := req.ZipReader.(io.ReaderAt); ok && req.ZipSize > 0 {
			readerAt = ra
			size = req.ZipSize
		} else {
			// Buffer into memory or temp file
			buf, err := io.ReadAll(req.ZipReader)
			if err != nil {
				return nil, fmt.Errorf("read uploaded zip stream: %w", err)
			}
			readerAt = bytes.NewReader(buf)
			size = int64(len(buf))
		}

		rt, extracted, err := d.storage.ExtractArchive(req.ID, readerAt, size)
		if err != nil {
			return nil, err
		}
		detectedRuntime = rt
		appDir = extracted
	} else {
		// Just ensure app directory exists
		if _, err := d.storage.PrepareDeploymentDir(req.ID); err != nil {
			return nil, err
		}
	}

	if req.Runtime == "" || req.Runtime == RuntimeCustom {
		if detectedRuntime != "" {
			req.Runtime = detectedRuntime
		} else {
			req.Runtime = RuntimeCustom
		}
	}

	dep := &Deployment{
		ID:           req.ID,
		ProjectName:  req.ProjectName,
		Slug:         req.Slug,
		SourceType:   req.SourceType,
		SourceRef:    req.SourceRef,
		Runtime:      req.Runtime,
		Status:       StatusPending,
		InternalPort: req.InternalPort,
		Domain:       req.Domain,
		EnvVars:      req.EnvVars,
		ProcessPID:   0,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Persist to store
	existing, err := d.store.GetDeployment(ctx, req.ID)
	if err == nil && existing != nil {
		dep.CreatedAt = existing.CreatedAt
		if err := d.store.UpdateDeployment(ctx, dep); err != nil {
			return nil, fmt.Errorf("update deployment: %w", err)
		}
	} else {
		if err := d.store.CreateDeployment(ctx, dep); err != nil {
			return nil, fmt.Errorf("create deployment: %w", err)
		}
	}

	// Record initial system log
	_ = d.store.AppendLog(ctx, &LogEntry{
		DeploymentID: dep.ID,
		Stream:       StreamSystem,
		Message:      fmt.Sprintf("Deployment %s (%s) initialized with runtime %s", dep.ProjectName, dep.ID, dep.Runtime),
		Timestamp:    time.Now().UTC(),
	})

	return &DeploymentResult{
		Deployment:      dep,
		DetectedRuntime: detectedRuntime,
		ExtractedPath:   appDir,
		Status:          dep.Status,
	}, nil
}

// Start transitions deployment to running status.
func (d *LocalDriver) Start(ctx context.Context, id string) error {
	dep, err := d.store.GetDeployment(ctx, id)
	if err != nil {
		return err
	}

	dep.Status = StatusRunning
	dep.UpdatedAt = time.Now().UTC()
	if err := d.store.UpdateDeployment(ctx, dep); err != nil {
		return err
	}

	_ = d.store.AppendLog(ctx, &LogEntry{
		DeploymentID: id,
		Stream:       StreamSystem,
		Message:      fmt.Sprintf("Deployment %s started (status: %s)", id, StatusRunning),
		Timestamp:    time.Now().UTC(),
	})

	return nil
}

// Stop cleanly transitions deployment to stopped status.
func (d *LocalDriver) Stop(ctx context.Context, id string) error {
	dep, err := d.store.GetDeployment(ctx, id)
	if err != nil {
		return err
	}

	dep.Status = StatusStopped
	dep.ProcessPID = 0
	dep.UpdatedAt = time.Now().UTC()
	if err := d.store.UpdateDeployment(ctx, dep); err != nil {
		return err
	}

	_ = d.store.AppendLog(ctx, &LogEntry{
		DeploymentID: id,
		Stream:       StreamSystem,
		Message:      fmt.Sprintf("Deployment %s stopped", id),
		Timestamp:    time.Now().UTC(),
	})

	return nil
}

// Restart stops and starts the deployment.
func (d *LocalDriver) Restart(ctx context.Context, id string) error {
	if err := d.Stop(ctx, id); err != nil {
		return err
	}
	return d.Start(ctx, id)
}

// GetLogs fetches logs from the store.
func (d *LocalDriver) GetLogs(ctx context.Context, id string, limit int) ([]LogEntry, error) {
	return d.store.GetLogs(ctx, id, limit)
}

func generateDeploymentID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("dep_%s", hex.EncodeToString(b))
}

var nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	slug := nonAlphaNumRegex.ReplaceAllString(lower, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "app-" + generateDeploymentID()[:8]
	}
	return slug
}

// Helper to check if file exists
func PathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func DirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func FileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// Ensure interface compliance at compile-time
var _ DeployDriver = (*LocalDriver)(nil)
