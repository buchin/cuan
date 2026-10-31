package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// inboxSandboxArgs builds an allowlisted filesystem rather than binding the host
// root. The selected provider must read its own authentication profile; other
// accounts, service configuration and host process metadata are not mounted.
// /proc belongs to the new PID namespace; AGY subprocesses need /proc/self/exe.
func inboxSandboxArgs(binary, home, workspace string, attachments []string) ([]string, error) {
	absolute := func(path string) (string, error) {
		p, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		return filepath.EvalSymlinks(p)
	}
	binary, err := absolute(binary)
	if err != nil {
		return nil, err
	}
	home, err = absolute(home)
	if err != nil {
		return nil, err
	}
	args := []string{"--die-with-parent", "--new-session", "--unshare-pid", "--unshare-ipc", "--unshare-uts", "--cap-drop", "ALL", "--clearenv"}
	for _, path := range []string{"/usr", "/bin", "/lib", "/lib64", "/opt/cuan/bin", "/opt/cuan/current", "/opt/cuan/releases"} {
		if _, err := os.Stat(path); err == nil {
			args = append(args, "--ro-bind", path, path)
		}
	}
	for _, path := range []string{"/etc/ssl", "/etc/resolv.conf", "/etc/hosts", "/etc/nsswitch.conf", "/etc/passwd", "/etc/group", "/etc/localtime"} {
		if _, err := os.Stat(path); err == nil {
			args = append(args, "--ro-bind", path, path)
		}
	}
	args = append(args, "--proc", "/proc", "--remount-ro", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--dir", home)
	// Keep token refresh and provider state persistent without exposing the rest of
	// the service HOME (database settings, SSH keys, other account profiles).
	profile := filepath.Join(home, ".gemini", "antigravity-cli")
	if info, err := os.Stat(profile); err == nil && info.IsDir() {
		resolved, e := absolute(profile)
		if e != nil || resolved != profile {
			return nil, fmt.Errorf("provider profile must not be a symlink")
		}
		args = append(args, "--bind", profile, profile)
	}
	// Mount .gemini/config so Antigravity can discover global mcp_config.json
	geminiConfig := filepath.Join(home, ".gemini", "config")
	if info, err := os.Stat(geminiConfig); err == nil && info.IsDir() {
		if resolved, e := absolute(geminiConfig); e == nil && resolved == geminiConfig {
			args = append(args, "--ro-bind", geminiConfig, geminiConfig)
		}
	}
	// Mount local PostgreSQL socket if available
	for _, sockDir := range []string{"/run/postgresql", "/var/run/postgresql"} {
		if info, err := os.Stat(sockDir); err == nil && info.IsDir() {
			if resolved, e := absolute(sockDir); e == nil {
				args = append(args, "--ro-bind", resolved, sockDir)
				break
			}
		}
	}
	cwd := home
	if workspace != "" {
		workspace, err = absolute(workspace)
		if err != nil {
			return nil, err
		}
		for _, protected := range []string{home, profile, "/usr", "/bin", "/lib", "/etc", "/proc", "/dev"} {
			if workspace == "/" || workspace == protected || strings.HasPrefix(protected, workspace+"/") || strings.HasPrefix(workspace, profile+"/") || (protected != home && strings.HasPrefix(workspace, protected+"/")) {
				return nil, fmt.Errorf("project workspace overlaps protected paths")
			}
		}
		args = append(args, "--ro-bind", workspace, workspace)
		cwd = workspace
	}
	for _, path := range attachments {
		path, err = absolute(path)
		if err != nil {
			return nil, err
		}
		info, e := os.Stat(path)
		if e != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("attachment is not a regular file")
		}
		args = append(args, "--ro-bind", path, path)
	}
	// A standalone AGY binary can live under the service's otherwise hidden HOME.
	args = append(args, "--ro-bind", binary, binary, "--chdir", cwd)
	return args, nil
}

func sandboxInboxCommand(binary string, args, env []string, home, workspace string, attachments []string) (string, []string, error) {
	binary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return "", nil, err
	}
	wrapper, err := exec.LookPath("bwrap")
	if err != nil {
		return "", nil, fmt.Errorf("external chat sandbox unavailable")
	}
	mounts, err := inboxSandboxArgs(binary, home, workspace, attachments)
	if err != nil {
		return "", nil, err
	}
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok && key != "TMPDIR" && key != "LD_LIBRARY_PATH" {
			mounts = append(mounts, "--setenv", key, value)
		}
	}
	mounts = append(mounts, "--setenv", "TMPDIR", "/tmp", "--", binary)
	return wrapper, append(mounts, args...), nil
}
