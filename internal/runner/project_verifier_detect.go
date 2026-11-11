package runner

import (
	"os"
	"path/filepath"
)

// ProjectVerifierSpec defines how to execute automated verification for a project.
type ProjectVerifierSpec struct {
	Command string
	Args    []string
	Kind    string
}

// detectProjectVerifier inspects a working directory to determine the appropriate test command.
// Priority:
// 1. scripts/test.sh (Universal repository contract - highest priority)
// 2. PHP / Laravel (composer.json): vendor/bin/pest, vendor/bin/phpunit, or php artisan test
// 3. Go (go.mod): go test ./...
// 4. Node.js (package.json): npm test
// 5. Python (pyproject.toml, pytest.ini, requirements.txt, setup.py): pytest
// 6. Rust (Cargo.toml): cargo test
func detectProjectVerifier(workingDir string) (ProjectVerifierSpec, bool) {
	if workingDir == "" {
		return ProjectVerifierSpec{}, false
	}

	// 1. Universal script contract: scripts/test.sh
	testScript := filepath.Join(workingDir, "scripts", "test.sh")
	if info, err := os.Stat(testScript); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
		return ProjectVerifierSpec{
			Command: testScript,
			Args:    []string{"--fast"},
			Kind:    "custom-script",
		}, true
	}

	// 2. PHP / Laravel
	if _, err := os.Stat(filepath.Join(workingDir, "composer.json")); err == nil {
		pestPath := filepath.Join(workingDir, "vendor", "bin", "pest")
		if info, err := os.Stat(pestPath); err == nil && !info.IsDir() {
			return ProjectVerifierSpec{
				Command: "php",
				Args:    []string{"vendor/bin/pest"},
				Kind:    "php-pest",
			}, true
		}
		phpunitPath := filepath.Join(workingDir, "vendor", "bin", "phpunit")
		if info, err := os.Stat(phpunitPath); err == nil && !info.IsDir() {
			return ProjectVerifierSpec{
				Command: "php",
				Args:    []string{"vendor/bin/phpunit", "--colors=never"},
				Kind:    "php-phpunit",
			}, true
		}
		artisanPath := filepath.Join(workingDir, "artisan")
		if info, err := os.Stat(artisanPath); err == nil && !info.IsDir() {
			return ProjectVerifierSpec{
				Command: "php",
				Args:    []string{"artisan", "test"},
				Kind:    "laravel-artisan",
			}, true
		}
	}

	// 3. Go
	if _, err := os.Stat(filepath.Join(workingDir, "go.mod")); err == nil {
		return ProjectVerifierSpec{
			Command: "go",
			Args:    []string{"test", "./..."},
			Kind:    "go-test",
		}, true
	}

	// 4. Node.js (root package.json)
	if _, err := os.Stat(filepath.Join(workingDir, "package.json")); err == nil {
		return ProjectVerifierSpec{
			Command: "npm",
			Args:    []string{"test", "--if-present"},
			Kind:    "npm-test",
		}, true
	}

	// 5. Python
	pyIndicators := []string{"pyproject.toml", "pytest.ini", "setup.py", "requirements.txt"}
	for _, ind := range pyIndicators {
		if _, err := os.Stat(filepath.Join(workingDir, ind)); err == nil {
			return ProjectVerifierSpec{
				Command: "pytest",
				Args:    []string{"-q"},
				Kind:    "python-pytest",
			}, true
		}
	}

	// 6. Rust
	if _, err := os.Stat(filepath.Join(workingDir, "Cargo.toml")); err == nil {
		return ProjectVerifierSpec{
			Command: "cargo",
			Args:    []string{"test"},
			Kind:    "rust-cargo",
		}, true
	}

	return ProjectVerifierSpec{}, false
}
