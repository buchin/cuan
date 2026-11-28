package project

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ExportMaxFileBytes caps a single file in an export archive. Larger files are
// listed in the manifest as skipped so the user knows they were left out.
const ExportMaxFileBytes = 50 << 20

var exportSkipDirs = map[string]bool{
	".git": true, "node_modules": true, ".next": true, "vendor": true,
	".cache": true, "__pycache__": true,
}

// ExportManifest describes the contents of a project export archive.
type ExportManifest struct {
	Version     int             `json:"version"`
	ExportedAt  time.Time       `json:"exported_at"`
	ProjectID   string          `json:"project_id"`
	ProjectName string          `json:"project_name"`
	ProjectType string          `json:"project_type"`
	RepoURL     string          `json:"repo_url,omitempty"`
	Branch      string          `json:"default_branch,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	Files       []ExportFile    `json:"files"`
	Skipped     []ExportSkipped `json:"skipped"`
}

type ExportFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type ExportSkipped struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func exportSecretName(name string) bool {
	l := strings.ToLower(name)
	if l == ".env.example" || l == ".env.sample" || l == ".env.template" {
		return false
	}
	return l == ".env" || strings.HasPrefix(l, ".env.") ||
		strings.HasSuffix(l, ".pem") || strings.HasSuffix(l, ".key") ||
		l == "id_rsa" || l == "id_ed25519"
}

// WriteExport streams a zip with the project files under files/, a
// manifest.json with SHA-256 per file, and a DEPLOY.md guide. Secrets (.env,
// keys), VCS data and dependency folders are skipped and recorded in the
// manifest. The stored access token is never included.
func WriteExport(w io.Writer, p *Project, now time.Time) error {
	if p == nil {
		return fmt.Errorf("project is nil")
	}
	m := ExportManifest{
		Version: 1, ExportedAt: now.UTC(), ProjectID: p.ID, ProjectName: p.Name,
		ProjectType: p.ProjectType, RepoURL: p.RepoURL, Branch: p.DefaultBranch,
		Metadata: p.Metadata, Files: []ExportFile{}, Skipped: []ExportSkipped{},
	}
	zw := zip.NewWriter(w)

	if root := p.LocalPath; root != "" {
		if st, err := os.Stat(root); err == nil && st.IsDir() {
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				rel, rerr := filepath.Rel(root, path)
				if rerr != nil || rel == "." {
					return rerr
				}
				rel = filepath.ToSlash(rel)
				if d.IsDir() {
					if exportSkipDirs[d.Name()] {
						m.Skipped = append(m.Skipped, ExportSkipped{rel + "/", "excluded directory"})
						return filepath.SkipDir
					}
					return nil
				}
				if !d.Type().IsRegular() {
					m.Skipped = append(m.Skipped, ExportSkipped{rel, "not a regular file"})
					return nil
				}
				if exportSecretName(d.Name()) {
					m.Skipped = append(m.Skipped, ExportSkipped{rel, "possible secret"})
					return nil
				}
				info, ierr := d.Info()
				if ierr != nil {
					return ierr
				}
				if info.Size() > ExportMaxFileBytes {
					m.Skipped = append(m.Skipped, ExportSkipped{rel, "larger than 50 MB"})
					return nil
				}
				f, oerr := os.Open(path)
				if oerr != nil {
					return oerr
				}
				defer f.Close()
				hdr := &zip.FileHeader{Name: "files/" + rel, Method: zip.Deflate, Modified: info.ModTime()}
				hdr.SetMode(info.Mode().Perm())
				zf, cerr := zw.CreateHeader(hdr)
				if cerr != nil {
					return cerr
				}
				h := sha256.New()
				n, cerr := io.Copy(io.MultiWriter(zf, h), f)
				if cerr != nil {
					return cerr
				}
				m.Files = append(m.Files, ExportFile{Path: rel, Size: n, SHA256: hex.EncodeToString(h.Sum(nil))})
				return nil
			})
			if err != nil {
				return fmt.Errorf("walk project files: %w", err)
			}
		}
	}

	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	sort.Slice(m.Skipped, func(i, j int) bool { return m.Skipped[i].Path < m.Skipped[j].Path })

	mw, err := zw.Create("manifest.json")
	if err != nil {
		return err
	}
	enc := json.NewEncoder(mw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return err
	}
	dw, err := zw.Create("DEPLOY.md")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(dw, exportDeployGuide(p, len(m.Files), len(m.Skipped))); err != nil {
		return err
	}
	return zw.Close()
}

func exportDeployGuide(p *Project, files, skipped int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s - export from Cuan\n\n", p.Name)
	fmt.Fprintf(&b, "This archive holds %d files from your project under `files/`.\n", files)
	b.WriteString("`manifest.json` lists every file with its SHA-256 so you can verify the copy.\n\n")
	b.WriteString("## What is not included\n\n")
	fmt.Fprintf(&b, "%d paths were skipped (see `skipped` in the manifest): `.env` files and keys, `.git`, `node_modules` and other dependency folders, and files over 50 MB. Your access token is never exported.\n\n", skipped)
	b.WriteString("## Run it elsewhere\n\n")
	b.WriteString("1. Unzip the archive and open the `files/` folder.\n")
	if p.RepoURL != "" {
		fmt.Fprintf(&b, "2. The original repository is %s (branch `%s`). Cloning it gives you full history.\n", p.RepoURL, p.DefaultBranch)
	} else {
		b.WriteString("2. This project has no remote repository. Run `git init` in `files/` to start history.\n")
	}
	b.WriteString("3. Recreate your secrets in a new `.env` from `.env.example` if the project has one.\n")
	b.WriteString("4. Install dependencies and start the project with the commands in its README.\n")
	return b.String()
}
