package inspector

import (
	"errors"
	"io"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
)

// Supported primary stacks
const (
	StackLaravel   = "laravel"
	StackNode      = "node"
	StackNext      = "next"
	StackReact     = "react"
	StackExpress   = "express"
	StackPython    = "python"
	StackDjango    = "django"
	StackFlask     = "flask"
	StackWordPress = "wordpress"
	StackHTML      = "html"
	StackUnknown   = "unknown"
)

// Limits and default constants
const (
	DefaultMaxZipSize    = 150 * 1024 * 1024 // 150 MB max uncompressed quota
	DefaultMaxFileSize   = 150 * 1024 * 1024 // 150 MB max single file size
	DefaultMaxFilesCount = 50000             // 50,000 maximum entries
)

// Sentinel errors
var (
	ErrZipSlip          = errors.New("zip slip detected: archive contains invalid relative path")
	ErrQuotaExceeded    = errors.New("extraction quota exceeded: archive size exceeds 150MB limit")
	ErrMaxFilesExceeded = errors.New("extraction quota exceeded: file count exceeds limit")
	ErrEmptyArchive     = errors.New("archive is empty or contains no files")
	ErrInvalidZip       = errors.New("invalid or corrupt zip archive")
	ErrNilProjectStore  = errors.New("project store is required")
	ErrNilStudioRepo    = errors.New("project studio repository is required")
	ErrNilAgentRepo     = errors.New("agent repository is required")
)

// UnpackResult contains details of an extracted project archive.
type UnpackResult struct {
	DestDir     string    `json:"dest_dir"`
	TotalFiles  int       `json:"total_files"`
	TotalDirs   int       `json:"total_dirs"`
	TotalBytes  int64     `json:"total_bytes"`
	RootPrefix  string    `json:"root_prefix,omitempty"`
	ExtractedAt time.Time `json:"extracted_at"`
	Files       []string  `json:"files,omitempty"`
}

// TechStackInfo describes the detected technology stack.
type TechStackInfo struct {
	PrimaryStack string   `json:"primary_stack"` // e.g. "laravel", "node", "django", "wordpress", "html"
	Framework    string   `json:"framework"`     // e.g. "Laravel", "Next.js", "Django", "WordPress", "Static HTML"
	Language     string   `json:"language"`      // e.g. "php", "javascript", "typescript", "python", "html"
	Confidence   float64  `json:"confidence"`    // 0.0 - 1.0
	Description  string   `json:"description"`
	Signatures   []string `json:"signatures"`    // matched signature files: e.g. ["composer.json", "artisan"]
}

// SQLFileInfo contains information about a detected SQL file.
type SQLFileInfo struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Dialect string `json:"dialect"` // "mysql", "postgres", "sqlite", "generic"
}

// EnvVarInfo contains extracted environment variable keys and metadata.
type EnvVarInfo struct {
	SourceFile   string   `json:"source_file,omitempty"`
	AllKeys      []string `json:"all_keys"`
	RequiredKeys []string `json:"required_keys"`
	DatabaseKeys []string `json:"database_keys"`
	SecurityKeys []string `json:"security_keys"`
}

// DocInfo holds information extracted from README / install.txt.
type DocInfo struct {
	SourceFile string `json:"source_file"`
	Title      string `json:"title,omitempty"`
	Excerpt    string `json:"excerpt,omitempty"`
}

// TaskTemplate defines an actionable setup task blueprint to instantiate on the Kanban board.
type TaskTemplate struct {
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	Priority     string       `json:"priority"` // low, normal, high, urgent
	Tags         []string     `json:"tags"`
	DoDChecklist []db.DoDItem `json:"dod_checklist"`
}

// InspectionResult holds all findings from inspecting a project archive.
type InspectionResult struct {
	ArchiveName        string         `json:"archive_name,omitempty"`
	TechStack          TechStackInfo  `json:"tech_stack"`
	Env                EnvVarInfo     `json:"env"`
	SQLFiles           []SQLFileInfo  `json:"sql_files"`
	Documentation      *DocInfo       `json:"documentation,omitempty"`
	TotalFiles         int            `json:"total_files"`
	TotalSize          int64          `json:"total_size"`
	RootPrefix         string         `json:"root_prefix,omitempty"`
	RecommendedModules []string       `json:"recommended_modules"`
	RecommendedTasks   []TaskTemplate `json:"recommended_tasks"`
}

// CreateFromZipRequest defines parameters for creating a new project from a ZIP file.
type CreateFromZipRequest struct {
	ProjectName string
	ArchiveName string
	ZipReader   io.ReaderAt
	ZipSize     int64
	TempZipPath string
}

// CreateFromZipResponse defines the atomic creation result.
type CreateFromZipResponse struct {
	Status     string                   `json:"status"`
	Project    *project.ProjectResponse `json:"project"`
	Inspection *InspectionResult        `json:"inspection"`
	Room       db.ProjectRoomDTO        `json:"room"`
	Agent      db.AgentDTO              `json:"agent"`
	Tasks      []db.ProjectTaskDTO      `json:"tasks"`
}
