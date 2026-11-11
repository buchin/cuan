package runner

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetectProjectVerifier(t *testing.T) {
	t.Run("empty directory returns false", func(t *testing.T) {
		spec, ok := detectProjectVerifier("")
		if ok || spec.Command != "" {
			t.Fatalf("expected false, got %+v", spec)
		}
	})

	t.Run("prefers scripts/test.sh if executable", func(t *testing.T) {
		dir := t.TempDir()
		scriptsDir := filepath.Join(dir, "scripts")
		_ = os.MkdirAll(scriptsDir, 0755)
		scriptPath := filepath.Join(scriptsDir, "test.sh")
		_ = os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
		// Even if go.mod exists
		_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644)

		spec, ok := detectProjectVerifier(dir)
		if !ok {
			t.Fatal("expected verifier to be detected")
		}
		if spec.Kind != "custom-script" || spec.Command != scriptPath || !reflect.DeepEqual(spec.Args, []string{"--fast"}) {
			t.Fatalf("unexpected spec: %+v", spec)
		}
	})

	t.Run("detects Go project", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644)

		spec, ok := detectProjectVerifier(dir)
		if !ok || spec.Kind != "go-test" || spec.Command != "go" || !reflect.DeepEqual(spec.Args, []string{"test", "./..."}) {
			t.Fatalf("unexpected Go spec: %+v", spec)
		}
	})

	t.Run("detects PHP Pest", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "composer.json"), []byte("{}"), 0644)
		vendorBin := filepath.Join(dir, "vendor", "bin")
		_ = os.MkdirAll(vendorBin, 0755)
		_ = os.WriteFile(filepath.Join(vendorBin, "pest"), []byte("#!/bin/sh\n"), 0755)

		spec, ok := detectProjectVerifier(dir)
		if !ok || spec.Kind != "php-pest" || spec.Command != "php" || !reflect.DeepEqual(spec.Args, []string{"vendor/bin/pest"}) {
			t.Fatalf("unexpected Pest spec: %+v", spec)
		}
	})

	t.Run("detects Laravel Artisan test", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "composer.json"), []byte("{}"), 0644)
		_ = os.WriteFile(filepath.Join(dir, "artisan"), []byte("#!/usr/bin/env php\n"), 0755)

		spec, ok := detectProjectVerifier(dir)
		if !ok || spec.Kind != "laravel-artisan" || spec.Command != "php" || !reflect.DeepEqual(spec.Args, []string{"artisan", "test"}) {
			t.Fatalf("unexpected Laravel Artisan spec: %+v", spec)
		}
	})

	t.Run("detects Node.js project", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte("{\"scripts\":{\"test\":\"echo ok\"}}"), 0644)

		spec, ok := detectProjectVerifier(dir)
		if !ok || spec.Kind != "npm-test" || spec.Command != "npm" || !reflect.DeepEqual(spec.Args, []string{"test", "--if-present"}) {
			t.Fatalf("unexpected Node spec: %+v", spec)
		}
	})

	t.Run("detects Python project via pyproject.toml", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.pytest]\n"), 0644)

		spec, ok := detectProjectVerifier(dir)
		if !ok || spec.Kind != "python-pytest" || spec.Command != "pytest" || !reflect.DeepEqual(spec.Args, []string{"-q"}) {
			t.Fatalf("unexpected Python spec: %+v", spec)
		}
	})

	t.Run("detects Rust project via Cargo.toml", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"foo\"\n"), 0644)

		spec, ok := detectProjectVerifier(dir)
		if !ok || spec.Kind != "rust-cargo" || spec.Command != "cargo" || !reflect.DeepEqual(spec.Args, []string{"test"}) {
			t.Fatalf("unexpected Rust spec: %+v", spec)
		}
	})
}
