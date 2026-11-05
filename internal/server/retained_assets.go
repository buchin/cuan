package server

import (
	"os"
	"path/filepath"
	"regexp"
)

var releaseDirectoryName = regexp.MustCompile(`^[a-f0-9]{40}$`)
var immutableAssetName = regexp.MustCompile(`^[A-Za-z0-9_-]+-[A-Za-z0-9_-]{8,}\.(js|css|woff2?|png|svg|webp|jpe?g)$`)

// Native releases retain a bounded set of immutable builds. A tab opened before
// activation can still request their hashed chunks; never substitute SPA HTML.
// Development directories and paths outside this application's release tree are
// excluded. Release pruning remains the only retention policy.
func retainedAssetPath(staticDir, requestPath string) string {
	name := filepath.Base(requestPath)
	if requestPath != "/assets/"+name || !immutableAssetName.MatchString(name) {
		return ""
	}
	root, err := filepath.Abs(staticDir)
	if err != nil {
		return ""
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return ""
	}
	if filepath.Base(root) != "dist" || filepath.Base(filepath.Dir(root)) != "web" {
		return ""
	}
	release := filepath.Dir(filepath.Dir(root))
	releases := filepath.Dir(release)
	if !releaseDirectoryName.MatchString(filepath.Base(release)) || filepath.Base(releases) != "releases" {
		return ""
	}
	entries, err := os.ReadDir(releases)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() || !releaseDirectoryName.MatchString(entry.Name()) || entry.Name() == filepath.Base(release) {
			continue
		}
		candidate := filepath.Join(releases, entry.Name(), "web", "dist", "assets", name)
		if info, err := os.Lstat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return ""
}
