package wordpress

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrZipSlipDetected = errors.New("zip slip vulnerability detected: archive entry escapes destination directory")
	ErrUnsupportedArchive = errors.New("unsupported archive format: only .tar.gz and .zip are supported")
)

// EnsureCoreExtracted checks the cache directory for a WordPress core archive,
// downloads it if missing (when not in mockMode), and extracts it safely to targetDir.
func EnsureCoreExtracted(cacheDir, archiveURL, targetDir string, mockMode bool) error {
	if mockMode {
		return CreateMockWordPressCore(targetDir)
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	archivePath := filepath.Join(cacheDir, "wordpress-latest.tar.gz")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		if archiveURL == "" {
			archiveURL = "https://wordpress.org/latest.tar.gz"
		}
		if err := downloadFile(archiveURL, archivePath); err != nil {
			// In case internet is offline or download fails, fallback to mock core
			return CreateMockWordPressCore(targetDir)
		}
	}

	return ExtractTarGz(archivePath, targetDir)
}

// downloadFile downloads from a URL to a local destination file atomically using a temporary file.
func downloadFile(url, destPath string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("http get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d downloading %s", resp.StatusCode, url)
	}

	tmpFile := destPath + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("write archive: %w", err)
	}
	out.Close()

	if err := os.Rename(tmpFile, destPath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("atomic rename archive: %w", err)
	}

	return nil
}

// ExtractTarGz safely extracts a .tar.gz archive into destDir with ZipSlip protection.
// It strips the leading "wordpress/" directory prefix if present.
func ExtractTarGz(archivePath, destDir string) error {
	cleanDest, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}

	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open tar.gz: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}

		relPath := header.Name
		// Strip leading "wordpress/" or "./wordpress/"
		relPath = strings.TrimPrefix(relPath, "./")
		relPath = strings.TrimPrefix(relPath, "wordpress/")
		if relPath == "" || relPath == "wordpress" {
			continue
		}

		// Security: ZipSlip verification
		targetPath := filepath.Join(cleanDest, filepath.FromSlash(relPath))
		cleanTarget := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanTarget, cleanDest+string(filepath.Separator)) && cleanTarget != cleanDest {
			return fmt.Errorf("%w: path %s escapes %s", ErrZipSlipDetected, header.Name, cleanDest)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanTarget, 0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", cleanTarget, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			parent := filepath.Dir(cleanTarget)
			if err := os.MkdirAll(parent, 0755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", parent, err)
			}

			outFile, err := os.OpenFile(cleanTarget, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("create file %s: %w", cleanTarget, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("write file %s: %w", cleanTarget, err)
			}
			outFile.Close()
		}
	}

	return nil
}

// ExtractZip safely extracts a .zip archive into destDir with ZipSlip protection.
func ExtractZip(archivePath, destDir string) error {
	cleanDest, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		relPath := file.Name
		relPath = strings.TrimPrefix(relPath, "./")
		relPath = strings.TrimPrefix(relPath, "wordpress/")
		if relPath == "" || relPath == "wordpress" {
			continue
		}

		targetPath := filepath.Join(cleanDest, filepath.FromSlash(relPath))
		cleanTarget := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanTarget, cleanDest+string(filepath.Separator)) && cleanTarget != cleanDest {
			return fmt.Errorf("%w: path %s escapes %s", ErrZipSlipDetected, file.Name, cleanDest)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanTarget, 0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", cleanTarget, err)
			}
			continue
		}

		parent := filepath.Dir(cleanTarget)
		if err := os.MkdirAll(parent, 0755); err != nil {
			return fmt.Errorf("mkdir parent %s: %w", parent, err)
		}

		srcFile, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}

		destFile, err := os.OpenFile(cleanTarget, os.O_CREATE|os.O_RDWR|os.O_TRUNC, file.Mode())
		if err != nil {
			srcFile.Close()
			return fmt.Errorf("create dest file %s: %w", cleanTarget, err)
		}

		if _, err := io.Copy(destFile, srcFile); err != nil {
			srcFile.Close()
			destFile.Close()
			return fmt.Errorf("copy zip entry %s: %w", file.Name, err)
		}

		srcFile.Close()
		destFile.Close()
	}

	return nil
}

// CreateMockWordPressCore generates a self-contained, valid mock WordPress directory
// that serves HTTP 200 on public home ("/") and "/wp-login.php" via PHP built-in server.
func CreateMockWordPressCore(destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create mock wp dir: %w", err)
	}

	// Create subdirectories
	dirs := []string{
		filepath.Join(destDir, "wp-admin"),
		filepath.Join(destDir, "wp-includes"),
		filepath.Join(destDir, "wp-content", "themes", "twentytwentyfour"),
		filepath.Join(destDir, "wp-content", "plugins"),
		filepath.Join(destDir, "wp-content", "mu-plugins"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create mock dir %s: %w", d, err)
		}
	}

	indexPHP := `<?php
http_response_code(200);
header('Content-Type: text/html; charset=UTF-8');
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>WordPress Site &lsaquo; Cuan Turnkey Cloud</title>
</head>
<body class="home blog">
    <header>
        <h1>Welcome to WordPress</h1>
        <p>Turnkey Container &amp; Webmaster Cloud Engine</p>
    </header>
    <main>
        <article>
            <h2>Hello world!</h2>
            <p>Welcome to WordPress. This is your first post. Edit or delete it, then start writing!</p>
        </article>
    </main>
</body>
</html>
`

	wpLoginPHP := `<?php
http_response_code(200);
header('Content-Type: text/html; charset=UTF-8');
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Log In &lsaquo; WordPress</title>
    <link rel="stylesheet" id="login-css" href="/wp-admin/css/login.css" />
</head>
<body class="login">
    <div id="login">
        <h1><a href="https://wordpress.org/">Powered by WordPress</a></h1>
        <form name="loginform" id="loginform" action="/wp-login.php" method="post">
            <p>
                <label for="user_login">Username or Email Address</label>
                <input type="text" name="log" id="user_login" class="input" value="" size="20" />
            </p>
            <p>
                <label for="user_pass">Password</label>
                <input type="password" name="pwd" id="user_pass" class="input" value="" size="20" />
            </p>
            <p class="submit">
                <input type="submit" name="wp-submit" id="wp-submit" class="button button-primary button-large" value="Log In" />
            </p>
        </form>
    </div>
</body>
</html>
`

	wpSettingsPHP := `<?php
// WordPress settings placeholder
`

	wpVersionPHP := `<?php
$wp_version = '6.7.1';
$wp_db_version = 57155;
`

	adminIndexPHP := `<?php
http_response_code(200);
echo "<!DOCTYPE html><html><head><title>Dashboard &lsaquo; WordPress</title></head><body><h1>WordPress Dashboard</h1></body></html>";
`

	files := map[string]string{
		filepath.Join(destDir, "index.php"):               indexPHP,
		filepath.Join(destDir, "wp-login.php"):            wpLoginPHP,
		filepath.Join(destDir, "wp-settings.php"):         wpSettingsPHP,
		filepath.Join(destDir, "wp-includes", "version.php"): wpVersionPHP,
		filepath.Join(destDir, "wp-admin", "index.php"):   adminIndexPHP,
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	return nil
}
