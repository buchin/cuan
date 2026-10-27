package inspector

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
)

var nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9]+`)

// Engine orchestrates ZIP file extraction, stack inspection, and project generation.
type Engine struct {
	mu           sync.RWMutex
	unpacker     *SafeUnpacker
	detector     *StackDetector
	parser       *ProjectParser
	pipeline     *PipelineGenerator
	projectStore project.Store
	projectRepo  db.ProjectRepository
	studioRepo   db.ProjectStudioRepository
	agentRepo    db.AgentRepository
	baseDir      string
	maxZipSize   int64
}

// Option configures the inspector Engine.
type Option func(*Engine)

// WithProjectStore sets the project.Store.
func WithProjectStore(s project.Store) Option {
	return func(e *Engine) {
		e.projectStore = s
	}
}

// WithProjectRepo sets the db.ProjectRepository.
func WithProjectRepo(r db.ProjectRepository) Option {
	return func(e *Engine) {
		e.projectRepo = r
	}
}

// WithStudioRepo sets the db.ProjectStudioRepository.
func WithStudioRepo(r db.ProjectStudioRepository) Option {
	return func(e *Engine) {
		e.studioRepo = r
	}
}

// WithAgentRepo sets the db.AgentRepository.
func WithAgentRepo(r db.AgentRepository) Option {
	return func(e *Engine) {
		e.agentRepo = r
	}
}

// WithBaseDir sets the directory where extracted projects reside.
func WithBaseDir(dir string) Option {
	return func(e *Engine) {
		e.baseDir = dir
	}
}

// WithMaxZipSize configures the maximum uncompressed extraction limit.
func WithMaxZipSize(size int64) Option {
	return func(e *Engine) {
		e.maxZipSize = size
	}
}

// NewEngine constructs an Engine instance with configured options.
func NewEngine(opts ...Option) *Engine {
	e := &Engine{
		unpacker:   NewSafeUnpacker(DefaultMaxZipSize),
		detector:   NewStackDetector(),
		parser:     NewProjectParser(),
		pipeline:   NewPipelineGenerator(),
		baseDir:    filepath.Join("data", "projects"),
		maxZipSize: DefaultMaxZipSize,
	}

	for _, opt := range opts {
		opt(e)
	}

	if e.maxZipSize > 0 {
		e.unpacker = NewSafeUnpacker(e.maxZipSize)
	}

	return e
}

// InspectDirectory analyzes an already unpacked directory.
func (e *Engine) InspectDirectory(dir string, archiveName string) (*InspectionResult, error) {
	// Detect top-level single directory prefix if any
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory '%s': %w", dir, err)
	}

	var rootPrefix string
	var dirCount, fileCount int
	var singleDirName string

	for _, entry := range entries {
		if entry.IsDir() {
			dirCount++
			singleDirName = entry.Name()
		} else {
			fileCount++
		}
	}

	if dirCount == 1 && fileCount == 0 && singleDirName != "" {
		rootPrefix = singleDirName
	}

	stackInfo := e.detector.Detect(dir, rootPrefix)
	envInfo := e.parser.ParseEnv(dir, rootPrefix)
	sqlFiles := e.parser.FindSQLFiles(dir)
	docInfo := e.parser.ParseDocumentation(dir, rootPrefix)

	result := &InspectionResult{
		ArchiveName:   archiveName,
		TechStack:     stackInfo,
		Env:           envInfo,
		SQLFiles:      sqlFiles,
		Documentation: docInfo,
		RootPrefix:    rootPrefix,
	}

	result.RecommendedModules = e.pipeline.RecommendedModules(stackInfo)
	result.RecommendedTasks = e.pipeline.GenerateSetupTasks(result)

	return result, nil
}

// InspectZip extracts a zip reader to an isolated temporary sandbox, inspects it, and cleans up.
func (e *Engine) InspectZip(r io.ReaderAt, size int64, archiveName string) (*InspectionResult, error) {
	tmpDir, err := os.MkdirTemp("", "cuan-inspect-*")
	if err != nil {
		return nil, fmt.Errorf("create temp inspect dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	unpackRes, err := e.unpacker.Unpack(r, size, tmpDir)
	if err != nil {
		return nil, err
	}

	inspection, err := e.InspectDirectory(tmpDir, archiveName)
	if err != nil {
		return nil, err
	}

	inspection.TotalFiles = unpackRes.TotalFiles
	inspection.TotalSize = unpackRes.TotalBytes
	if inspection.RootPrefix == "" {
		inspection.RootPrefix = unpackRes.RootPrefix
	}

	return inspection, nil
}

// InspectZipStream streams a zip file to a temporary sandbox, inspects it, and cleans up.
func (e *Engine) InspectZipStream(r io.Reader, archiveName string) (*InspectionResult, error) {
	tmpDir, err := os.MkdirTemp("", "cuan-inspect-*")
	if err != nil {
		return nil, fmt.Errorf("create temp inspect dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	unpackRes, err := e.unpacker.UnpackStream(r, tmpDir)
	if err != nil {
		return nil, err
	}

	inspection, err := e.InspectDirectory(tmpDir, archiveName)
	if err != nil {
		return nil, err
	}

	inspection.TotalFiles = unpackRes.TotalFiles
	inspection.TotalSize = unpackRes.TotalBytes
	if inspection.RootPrefix == "" {
		inspection.RootPrefix = unpackRes.RootPrefix
	}

	return inspection, nil
}

// CreateProjectFromZip extracts the archive, inspects it, and atomically creates the project, room, agent, and setup tasks.
func (e *Engine) CreateProjectFromZip(ctx context.Context, req CreateFromZipRequest) (*CreateFromZipResponse, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.projectStore == nil && e.projectRepo == nil {
		return nil, ErrNilProjectStore
	}
	if e.studioRepo == nil {
		return nil, ErrNilStudioRepo
	}
	if e.agentRepo == nil {
		return nil, ErrNilAgentRepo
	}

	projectID := fmt.Sprintf("proj_%s", generateRandomHex(6))
	targetDir := filepath.Join(e.baseDir, projectID)

	// Ensure destination directory does not pre-exist
	_ = os.RemoveAll(targetDir)

	// 1. Unpack archive into isolated project location
	var unpackRes *UnpackResult
	var err error

	if req.TempZipPath != "" {
		f, fErr := os.Open(req.TempZipPath)
		if fErr != nil {
			return nil, fmt.Errorf("open uploaded archive: %w", fErr)
		}
		stat, sErr := f.Stat()
		if sErr != nil {
			_ = f.Close()
			return nil, fmt.Errorf("stat uploaded archive: %w", sErr)
		}
		unpackRes, err = e.unpacker.Unpack(f, stat.Size(), targetDir)
		_ = f.Close()
	} else if req.ZipReader != nil && req.ZipSize > 0 {
		unpackRes, err = e.unpacker.Unpack(req.ZipReader, req.ZipSize, targetDir)
	} else {
		return nil, errors.New("zip reader or temp file path is required")
	}

	if err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, err
	}

	// Rollback helper
	rollback := func(createdTasks []string, createdRoomID string) {
		for _, tid := range createdTasks {
			_ = e.studioRepo.DeleteTask(ctx, tid)
		}
		if createdRoomID != "" {
			_ = e.studioRepo.DeleteRoom(ctx, createdRoomID)
		}
		if e.projectStore != nil {
			_ = e.projectStore.DeleteProject(ctx, projectID)
		}
		if e.projectRepo != nil {
			_ = e.projectRepo.DeleteProject(ctx, projectID)
		}
		_ = os.RemoveAll(targetDir)
	}

	// 2. Inspect project directory
	inspection, err := e.InspectDirectory(targetDir, req.ArchiveName)
	if err != nil {
		rollback(nil, "")
		return nil, fmt.Errorf("inspect project directory: %w", err)
	}
	inspection.TotalFiles = unpackRes.TotalFiles
	inspection.TotalSize = unpackRes.TotalBytes
	if inspection.RootPrefix == "" {
		inspection.RootPrefix = unpackRes.RootPrefix
	}

	// 3. Resolve project name
	projectName := strings.TrimSpace(req.ProjectName)
	if projectName == "" {
		if inspection.Documentation != nil && inspection.Documentation.Title != "" {
			projectName = inspection.Documentation.Title
		} else if req.ArchiveName != "" {
			baseName := strings.TrimSuffix(req.ArchiveName, filepath.Ext(req.ArchiveName))
			projectName = strings.Title(strings.ReplaceAll(baseName, "-", " "))
		} else {
			projectName = fmt.Sprintf("%s Project", inspection.TechStack.Framework)
		}
	}

	// 4. Build source_metadata and enabled_modules
	metaBytes, err := e.pipeline.BuildSourceMetadata(inspection, req.ArchiveName, unpackRes.TotalFiles, unpackRes.TotalBytes)
	if err != nil {
		metaBytes = json.RawMessage("{}")
	}

	modulesJSON, err := json.Marshal(inspection.RecommendedModules)
	if err != nil {
		modulesJSON = []byte(`["tasks", "discussions", "servers", "repositories"]`)
	}

	now := time.Now().UTC()
	slug := slugify(projectName)
	repoOwner := "zip"
	repoName := fmt.Sprintf("%s-%s", slug, generateRandomHex(4))
	repoURL := fmt.Sprintf("file://%s", targetDir)

	projEntity := &project.Project{
		ID:             projectID,
		Name:           projectName,
		RepoURL:        repoURL,
		RepoOwner:      repoOwner,
		RepoName:       repoName,
		DefaultBranch:  "main",
		LocalPath:      targetDir,
		CloneStatus:    "connected",
		ProjectType:    inspection.TechStack.PrimaryStack,
		Metadata:       metaBytes,
		EnabledModules: modulesJSON,
		SourceType:     "zip",
		SourceMetadata: metaBytes,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// 5. Create Project Record
	if e.projectStore != nil {
		if err := e.projectStore.CreateProject(ctx, projEntity); err != nil {
			rollback(nil, "")
			return nil, fmt.Errorf("create project in store: %w", err)
		}
	}
	if e.projectRepo != nil && (e.projectStore == nil || isMemoryProjectRepo(e.projectRepo)) {
		dbProj := toDBProject(projEntity)
		_ = e.projectRepo.CreateProject(ctx, dbProj)
	}

	// 6. Create Collaboration Room
	roomID := db.GenerateUUID()
	room := &db.ProjectRoom{
		ID:        roomID,
		ProjectID: projectID,
		Title:     "Script Setup & Onboarding",
		Topic:     fmt.Sprintf("AI-guided setup, environment configuration, and deployment pipeline for %s (%s)", projectName, inspection.TechStack.Framework),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := e.studioRepo.CreateRoom(ctx, room); err != nil {
		rollback(nil, "")
		return nil, fmt.Errorf("create project room: %w", err)
	}

	// 7. Resolve or Create Agent Persona
	agentName := "DevOps Setup Assistant"
	var targetAgent *db.Agent
	existingAgents, err := e.agentRepo.ListAgents(ctx, db.AgentFilter{Search: agentName})
	if err == nil {
		for i := range existingAgents {
			if strings.EqualFold(existingAgents[i].Name, agentName) {
				targetAgent = &existingAgents[i]
				break
			}
		}
	}

	if targetAgent == nil {
		newAgent := &db.Agent{
			ID:           db.GenerateUUID(),
			Name:         agentName,
			Avatar:       "server",
			Role:         "DevOps & Script Setup Specialist",
			SystemPrompt: "You are an expert DevOps engineer and Script Setup Specialist in Cuan. You assist users in configuring environments, importing databases, installing dependencies, and deploying applications.",
			ModelID:      "gemini-2.5-pro",
			Provider:     "gemini",
			Status:       db.AgentStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := e.agentRepo.CreateAgent(ctx, newAgent); err != nil {
			// Retry fetch in case of concurrent insert
			retryAgents, rErr := e.agentRepo.ListAgents(ctx, db.AgentFilter{Search: agentName})
			if rErr == nil && len(retryAgents) > 0 {
				targetAgent = &retryAgents[0]
			} else {
				rollback(nil, roomID)
				return nil, fmt.Errorf("create setup assistant agent: %w", err)
			}
		} else {
			targetAgent = newAgent
		}
	}

	// 8. Create Actionable Setup Tasks with DoD Checklist
	createdTaskIDs := make([]string, 0, len(inspection.RecommendedTasks))
	createdTasks := make([]db.ProjectTaskDTO, 0, len(inspection.RecommendedTasks))

	for _, taskTmpl := range inspection.RecommendedTasks {
		taskID := db.GenerateUUID()
		dodCopy := make([]db.DoDItem, len(taskTmpl.DoDChecklist))
		for i, item := range taskTmpl.DoDChecklist {
			dodCopy[i] = db.DoDItem{
				ID:        db.GenerateUUID(),
				Text:      item.Text,
				Completed: item.Completed,
			}
		}

		tagsCopy := make([]string, len(taskTmpl.Tags))
		copy(tagsCopy, taskTmpl.Tags)

		agentIDStr := targetAgent.ID
		roomIDStr := roomID

		taskEntity := &db.ProjectTask{
			ID:              taskID,
			ProjectID:       projectID,
			RoomID:          &roomIDStr,
			Title:           taskTmpl.Title,
			Description:     taskTmpl.Description,
			Status:          "backlog",
			Priority:        taskTmpl.Priority,
			DoDChecklist:    dodCopy,
			Tags:            tagsCopy,
			AssigneeID:      &agentIDStr,
			ExecutionStatus: "idle",
			AgentOutput:     "",
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := e.studioRepo.CreateTask(ctx, taskEntity); err != nil {
			rollback(createdTaskIDs, roomID)
			return nil, fmt.Errorf("create setup task '%s': %w", taskTmpl.Title, err)
		}

		createdTaskIDs = append(createdTaskIDs, taskID)
		createdTasks = append(createdTasks, taskEntity.ToDTO())
	}

	return &CreateFromZipResponse{
		Status:     "success",
		Project:    projEntity.ToResponse(),
		Inspection: inspection,
		Room:       room.ToDTO(),
		Agent:      targetAgent.ToDTO(),
		Tasks:      createdTasks,
	}, nil
}

func generateRandomHex(byteCount int) string {
	b := make([]byte, byteCount)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())[:byteCount*2]
	}
	return hex.EncodeToString(b)
}

func slugify(s string) string {
	clean := strings.ToLower(strings.TrimSpace(s))
	clean = nonAlphaNumRegex.ReplaceAllString(clean, "-")
	clean = strings.Trim(clean, "-")
	if clean == "" {
		return "zip-project"
	}
	return clean
}

func toDBProject(p *project.Project) *db.Project {
	if p == nil {
		return nil
	}
	return &db.Project{
		ID:             p.ID,
		Name:           p.Name,
		RepoURL:        p.RepoURL,
		RepoOwner:      p.RepoOwner,
		RepoName:       p.RepoName,
		DefaultBranch:  p.DefaultBranch,
		LocalPath:      p.LocalPath,
		EncryptedToken: p.EncryptedToken,
		TokenPreview:   p.TokenPreview,
		CloneStatus:    p.CloneStatus,
		ProjectType:    p.ProjectType,
		Metadata:       p.Metadata,
		EnabledModules: p.EnabledModules,
		SourceType:     p.SourceType,
		SourceMetadata: p.SourceMetadata,
		LastFetchedAt:  p.LastFetchedAt,
		LastError:      p.LastError,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func isMemoryProjectRepo(r db.ProjectRepository) bool {
	_, ok := r.(*db.MemoryProjectRepository)
	return ok
}
