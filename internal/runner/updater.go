package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	DefaultCurrentBinPath = "/opt/cuan/current/bin/cuan-runner"
)

// UpdateManager inspects disk binaries and performs verified graceful self-updates.
type UpdateManager struct {
	currentBinPath   string
	targetBinPath    string
	restartFunc      func(binaryPath string, args []string, env []string) error
	hasCustomRestart bool
}

func isTestBinary(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(base, ".test") || strings.Contains(path, "/go-build")
}

func NewUpdateManager(targetBinPath string) *UpdateManager {
	currentPath, _ := os.Executable()
	if currentPath != "" {
		if resolved, err := filepath.EvalSymlinks(currentPath); err == nil {
			currentPath = resolved
		}
	}

	if targetBinPath == "" {
		if envBin := os.Getenv("CUAN_RUNNER_BIN"); envBin != "" {
			targetBinPath = envBin
		} else {
			targetBinPath = DefaultCurrentBinPath
		}
	}

	return &UpdateManager{
		currentBinPath: currentPath,
		targetBinPath:  targetBinPath,
		restartFunc:    defaultExecRestart,
	}
}

func defaultExecRestart(binaryPath string, args []string, env []string) error {
	if isTestBinary(os.Args[0]) {
		return errors.New("cannot perform self-update exec in test binary")
	}
	return syscall.Exec(binaryPath, args, env)
}

func (u *UpdateManager) SetRestartFunc(fn func(string, []string, []string) error) {
	u.restartFunc = fn
	u.hasCustomRestart = fn != nil
}

func (u *UpdateManager) CurrentBinaryPath() string {
	return u.currentBinPath
}

func (u *UpdateManager) TargetBinaryPath() string {
	return u.targetBinPath
}

// ComputeFileSHA256 returns hex-encoded SHA-256 checksum of a file.
func ComputeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyBinaryIntegrity validates that the candidate binary:
// 1. Exists and is not a directory.
// 2. Has executable permission bits set.
// 3. Has non-zero size.
// 4. Can execute without segmentation fault or crashing probe.
func VerifyBinaryIntegrity(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat binary: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("candidate path is a directory")
	}
	if info.Size() == 0 {
		return fmt.Errorf("candidate binary has 0 bytes")
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("candidate binary does not have executable bit set (mode: %v)", info.Mode())
	}

	// Verify execution with a quick probe command
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--help")
	_ = cmd.Run() // probe command, non-zero exit like 2 for usage is acceptable, crash is not
	if cmd.ProcessState != nil && cmd.ProcessState.Sys() != nil {
		if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
			if ws.Signaled() {
				return fmt.Errorf("candidate binary crashed with signal %v during probe", ws.Signal())
			}
		}
	}
	return nil
}

// CheckForUpdate checks if targetBinPath contains a verified, distinct binary from currentBinPath.
func (u *UpdateManager) CheckForUpdate() (bool, string, error) {
	if u.targetBinPath == "" {
		return false, "", nil
	}
	// In test binaries, do not check or perform real self-update if current binary is a test runner
	if isTestBinary(u.currentBinPath) && !u.hasCustomRestart {
		return false, "", nil
	}
	targetResolved, err := filepath.EvalSymlinks(u.targetBinPath)
	if err != nil {
		// Target does not exist on disk
		return false, "", nil
	}

	// If resolved path is identical to current binary path, no update
	if u.currentBinPath != "" && targetResolved == u.currentBinPath {
		return false, "", nil
	}

	// Verify target integrity before proceeding
	if err := VerifyBinaryIntegrity(targetResolved); err != nil {
		return false, "", fmt.Errorf("target binary verification failed: %w", err)
	}

	// Compare checksums
	currentSHA := ""
	if u.currentBinPath != "" {
		currentSHA, _ = ComputeFileSHA256(u.currentBinPath)
	}
	targetSHA, err := ComputeFileSHA256(targetResolved)
	if err != nil {
		return false, "", fmt.Errorf("compute target checksum: %w", err)
	}

	if currentSHA != "" && currentSHA == targetSHA {
		return false, "", nil
	}

	return true, targetSHA, nil
}

// PerformSelfUpdate executes the verified target binary, replacing the current process image.
func (u *UpdateManager) PerformSelfUpdate(args []string, env []string) error {
	hasUpdate, targetSHA, err := u.CheckForUpdate()
	if err != nil {
		return fmt.Errorf("self-update check failed: %w", err)
	}
	if !hasUpdate {
		return errors.New("no new binary available for self-update")
	}

	targetResolved, _ := filepath.EvalSymlinks(u.targetBinPath)
	if targetResolved == "" {
		targetResolved = u.targetBinPath
	}

	log.Printf("[SelfUpdate] Handing over execution to new binary '%s' (SHA: %s)...", targetResolved, targetSHA[:12])

	if len(args) == 0 {
		args = os.Args
	}
	if len(env) == 0 {
		env = os.Environ()
	}

	return u.restartFunc(targetResolved, args, env)
}
