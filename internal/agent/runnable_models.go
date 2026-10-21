package agent

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

var nativeCatalogCache = struct {
	sync.Mutex
	entries map[string]nativeCatalogEntry
}{entries: map[string]nativeCatalogEntry{}}

type nativeCatalogEntry struct {
	at     time.Time
	models map[string]bool
}
type catalogOutput struct {
	data     []byte
	overflow bool
}

func (b *catalogOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 65536 - len(b.data)
	if n > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	b.data = append(b.data, p...)
	return n, nil
}

// FetchRunnableAccountModelUsage intersects quota evidence with the actual
// installed CLI's catalog. The quota RPC may include obsolete model IDs that
// the CLI rejects before starting any runtime session. This never runs inference.
func FetchRunnableAccountModelUsage(ctx context.Context, owner, id string) AccountModelUsage {
	usage := FetchAccountModelUsage(ctx, owner, id)
	if !usage.Available {
		return usage
	}
	models, ok := nativeAccountModelCatalog(ctx, owner, id)
	if !ok {
		usage.Models = []ModelUsage{}
		usage.Message = "The installed AI model catalog could not be checked. Try again when the account is idle."
		return usage
	}
	filtered := make([]ModelUsage, 0, len(usage.Models))
	for _, model := range usage.Models {
		if models[model.Model] {
			filtered = append(filtered, model)
		}
	}
	usage.Models = filtered
	if len(filtered) == 0 {
		usage.Message = "No installed AI model has current quota evidence."
	}
	return usage
}

func nativeAccountModelCatalog(ctx context.Context, owner, id string) (map[string]bool, bool) {
	home, err := ProviderAccountHome(owner, id)
	if err != nil {
		return nil, false
	}
	binary, err := exec.LookPath("agy")
	if err != nil {
		return nil, false
	}
	info, err := os.Stat(binary)
	if err != nil {
		return nil, false
	}
	key := home + "\x00" + binary + "\x00" + info.ModTime().UTC().Format(time.RFC3339Nano)
	nativeCatalogCache.Lock()
	cached, ok := nativeCatalogCache.entries[key]
	nativeCatalogCache.Unlock()
	if ok && time.Since(cached.at) < 5*time.Minute {
		return cached.models, true
	}
	release, err := TryLeaseProviderAccountRefresh(owner, id)
	if err != nil {
		return nil, false
	}
	defer release()
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probe, binary, "models")
	cmd.Env = AntigravityProfileEnvironment(home)
	cmd.Dir = home
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
	output := &catalogOutput{}
	cmd.Stdout = output
	cmd.Stderr = io.Discard
	if cmd.Run() != nil || output.overflow {
		return nil, false
	}
	models := map[string]bool{}
	for _, line := range strings.Split(string(output.data), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && IsTextModel(fields[0]) {
			models[fields[0]] = true
		}
	}
	if len(models) == 0 {
		return nil, false
	}
	nativeCatalogCache.Lock()
	nativeCatalogCache.entries[key] = nativeCatalogEntry{time.Now(), models}
	nativeCatalogCache.Unlock()
	return models, true
}
