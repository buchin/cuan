package diagnostics

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAdminCLIHelp(t *testing.T) {
	for _, cmd := range [][]string{
		{"--help"},
		{"admin", "--help"},
		{"backup", "--help"},
		{"restore", "--help"},
	} {
		var out, errOut bytes.Buffer
		code := RunAdminCLI(cmd, &out, &errOut)
		if code != 0 {
			t.Fatalf("expected exit code 0 for help on %v, got %d", cmd, code)
		}
		if !strings.Contains(out.String(), "Cuan Administrative") && !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("expected usage text in help output, got %s", out.String())
		}
	}
}

func TestAdminCLIUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := RunAdminCLI([]string{"admin", "invalid-subcommand"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(errOut.String(), "Unknown admin command") {
		t.Fatalf("expected error message in output, got: %s", errOut.String())
	}
}

func TestAdminTarGzPackUnpack(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")
	tarPath := filepath.Join(tempDir, "test.tar.gz")

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "manifest.json"), []byte(`{"version":"1.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "database.sql"), []byte(`-- test dump`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := createTarGz(tarPath, sourceDir); err != nil {
		t.Fatalf("createTarGz failed: %v", err)
	}

	if _, err := os.Stat(tarPath); err != nil {
		t.Fatalf("archive file not found: %v", err)
	}

	if err := extractTarGz(tarPath, destDir); err != nil {
		t.Fatalf("extractTarGz failed: %v", err)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(destDir, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest not extracted: %v", err)
	}
	if string(manifestBytes) != `{"version":"1.0"}` {
		t.Fatalf("unexpected manifest content: %s", string(manifestBytes))
	}

	sqlBytes, err := os.ReadFile(filepath.Join(destDir, "database.sql"))
	if err != nil {
		t.Fatalf("database.sql not extracted: %v", err)
	}
	if string(sqlBytes) != `-- test dump` {
		t.Fatalf("unexpected sql content: %s", string(sqlBytes))
	}
}

func TestAdminResolveDatabaseURL(t *testing.T) {
	explicit := "postgres://user:pass@localhost:5432/testdb"
	if got := resolveDatabaseURL(explicit); got != explicit {
		t.Fatalf("expected explicit url, got %s", got)
	}

	t.Setenv("DATABASE_URL", "postgres://envuser:envpass@localhost:5432/envdb")
	if got := resolveDatabaseURL(""); got != "postgres://envuser:envpass@localhost:5432/envdb" {
		t.Fatalf("expected env url, got %s", got)
	}
}

func TestAdminBackupManifestMetadata(t *testing.T) {
	now := time.Now().UTC()
	manifest := BackupManifest{
		Version:        "1.0",
		CreatedAt:      now,
		DatabaseEngine: "postgresql",
		PublicTables:   42,
		DumpFile:       "database.sql",
	}
	if manifest.Version != "1.0" || manifest.PublicTables != 42 || manifest.DatabaseEngine != "postgresql" {
		t.Fatalf("manifest fields mismatch: %+v", manifest)
	}
}
