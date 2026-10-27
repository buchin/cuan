package inspector

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	envVarKeyRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

	mysqlDialectRegex = regexp.MustCompile(`(?i)(?:ENGINE\s*=\s*InnoDB|AUTO_INCREMENT|DEFAULT\s+CHARSET|utf8mb4|LOCK\s+TABLES)`)
	pgDialectRegex    = regexp.MustCompile(`(?i)(?:SERIAL\s+PRIMARY\s+KEY|CREATE\s+EXTENSION|plpgsql|uuid_generate_v4|timestamptz)`)
	sqliteDialectRegex = regexp.MustCompile(`(?i)(?:sqlite_sequence|PRAGMA\s+foreign_keys)`)

	docHeadingRegex = regexp.MustCompile(`(?i)^#+\s+(.*(?:install|setup|getting\s+started|quick\s*start|panduan|cara\s+install).*)`)
)

// ProjectParser extracts environment configuration, SQL schemas, and install documentation.
type ProjectParser struct{}

// NewProjectParser constructs a ProjectParser.
func NewProjectParser() *ProjectParser {
	return &ProjectParser{}
}

// ParseEnv extracts environment variables from candidate .env files in dir.
func (p *ProjectParser) ParseEnv(dir string, rootPrefix string) EnvVarInfo {
	searchDirs := []string{dir}
	if rootPrefix != "" {
		inner := filepath.Join(dir, rootPrefix)
		if info, err := os.Stat(inner); err == nil && info.IsDir() {
			searchDirs = append([]string{inner}, dir)
		}
	}

	envCandidates := []string{
		".env.example",
		".env.sample",
		".env.template",
		".env.dist",
		".env.default",
		".env.local.example",
		".env",
	}

	for _, dPath := range searchDirs {
		for _, cand := range envCandidates {
			target := filepath.Join(dPath, cand)
			if fileExists(target) {
				return p.parseEnvFile(target)
			}
		}
	}

	return EnvVarInfo{
		AllKeys:      []string{},
		RequiredKeys: []string{},
		DatabaseKeys: []string{},
		SecurityKeys: []string{},
	}
}

func (p *ProjectParser) parseEnvFile(filePath string) EnvVarInfo {
	file, err := os.Open(filePath)
	if err != nil {
		return EnvVarInfo{}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	keySet := make(map[string]bool)
	var (
		allKeys      []string
		reqKeys      []string
		dbKeys       []string
		secKeys      []string
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// Strip leading "export "
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		eqIdx := strings.Index(line, "=")
		if eqIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:eqIdx])
		val := strings.TrimSpace(line[eqIdx+1:])

		if !envVarKeyRegex.MatchString(key) {
			continue
		}

		if keySet[key] {
			continue
		}
		keySet[key] = true
		allKeys = append(allKeys, key)

		upperKey := strings.ToUpper(key)
		upperVal := strings.ToUpper(val)

		// Database keys
		if strings.Contains(upperKey, "DB_") ||
			strings.Contains(upperKey, "DATABASE") ||
			strings.Contains(upperKey, "MYSQL_") ||
			strings.Contains(upperKey, "POSTGRES_") ||
			strings.Contains(upperKey, "PG_") ||
			strings.Contains(upperKey, "REDIS_") {
			dbKeys = append(dbKeys, key)
		}

		// Security keys
		if strings.Contains(upperKey, "KEY") ||
			strings.Contains(upperKey, "SECRET") ||
			strings.Contains(upperKey, "TOKEN") ||
			strings.Contains(upperKey, "PASSWORD") ||
			strings.Contains(upperKey, "AUTH") ||
			strings.Contains(upperKey, "JWT") ||
			strings.Contains(upperKey, "SALT") {
			secKeys = append(secKeys, key)
		}

		// Required keys heuristics (empty value, placeholder strings, or critical keys)
		isPlaceholder := val == "" ||
			strings.Contains(upperVal, "YOUR_") ||
			strings.Contains(upperVal, "CHANGE_ME") ||
			strings.Contains(upperVal, "SECRET") ||
			strings.Contains(upperVal, "XXX")
		if isPlaceholder || upperKey == "APP_KEY" || upperKey == "DB_DATABASE" || upperKey == "DB_PASSWORD" {
			reqKeys = append(reqKeys, key)
		}
	}

	sort.Strings(allKeys)
	sort.Strings(reqKeys)
	sort.Strings(dbKeys)
	sort.Strings(secKeys)

	return EnvVarInfo{
		SourceFile:   filepath.Base(filePath),
		AllKeys:      allKeys,
		RequiredKeys: reqKeys,
		DatabaseKeys: dbKeys,
		SecurityKeys: secKeys,
	}
}

// FindSQLFiles searches the directory for database schema dump files (*.sql).
func (p *ProjectParser) FindSQLFiles(dir string) []SQLFileInfo {
	var sqlFiles []SQLFileInfo

	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "storage" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(strings.ToLower(name), ".sql") {
			info, err := d.Info()
			if err != nil {
				return nil
			}

			rel, _ := filepath.Rel(dir, path)
			dialect := p.detectSQLDialect(path)

			sqlFiles = append(sqlFiles, SQLFileInfo{
				Path:    rel,
				Size:    info.Size(),
				Dialect: dialect,
			})
		}
		return nil
	})

	sort.Slice(sqlFiles, func(i, j int) bool {
		return sqlFiles[i].Path < sqlFiles[j].Path
	})

	return sqlFiles
}

func (p *ProjectParser) detectSQLDialect(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "generic"
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	content := buf[:n]

	if mysqlDialectRegex.Match(content) || bytes.Contains(content, []byte("`")) {
		return "mysql"
	}
	if pgDialectRegex.Match(content) {
		return "postgres"
	}
	if sqliteDialectRegex.Match(content) {
		return "sqlite"
	}

	return "generic"
}

// ParseDocumentation extracts setup guides and readme excerpts.
func (p *ProjectParser) ParseDocumentation(dir string, rootPrefix string) *DocInfo {
	searchDirs := []string{dir}
	if rootPrefix != "" {
		inner := filepath.Join(dir, rootPrefix)
		if info, err := os.Stat(inner); err == nil && info.IsDir() {
			searchDirs = append([]string{inner}, dir)
		}
	}

	candidates := []string{
		"INSTALL.md",
		"install.txt",
		"INSTALL.txt",
		"SETUP.md",
		"setup.txt",
		"README.md",
		"readme.md",
		"README.txt",
		"README",
	}

	for _, dPath := range searchDirs {
		for _, cand := range candidates {
			target := filepath.Join(dPath, cand)
			if fileExists(target) {
				return p.extractDocInfo(target)
			}
		}
	}

	return nil
}

func (p *ProjectParser) extractDocInfo(path string) *DocInfo {
	content, err := os.ReadFile(path)
	if err != nil || len(content) == 0 {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	var title string
	var excerptLines []string
	captureExcerpt := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if title == "" && strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimPrefix(trimmed, "# ")
			continue
		}

		if docHeadingRegex.MatchString(trimmed) {
			captureExcerpt = true
			continue
		}

		if captureExcerpt {
			if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") {
				// Reached another major section
				break
			}
			if len(excerptLines) < 15 && trimmed != "" {
				excerptLines = append(excerptLines, trimmed)
			}
		}
	}

	// If no install heading found, capture first few non-empty lines
	if len(excerptLines) == 0 {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				excerptLines = append(excerptLines, trimmed)
				if len(excerptLines) >= 6 {
					break
				}
			}
		}
	}

	excerpt := strings.Join(excerptLines, "\n")
	if len(excerpt) > 1000 {
		excerpt = excerpt[:1000] + "..."
	}

	return &DocInfo{
		SourceFile: filepath.Base(path),
		Title:      title,
		Excerpt:    excerpt,
	}
}
