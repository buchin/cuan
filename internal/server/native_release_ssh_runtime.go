package server

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/publication"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Native release SSH consumes owner-saved host trust only. Explicit -F also
// avoids OpenSSH's passwd-home configuration, which HOME alone does not isolate.
func prepareNativeReleaseSSH(ctx context.Context, root *os.Root, workspace string, config publication.Config) (string, error) {
	lock, lockErr := root.OpenFile(".git/cuan-native-ssh.lock", os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0600)
	if lockErr != nil {
		return "", lockErr
	}
	defer lock.Close()
	lockInfo, statErr := lock.Stat()
	if statErr != nil || !lockInfo.Mode().IsRegular() || lockInfo.Mode().Perm()&0077 != 0 {
		return "", errors.New("native SSH runtime lock is not private")
	}
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return "", err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	key, _, options, rest, err := ssh.ParseAuthorizedKey([]byte(config.HostKey))
	if err != nil || key == nil || len(options) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return "", errors.New("saved native SSH host trust unavailable")
	}
	base := ".git/cuan-native-runtime"
	for _, path := range []string{base + "/ssh", base + "/bin"} {
		if err := root.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
			return "", err
		}
		info, err := root.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
			return "", errors.New("native SSH runtime is not private")
		}
	}
	absolute := filepath.Join(workspace, filepath.FromSlash(base))
	hostFile := filepath.Join(absolute, "ssh", "known_hosts")
	configFile := filepath.Join(absolute, "ssh", "config")
	quoteConfig := func(value string) string {
		return "\"" + strings.ReplaceAll(strings.ReplaceAll(value, "\\", "\\\\"), "\"", "\\\"") + "\""
	}
	if strings.ContainsAny(configFile+hostFile, "\r\n\x00") {
		return "", errors.New("invalid native SSH runtime path")
	}
	trust := knownhosts.Line([]string{config.Host}, key) + "\n"
	configuration := "Host *\n  UserKnownHostsFile " + quoteConfig(hostFile) + "\n  GlobalKnownHostsFile /dev/null\n  StrictHostKeyChecking yes\n  IdentityAgent none\n  IdentitiesOnly yes\n  PubkeyAuthentication no\n  PreferredAuthentications password\n"
	if err := writeNativeRuntimeFile(root, base+"/ssh/known_hosts", []byte(trust), 0600); err != nil {
		return "", err
	}
	if err := writeNativeRuntimeFile(root, base+"/ssh/config", []byte(configuration), 0600); err != nil {
		return "", err
	}
	quoteShell := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	for _, tool := range []string{"ssh", "scp"} {
		binary, err := exec.LookPath(tool)
		if err != nil {
			return "", errors.New("native SSH tools unavailable")
		}
		binary, err = filepath.EvalSymlinks(binary)
		if err != nil || !filepath.IsAbs(binary) {
			return "", errors.New("native SSH executable unavailable")
		}
		script := "#!/bin/sh\nexec " + quoteShell(binary) + " -F " + quoteShell(configFile) + " \"$@\"\n"
		if err := writeNativeRuntimeFile(root, base+"/bin/"+tool, []byte(script), 0700); err != nil {
			return "", err
		}
	}
	return filepath.Join(absolute, "bin"), nil
}

func writeNativeRuntimeFile(root *os.Root, path string, data []byte, mode os.FileMode) error {
	file, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, mode)
	if os.IsExist(err) {
		file, err = root.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if err != nil {
			return err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != mode || info.Size() != int64(len(data)) {
			return errors.New("saved native runtime file changed")
		}
		saved := make([]byte, len(data))
		if _, err := file.ReadAt(saved, 0); err != nil {
			return err
		}
		if !bytes.Equal(saved, data) {
			return errors.New("saved native runtime binding changed")
		}
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
