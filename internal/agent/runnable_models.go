package agent

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/taskprocess"
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

// ProviderAccountModels returns the installed provider's catalog for this
// signed-in account. Catalog membership is not a promise of available quota.
func ProviderAccountModels(ctx context.Context, owner, id string) ([]ModelInfo, error) {
	account, err := GetProviderAccount(owner, id)
	if err != nil {
		return nil, err
	}
	if account.Legacy || !account.CredentialsPresent {
		return nil, errors.New("Connect this account before choosing a model")
	}
	if account.Provider != "agy" {
		return nil, errors.New("Model discovery is not available for this provider yet")
	}
	catalog, ok := nativeAccountModelCatalog(ctx, owner, id)
	if !ok {
		return nil, errors.New("Could not load models. Try again when the account is idle")
	}
	ids := make([]string, 0, len(catalog))
	for model := range catalog {
		ids = append(ids, model)
	}
	sort.Strings(ids)
	models := make([]ModelInfo, 0, len(ids))
	for _, model := range ids {
		supportsVision := IsModelVisionCapable(account.Provider, model)
		models = append(models, ModelInfo{
			ID:             model,
			Name:           model,
			Provider:       account.Provider,
			SupportsVision: supportsVision,
			Capabilities: ModelCapabilities{
				Vision:         supportsVision,
				SupportsVision: supportsVision,
			},
		})
	}
	return models, nil
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

// FetchRunnableAccountModelUsage combines quota evidence with the actual
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
	// Supported catalog entries without quota metadata remain unknown, not
	// exhausted and not 100% available. Runtime results still decide availability.
	byID := map[string]ModelUsage{}
	for _, model := range usage.Models {
		norm := NormalizeModelAlias(model.Model)
		if models[model.Model] {
			byID[model.Model] = model
		} else if models[norm] {
			model.Model = norm
			byID[norm] = model
		}
	}
	ids := make([]string, 0, len(models))
	for id := range models {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	filtered := make([]ModelUsage, 0, len(ids))
	for _, id := range ids {
		if known, ok := byID[id]; ok {
			filtered = append(filtered, known)
		} else {
			filtered = append(filtered, ModelUsage{Model: id, Remaining: -1})
		}
	}
	usage.Models = filtered
	return usage
}

func nativeAccountModelCatalog(ctx context.Context, owner, id string) (map[string]bool, bool) {
	home, err := ProviderAccountHome(owner, id)
	if err != nil {
		return nil, false
	}
	binary, err := ResolveAGYBinaryPath()
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
		// A supported-name cache does not grant quota. Reuse a bounded stale
		// catalog for this exact binary while another session holds the profile,
		// rather than making a second execution slot unusable after five minutes.
		if errors.Is(err, ErrAccountBusy) && ok && time.Since(cached.at) < 24*time.Hour {
			return cached.models, true
		}
		return nil, false
	}
	defer release()
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probe, binary, "models")
	cmd.Env = AntigravityProfileEnvironment(home)
	cmd.Dir = home
	cmd.SysProcAttr = taskprocess.SysProcAttr()
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
	if len(nativeCatalogCache.entries) >= 128 {
		oldestKey := ""
		var oldest time.Time
		for candidate, entry := range nativeCatalogCache.entries {
			if oldestKey == "" || entry.at.Before(oldest) {
				oldestKey, oldest = candidate, entry.at
			}
		}
		delete(nativeCatalogCache.entries, oldestKey)
	}
	nativeCatalogCache.entries[key] = nativeCatalogEntry{time.Now(), models}
	nativeCatalogCache.Unlock()
	return models, true
}
