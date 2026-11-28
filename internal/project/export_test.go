package project

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteExport(t *testing.T) {
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "src"), 0o755))
	must(os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "node_modules", "x"), 0o755))
	must(os.WriteFile(filepath.Join(root, "src", "a.txt"), []byte("hello"), 0o644))
	must(os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=1"), 0o600))
	must(os.WriteFile(filepath.Join(root, ".env.example"), []byte("SECRET="), 0o644))
	must(os.WriteFile(filepath.Join(root, ".git", "config"), []byte("x"), 0o644))
	must(os.WriteFile(filepath.Join(root, "node_modules", "x", "i.js"), []byte("x"), 0o644))

	var buf bytes.Buffer
	p := &Project{ID: "p1", Name: "Demo", LocalPath: root, EncryptedToken: "TOPSECRET"}
	must(WriteExport(&buf, p, time.Unix(0, 0)))
	if bytes.Contains(buf.Bytes(), []byte("TOPSECRET")) {
		t.Fatal("token leaked")
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	must(err)
	names := map[string]*zip.File{}
	for _, f := range zr.File {
		names[f.Name] = f
	}
	for _, want := range []string{"files/src/a.txt", "files/.env.example", "manifest.json", "DEPLOY.md"} {
		if names[want] == nil {
			t.Errorf("missing %s", want)
		}
	}
	for _, bad := range []string{"files/.env", "files/.git/config", "files/node_modules/x/i.js"} {
		if names[bad] != nil {
			t.Errorf("should be skipped: %s", bad)
		}
	}
	rc, _ := names["manifest.json"].Open()
	raw, _ := io.ReadAll(rc)
	var m ExportManifest
	must(json.Unmarshal(raw, &m))
	if len(m.Files) != 2 || len(m.Skipped) != 3 {
		t.Fatalf("files=%d skipped=%d", len(m.Files), len(m.Skipped))
	}
	if m.Files[1].SHA256 != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("sha mismatch: %+v", m.Files)
	}
}
