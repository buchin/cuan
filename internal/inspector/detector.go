package inspector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	wpPluginHeaderRegex = regexp.MustCompile(`(?i)\/\*[\s\S]*?Plugin Name:\s*([^\r\n*]+)`)
	wpThemeHeaderRegex  = regexp.MustCompile(`(?i)Theme Name:\s*([^\r\n]+)`)
	djangoReqRegex      = regexp.MustCompile(`(?i)(?:^|\n)\s*django(?:[=<>!~]|$)`)
	flaskReqRegex       = regexp.MustCompile(`(?i)(?:^|\n)\s*flask(?:[=<>!~]|$)`)
	laravelReqRegex     = regexp.MustCompile(`(?i)laravel\/framework`)
	symfonyReqRegex     = regexp.MustCompile(`(?i)symfony\/`)
)

// StackDetector analyzes directory contents and identifies technology signatures.
type StackDetector struct{}

// NewStackDetector constructs a StackDetector.
func NewStackDetector() *StackDetector {
	return &StackDetector{}
}

// Detect analyzes the project directory (including resolving single root directories)
// and returns the best matching TechStackInfo.
func (d *StackDetector) Detect(dir string, rootPrefix string) TechStackInfo {
	searchDirs := []string{dir}
	if rootPrefix != "" {
		inner := filepath.Join(dir, rootPrefix)
		if info, err := os.Stat(inner); err == nil && info.IsDir() {
			searchDirs = append([]string{inner}, dir)
		}
	}

	for _, dPath := range searchDirs {
		// 1. Laravel (artisan + composer.json or laravel/framework in composer.json)
		if info, ok := d.detectLaravel(dPath); ok {
			return info
		}

		// 2. WordPress (wp-config.php, plugin header, or theme header)
		if info, ok := d.detectWordPress(dPath); ok {
			return info
		}

		// 3. Node.js / Next.js / React / Express
		if info, ok := d.detectNode(dPath); ok {
			return info
		}

		// 4. Python (Django, Flask, generic)
		if info, ok := d.detectPython(dPath); ok {
			return info
		}

		// 5. Generic Composer / PHP
		if info, ok := d.detectPHP(dPath); ok {
			return info
		}

		// 6. Static HTML
		if info, ok := d.detectStaticHTML(dPath); ok {
			return info
		}
	}

	return TechStackInfo{
		PrimaryStack: StackUnknown,
		Framework:    "Unknown Project",
		Language:     "text",
		Confidence:   0.10,
		Description:  "Unrecognized source code archive",
		Signatures:   []string{},
	}
}

func (d *StackDetector) detectLaravel(dir string) (TechStackInfo, bool) {
	hasArtisan := fileExists(filepath.Join(dir, "artisan"))
	hasComposer := fileExists(filepath.Join(dir, "composer.json"))

	if hasArtisan && hasComposer {
		return TechStackInfo{
			PrimaryStack: StackLaravel,
			Framework:    "Laravel",
			Language:     "php",
			Confidence:   0.98,
			Description:  "Laravel PHP Framework Web Application with artisan CLI and Composer",
			Signatures:   []string{"composer.json", "artisan"},
		}, true
	}

	if hasComposer {
		content, err := os.ReadFile(filepath.Join(dir, "composer.json"))
		if err == nil && laravelReqRegex.Match(content) {
			sigs := []string{"composer.json"}
			if hasArtisan {
				sigs = append(sigs, "artisan")
			}
			return TechStackInfo{
				PrimaryStack: StackLaravel,
				Framework:    "Laravel",
				Language:     "php",
				Confidence:   0.95,
				Description:  "Laravel PHP Framework Web Application",
				Signatures:   sigs,
			}, true
		}
	}

	return TechStackInfo{}, false
}

func (d *StackDetector) detectWordPress(dir string) (TechStackInfo, bool) {
	// Site check: wp-config.php or wp-login.php
	if fileExists(filepath.Join(dir, "wp-config.php")) || fileExists(filepath.Join(dir, "wp-config-sample.php")) {
		return TechStackInfo{
			PrimaryStack: StackWordPress,
			Framework:    "WordPress Site",
			Language:     "php",
			Confidence:   0.98,
			Description:  "WordPress CMS installation",
			Signatures:   []string{"wp-config.php"},
		}, true
	}

	// Plugin check: inspect PHP files in root for "Plugin Name:"
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".php") {
				content, err := os.ReadFile(filepath.Join(dir, e.Name()))
				if err == nil && wpPluginHeaderRegex.Match(content) {
					matches := wpPluginHeaderRegex.FindSubmatch(content)
					pluginName := "WordPress Plugin"
					if len(matches) > 1 {
						pluginName = strings.TrimSpace(string(matches[1]))
					}
					return TechStackInfo{
						PrimaryStack: StackWordPress,
						Framework:    pluginName,
						Language:     "php",
						Confidence:   0.95,
						Description:  "WordPress Plugin extension package",
						Signatures:   []string{e.Name() + " (Plugin Name header)"},
					}, true
				}
			}
		}
	}

	// Theme check: style.css with "Theme Name:"
	styleCSS := filepath.Join(dir, "style.css")
	if fileExists(styleCSS) {
		content, err := os.ReadFile(styleCSS)
		if err == nil && wpThemeHeaderRegex.Match(content) {
			matches := wpThemeHeaderRegex.FindSubmatch(content)
			themeName := "WordPress Theme"
			if len(matches) > 1 {
				themeName = strings.TrimSpace(string(matches[1]))
			}
			return TechStackInfo{
				PrimaryStack: StackWordPress,
				Framework:    themeName,
				Language:     "php",
				Confidence:   0.95,
				Description:  "WordPress Theme template package",
				Signatures:   []string{"style.css (Theme Name)"},
			}, true
		}
	}

	return TechStackInfo{}, false
}

func (d *StackDetector) detectNode(dir string) (TechStackInfo, bool) {
	pkgPath := filepath.Join(dir, "package.json")
	if !fileExists(pkgPath) {
		return TechStackInfo{}, false
	}

	content, err := os.ReadFile(pkgPath)
	if err != nil {
		return TechStackInfo{}, false
	}

	var pkg struct {
		Name            string            `json:"name"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
	}
	_ = json.Unmarshal(content, &pkg)

	hasDep := func(name string) bool {
		if _, ok := pkg.Dependencies[name]; ok {
			return true
		}
		if _, ok := pkg.DevDependencies[name]; ok {
			return true
		}
		return false
	}

	hasScriptUsing := func(tool string) bool {
		for _, cmd := range pkg.Scripts {
			if strings.Contains(cmd, tool) {
				return true
			}
		}
		return false
	}

	isTS := fileExists(filepath.Join(dir, "tsconfig.json"))
	lang := "javascript"
	if isTS {
		lang = "typescript"
	}

	// Next.js
	hasNextConfig := fileExists(filepath.Join(dir, "next.config.js")) ||
		fileExists(filepath.Join(dir, "next.config.mjs")) ||
		fileExists(filepath.Join(dir, "next.config.ts"))
	if hasDep("next") || hasNextConfig || hasScriptUsing("next ") {
		sigs := []string{"package.json"}
		if hasNextConfig {
			sigs = append(sigs, "next.config.js")
		}
		return TechStackInfo{
			PrimaryStack: StackNext,
			Framework:    "Next.js",
			Language:     lang,
			Confidence:   0.98,
			Description:  "Next.js Full-Stack React Framework Application",
			Signatures:   sigs,
		}, true
	}

	// Express
	if hasDep("express") {
		return TechStackInfo{
			PrimaryStack: StackExpress,
			Framework:    "Express.js",
			Language:     lang,
			Confidence:   0.95,
			Description:  "Express.js Node.js REST API / Web Application",
			Signatures:   []string{"package.json (express)"},
		}, true
	}

	// React SPA
	if hasDep("react") {
		return TechStackInfo{
			PrimaryStack: StackReact,
			Framework:    "React",
			Language:     lang,
			Confidence:   0.95,
			Description:  "React Single Page Web Application",
			Signatures:   []string{"package.json (react)"},
		}, true
	}

	// Generic Node.js
	return TechStackInfo{
		PrimaryStack: StackNode,
		Framework:    "Node.js",
		Language:     lang,
		Confidence:   0.90,
		Description:  "Node.js JavaScript Application",
		Signatures:   []string{"package.json"},
	}, true
}

func (d *StackDetector) detectPython(dir string) (TechStackInfo, bool) {
	hasManagePy := fileExists(filepath.Join(dir, "manage.py"))
	hasReqs := fileExists(filepath.Join(dir, "requirements.txt"))
	hasAppPy := fileExists(filepath.Join(dir, "app.py"))
	hasPipfile := fileExists(filepath.Join(dir, "Pipfile"))
	hasPyproject := fileExists(filepath.Join(dir, "pyproject.toml"))

	var reqsContent []byte
	if hasReqs {
		reqsContent, _ = os.ReadFile(filepath.Join(dir, "requirements.txt"))
	}

	// Django
	if hasManagePy || (len(reqsContent) > 0 && djangoReqRegex.Match(reqsContent)) {
		sigs := []string{}
		if hasManagePy {
			sigs = append(sigs, "manage.py")
		}
		if hasReqs {
			sigs = append(sigs, "requirements.txt")
		}
		return TechStackInfo{
			PrimaryStack: StackDjango,
			Framework:    "Django",
			Language:     "python",
			Confidence:   0.98,
			Description:  "Django Python Web Application Framework",
			Signatures:   sigs,
		}, true
	}

	// Flask
	if len(reqsContent) > 0 && flaskReqRegex.Match(reqsContent) {
		return TechStackInfo{
			PrimaryStack: StackFlask,
			Framework:    "Flask",
			Language:     "python",
			Confidence:   0.95,
			Description:  "Flask Python Micro-Framework Web Application",
			Signatures:   []string{"requirements.txt (flask)"},
		}, true
	}
	if hasAppPy {
		content, err := os.ReadFile(filepath.Join(dir, "app.py"))
		if err == nil && (strings.Contains(string(content), "Flask(__name__)") || strings.Contains(string(content), "import Flask") || strings.Contains(string(content), "from flask import")) {
			return TechStackInfo{
				PrimaryStack: StackFlask,
				Framework:    "Flask",
				Language:     "python",
				Confidence:   0.95,
				Description:  "Flask Python Micro-Framework Web Application",
				Signatures:   []string{"app.py (Flask import)"},
			}, true
		}
	}

	// Generic Python
	if hasReqs || hasPipfile || hasPyproject {
		sigs := []string{}
		if hasReqs {
			sigs = append(sigs, "requirements.txt")
		}
		if hasPipfile {
			sigs = append(sigs, "Pipfile")
		}
		if hasPyproject {
			sigs = append(sigs, "pyproject.toml")
		}
		return TechStackInfo{
			PrimaryStack: StackPython,
			Framework:    "Python",
			Language:     "python",
			Confidence:   0.90,
			Description:  "Python Application with dependency manifests",
			Signatures:   sigs,
		}, true
	}

	return TechStackInfo{}, false
}

func (d *StackDetector) detectPHP(dir string) (TechStackInfo, bool) {
	hasComposer := fileExists(filepath.Join(dir, "composer.json"))
	if hasComposer {
		content, err := os.ReadFile(filepath.Join(dir, "composer.json"))
		if err == nil && symfonyReqRegex.Match(content) {
			return TechStackInfo{
				PrimaryStack: "symfony",
				Framework:    "Symfony",
				Language:     "php",
				Confidence:   0.95,
				Description:  "Symfony PHP Web Framework Application",
				Signatures:   []string{"composer.json (symfony)"},
			}, true
		}
		return TechStackInfo{
			PrimaryStack: "php",
			Framework:    "PHP (Composer)",
			Language:     "php",
			Confidence:   0.90,
			Description:  "PHP Application with Composer packages",
			Signatures:   []string{"composer.json"},
		}, true
	}

	// Look for index.php
	if fileExists(filepath.Join(dir, "index.php")) {
		return TechStackInfo{
			PrimaryStack: "php",
			Framework:    "Plain PHP",
			Language:     "php",
			Confidence:   0.85,
			Description:  "PHP Script Application",
			Signatures:   []string{"index.php"},
		}, true
	}

	return TechStackInfo{}, false
}

func (d *StackDetector) detectStaticHTML(dir string) (TechStackInfo, bool) {
	if fileExists(filepath.Join(dir, "index.html")) {
		return TechStackInfo{
			PrimaryStack: StackHTML,
			Framework:    "Static HTML",
			Language:     "html",
			Confidence:   0.85,
			Description:  "Static HTML/CSS/JavaScript Website",
			Signatures:   []string{"index.html"},
		}, true
	}
	return TechStackInfo{}, false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
