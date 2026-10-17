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
	envDesc := "Konfigurasi environment project untuk runtime aplikasi."
	if env.SourceFile != "" {
		envDesc = fmt.Sprintf("Template environment terdeteksi dari '%s'.", env.SourceFile)
		if len(env.RequiredKeys) > 0 {
			previewCount := 6
			if len(env.RequiredKeys) < previewCount {
				previewCount = len(env.RequiredKeys)
			}
			envDesc += fmt.Sprintf(" Variabel penting terdeteksi: %s.", strings.Join(env.RequiredKeys[:previewCount], ", "))
		}
	} else {
		envDesc = "Buat berkas konfigurasi .env sesuai kebutuhan stack " + stack.Framework + "."
	}

	task1 := TaskTemplate{
		Title:       "Setup Environment & .env Configuration",
		Description: envDesc,
		Priority:    "high",
		Tags:        []string{"setup", "environment", stack.PrimaryStack},
		DoDChecklist: []db.DoDItem{
			{Text: "Salin berkas template .env.example / .env.sample menjadi .env", Completed: false},
			{Text: "Konfigurasi kredensial koneksi database (DB_HOST, DB_DATABASE, DB_USERNAME, DB_PASSWORD)", Completed: false},
			{Text: "Generate application secret key / APP_KEY yang dibutuhkan aplikasi", Completed: false},
			{Text: "Verifikasi permission file .env aman dari akses publik (chmod 600/640)", Completed: false},
		},
	}
	tasks = append(tasks, task1)

	// 2. Database Setup & SQL Import
	dbPriority := "normal"
	dbDesc := "Persiapan database dan impor skema data aplikasi."
	if len(sqlFiles) > 0 {
		dbPriority = "high"
		sqlPaths := make([]string, 0, len(sqlFiles))
		for _, f := range sqlFiles {
			sqlPaths = append(sqlPaths, f.Path)
		}
		dbDesc = fmt.Sprintf("Berkas skema database terdeteksi: %s (Dialek: %s).", strings.Join(sqlPaths, ", "), sqlFiles[0].Dialect)
	} else if stack.PrimaryStack == StackLaravel {
		dbDesc = "Jalankan migrasi database Laravel (php artisan migrate)."
		dbPriority = "high"
	} else if stack.PrimaryStack == StackDjango {
		dbDesc = "Jalankan migrasi database Django (python manage.py migrate)."
		dbPriority = "high"
	}

	task2 := TaskTemplate{
		Title:       "Database Setup & SQL Import",
		Description: dbDesc,
		Priority:    dbPriority,
		Tags:        []string{"database", "sql", stack.PrimaryStack},
		DoDChecklist: []db.DoDItem{
			{Text: "Buat database target pada DBMS server (MySQL/PostgreSQL/SQLite)", Completed: false},
			{Text: "Impor skema SQL terdeteksi atau jalankan migrasi database awal", Completed: false},
			{Text: "Pastikan seluruh struktur tabel dan data master/seeder terisi tanpa error", Completed: false},
			{Text: "Uji konektivitas runtime aplikasi ke database berhasil", Completed: false},
		},
	}
	tasks = append(tasks, task2)

	// 3. Dependency Installation (composer/npm/pip)
	depTitle := "Dependency Installation (composer/npm/pip)"
	depDesc := "Instalasi seluruh dependensi third-party runtime aplikasi."
	switch stack.PrimaryStack {
	case StackLaravel, "php":
		depTitle = "Dependency Installation (composer)"
		depDesc = "Jalankan composer install --no-dev --optimize-autoloader untuk memasang dependensi PHP."
	case StackNode, StackNext, StackReact, StackExpress:
		depTitle = "Dependency Installation (npm)"
		depDesc = "Jalankan npm install && npm run build untuk memasang packages dan compile front-end assets."
	case StackPython, StackDjango, StackFlask:
		depTitle = "Dependency Installation (pip)"
		depDesc = "Jalankan pip install -r requirements.txt pada virtualenv python aplikasi."
	case StackWordPress:
		depTitle = "Dependency Installation (composer/plugins)"
		depDesc = "Verifikasi dependensi plugin WordPress dan eksekusi composer install jika tersedia."
	case StackHTML:
		depTitle = "Dependency & Asset Verification"
		depDesc = "Verifikasi asset static (CSS, JS, Fonts, Images) terpasang lengkap dan valid."
	}

	task3 := TaskTemplate{
		Title:       depTitle,
		Description: depDesc,
		Priority:    "normal",
		Tags:        []string{"dependencies", "build", stack.PrimaryStack},
		DoDChecklist: []db.DoDItem{
			{Text: "Jalankan instalasi dependensi paket sesuai package manager proyek", Completed: false},
			{Text: "Pastikan seluruh library dependensi runtime terpenuhi tanpa missing packages", Completed: false},
			{Text: "Jalankan build script untuk meng-compile static assets atau autoloader", Completed: false},
			{Text: "Verifikasi direktori vendor / node_modules terpasang dengan integritas lengkap", Completed: false},
		},
	}
	tasks = append(tasks, task3)

	// 4. Local Verification & Server Deployment
	task4 := TaskTemplate{
		Title:       "Local Verification & Server Deployment",
		Description: fmt.Sprintf("Verifikasi jalannya aplikasi %s di environment lokal/staging dan lakukan deployment ke server.", stack.Framework),
		Priority:    "high",
		Tags:        []string{"deployment", "verification", stack.PrimaryStack},
		DoDChecklist: []db.DoDItem{
			{Text: "Jalankan local server / smoke test rilis awal", Completed: false},
			{Text: "Verifikasi endpoint utama dan web routing merespons HTTP 200", Completed: false},
			{Text: "Konfigurasi virtual host web server (Nginx / Caddy) dan service supervisor", Completed: false},
			{Text: "Verifikasi deployment live dan health check aktif stabil", Completed: false},
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
