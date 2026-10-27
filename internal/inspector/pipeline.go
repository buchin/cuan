package inspector

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
)

// PipelineGenerator builds project metadata and actionable Kanban task blueprints.
type PipelineGenerator struct{}

// NewPipelineGenerator constructs a PipelineGenerator.
func NewPipelineGenerator() *PipelineGenerator {
	return &PipelineGenerator{}
}

// RecommendedModules returns active modules for the project.
// Per Sprint 11 SSOT, ZIP uploaded projects instantiate with:
// ["tasks", "discussions", "servers", "repositories"]
func (g *PipelineGenerator) RecommendedModules(stack TechStackInfo) []string {
	return []string{"tasks", "discussions", "servers", "repositories"}
}

// GenerateSetupTasks creates the standard 4 actionable Kanban setup tasks.
func (g *PipelineGenerator) GenerateSetupTasks(inspection *InspectionResult) []TaskTemplate {
	stack := inspection.TechStack
	env := inspection.Env
	sqlFiles := inspection.SQLFiles

	tasks := make([]TaskTemplate, 0, 4)

	// 1. Setup Environment & .env Configuration
	envDesc := "Configure the project environment for the application runtime."
	if env.SourceFile != "" {
		envDesc = fmt.Sprintf("Environment template detected in '%s'.", env.SourceFile)
		if len(env.RequiredKeys) > 0 {
			previewCount := 6
			if len(env.RequiredKeys) < previewCount {
				previewCount = len(env.RequiredKeys)
			}
			envDesc += fmt.Sprintf(" Important variables detected: %s.", strings.Join(env.RequiredKeys[:previewCount], ", "))
		}
	} else {
		envDesc = "Create a .env configuration file for stack " + stack.Framework + "."
	}

	task1 := TaskTemplate{
		Title:       "Setup Environment & .env Configuration",
		Description: envDesc,
		Priority:    "high",
		Tags:        []string{"setup", "environment", stack.PrimaryStack},
		DoDChecklist: []db.DoDItem{
			{Text: "Copy .env.example / .env.sample to .env", Completed: false},
			{Text: "Configure database connection credentials (DB_HOST, DB_DATABASE, DB_USERNAME, DB_PASSWORD)", Completed: false},
			{Text: "Generate the application secret key / APP_KEY", Completed: false},
			{Text: "Verify .env permissions prevent public access (chmod 600/640)", Completed: false},
		},
	}
	tasks = append(tasks, task1)

	// 2. Database Setup & SQL Import
	dbPriority := "normal"
	dbDesc := "Set up the database and import the application schema."
	if len(sqlFiles) > 0 {
		dbPriority = "high"
		sqlPaths := make([]string, 0, len(sqlFiles))
		for _, f := range sqlFiles {
			sqlPaths = append(sqlPaths, f.Path)
		}
		dbDesc = fmt.Sprintf("Database schema files detected: %s (dialect: %s).", strings.Join(sqlPaths, ", "), sqlFiles[0].Dialect)
	} else if stack.PrimaryStack == StackLaravel {
		dbDesc = "Run Laravel database migrations (php artisan migrate)."
		dbPriority = "high"
	} else if stack.PrimaryStack == StackDjango {
		dbDesc = "Run Django database migrations (python manage.py migrate)."
		dbPriority = "high"
	}

	task2 := TaskTemplate{
		Title:       "Database Setup & SQL Import",
		Description: dbDesc,
		Priority:    dbPriority,
		Tags:        []string{"database", "sql", stack.PrimaryStack},
		DoDChecklist: []db.DoDItem{
			{Text: "Create the target database on the DBMS server (MySQL/PostgreSQL/SQLite)", Completed: false},
			{Text: "Import the detected SQL schema or run initial database migrations", Completed: false},
			{Text: "Verify tables and required seed data are created without errors", Completed: false},
			{Text: "Verify the application runtime can connect to the database", Completed: false},
		},
	}
	tasks = append(tasks, task2)

	// 3. Dependency Installation (composer/npm/pip)
	depTitle := "Dependency Installation (composer/npm/pip)"
	depDesc := "Install all third-party runtime dependencies."
	switch stack.PrimaryStack {
	case StackLaravel, "php":
		depTitle = "Dependency Installation (composer)"
		depDesc = "Run composer install --no-dev --optimize-autoloader to install PHP dependencies."
	case StackNode, StackNext, StackReact, StackExpress:
		depTitle = "Dependency Installation (npm)"
		depDesc = "Run npm install && npm run build to install packages and compile frontend assets."
	case StackPython, StackDjango, StackFlask:
		depTitle = "Dependency Installation (pip)"
		depDesc = "Run pip install -r requirements.txt in the application's Python virtual environment."
	case StackWordPress:
		depTitle = "Dependency Installation (composer/plugins)"
		depDesc = "Verify WordPress plugin dependencies and run composer install if available."
	case StackHTML:
		depTitle = "Dependency & Asset Verification"
		depDesc = "Verify static assets (CSS, JS, fonts, images) are installed and valid."
	}

	task3 := TaskTemplate{
		Title:       depTitle,
		Description: depDesc,
		Priority:    "normal",
		Tags:        []string{"dependencies", "build", stack.PrimaryStack},
		DoDChecklist: []db.DoDItem{
			{Text: "Install dependencies using the project's package manager", Completed: false},
			{Text: "Verify all runtime dependencies are available with no missing packages", Completed: false},
			{Text: "Run the build script to compile static assets or the autoloader", Completed: false},
			{Text: "Verify vendor / node_modules directories are installed and intact", Completed: false},
		},
	}
	tasks = append(tasks, task3)

	// 4. Local Verification & Server Deployment
	task4 := TaskTemplate{
		Title:       "Local Verification & Server Deployment",
		Description: fmt.Sprintf("Verify the %s application runs locally or in staging, then deploy it to the server.", stack.Framework),
		Priority:    "high",
		Tags:        []string{"deployment", "verification", stack.PrimaryStack},
		DoDChecklist: []db.DoDItem{
			{Text: "Start the local server and smoke-test the initial release", Completed: false},
			{Text: "Verify primary endpoints and web routes return HTTP 200", Completed: false},
			{Text: "Configure the web server virtual host (Nginx / Caddy) and service supervisor", Completed: false},
			{Text: "Verify the live deployment and stable health checks", Completed: false},
		},
	}
	tasks = append(tasks, task4)

	return tasks
}

// BuildSourceMetadata formats the metadata stored in projects.source_metadata JSONB.
func (g *PipelineGenerator) BuildSourceMetadata(inspection *InspectionResult, archiveName string, totalFiles int, totalBytes int64) (json.RawMessage, error) {
	meta := map[string]interface{}{
		"source":                "zip_uploader",
		"archive_name":          archiveName,
		"primary_stack":         inspection.TechStack.PrimaryStack,
		"framework":             inspection.TechStack.Framework,
		"language":              inspection.TechStack.Language,
		"confidence":            inspection.TechStack.Confidence,
		"tech_stack_desc":       inspection.TechStack.Description,
		"signatures":            inspection.TechStack.Signatures,
		"env_source":            inspection.Env.SourceFile,
		"env_variables":         inspection.Env.AllKeys,
		"sql_files":             inspection.SQLFiles,
		"has_documentation":     inspection.Documentation != nil,
		"extracted_files_count": totalFiles,
		"extracted_total_size":  totalBytes,
		"extracted_at":          time.Now().UTC().Format(time.RFC3339),
	}
	if inspection.Documentation != nil {
		meta["doc_title"] = inspection.Documentation.Title
		meta["doc_excerpt"] = inspection.Documentation.Excerpt
	}

	bytes, err := json.Marshal(meta)
	if err != nil {
		return json.RawMessage("{}"), err
	}
	return json.RawMessage(bytes), nil
}
