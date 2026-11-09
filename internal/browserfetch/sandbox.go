package browserfetch

import (
	"errors"
	"os"
	"path/filepath"
)

// sandboxCommand exposes OS libraries, the selected engine and one disposable
// directory. Provider profiles, host HOME, application data and the lock stay
// outside the browser namespace. Network access uses the engine's public-only
// transport policy; filesystem isolation does not itself enforce network policy.
func (r Runtime) sandboxCommand(home string, engineArgs []string) (string, []string, error) {
	if !filepath.IsAbs(r.Sandbox) {
		return "", nil, errors.New("browser filesystem sandbox is unavailable")
	}
	executable := func(path string) (string, error) {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", errors.New("browser runtime executable unavailable")
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			return "", errors.New("browser runtime executable unavailable")
		}
		return resolved, nil
	}
	sandbox, err := executable(r.Sandbox)
	if err != nil {
		return "", nil, err
	}
	binary, err := executable(r.Binary)
	if err != nil {
		return "", nil, err
	}
	args := []string{"--die-with-parent", "--new-session", "--unshare-pid", "--unshare-ipc", "--unshare-uts", "--cap-drop", "ALL", "--clearenv"}
	for _, path := range []string{"/bin", "/usr/bin", "/lib", "/lib64", "/usr/lib", "/etc/ssl", "/etc/resolv.conf", "/etc/hosts", "/etc/nsswitch.conf"} {
		if _, err := os.Stat(path); err == nil {
			args = append(args, "--ro-bind", path, path)
		}
	}
	args = append(args, "--dir", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--dir", "/home", "--bind", home, "/home/browser", "--ro-bind", binary, "/cuan-obscura")
	command := []string{"/cuan-obscura"}
	if r.Loader != "" || r.LibraryDir != "" {
		if !filepath.IsAbs(r.Loader) || !filepath.IsAbs(r.LibraryDir) {
			return "", nil, errors.New("browser compatibility runtime is incomplete")
		}
		loader, err := executable(r.Loader)
		if err != nil {
			return "", nil, err
		}
		library, err := filepath.EvalSymlinks(r.LibraryDir)
		if err != nil {
			return "", nil, errors.New("browser compatibility libraries unavailable")
		}
		info, err := os.Stat(library)
		if err != nil || !info.IsDir() {
			return "", nil, errors.New("browser compatibility libraries unavailable")
		}
		args = append(args, "--ro-bind", loader, "/cuan-loader", "--ro-bind", library, "/cuan-libs")
		command = []string{"/cuan-loader", "--library-path", "/cuan-libs", "/cuan-obscura"}
	}
	for _, entry := range [][2]string{{"HOME", "/home/browser"}, {"XDG_CACHE_HOME", "/home/browser/cache"}, {"XDG_CONFIG_HOME", "/home/browser/config"}, {"TMPDIR", "/tmp"}, {"PATH", "/usr/bin:/bin"}, {"LANG", "C.UTF-8"}, {"OBSCURA_ALLOW_PRIVATE_NETWORK", "0"}, {"OBSCURA_SCRIPT_DEADLINE_MS", "3000"}} {
		args = append(args, "--setenv", entry[0], entry[1])
	}
	args = append(args, "--chdir", "/home/browser", "--")
	args = append(args, command...)
	args = append(args, engineArgs...)
	return sandbox, args, nil
}
