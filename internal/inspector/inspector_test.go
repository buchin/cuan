package inspector_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/inspector"
	"github.com/buchin/cuan/internal/project"
)

// Helper to create an in-memory zip archive with arbitrary files.
func createTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	return buf.Bytes()
}

func TestSanitizeZipEntryPath_ZipSlip(t *testing.T) {
	tempDest, err := os.MkdirTemp("", "cuan-test-dest-*")
	if err != nil {
		t.Fatalf("failed to create temp dest: %v", err)
	}
	defer os.RemoveAll(tempDest)

	validCases := []string{
		"index.html",
		"app/Http/Controllers/HomeController.php",
		"src/components/Header.tsx",
		"config.json",
		"nested/sub/folder/file.txt",
	}

	for _, p := range validCases {
		sanitized, err := inspector.SanitizeZipEntryPath(tempDest, p)
		if err != nil {
			t.Errorf("expected valid path for %q, got err: %v", p, err)
		}
		if !strings.HasPrefix(sanitized, tempDest) {
			t.Errorf("sanitized path %q escaped tempDest %q", sanitized, tempDest)
		}
	}

	maliciousCases := []string{
		"../evil.sh",
		"../../evil.sh",
		"../../../etc/passwd",
		"app/../../evil.sh",
		"foo/bar/../../../evil.sh",
		"..\\evil.bat",
		"foo\\..\\..\\evil.bat",
		"/etc/passwd",
		"/var/www/html",
		"C:\\Windows\\System32\\cmd.exe",
		"C:/evil.exe",
		"..",
		"foo/..",
	}

	for _, p := range maliciousCases {
		_, err := inspector.SanitizeZipEntryPath(tempDest, p)
		if err == nil {
			t.Errorf("expected ErrZipSlip for malicious path %q, got nil", p)
		} else if !errors.Is(err, inspector.ErrZipSlip) {
			t.Errorf("expected ErrZipSlip, got: %v for path %q", err, p)
		}
	}
}

func TestSafeUnpacker_ZipSlip_Rejected(t *testing.T) {
	destDir, err := os.MkdirTemp("", "cuan-test-unpack-slip-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	// Create zip with malicious traversal header
	zipBytes := createTestZip(t, map[string]string{
		"valid.txt":               "safe content",
		"nested/safe.txt":         "safe content 2",
		"nested/../../traversal": "malicious payload",
	})

	unpacker := inspector.NewSafeUnpacker(150 * 1024 * 1024)
	reader := bytes.NewReader(zipBytes)

	_, err = unpacker.Unpack(reader, int64(len(zipBytes)), destDir)
	if err == nil {
		t.Fatal("expected unpacker to reject zip slip, but got nil err")
	}

	if !errors.Is(err, inspector.ErrZipSlip) {
		t.Fatalf("expected ErrZipSlip, got: %v", err)
	}

	// Verify no traversal file was written
	escapedFile := filepath.Join(filepath.Dir(destDir), "traversal")
	if _, err := os.Stat(escapedFile); err == nil {
		t.Fatalf("zip slip vulnerability! File escaped to %s", escapedFile)
	}
}

func TestSafeUnpacker_QuotaExceeded(t *testing.T) {
	destDir, err := os.MkdirTemp("", "cuan-test-quota-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	// Create 200 bytes of content, set quota to 100 bytes
	largeContent := strings.Repeat("A", 200)
	zipBytes := createTestZip(t, map[string]string{
		"file.txt": largeContent,
	})

	unpacker := inspector.NewSafeUnpacker(100) // 100 bytes max quota
	reader := bytes.NewReader(zipBytes)

	_, err = unpacker.Unpack(reader, int64(len(zipBytes)), destDir)
	if err == nil {
		t.Fatal("expected quota exceeded error, got nil")
	}

	if !errors.Is(err, inspector.ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got: %v", err)
	}
}

func TestStackDetector_Laravel(t *testing.T) {
	destDir, err := os.MkdirTemp("", "cuan-test-laravel-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	_ = os.WriteFile(filepath.Join(destDir, "artisan"), []byte("#!/usr/bin/env php\n<?php"), 0755)
	composerContent := `{
		"name": "laravel/laravel",
		"require": {
			"php": "^8.2",
			"laravel/framework": "^11.0"
		}
	}`
	_ = os.WriteFile(filepath.Join(destDir, "composer.json"), []byte(composerContent), 0644)

	detector := inspector.NewStackDetector()
	info := detector.Detect(destDir, "")

	if info.PrimaryStack != inspector.StackLaravel {
		t.Fatalf("expected primary stack %s, got %s", inspector.StackLaravel, info.PrimaryStack)
	}
	if info.Framework != "Laravel" {
		t.Errorf("expected framework Laravel, got %s", info.Framework)
	}
	if info.Language != "php" {
		t.Errorf("expected language php, got %s", info.Language)
	}
	if info.Confidence < 0.90 {
		t.Errorf("expected high confidence >= 0.90, got %f", info.Confidence)
	}
}

func TestStackDetector_NodeNext(t *testing.T) {
	destDir, err := os.MkdirTemp("", "cuan-test-next-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	pkgContent := `{
		"name": "my-next-app",
		"dependencies": {
			"next": "^14.2.0",
			"react": "^18.2.0",
			"react-dom": "^18.2.0"
		}
	}`
	_ = os.WriteFile(filepath.Join(destDir, "package.json"), []byte(pkgContent), 0644)
	_ = os.WriteFile(filepath.Join(destDir, "tsconfig.json"), []byte("{}"), 0644)

	detector := inspector.NewStackDetector()
	info := detector.Detect(destDir, "")

	if info.PrimaryStack != inspector.StackNext {
		t.Fatalf("expected primary stack %s, got %s", inspector.StackNext, info.PrimaryStack)
	}
	if info.Framework != "Next.js" {
		t.Errorf("expected framework Next.js, got %s", info.Framework)
	}
	if info.Language != "typescript" {
		t.Errorf("expected language typescript, got %s", info.Language)
	}
}

func TestStackDetector_PythonDjangoAndFlask(t *testing.T) {
	// Django test
	djangoDir, err := os.MkdirTemp("", "cuan-test-django-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(djangoDir)

	_ = os.WriteFile(filepath.Join(djangoDir, "manage.py"), []byte("#!/usr/bin/env python"), 0755)
	_ = os.WriteFile(filepath.Join(djangoDir, "requirements.txt"), []byte("django>=4.2\npsycopg2-binary>=2.9\n"), 0644)

	detector := inspector.NewStackDetector()
	info := detector.Detect(djangoDir, "")

	if info.PrimaryStack != inspector.StackDjango {
		t.Fatalf("expected django, got %s", info.PrimaryStack)
	}
	if info.Framework != "Django" {
		t.Errorf("expected framework Django, got %s", info.Framework)
	}

	// Flask test
	flaskDir, err := os.MkdirTemp("", "cuan-test-flask-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(flaskDir)

	_ = os.WriteFile(filepath.Join(flaskDir, "app.py"), []byte("from flask import Flask\napp = Flask(__name__)\n"), 0644)
	_ = os.WriteFile(filepath.Join(flaskDir, "requirements.txt"), []byte("flask==3.0.0\ngunicorn>=21.2\n"), 0644)

	fInfo := detector.Detect(flaskDir, "")
	if fInfo.PrimaryStack != inspector.StackFlask {
		t.Fatalf("expected flask, got %s", fInfo.PrimaryStack)
	}
}

func TestStackDetector_WordPress(t *testing.T) {
	wpDir, err := os.MkdirTemp("", "cuan-test-wp-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(wpDir)

	pluginContent := `<?php
/*
Plugin Name: Cuan Payment Gateway
Description: Custom payment gateway plugin.
Version: 1.0.0
Author: Dropsugar
*/
`
	_ = os.WriteFile(filepath.Join(wpDir, "cuan-gateway.php"), []byte(pluginContent), 0644)

	detector := inspector.NewStackDetector()
	info := detector.Detect(wpDir, "")

	if info.PrimaryStack != inspector.StackWordPress {
		t.Fatalf("expected wordpress, got %s", info.PrimaryStack)
	}
	if !strings.Contains(info.Framework, "Cuan Payment Gateway") {
		t.Errorf("expected plugin name in framework, got %s", info.Framework)
	}
}

func TestStackDetector_StaticHTML(t *testing.T) {
	htmlDir, err := os.MkdirTemp("", "cuan-test-html-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(htmlDir)

	_ = os.WriteFile(filepath.Join(htmlDir, "index.html"), []byte("<!DOCTYPE html><html><body><h1>Hello</h1></body></html>"), 0644)

	detector := inspector.NewStackDetector()
	info := detector.Detect(htmlDir, "")

	if info.PrimaryStack != inspector.StackHTML {
		t.Fatalf("expected html, got %s", info.PrimaryStack)
	}
	if info.Framework != "Static HTML" {
		t.Errorf("expected Static HTML, got %s", info.Framework)
	}
}

func TestProjectParser_EnvAndSQL(t *testing.T) {
	projDir, err := os.MkdirTemp("", "cuan-test-parser-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(projDir)

	envContent := `# Application Configuration
APP_NAME=CuanShop
APP_ENV=local
APP_KEY=base64:xxx
APP_DEBUG=true
APP_URL=http://localhost

# Database Credentials
DB_CONNECTION=mysql
DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE=cuan_store
DB_USERNAME=root
DB_PASSWORD=secret

# Third party
JWT_SECRET=your_jwt_secret
STRIPE_KEY=
`
	_ = os.WriteFile(filepath.Join(projDir, ".env.example"), []byte(envContent), 0644)

	sqlContent := `-- Database Dump
CREATE TABLE users (
  id int(11) NOT NULL AUTO_INCREMENT,
  name varchar(255) NOT NULL,
  email varchar(255) NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`
	_ = os.WriteFile(filepath.Join(projDir, "database.sql"), []byte(sqlContent), 0644)

	readmeContent := `# Cuan Shop Script

## Requirements
- PHP 8.2+
- MySQL 8.0+

## Installation Guide
1. Copy .env.example to .env and configure database credentials
2. Import database.sql to your MySQL server
3. Run composer install
4. Run php artisan key:generate
`
	_ = os.WriteFile(filepath.Join(projDir, "README.md"), []byte(readmeContent), 0644)

	parser := inspector.NewProjectParser()
	envInfo := parser.ParseEnv(projDir, "")

	if len(envInfo.AllKeys) == 0 {
		t.Fatal("expected env keys to be extracted")
	}
	if envInfo.SourceFile != ".env.example" {
		t.Errorf("expected source file .env.example, got %s", envInfo.SourceFile)
	}
	if len(envInfo.DatabaseKeys) < 4 {
		t.Errorf("expected database keys, got %v", envInfo.DatabaseKeys)
	}
	if len(envInfo.SecurityKeys) < 2 {
		t.Errorf("expected security keys, got %v", envInfo.SecurityKeys)
	}

	sqlFiles := parser.FindSQLFiles(projDir)
	if len(sqlFiles) != 1 {
		t.Fatalf("expected 1 sql file, got %d", len(sqlFiles))
	}
	if sqlFiles[0].Path != "database.sql" {
		t.Errorf("expected database.sql, got %s", sqlFiles[0].Path)
	}
	if sqlFiles[0].Dialect != "mysql" {
		t.Errorf("expected mysql dialect, got %s", sqlFiles[0].Dialect)
	}

	docInfo := parser.ParseDocumentation(projDir, "")
	if docInfo == nil {
		t.Fatal("expected docInfo, got nil")
	}
	if docInfo.Title != "Cuan Shop Script" {
		t.Errorf("expected title 'Cuan Shop Script', got %s", docInfo.Title)
	}
	if !strings.Contains(docInfo.Excerpt, "Copy .env.example") {
		t.Errorf("expected excerpt to contain installation steps, got %s", docInfo.Excerpt)
	}
}

func TestEngine_InspectZip_And_CreateProjectFromZip(t *testing.T) {
	// Build a realistic sample Laravel zip archive
	zipFiles := map[string]string{
		"artisan": "#!/usr/bin/env php\n<?php",
		"composer.json": `{
			"name": "cuan/ecommerce-script",
			"require": {
				"php": "^8.2",
				"laravel/framework": "^11.0"
			}
		}`,
		".env.example": `APP_NAME=LaravelShop
APP_KEY=
DB_CONNECTION=mysql
DB_DATABASE=shop_db
DB_USERNAME=root
DB_PASSWORD=
`,
		"database.sql": `CREATE TABLE ` + "`products`" + ` (
			id int(11) NOT NULL AUTO_INCREMENT,
			title varchar(255) NOT NULL,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		"README.md": `# Laravel Ecommerce Script

## Installation
1. Setup .env
2. Import database.sql
3. Run composer install
`,
	}

	zipBytes := createTestZip(t, zipFiles)

	tempBaseDir, err := os.MkdirTemp("", "cuan-test-projects-base-*")
	if err != nil {
		t.Fatalf("create temp base dir: %v", err)
	}
	defer os.RemoveAll(tempBaseDir)

	// Setup in-memory stores
	projStore := project.NewMemoryStore()
	studioRepo := db.NewMemoryProjectStudioRepository()
	agentRepo := db.NewMemoryAgentRepository()

	engine := inspector.NewEngine(
		inspector.WithProjectStore(projStore),
		inspector.WithStudioRepo(studioRepo),
		inspector.WithAgentRepo(agentRepo),
		inspector.WithBaseDir(tempBaseDir),
	)

	// 1. Inspect without project creation
	reader := bytes.NewReader(zipBytes)
	inspectRes, err := engine.InspectZip(reader, int64(len(zipBytes)), "ecommerce.zip")
	if err != nil {
		t.Fatalf("InspectZip failed: %v", err)
	}

	if inspectRes.TechStack.PrimaryStack != inspector.StackLaravel {
		t.Errorf("expected primary stack laravel, got %s", inspectRes.TechStack.PrimaryStack)
	}
	if len(inspectRes.SQLFiles) != 1 {
		t.Errorf("expected 1 sql file, got %d", len(inspectRes.SQLFiles))
	}
	if len(inspectRes.RecommendedModules) != 4 {
		t.Errorf("expected 4 recommended modules, got %d", len(inspectRes.RecommendedModules))
	}
	if len(inspectRes.RecommendedTasks) != 4 {
		t.Errorf("expected 4 setup tasks, got %d", len(inspectRes.RecommendedTasks))
	}

	// 2. Create project from zip
	ctx := context.Background()
	req := inspector.CreateFromZipRequest{
		ProjectName: "Awesome Ecommerce",
		ArchiveName: "ecommerce.zip",
		ZipReader:   bytes.NewReader(zipBytes),
		ZipSize:     int64(len(zipBytes)),
	}

	resp, err := engine.CreateProjectFromZip(ctx, req)
	if err != nil {
		t.Fatalf("CreateProjectFromZip failed: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("expected status success, got %s", resp.Status)
	}
	if resp.Project == nil {
		t.Fatal("expected project to be created, got nil")
	}
	if resp.Project.SourceType != "zip" {
		t.Errorf("expected source_type 'zip', got %s", resp.Project.SourceType)
	}
	if resp.Room.Title != "Script Setup & Onboarding" {
		t.Errorf("expected room title 'Script Setup & Onboarding', got %s", resp.Room.Title)
	}
	if resp.Agent.Name != "DevOps Setup Assistant" {
		t.Errorf("expected agent 'DevOps Setup Assistant', got %s", resp.Agent.Name)
	}
	if len(resp.Tasks) != 4 {
		t.Fatalf("expected 4 setup tasks created, got %d", len(resp.Tasks))
	}

	// Verify each task has actionable DoD checklist items
	for i, task := range resp.Tasks {
		if task.Title == "" {
			t.Errorf("task %d title is empty", i)
		}
		if len(task.DoDChecklist) == 0 {
			t.Errorf("task %q has empty DoD checklist", task.Title)
		}
		for j, item := range task.DoDChecklist {
			if item.Text == "" {
				t.Errorf("task %q DoD item %d text is empty", task.Title, j)
			}
		}
	}

	// Verify files actually exist in destination directory
	projectPath := resp.Project.LocalPath
	if _, err := os.Stat(filepath.Join(projectPath, "artisan")); err != nil {
		t.Errorf("expected artisan to exist in project directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectPath, "database.sql")); err != nil {
		t.Errorf("expected database.sql to exist in project directory: %v", err)
	}
}
