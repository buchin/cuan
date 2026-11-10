package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	runtimeProvisionMu   sync.Mutex
	lastProvisionAttempt time.Time
)

// isIsolatedTestEnv returns true if PATH is restricted to a temporary test directory.
func isIsolatedTestEnv() bool {
	p := os.Getenv("PATH")
	return !strings.Contains(p, "/bin") && !strings.Contains(p, "/usr")
}

// ensureCandidateInPath adds dir to the process PATH if not already present.
func ensureCandidateInPath(dir string) {
	if dir == "" {
		return
	}
	curPath := os.Getenv("PATH")
	for _, part := range filepath.SplitList(curPath) {
		if part == dir {
			return
		}
	}
	os.Setenv("PATH", dir+string(os.PathListSeparator)+curPath)
}

// findCandidateSandbox looks for bwrap in standard candidate paths.
func findCandidateSandbox() string {
	candidates := []string{
		"/opt/cuan/runtime/bwrap",
		"/usr/local/bin/bwrap",
		"/usr/bin/bwrap",
		"/home/dev/.local/bin/bwrap",
		"/home/ubuntu/.local/bin/bwrap",
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append([]string{
			filepath.Join(home, ".local", "bin", "bwrap"),
			filepath.Join(home, ".local", "share", "cuan", "runtime", "bin", "bwrap"),
		}, candidates...)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && (info.Mode()&0111 != 0) {
			ensureCandidateInPath(filepath.Dir(candidate))
			return candidate
		}
	}
	return ""
}

// findCandidateCLI looks for harness CLI in standard candidate paths.
func findCandidateCLI(name string) string {
	candidates := []string{
		"/usr/local/bin/" + name,
		"/usr/bin/" + name,
		"/opt/cuan/runtime/" + name,
		"/home/dev/.local/bin/" + name,
		"/home/ubuntu/.local/bin/" + name,
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append([]string{
			filepath.Join(home, ".local", "bin", name),
			filepath.Join(home, ".local", "share", "cuan", "runtime", "bin", name),
			filepath.Join(home, ".gemini", "antigravity-cli", "bin", name),
			filepath.Join(home, ".antigravity", "bin", name),
		}, candidates...)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && (info.Mode()&0111 != 0) {
			ensureCandidateInPath(filepath.Dir(candidate))
			return candidate
		}
	}
	return ""
}

// autoProvisionBubblewrap downloads and extracts bubblewrap package without root/sudo.
func autoProvisionBubblewrap(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("bubblewrap is only supported on Linux")
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return fmt.Errorf("unable to determine user home directory: %w", err)
	}

	destDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	destPath := filepath.Join(destDir, "bwrap")

	// Method 1: Check if /opt/cuan/runtime/bwrap exists and is accessible
	if info, err := os.Stat("/opt/cuan/runtime/bwrap"); err == nil && !info.IsDir() && (info.Mode()&0111 != 0) {
		_ = os.Remove(destPath)
		if err := os.Symlink("/opt/cuan/runtime/bwrap", destPath); err == nil {
			ensureCandidateInPath(destDir)
			return nil
		}
	}

	// Method 2: Check if apt-get and dpkg-deb exist
	if _, err := exec.LookPath("apt-get"); err == nil {
		if _, err := exec.LookPath("dpkg-deb"); err == nil {
			tmpDir, err := os.MkdirTemp("", "cuan-bwrap-provision-*")
			if err == nil {
				defer os.RemoveAll(tmpDir)

				cmd := exec.CommandContext(ctx, "apt-get", "download", "bubblewrap")
				cmd.Dir = tmpDir
				if out, err := cmd.CombinedOutput(); err == nil {
					matches, _ := filepath.Glob(filepath.Join(tmpDir, "bubblewrap*.deb"))
					if len(matches) > 0 {
						extractDir := filepath.Join(tmpDir, "extract")
						extractCmd := exec.CommandContext(ctx, "dpkg-deb", "-x", matches[0], extractDir)
						if out, err := extractCmd.CombinedOutput(); err == nil {
							extractedBwrap := filepath.Join(extractDir, "usr", "bin", "bwrap")
							if _, err := os.Stat(extractedBwrap); err == nil {
								src, err := os.Open(extractedBwrap)
								if err == nil {
									tmpDest := destPath + ".tmp"
									dst, err := os.OpenFile(tmpDest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
									if err == nil {
										_, copyErr := io.Copy(dst, src)
										_ = dst.Close()
										_ = src.Close()
										if copyErr == nil {
											if err := os.Rename(tmpDest, destPath); err == nil {
												_ = os.Chmod(destPath, 0755)
												ensureCandidateInPath(destDir)
												log.Printf("[RuntimeProvisioner] Successfully provisioned bubblewrap into %s", destPath)
												return nil
											}
										}
										_ = os.Remove(tmpDest)
									} else {
										_ = src.Close()
									}
								}
							}
						} else {
							log.Printf("[RuntimeProvisioner] dpkg-deb failed: %v, out: %s", err, string(out))
						}
					}
				} else {
					log.Printf("[RuntimeProvisioner] apt-get download bubblewrap failed: %v, out: %s", err, string(out))
				}
			}
		}
	}

	// Method 3: Try running scripts/setup-inbox-sandbox.py if present
	for _, candScript := range []string{
		"scripts/setup-inbox-sandbox.py",
		"/opt/cuan/scripts/setup-inbox-sandbox.py",
	} {
		if _, err := os.Stat(candScript); err == nil {
			if py, err := exec.LookPath("python3"); err == nil {
				username := os.Getenv("USER")
				if username == "" {
					username = "cuan"
				}
				cmd := exec.CommandContext(ctx, py, candScript, "--user", username)
				_ = cmd.Run()
				if _, err := os.Stat(destPath); err == nil {
					ensureCandidateInPath(destDir)
					return nil
				}
			}
		}
	}

	return fmt.Errorf("could not auto-provision bubblewrap")
}

// autoProvisionAntigravity installs official verified Antigravity CLI if missing.
func autoProvisionAntigravity(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("auto-provisioning agy is currently only supported on Linux")
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return fmt.Errorf("unable to determine user home directory: %w", err)
	}

	destDir := filepath.Join(home, ".local", "bin")
	_ = os.MkdirAll(destDir, 0755)

	// Check if scripts/install-antigravity.py exists
	for _, candScript := range []string{
		"scripts/install-antigravity.py",
		"/opt/cuan/scripts/install-antigravity.py",
	} {
		if _, err := os.Stat(candScript); err == nil {
			if py, err := exec.LookPath("python3"); err == nil {
				cmd := exec.CommandContext(ctx, py, candScript, "--prefix", filepath.Join(home, ".local"))
				if out, err := cmd.CombinedOutput(); err == nil {
					destPath := filepath.Join(destDir, "agy")
					if _, err := os.Stat(destPath); err == nil {
						ensureCandidateInPath(destDir)
						log.Printf("[RuntimeProvisioner] Successfully provisioned agy via script: %s", string(out))
						return nil
					}
				}
			}
		}
	}

	return fmt.Errorf("could not auto-provision antigravity CLI")
}

// EnsureRuntimePrerequisites checks whether bwrap and agy are ready, and auto-provisions them if missing.
func EnsureRuntimePrerequisites(ctx context.Context) {
	if isIsolatedTestEnv() {
		return
	}
	if runtime.GOOS != "linux" {
		return
	}

	runtimeProvisionMu.Lock()
	defer runtimeProvisionMu.Unlock()

	// Rate-limit attempts to at most once every 15 seconds
	if time.Since(lastProvisionAttempt) < 15*time.Second {
		return
	}
	lastProvisionAttempt = time.Now()

	// 1. Resolve or provision bwrap
	if _, err := exec.LookPath("bwrap"); err != nil {
		if cand := findCandidateSandbox(); cand != "" {
			ensureCandidateInPath(filepath.Dir(cand))
		} else {
			if err := autoProvisionBubblewrap(ctx); err != nil {
				log.Printf("[RuntimeProvisioner] Auto-provision bwrap notice: %v", err)
			}
		}
	}

	// 2. Resolve or provision agy
	if _, err := exec.LookPath("agy"); err != nil {
		if cand := findCandidateCLI("agy"); cand != "" {
			ensureCandidateInPath(filepath.Dir(cand))
		} else {
			if err := autoProvisionAntigravity(ctx); err != nil {
				log.Printf("[RuntimeProvisioner] Auto-provision agy notice: %v", err)
			}
		}
	}
}
