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
	mounted := make(map[string]bool)
	bindRO := func(src, dst string) {
		if mounted[dst] {
			return
		}
		mounted[dst] = true
		args = append(args, "--ro-bind", src, dst)
	}
	for _, path := range []string{"/usr", "/bin", "/lib", "/lib64", "/usr/local/bin", "/opt/cuan/bin", "/opt/cuan/current", "/opt/cuan/releases"} {
		if _, err := os.Stat(path); err == nil {
			bindRO(path, path)
		}
	}
	for _, path := range []string{"/etc/ssl", "/etc/resolv.conf", "/etc/hosts", "/etc/nsswitch.conf", "/etc/passwd", "/etc/group", "/etc/localtime"} {
		if _, err := os.Stat(path); err == nil {
			bindRO(path, path)
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
			bindRO(geminiConfig, geminiConfig)
		}
	}
	// Mount user local bin so installed agent CLI tools (e.g. cuan on ~/.local/bin) are accessible
	userLocalBin := filepath.Join(home, ".local", "bin")
	if info, err := os.Stat(userLocalBin); err == nil && info.IsDir() {
		if resolved, e := absolute(userLocalBin); e == nil {
			bindRO(resolved, userLocalBin)
		}
	}
	// Mount cuan binary directory if located outside standard paths
	if cuanPath, err := exec.LookPath("cuan"); err == nil {
		if resolved, e := absolute(cuanPath); e == nil {
			cuanDir := filepath.Dir(resolved)
			if cuanDir != "/" && cuanDir != "/usr" && cuanDir != "/bin" {
				if info, err := os.Stat(cuanDir); err == nil && info.IsDir() {
					bindRO(cuanDir, cuanDir)
				}
			}
		}
	}
	// Pin the native MCP executable independently of provider PATH or profile.
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	executable, err = absolute(executable)
	if err != nil {
		return nil, err
	}
	bindRO(executable, "/cuan-public-web-mcp")
	// Mount local PostgreSQL socket if available
	for _, sockDir := range []string{"/run/postgresql", "/var/run/postgresql"} {
		if info, err := os.Stat(sockDir); err == nil && info.IsDir() {
			if resolved, e := absolute(sockDir); e == nil {
				bindRO(resolved, sockDir)
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
	// Chat consumes owner-scoped backend snapshots and emits validated coordinator
	// actions. Never inherit an account/workspace MCP connection (or advertise an
	// unconfigured Owner CLI) into this provider process. Keep provider auth and
	// settings mounted, overlay only MCP discovery files inside this namespace.
	mcpFile, err := prepareInboxMCPBoundary(home)
	if err != nil {
		return nil, err
	}
	mcpTargets := []string{
		filepath.Join(home, ".gemini", "config", "mcp_config.json"),
		filepath.Join(home, ".gemini", "antigravity-cli", "mcp_config.json"),
		filepath.Join(home, ".config", "antigravity", "mcp_config.json"),
	}
	if workspace != "" {
		for _, target := range []string{filepath.Join(workspace, ".gemini", "config", "mcp_config.json"), filepath.Join(workspace, ".agents", "mcp_config.json")} {
			if _, e := os.Lstat(target); e == nil {
				mcpTargets = append(mcpTargets, target)
			} else if !os.IsNotExist(e) {
				return nil, e
			}
		}
	}
	for _, target := range mcpTargets {
		args = append(args, "--ro-bind", mcpFile, target)
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

const inboxPublicWebMCPConfig = "{\"mcpServers\":{\"cuan-inspect\":{\"command\":\"/cuan-public-web-mcp\",\"args\":[\"mcp\",\"--public-web\"]}}}\n"

func prepareInboxMCPBoundary(home string) (string, error) {
	root, err := os.OpenRoot(home)
	if err != nil {
		return "", err
	}
	defer root.Close()
	const directory = ".cuan-chat-sandbox"
	if err := root.Mkdir(directory, 0700); err != nil && !os.IsExist(err) {
		return "", err
	}
	info, err := root.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return "", fmt.Errorf("chat MCP boundary directory must be private")
	}
	path := filepath.Join(directory, "mcp_config.json")
	if err := writeNativeRuntimeFile(root, path, []byte(inboxPublicWebMCPConfig), 0600); err != nil {
		return "", err
	}
	return filepath.Join(home, path), nil
}
