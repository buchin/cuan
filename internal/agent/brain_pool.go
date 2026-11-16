package agent

import (
	"context"
	"errors"
	"hash/crc32"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

var brainCursor atomic.Uint64
var ErrBrainCapacity = errors.New("AI capacity is temporarily full. Your work is saved and waiting for capacity.")

// LeaseHealthyBrain selects only explicitly connected profiles owned by this user.
// Cross-process leases prevent oversubscribing a brain; cooldowns prevent repeatedly
// routing to an exhausted account. Identity and conversation stay in Cuan.
func LeaseHealthyBrain(owner string, excluded map[string]bool, requiresVision ...bool) (string, *ProviderAccount, func(), error) {
	return leaseHealthyBrain(owner, excluded, TryLeaseProviderAccount, requiresVision...)
}

func LeaseHealthyBrainForTask(owner string, excluded map[string]bool, requiresVision ...bool) (string, *ProviderAccount, func(), error) {
	return leaseHealthyBrain(owner, excluded, TryLeaseProviderAccountForTask, requiresVision...)
}

// LeaseHealthyBrainForProject leases a consistent brain profile for a project to ensure account stickiness
// across project terminal sessions rather than rotating accounts randomly on each reconnect or spawn.
func LeaseHealthyBrainForProject(owner, projectID string, excluded map[string]bool, requiresVision ...bool) (string, *ProviderAccount, func(), error) {
	if owner == "" {
		return "", nil, nil, ErrBrainCapacity
	}
	profiles, err := ListProviderAccounts(owner)
	if err != nil || len(profiles) == 0 {
		return "", nil, nil, ErrBrainCapacity
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	var seed uint32
	if strings.TrimSpace(projectID) != "" {
		seed = crc32.ChecksumIEEE([]byte(strings.TrimSpace(projectID)))
	}
	start := int(seed) % len(profiles)
	return leaseHealthyBrainWithStart(owner, profiles, start, excluded, TryLeaseProviderAccountForTask, requiresVision...)
}

func leaseHealthyBrain(owner string, excluded map[string]bool, lease func(string, string) (func(), error), requiresVision ...bool) (string, *ProviderAccount, func(), error) {
	if owner == "" {
		return "", nil, nil, ErrBrainCapacity
	}
	profiles, err := ListProviderAccounts(owner)
	if err != nil {
		return "", nil, nil, ErrBrainCapacity
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	if len(profiles) == 0 {
		return "", nil, nil, ErrBrainCapacity
	}
	start := int(brainCursor.Add(1)-1) % len(profiles)
	return leaseHealthyBrainWithStart(owner, profiles, start, excluded, lease, requiresVision...)
}

func leaseHealthyBrainWithStart(owner string, profiles []*ProviderAccount, start int, excluded map[string]bool, lease func(string, string) (func(), error), requiresVision ...bool) (string, *ProviderAccount, func(), error) {
	// Try every preferred-model route before any explicitly configured fallback.
	// A quota failure excludes a model route, not all models on that account.
	for tier := 0; tier < 2; tier++ {
		for i := 0; i < len(profiles); i++ {
			p := profiles[(start+i)%len(profiles)]
			if p.Provider != "agy" && p.Provider != "antigravity" && p.Provider != "google" {
				continue
			}
			model := p.ModelID
			if tier == 1 {
				if !p.AllowModelFallback || p.FallbackModelID == "" || p.FallbackModelID == p.ModelID {
					continue
				}
				model = p.FallbackModelID
			}
			normModel := NormalizeModelAlias(model)
			if p.Legacy || excluded[p.ID] || excluded[BrainRouteKey(p.ID, model)] || excluded[BrainRouteKey(p.ID, normModel)] || !p.CredentialsPresent || p.State == "login_required" || p.State == "unavailable" || p.State == "setup_incomplete" {
				continue
			}
			if len(requiresVision) > 0 && requiresVision[0] && model != "" && !IsModelVisionCapable("agy", model) && !IsModelVisionCapable("agy", normModel) {
				continue
			}
			if p.State == "quota_limited" && !p.AllowModelFallback && (p.QuotaRetryAt == nil || time.Now().Before(*p.QuotaRetryAt)) {
				continue
			}
			if time.Now().Before(p.ModelCooldowns[model]) || time.Now().Before(p.ModelCooldowns[normModel]) {
				continue
			}
			if tier == 1 {
				// Discover supported IDs, never infer runnable models from quota labels.
				catalog, ok := nativeAccountModelCatalog(context.Background(), owner, p.ID)
				if !ok || (!catalog[model] && !catalog[normModel]) {
					continue
				}
				if !catalog[model] && catalog[normModel] {
					model = normModel
				}
			}
			release, err := lease(owner, p.ID)
			if err != nil {
				continue
			}
			current, err := GetProviderAccount(owner, p.ID)
			if err != nil || !current.CredentialsPresent || current.State == "quota_limited" || current.State == "unavailable" || current.State == "login_required" || current.State == "setup_incomplete" || time.Now().Before(current.ModelCooldowns[model]) || time.Now().Before(current.ModelCooldowns[normModel]) {
				release()
				continue
			}
			if (tier == 0 && current.ModelID != model && current.ModelID != normModel) || (tier == 1 && (!current.AllowModelFallback || (current.FallbackModelID != model && current.FallbackModelID != normModel) || current.ModelID == model)) {
				release()
				continue
			}
			home, err := ProviderAccountHome(owner, p.ID)
			if err != nil || !IsAntigravityLoggedInInDir(home) {
				release()
				continue
			}
			// The selected preference is immutable; only this lease chooses fallback.
			current.RuntimeModelID = model
			return home, current, release, nil
		}
	}
	return "", nil, nil, ErrBrainCapacity
}

// BrainRouteKey keeps a failed model from suppressing another permitted model
// on the same account. Account-only exclusions remain supported for callers.
func BrainRouteKey(account, model string) string { return account + "\x00" + model }
